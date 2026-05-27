package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/relaymonkey/relaymesh-edge/internal/apiclient"
	"github.com/relaymonkey/relaymesh-edge/internal/cliconfig"
	"github.com/relaymonkey/relaymesh-edge/internal/cliui"
	"github.com/relaymonkey/relaymesh-edge/internal/deviceconfigs"
)

var editFlags struct {
	from              string
	format            string
	allowRegionChange bool
	rebootWait        time.Duration
	verbose           bool
	dryRun            bool
}

var deviceConfigEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Open a device configuration in $EDITOR and write the result back to its source",
	Long: "Read a device configuration from any source (live device, local\n" +
		"file, or saved cloud config), open the canonical JSON / YAML in\n" +
		"your default editor and write the edited payload back to the\n" +
		"same source on save.\n\n" +
		"The source resolved by --from is also the destination — there is\n" +
		"no --to. Examples:\n\n" +
		"  rmesh device config edit --from device\n" +
		"  rmesh device config edit --from ./backup.yaml\n" +
		"  rmesh device config edit --from cloud:home/eu-868\n" +
		"  rmesh device config edit --from cloud:home/mine/scratch --format json\n\n" +
		"How each source is written back:\n\n" +
		"  device  the edited payload is applied with the same admin-edit-\n" +
		"          session apply path as `set --to device` (one Begin/Commit\n" +
		"          EditSettings session, field-level drift report,\n" +
		"          --allow-region-change gate).\n" +
		"  file    the edited body is written back to the file in the same\n" +
		"          format as its extension (.yaml/.yml stay YAML; .json\n" +
		"          stays JSON).\n" +
		"  cloud   a PATCH is issued against the row at --from. The payload\n" +
		"          is always read with secrets revealed so the editor sees\n" +
		"          real values (otherwise `***` placeholders would be\n" +
		"          written back to the cloud and overwrite the real\n" +
		"          secrets); revealing requires the same elevated role the\n" +
		"          cloud surface gates on.\n\n" +
		"The editor is chosen from $EDITOR, then $VISUAL, then `nano`. If\n" +
		"the edited content is identical to the original (byte-for-byte),\n" +
		"the source is left untouched and the command exits 0 with\n" +
		"\"no changes\".",
	RunE: func(cmd *cobra.Command, args []string) error {
		src, err := deviceconfigs.ParseSource(editFlags.from)
		if err != nil {
			return err
		}
		if src.Kind == deviceconfigs.SourceStdout || src.Kind == deviceconfigs.SourceUnknown {
			return fmt.Errorf("--from %q is not editable; expected device, file, or cloud", editFlags.from)
		}

		format, err := deviceconfigs.ParseFormat(editFlags.format)
		if err != nil {
			return err
		}
		// `tree` is a read-only rendering and is not round-trippable
		// through ParseBytes — reject it explicitly so we don't end up
		// re-uploading garbage from a tree-rendered edit.
		if format == deviceconfigs.FormatTree {
			return fmt.Errorf("--format tree is not editable; use yaml (default) or json")
		}

		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		// Cloud reads / writes share a session; lazy-init only when needed.
		var (
			client      apiclient.CloudClient
			netID       string
			cfgID       string
			needsClient = src.Kind == deviceconfigs.SourceCloud
		)
		if needsClient {
			_, c, err := requireSession()
			if err != nil {
				return err
			}
			client = c
			netID, err = resolveCloudNetworkID(ctx, client, src.Network)
			if err != nil {
				return err
			}
			cfgID, err = concrete(client).ResolveDeviceConfigID(ctx, netID, src.Label, apiclient.OwnerHint(src.Owner))
			if err != nil {
				return err
			}
		}

		// reveal=true: the operator is about to round-trip the payload
		// back to the source, so redacted placeholders would clobber
		// real secrets on save. Backend gates the reveal on role; the
		// CLI surfaces whatever error the cloud returns.
		payload, _, err := readPayloadFromSource(ctx, src, client, true)
		if err != nil {
			return err
		}

		ext := ".yaml"
		if format == deviceconfigs.FormatJSON {
			ext = ".json"
		}
		tmp, err := os.CreateTemp("", "rmesh-device-config-*"+ext)
		if err != nil {
			return fmt.Errorf("create temp file: %w", err)
		}
		tmpPath := tmp.Name()
		defer os.Remove(tmpPath)

		if err := deviceconfigs.Render(tmp, payload, format); err != nil {
			tmp.Close()
			return fmt.Errorf("render payload: %w", err)
		}
		if err := tmp.Close(); err != nil {
			return fmt.Errorf("close temp file: %w", err)
		}

		originalBytes, err := os.ReadFile(tmpPath)
		if err != nil {
			return fmt.Errorf("read temp file: %w", err)
		}

		if err := runEditor(tmpPath); err != nil {
			return err
		}

		editedBytes, err := os.ReadFile(tmpPath)
		if err != nil {
			return fmt.Errorf("re-read temp file: %w", err)
		}
		if bytes.Equal(originalBytes, editedBytes) {
			return cliui.New(cmd.OutOrStdout()).NoChanges(src.String())
		}

		edited, err := deviceconfigs.ParseBytes(editedBytes, tmpPath)
		if err != nil {
			return fmt.Errorf("parse edited payload: %w (re-edit and try again, or discard the buffer)", err)
		}
		if edited.IsEmpty() {
			return errors.New("edited payload is empty; refusing to write back")
		}

		switch src.Kind {
		case deviceconfigs.SourceFile:
			if err := deviceconfigs.WriteToFile(src.Path, edited); err != nil {
				return err
			}
			return cliui.New(cmd.OutOrStdout()).WroteFile(src.Path)

		case deviceconfigs.SourceDevice:
			return applyPayloadToDevice(ctx, cmd, src, edited, applyOptions{
				AllowRegionChange: editFlags.allowRegionChange,
				DryRun:            editFlags.dryRun,
				RebootWait:        editFlags.rebootWait,
				Verbose:           editFlags.verbose,
			})

		case deviceconfigs.SourceCloud:
			if editFlags.dryRun {
				return cliui.New(cmd.OutOrStdout()).DryRun("PATCH not sent",
					cliui.Field{Key: "source", Value: src.String()},
				)
			}
			c := concrete(client)
			if c == nil {
				return errors.New("cloud edit requires the concrete *apiclient.Client")
			}
			out, err := c.UpdateDeviceConfig(ctx, netID, cfgID, apiclient.UpdateDeviceConfigRequest{
				Payload: &edited,
			})
			if err != nil {
				return fmt.Errorf("update %s: %w", src, err)
			}
			updated := src
			updated.Label = out.Label
			return cliui.New(cmd.OutOrStdout()).UpdatedCloudConfig(
				out.Label, updated.String(), out.ID,
			)
		}
		return fmt.Errorf("unsupported source kind: %s", src)
	},
}

