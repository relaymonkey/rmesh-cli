package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/relaymonkey/relaymesh-edge/internal/deviceconfigs"
)

var getFlags commonFlags

var deviceConfigGetCmd = &cobra.Command{
	Use:        "get",
	Deprecated: "use `rmesh device config show` to print, or `rmesh device config copy --to <path>` to save to a file.",
	Hidden:     true,
	Short:      "Read a device configuration from any source and render or save it",
	Long: `Read a device configuration from a live device, a local file, or
a saved cloud config, and render it to stdout or save it to a file.

Examples:

  # Live device → terminal (tree view).
  rmesh device config get --from device

  # Live device → YAML file (the "export" workflow).
  rmesh device config get --from device --to ./backup.yaml

  # Local file → JSON to stdout (preview).
  rmesh device config get --from ./config.yaml -o json

  # Cloud config → terminal.
  rmesh device config get --from cloud:home/eu-868`,
	RunE: func(cmd *cobra.Command, args []string) error {
		src, err := deviceconfigs.ParseSource(getFlags.from)
		if err != nil {
			return err
		}
		// `get` is a read verb: --to is a file path or stdout, never a
		// destination that mutates state. The grammar would otherwise
		// silently treat `--to device` (or `--to cloud:...`) as a path
		// and create a file with that literal name in cwd. Reject the
		// two reserved tokens explicitly and point the operator at the
		// verb that does the thing they meant.
		if dst, perr := deviceconfigs.ParseSource(getFlags.to); perr == nil {
			switch dst.Kind {
			case deviceconfigs.SourceDevice:
				return fmt.Errorf(
					"`get --to device` is not supported; `get` only reads. "+
						"Use `rmesh device config set --from %s --to device` to push a config to a device.",
					src,
				)
			case deviceconfigs.SourceCloud:
				return fmt.Errorf(
					"`get --to cloud:...` is not supported; `get` only reads. "+
						"Use `rmesh device config set --from %s --to %s` to save a config to the cloud.",
					src, dst,
				)
			}
		}
		if err := deviceconfigs.ValidateSections(getFlags.section); err != nil {
			return err
		}
		fmtChoice, err := deviceconfigs.ParseFormat(getFlags.output)
		if err != nil {
			return err
		}

		// Cloud reads honour --reveal-secrets by passing through to
		// the backend; device + file reads do their own redaction.
		var client interface{}
		var cliClient = (*interface{})(nil)
		_ = client
		_ = cliClient

		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		var (
			payload deviceconfigs.CanonicalPayload
		)
		// Cloud reads need a session; device/file reads don't.
		if src.Kind == deviceconfigs.SourceCloud {
			_, c, err := requireSession()
			if err != nil {
				return err
			}
			payload, _, err = readPayloadFromSource(ctx, src, c, getFlags.reveal)
			if err != nil {
				return err
			}
		} else {
			payload, _, err = readPayloadFromSource(ctx, src, nil, getFlags.reveal)
			if err != nil {
				return err
			}
			if !getFlags.reveal {
				payload = deviceconfigs.Redact(payload)
			}
		}

		if len(getFlags.section) > 0 {
			payload = payload.CloneSections(getFlags.section)
		}

		w, closer, err := openOutput(getFlags.to)
		if err != nil {
			return err
		}
		defer closer()
		return deviceconfigs.Render(w, payload, fmtChoice)
	},
}

func init() {
	deviceConfigCmd.AddCommand(deviceConfigGetCmd)
	f := deviceConfigGetCmd.Flags()
	f.StringVar(&getFlags.from, "from", "device", "Source: device[:url], file:<path>, ./path.yaml, cloud:<network>/<label>")
	f.StringVar(&getFlags.to, "to", "-", "Destination: file path, or `-` for stdout")
	f.StringVarP(&getFlags.output, "output", "o", "tree", "Output format: tree, json, yaml")
	f.StringSliceVar(&getFlags.section, "section", nil, "Comma-separated list of submessage keys to include (e.g. lora,mqtt)")
	f.BoolVar(&getFlags.reveal, "reveal-secrets", false, "Print PSK / admin-key fields in the clear (default: redact as ***)")
}

// validateSectionList is a tiny helper retained for the diff verb,
// which shares the same section semantics.
func validateSectionList(s []string) error {
	for _, k := range s {
		if strings.TrimSpace(k) == "" {
			return fmt.Errorf("empty section key")
		}
	}
	return nil
}

var _ = validateSectionList // keep referenced when only used by diff
