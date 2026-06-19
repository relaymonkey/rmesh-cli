package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/relaymonkey/relaymesh-edge/internal/apiclient"
	"github.com/relaymonkey/relaymesh-edge/internal/cliui"
	"github.com/relaymonkey/relaymesh-edge/internal/deviceconfigs"
)

var setFlags commonFlags

// regionChangeRefused signals the operator-explicit-consent failure.
// exit code 2 (distinct from 1) so wrappers can branch on it.
var errRegionChangeRefused = errors.New("region change refused; pass --allow-region-change to proceed")

var deviceConfigSetCmd = &cobra.Command{
	Use:        "set",
	Deprecated: "use `rmesh device config copy` instead. `copy --to <dst>` accepts every destination (device, cloud, file, stdout) without the file/stdout rejection that `set` carries.",
	Hidden:     true,
	Short:      "Apply a device configuration to a device, or upload it to the cloud",
	Long: `Write a configuration to a destination. Source / destination combinations:

  --from device --to cloud         capture current state and save as a named cloud config
  --from cloud --to device         pull a saved cloud config and apply to the device
  --from file  --to device         apply a local file (the "import" workflow)
  --from device --to file/-        identical to "get" (kept off this verb; use ` + "`get`" + `)

Applying to a device wraps every Set* admin message in a single
BeginEditSettings / CommitEditSettings session so the firmware reboots
at most once. When the source payload's lora.region differs
from the device's currently reported region, the apply refuses unless
` + "`--allow-region-change`" + ` is passed.`,
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
			return uploadToCloud(ctx, cmd, dst, payload, fwHint, client, cloudUploadOptions{
				Label:       setFlags.label,
				Description: setFlags.description,
			})
		}
		return fmt.Errorf("unsupported --to %s", dst)
	},
}

// applyOptions captures the subset of flag state that
// applyPayloadToDevice consumes. Both `set` and `edit` share the
// same apply path; this struct lets each verb own its own flag
// block without coupling to the other's globals.
type applyOptions struct {
	AllowRegionChange bool
	DryRun            bool
	RebootWait        time.Duration
	Verbose           bool
}

// applyToDevice is `set`'s wrapper around applyPayloadToDevice;
// kept for source-locality with the rest of the `set` handler.
func applyToDevice(
	ctx context.Context,
	cmd *cobra.Command,
	dst deviceconfigs.Source,
	payload deviceconfigs.CanonicalPayload,
) error {
	return applyPayloadToDevice(ctx, cmd, dst, payload, applyOptions{
		AllowRegionChange: setFlags.allowRegionChange,
		DryRun:            setFlags.dryRun,
		RebootWait:        setFlags.rebootWait,
		Verbose:           setFlags.verbose,
	})
}

// cloudUploadOptions decouples uploadToCloud from any single verb's
// flag struct; both `set` and `copy` populate this and call
// the shared helper.
type cloudUploadOptions struct {
	Label       string
	Description string
}

func uploadToCloud(
	ctx context.Context,
	cmd *cobra.Command,
	dst deviceconfigs.Source,
	payload deviceconfigs.CanonicalPayload,
	fwHint string,
	client apiclient.CloudClient,
	opts cloudUploadOptions,
) error {
	// Per D-219 writing to cloud always creates a personal row in
	// the caller's user-scoped library. Templates can only be
	// authored via `promote`; any per-network destination is rejected.
	if dst.Owner == deviceconfigs.CloudOwnerTemplate {
		return errors.New(
			"`--to cloud:<n>/template/<label>` is not supported; " +
				"writing to cloud always creates a personal row. " +
				"Use `rmesh device config promote` to publish a personal config as a network template.",
		)
	}
	if dst.Owner == deviceconfigs.CloudOwnerEither && dst.Network != "" {
		return errors.New(
			"`--to cloud:<n>/<label>` is not supported (D-219); use " +
				"`--to cloud:mine/<label>` for a personal save, or " +
				"`rmesh device config promote` to publish a template.",
		)
	}
	label := opts.Label
	if label == "" && dst.Label != "" {
		label = dst.Label
	}
	if label == "" {
		return errors.New("--label is required when --to cloud (or set the destination as cloud:mine/<label>)")
	}
	if _, err := json.Marshal(payload); err != nil {
		return fmt.Errorf("payload is not JSON-serialisable: %w", err)
	}
	c := concrete(client)
	if c == nil {
		return errors.New("cloud destination requires the concrete *apiclient.Client (alternative implementations not supported)")
	}
	out, err := c.CreateMyDeviceConfig(ctx, apiclient.CreatePersonalRequest{
		Label:           label,
		Description:     opts.Description,
		Payload:         payload,
		FirmwareVersion: fwHint,
	})
	if err != nil {
		return err
	}
	path := deviceconfigs.Source{
		Kind:  deviceconfigs.SourceCloud,
		Owner: deviceconfigs.CloudOwnerMine,
		Label: out.Label,
	}.String()
	return cliui.New(cmd.OutOrStdout()).SavedCloudConfig(out.Label, path, out.ID)
}

func init() {
	deviceConfigCmd.AddCommand(deviceConfigSetCmd)
	deviceConfigSetCmd.SilenceUsage = true
	f := deviceConfigSetCmd.Flags()
	f.StringVar(&setFlags.from, "from", "", "Source: device[:url], file:<path>, ./path.yaml, cloud:<network>/<label> (required)")
	f.StringVar(&setFlags.to, "to", "", "Destination: device[:url] or cloud:<network>[/<label>] (required)")
	f.StringSliceVar(&setFlags.section, "section", nil, "Comma-separated list of submessage keys to apply (default: all)")
	f.StringSliceVar(&setFlags.exclude, "exclude", nil, "Comma-separated sections or fields to drop, matching the names shown in the diff (e.g. config.lora, lora, lora.region, owner)")
	f.BoolVar(&setFlags.dryRun, "dry-run", false, "Print the diff that would be applied; do not write to the device")
	f.BoolVar(&setFlags.allowRegionChange, "allow-region-change", false, "Acknowledge that the apply changes the radio's regulatory region")
	f.StringVar(&setFlags.label, "label", "", "Label for the new personal cloud config (required when --to cloud)")
	f.StringVar(&setFlags.description, "description", "", "Optional description stored alongside the cloud config")
	f.DurationVar(&setFlags.rebootWait, "reboot-wait", 20*time.Second, "How long to wait for the device to reconnect after a mid-apply reboot")
	f.BoolVarP(&setFlags.verbose, "verbose", "v", false, "Print the exact SetConfig / SetModuleConfig payload sent to the device for each section")
	_ = deviceConfigSetCmd.MarkFlagRequired("from")
	_ = deviceConfigSetCmd.MarkFlagRequired("to")
}

// Silence "errRegionChangeRefused unused" until the cobra `--from` returns it.
var _ = errRegionChangeRefused
