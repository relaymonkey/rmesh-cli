package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/relaymonkey/relaymesh-edge/internal/deviceconfigs"
)

// showFlags is the flag block for `rmesh device config show`.
// `show` is a pure read verb: no `--to`. Output always goes to stdout;
// operators redirect with `>` or use `copy --to <file>` when they want
// a file on disk with the transfer-intent reveal semantics.
var showFlags struct {
	from    string
	output  string
	section []string
	reveal  bool
}

var deviceConfigShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Read a device configuration from any source and render it to stdout",
	Long: `Read a configuration from a live device, a local file, or a saved
cloud config and render it to stdout. ` + "`show`" + ` never writes — for
file-on-disk backups use ` + "`copy --to <path>`" + `.

Examples:

  # Live device → terminal (tree view).
  rmesh device config show --from device

  # Local file → JSON to stdout (preview / convert).
  rmesh device config show --from ./config.yaml -o json

  # Cloud config → terminal.
  rmesh device config show --from cloud:home/eu-868

  # Pipe to a file (redacted; for an unredacted backup use ` + "`copy`" + `).
  rmesh device config show --from device -o yaml > snapshot.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		src, err := deviceconfigs.ParseSource(showFlags.from)
		if err != nil {
			return err
		}
		if err := deviceconfigs.ValidateSections(showFlags.section); err != nil {
			return err
		}
		fmtChoice, err := deviceconfigs.ParseFormat(showFlags.output)
		if err != nil {
			return err
		}

		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		var payload deviceconfigs.CanonicalPayload
		if src.Kind == deviceconfigs.SourceCloud {
			_, c, err := requireSession()
			if err != nil {
				return err
			}
			payload, _, err = readPayloadFromSource(ctx, src, c, showFlags.reveal)
			if err != nil {
				return err
			}
		} else {
			payload, _, err = readPayloadFromSource(ctx, src, nil, showFlags.reveal)
			if err != nil {
				return err
			}
			if !showFlags.reveal {
				payload = deviceconfigs.Redact(payload)
			}
		}

		if len(showFlags.section) > 0 {
			payload = payload.CloneSections(showFlags.section)
		}

		return deviceconfigs.Render(cmd.OutOrStdout(), payload, fmtChoice)
	},
}

func init() {
	deviceConfigCmd.AddCommand(deviceConfigShowCmd)
	f := deviceConfigShowCmd.Flags()
	f.StringVar(&showFlags.from, "from", "device", "Source: device[:url], file:<path>, ./path.yaml, cloud:<network>/<label>")
	f.StringVarP(&showFlags.output, "output", "o", "tree", "Output format: tree, json, yaml")
	f.StringSliceVar(&showFlags.section, "section", nil, "Comma-separated list of submessage keys to include (e.g. lora,mqtt)")
	f.BoolVar(&showFlags.reveal, "reveal-secrets", false, "Print PSK / admin-key fields in the clear (default: redact as ***)")
}
