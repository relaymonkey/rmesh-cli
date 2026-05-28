package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/relaymonkey/relaymesh-edge/internal/apiclient"
	"github.com/relaymonkey/relaymesh-edge/internal/clidefault"
	"github.com/relaymonkey/relaymesh-edge/internal/cliui"
)

// tplSkip carries the structured reason templates couldn't be loaded
// so the renderer can drive the cliui framework (Warn + Note + Hint)
// instead of dumping a raw wrapped-error string.
type tplSkip struct {
	headline string // short Warn headline
	cause    string // optional Note line with the underlying detail
	hint     string // optional Hint line with the next step
}

func skipFromResolve(err error) tplSkip {
	if errors.Is(err, clidefault.ErrNotSet) {
		return tplSkip{
			headline: "templates skipped: no default network set",
			hint:     "pass --network <id> or run: rmesh network use <id>",
		}
	}
	return tplSkip{
		headline: "templates skipped: could not resolve network",
		cause:    err.Error(),
		hint:     "pass --network <id> or run: rmesh network use <id>",
	}
}

func skipFromFetch(err error) tplSkip {
	return tplSkip{
		headline: "templates skipped: could not fetch network templates",
		cause:    err.Error(),
		hint:     "verify the network with: rmesh network list",
	}
}

var listFlags struct {
	network   string
	output    string
	mine      bool
	templates bool
}

var deviceConfigListCmd = &cobra.Command{
	Use:   "list",
	Short: "List saved device configurations on a network",
	Long: `List device configs stored on a network. By default both
your personal library AND the network's templates are listed,
grouped in the table output.

  rmesh device config list                              # mine + templates of the current network
  rmesh device config list --network home               # mine + templates of "home"
  rmesh device config list --mine                       # only my personal configs in the current network
  rmesh device config list --templates                  # only the network's templates
  rmesh device config list -o json                      # machine-readable union (owner_kind on each row)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		_, client, err := requireSession()
		if err != nil {
			return err
		}
		c := concrete(client)

		showMine := listFlags.mine || (!listFlags.mine && !listFlags.templates)
		showTpl := listFlags.templates || (!listFlags.mine && !listFlags.templates)
		// `--templates` (explicit) is a hard requirement: surface
		// network resolution / fetch failures as errors. The
		// default (mine + templates) treats template failures as a
		// warning so the personal library still renders when the
		// saved default network is missing or unset.
		strictTpl := listFlags.templates && !listFlags.mine

		var (
			mine  apiclient.DeviceConfigList
			tpl   apiclient.DeviceConfigList
			skip  *tplSkip
			netID string
		)
		if showTpl {
			netID, err = resolveNetworkID(cmd, client, listFlags.network)
			if err != nil {
				if strictTpl {
					return err
				}
				s := skipFromResolve(err)
				skip = &s
				showTpl = false
			}
		}
		if showMine {
			mine, err = c.ListMyDeviceConfigs(ctx)
			if err != nil {
				return err
			}
		}
		if showTpl {
			tpl, err = c.ListDeviceConfigs(ctx, netID)
			if err != nil {
				if strictTpl {
					return err
				}
				s := skipFromFetch(err)
				skip = &s
				tpl = apiclient.DeviceConfigList{}
			}
		}
		if skip != nil {
			u := cliui.New(cmd.ErrOrStderr())
			_ = u.Warn(skip.headline)
			if skip.cause != "" {
				_ = u.Note(skip.cause)
			}
			if skip.hint != "" {
				_ = u.Hint(skip.hint)
			}
		}

		switch strings.ToLower(listFlags.output) {
		case "json":
			out := struct {
				Mine      []apiclient.DeviceConfigSummary `json:"mine"`
				Templates []apiclient.DeviceConfigSummary `json:"templates"`
			}{Mine: mine.Items, Templates: tpl.Items}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(out)
		case "yaml":
			out := struct {
				Mine      []apiclient.DeviceConfigSummary `yaml:"mine"`
				Templates []apiclient.DeviceConfigSummary `yaml:"templates"`
			}{Mine: mine.Items, Templates: tpl.Items}
			enc := yaml.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent(2)
			return enc.Encode(out)
		case "", "table":
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			if showMine {
				fmt.Fprintf(tw, "MINE (%d)\n", len(mine.Items))
				renderTable(tw, mine.Items, "me")
				fmt.Fprintln(tw)
			}
			if showTpl {
				fmt.Fprintf(tw, "TEMPLATES (%d)\n", len(tpl.Items))
				renderTable(tw, tpl.Items, "net")
			}
			return tw.Flush()
		}
		return fmt.Errorf("unknown --output %q", listFlags.output)
	},
}

func renderTable(w *tabwriter.Writer, items []apiclient.DeviceConfigSummary, ownerLabel string) {
	fmt.Fprintln(w, "  OWNER\tID\tLABEL\tREGION\tPRESET\tFW\tVISIBILITY\tFEATURED\tUPDATED")
	for _, it := range items {
		fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\t%s\t%s\t%t\t%s\n",
			ownerLabel, shortenID(it.ID), it.Label, it.Region, it.ModemPreset,
			it.FirmwareVersion, it.Visibility, it.IsFeatured, it.UpdatedAt)
	}
}

func shortenID(id string) string {
	if len(id) >= 8 {
		return id[:8]
	}
	return id
}

func init() {
	deviceConfigCmd.AddCommand(deviceConfigListCmd)
	f := deviceConfigListCmd.Flags()
	f.StringVarP(&listFlags.network, "network", "n", "", "Network UUID (defaults to `rmesh network use`)")
	f.StringVarP(&listFlags.output, "output", "o", "table", "Output format: table, json, yaml")
	f.BoolVar(&listFlags.mine, "mine", false, "Only my personal device configs")
	f.BoolVar(&listFlags.templates, "templates", false, "Only the network's templates")
}
