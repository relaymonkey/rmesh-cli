package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/relaymonkey/relaymesh-edge/internal/apiclient"
	rmdevice "github.com/relaymonkey/relaymesh-edge/internal/device"
	"github.com/relaymonkey/relaymesh-edge/internal/deviceconfigs"
	rmtransport "github.com/relaymonkey/relaymesh-edge/internal/transport"
)

var setFlags commonFlags

// regionChangeRefused signals the operator-explicit-consent failure
// described by D-210. exit code 2 (distinct from 1) so wrappers can
// branch on it.
var errRegionChangeRefused = errors.New("region change refused; pass --allow-region-change to proceed (D-210)")

var deviceConfigSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Apply a device configuration to a device, or upload it to the cloud",
	Long: `Write a configuration to a destination. Source / destination combinations:

  --from device --to cloud         capture current state and save as a named cloud config
  --from cloud --to device         pull a saved cloud config and apply to the device
  --from file  --to device         apply a local file (the "import" workflow)
  --from device --to file/-        identical to "get" (kept off this verb; use ` + "`get`" + `)

Applying to a device wraps every Set* admin message in a single
BeginEditSettings / CommitEditSettings session so the firmware reboots
at most once (D-209). When the source payload's lora.region differs
from the device's currently reported region, the apply refuses unless
` + "`--allow-region-change`" + ` is passed (D-210; exit code 2 on refusal).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		src, err := deviceconfigs.ParseSource(setFlags.from)
		if err != nil {
			return err
		}
		dst, err := deviceconfigs.ParseSource(setFlags.to)
		if err != nil {
			return err
		}
		if dst.Kind == deviceconfigs.SourceFile || dst.Kind == deviceconfigs.SourceStdout {
			return fmt.Errorf("`set --to file/-` is not supported; use `rmesh device config get --to %s` for that", dst)
		}
		if dst.Kind == deviceconfigs.SourceUnknown {
			return errors.New("--to is required for set")
		}
		if err := deviceconfigs.ValidateSections(setFlags.section); err != nil {
			return err
		}

		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		// Cloud reads / writes share a session; lazy-init only when needed.
		var client apiclient.CloudClient
		needsSession := src.Kind == deviceconfigs.SourceCloud || dst.Kind == deviceconfigs.SourceCloud
		if needsSession {
			_, c, err := requireSession()
			if err != nil {
				return err
			}
			client = c
		}

		payload, fwHint, err := readPayloadFromSource(ctx, src, client, true)
		if err != nil {
			return err
		}
		if payload.IsEmpty() {
			return errors.New("source payload is empty; nothing to apply")
		}
		if len(setFlags.section) > 0 {
			payload = payload.CloneSections(setFlags.section)
		}
		if len(setFlags.exclude) > 0 {
			payload = payload.ExcludeFields(setFlags.exclude)
		}

		switch dst.Kind {
		case deviceconfigs.SourceDevice:
			return applyToDevice(ctx, cmd, dst, payload)
		case deviceconfigs.SourceCloud:
			return uploadToCloud(ctx, cmd, dst, payload, fwHint, client)
		}
		return fmt.Errorf("unsupported --to %s", dst)
	},
}

// applyToDevice runs the admin-edit-session apply and surfaces the
// region-change pre-check (D-210).
func applyToDevice(
	ctx context.Context,
	cmd *cobra.Command,
	dst deviceconfigs.Source,
	payload deviceconfigs.CanonicalPayload,
) error {
	url, err := resolveDeviceURL(dst.URL)
	if err != nil {
		return err
	}
	transport, err := rmtransport.Open(url)
	if err != nil {
		return fmt.Errorf("open transport %s: %w", url, err)
	}
	defer rmtransport.Close(transport)

	// Region safety pre-check (D-210).
	readCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	current, err := rmdevice.GetState(readCtx, transport)
	if err != nil {
		return fmt.Errorf("read current device state: %w", err)
	}
	currentPayload, _ := rmdevice.ToCanonicalPayload(current)
	currentHints := deviceconfigs.HintsFromPayload(currentPayload)
	intendedHints := deviceconfigs.HintsFromPayload(payload)
	if intendedHints.Region != "" && intendedHints.Region != currentHints.Region && currentHints.Region != "" {
		if !setFlags.allowRegionChange {
			fmt.Fprintf(cmd.ErrOrStderr(),
				"error: region change %s → %s would alter regulatory band; pass --allow-region-change to proceed (D-210)\n",
				currentHints.Region, intendedHints.Region)
			// Signal exit code 2 to the cobra wrapper.
			os.Exit(2)
		}
	}

	// Apply-by-diff (D-209 + D-214): only ship admin messages for
	// submessages that actually need to change. Burning flash on
	// SetConfig-with-no-delta and reporting "21 sections, 1 drift"
	// makes operator triage impossible — they can't tell which of
	// the 21 messages the firmware refused. Filtering down to the
	// real delta means a non-zero drift afterwards points at the
	// exact submessage the firmware kept on rejecting.
	diff := deviceconfigs.Diff(currentPayload, payload)
	tty := term.IsTerminal(int(os.Stdout.Fd()))

	if setFlags.dryRun {
		fmt.Fprintln(cmd.OutOrStdout(), "dry-run — pending changes:")
		deviceconfigs.RenderDiff(cmd.OutOrStdout(), diff, deviceconfigs.DiffRenderOptions{
			FromLabel: "device (current)",
			ToLabel:   "intended",
			Color:     tty,
		})
		return nil
	}

	if len(diff) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "✓ device matches intended — nothing to apply.")
		return nil
	}

	filtered := deviceconfigs.PayloadFromDiff(payload, diff)

	// Apply takes 1..30s depending on how many sections changed and
	// whether the firmware reboots on commit. Without progress lines
	// it looks frozen; with them every milestone (per-section ack,
	// reboot wait, re-read) is visible. Spinner only on TTY — CI
	// logs stay clean.
	progressW := cmd.ErrOrStderr()
	progress := newApplyProgress(progressW, tty, setFlags.verbose)
	defer progress.Close()

	// Print the exact field-level changes about to go on the wire,
	// so when post-apply drift shows the same field unchanged the
	// operator can see at a glance the CLI did ship the value — the
	// firmware just kept the old one. Same renderer as `--dry-run`.
	fmt.Fprintln(progressW, "applying changes:")
	deviceconfigs.RenderDiff(progressW, diff, deviceconfigs.DiffRenderOptions{
		FromLabel: "device (current)",
		ToLabel:   "intended",
		Color:     tty,
	})
	fmt.Fprintln(progressW)

	res, err := rmdevice.Apply(ctx, transport, filtered, rmdevice.ApplyOptions{
		RebootWait:    setFlags.rebootWait,
		PreApplyState: &currentPayload,
		OnProgress:    progress.handle,
	})
	progress.Close()
	if err != nil {
		return fmt.Errorf("apply: %w", err)
	}
	rebootMsg := "0 reboot"
	if res.Rebooted {
		rebootMsg = "1 reboot"
	}
	if res.RereadInconclusive {
		fmt.Fprintf(cmd.OutOrStdout(),
			"applied %d sections, %d channels, %s\n",
			len(res.Sections), res.ChannelsSent, rebootMsg)
		fmt.Fprintln(cmd.ErrOrStderr(),
			"warning: drift not verified — the radio returned a partial state on re-read "+
				"(usually means it's still reconnecting after CommitEditSettings). "+
				"Run `rmesh device config get` in a moment to confirm.")
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(),
		"applied %d sections, %d channels, %d drift, %s\n",
		len(res.Sections), res.ChannelsSent, res.DriftCount, rebootMsg)
	if res.DriftCount > 0 {
		// Non-zero drift = the device didn't end up where we asked.
		// Print the actual delta so the operator can see which
		// submessage the firmware refused (regulatory clamp, missing
		// session passkey, modem-preset coupling, …) instead of
		// having to re-read by hand.
		fmt.Fprintln(cmd.ErrOrStderr(), "\npost-apply drift — firmware did not accept:")
		deviceconfigs.RenderDiff(cmd.ErrOrStderr(), res.Drift, deviceconfigs.DiffRenderOptions{
			FromLabel: "intended",
			ToLabel:   "device (after)",
			Color:     tty,
		})
		// Exit code 1 so scripts can branch on it.
		return errors.New("post-apply drift detected (see above)")
	}
	return nil
}

func uploadToCloud(
	ctx context.Context,
	cmd *cobra.Command,
	dst deviceconfigs.Source,
	payload deviceconfigs.CanonicalPayload,
	fwHint string,
	client apiclient.CloudClient,
) error {
	// `set --to cloud` is always a personal save (D-213). To publish
	// a personal row as a network template, the operator runs the
	// dedicated `rmesh device config promote` verb. Any explicit
	// `cloud:<n>/template/<label>` destination is rejected here so
	// the operator gets a clear error instead of an opaque 403.
	if dst.Owner == deviceconfigs.CloudOwnerTemplate {
		return errors.New(
			"`set --to cloud:<n>/template/<label>` is not supported; " +
				"`set --to cloud` always writes a personal row. " +
				"Use `rmesh device config promote` to publish a personal config as a network template (D-213).",
		)
	}
	if setFlags.label == "" {
		// Allow the destination label to come from the URI tail when --label is unset
		// (e.g. `--to cloud:home/eu-868` => label "eu-868").
		if dst.Label != "" {
			setFlags.label = dst.Label
		}
	}
	if setFlags.label == "" {
		return errors.New("--label is required when --to cloud (or set the destination as cloud:<network>/<label>)")
	}
	netID, err := resolveCloudNetworkID(ctx, client, dst.Network)
	if err != nil {
		return err
	}
	// Echo the payload as JSON to verify shape early — failures here
	// (e.g. a stray non-JSON-serialisable type from a future field)
	// surface as a clear local error instead of an opaque 400.
	if _, err := json.Marshal(payload); err != nil {
		return fmt.Errorf("payload is not JSON-serialisable: %w", err)
	}
	c := concrete(client)
	if c == nil {
		return errors.New("cloud destination requires the concrete *apiclient.Client (alternative implementations not supported)")
	}
	out, err := c.CreateMyDeviceConfig(ctx, apiclient.CreatePersonalRequest{
		NetworkID:       netID,
		Label:           setFlags.label,
		Description:     setFlags.description,
		Payload:         payload,
		FirmwareVersion: fwHint,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "saved cloud:%s/mine/%s (id=%s)\n", dst.Network, out.Label, out.ID)
	return nil
}

func init() {
	deviceConfigCmd.AddCommand(deviceConfigSetCmd)
	deviceConfigSetCmd.SilenceUsage = true
	f := deviceConfigSetCmd.Flags()
	f.StringVar(&setFlags.from, "from", "", "Source: device[:url], file:<path>, ./path.yaml, cloud:<network>/<label> (required)")
	f.StringVar(&setFlags.to, "to", "", "Destination: device[:url] or cloud:<network>[/<label>] (required)")
	f.StringSliceVar(&setFlags.section, "section", nil, "Comma-separated list of submessage keys to apply (default: all)")
	f.StringSliceVar(&setFlags.exclude, "exclude", nil, "Comma-separated list of dotted paths to drop (e.g. owner,lora.region)")
	f.BoolVar(&setFlags.dryRun, "dry-run", false, "Print the diff that would be applied; do not write to the device")
	f.BoolVar(&setFlags.allowRegionChange, "allow-region-change", false, "Acknowledge that the apply changes the radio's regulatory region (D-210)")
	f.StringVar(&setFlags.label, "label", "", "Label for the new personal cloud config (required when --to cloud)")
	f.StringVar(&setFlags.description, "description", "", "Optional description stored alongside the cloud config")
	f.DurationVar(&setFlags.rebootWait, "reboot-wait", 15*time.Second, "How long to wait for FromRadio_Rebooted after CommitEditSettings")
	f.BoolVarP(&setFlags.verbose, "verbose", "v", false, "Print the exact SetConfig / SetModuleConfig payload sent to the device for each section")
	_ = deviceConfigSetCmd.MarkFlagRequired("from")
	_ = deviceConfigSetCmd.MarkFlagRequired("to")
}

// Silence "errRegionChangeRefused unused" until the cobra `--from` returns it.
var _ = errRegionChangeRefused