// runEditor launches $EDITOR (fallback $VISUAL → nano) on path,
// inheriting stdio so terminal editors render correctly.
func runEditor(path string) error {
	editor := cliconfig.Editor()
	parts := strings.Fields(editor)
	if len(parts) == 0 {
		return errors.New("no editor configured (set $EDITOR)")
	}
	c := exec.Command(parts[0], append(parts[1:], path)...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("editor %q: %w", editor, err)
	}
	return nil
}

func init() {
	deviceConfigCmd.AddCommand(deviceConfigEditCmd)
	deviceConfigEditCmd.SilenceUsage = true
	f := deviceConfigEditCmd.Flags()
	f.StringVar(&editFlags.from, "from", "", "Source: device[:url], file:<path>, ./path.yaml, cloud:<network>/<label> (required; also the destination)")
	f.StringVar(&editFlags.format, "format", "yaml", "Editor buffer format: yaml or json")
	f.BoolVar(&editFlags.allowRegionChange, "allow-region-change", false, "Acknowledge a regulatory-region change on apply (device only)")
	f.DurationVar(&editFlags.rebootWait, "reboot-wait", 15*time.Second, "How long to wait for FromRadio_Rebooted after CommitEditSettings (device only)")
	f.BoolVarP(&editFlags.verbose, "verbose", "v", false, "Print the exact SetConfig / SetModuleConfig payload sent to the device for each section (device only)")
	f.BoolVar(&editFlags.dryRun, "dry-run", false, "Show the diff that would be applied (device) or skip the PATCH (cloud); never write back")
	_ = deviceConfigEditCmd.MarkFlagRequired("from")
}
