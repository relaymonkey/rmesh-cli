package cmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/relaymonkey/relaymesh-edge/internal/apiclient"
	"github.com/relaymonkey/relaymesh-edge/internal/deviceconfigs"
)

// copyFlags is the flag block for `rmesh device config copy`.
// `copy` is the transfer verb: any source → any destination, side
// effects (radio reboot, cloud row creation, file write) decided by
// the destination kind in the per-destination handler.
var copyFlags struct {
	from              string
	to                string
	output            string
	section           []string
	exclude           []string
	dryRun            bool
	allowRegionChange bool
	label             string
	description       string
	rebootWait        time.Duration
	verbose           bool
}

var deviceConfigCopyCmd = &cobra.Command{
	Use:   "copy",
	Short: "Copy a device configuration from any source to any destination",
	Long: `Transfer a configuration payload from a source to a destination.
The destination's side effects (radio reboot, cloud row creation, file
write) live in the per-destination handler; the verb itself is
destination-agnostic.

Sources and destinations: ` + "`device[:url]`" + `, ` + "`file:<path>`" + ` (or ` + "`./path.yaml`" + `),
` + "`cloud:<network>[/{mine|template}]/<label>`" + `; ` + "`-`" + ` is also a valid
destination (= stdout).

Examples:

  # Snapshot live device → file (backup).
  rmesh device config copy --from device --to ./backup.yaml

  # Snapshot live device → personal cloud library.
  rmesh device config copy --from device --to cloud:home/eu-868

  # Apply a saved cloud config to a live device.
  rmesh device config copy --from cloud:home/eu-868 --to device

  # Apply a local file to a live device.
  rmesh device config copy --from ./eu-868.yaml --to device

  # Preview a device apply without writing.
  rmesh device config copy --from ./eu-868.yaml --to device --dry-run

  # Export a saved cloud config to disk.
  rmesh device config copy --from cloud:home/eu-868 --to ./eu-868.yaml

Reveals secrets unconditionally: ` + "`copy`" + ` is the transfer verb, so
PSK / admin-key / MQTT-password fields are read in the clear from the
source so the destination receives a usable payload. For redacted
inspection use ` + "`rmesh device config show`" + ` instead.

Applying to a device wraps every Set* admin message in a single
BeginEditSettings / CommitEditSettings session so the firmware reboots
at most once. When the source payload's ` + "`lora.region`" + `
differs from the device's currently reported region, the apply refuses
unless ` + "`--allow-region-change`" + ` is passed.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		src, err := deviceconfigs.ParseSource(copyFlags.from)
		if err != nil {
			return err
		}
		dst, err := deviceconfigs.ParseSource(copyFlags.to)
		if err != nil {
			return err
		}
		if dst.Kind == deviceconfigs.SourceUnknown {
			return errors.New("--to is required for copy")
		}
		if err := deviceconfigs.ValidateSections(copyFlags.section); err != nil {
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
			return errors.New("source payload is empty; nothing to copy")
		}
		if len(copyFlags.section) > 0 {
			payload = payload.CloneSections(copyFlags.section)
		}
		if len(copyFlags.exclude) > 0 {
			payload = payload.ExcludeFields(copyFlags.exclude)
		}

		switch dst.Kind {
		case deviceconfigs.SourceDevice:
			return applyPayloadToDevice(ctx, cmd, dst, payload, applyOptions{
				AllowRegionChange: copyFlags.allowRegionChange,
				DryRun:            copyFlags.dryRun,
				RebootWait:        copyFlags.rebootWait,
				Verbose:           copyFlags.verbose,
			})
		case deviceconfigs.SourceCloud:
			if copyFlags.dryRun {
				fmt.Fprintf(cmd.OutOrStdout(),
					"dry-run — would upload to cloud:%s as label=%q (description=%q)\n",
					dst.Network, firstNonEmpty(copyFlags.label, dst.Label), copyFlags.description)
				return nil
			}
			return uploadToCloud(ctx, cmd, dst, payload, fwHint, client, cloudUploadOptions{
				Label:       copyFlags.label,
				Description: copyFlags.description,
			})
		case deviceconfigs.SourceFile, deviceconfigs.SourceStdout:
			return writePayloadToFile(cmd, dst, payload, copyFlags.output, copyFlags.dryRun)
		}
		return fmt.Errorf("unsupported --to %s", dst)
	},
}

// writePayloadToFile renders a canonical payload to a file (or stdout
// when dst is `-`). Format is inferred from the file extension when
// --output is unset; the `outFormat` argument wins when explicit.
func writePayloadToFile(
	cmd *cobra.Command,
	dst deviceconfigs.Source,
	payload deviceconfigs.CanonicalPayload,
	outFormat string,
	dryRun bool,
) error {
	if dryRun {
		fmt.Fprintf(cmd.OutOrStdout(), "dry-run — would write to %s\n", dst)
		return nil
	}
	fmtChoice, err := deviceconfigs.ParseFormat(outFormat)
	if err != nil {
		return err
	}
	target := ""
	if dst.Kind == deviceconfigs.SourceFile {
		target = dst.Path
	} else {
		target = "-"
	}
	w, closer, err := openOutput(target)
	if err != nil {
		return err
	}
	defer closer()
	return deviceconfigs.Render(w, payload, fmtChoice)
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func init() {
	deviceConfigCmd.AddCommand(deviceConfigCopyCmd)
	deviceConfigCopyCmd.SilenceUsage = true
	f := deviceConfigCopyCmd.Flags()
	f.StringVar(&copyFlags.from, "from", "", "Source: device[:url], file:<path>, ./path.yaml, cloud:<network>/<label> (required)")
	f.StringVar(&copyFlags.to, "to", "", "Destination: device[:url], cloud:<network>[/<label>], file:<path>, ./path.yaml, or - for stdout (required)")
	f.StringVarP(&copyFlags.output, "output", "o", "yaml", "Output format for file/stdout destinations: yaml, json, tree")
	f.StringSliceVar(&copyFlags.section, "section", nil, "Comma-separated list of submessage keys to include (default: all)")
	f.StringSliceVar(&copyFlags.exclude, "exclude", nil, "Comma-separated list of dotted paths to drop (e.g. owner,lora.region)")
	f.BoolVar(&copyFlags.dryRun, "dry-run", false, "Print what would be transferred; do not write")
	f.BoolVar(&copyFlags.allowRegionChange, "allow-region-change", false, "Acknowledge that applying to a device changes the radio's regulatory region")
	f.StringVar(&copyFlags.label, "label", "", "Label for the new personal cloud config (required when --to cloud and the destination has no label tail)")
	f.StringVar(&copyFlags.description, "description", "", "Optional description stored alongside the cloud config")
	f.DurationVar(&copyFlags.rebootWait, "reboot-wait", 15*time.Second, "How long to wait for FromRadio_Rebooted after CommitEditSettings (device destinations only)")
	f.BoolVarP(&copyFlags.verbose, "verbose", "v", false, "Print the exact SetConfig / SetModuleConfig payload sent to the device for each section")
	_ = deviceConfigCopyCmd.MarkFlagRequired("from")
	_ = deviceConfigCopyCmd.MarkFlagRequired("to")
}
