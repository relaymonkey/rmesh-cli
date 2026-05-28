package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/relaymonkey/relaymesh-edge/internal/apiclient"
	"github.com/relaymonkey/relaymesh-edge/internal/cliui"
	"github.com/relaymonkey/relaymesh-edge/internal/deviceconfigs"
)

var deleteFlags struct {
	from string
	yes  bool
}

var deviceConfigDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a saved cloud device configuration",
	Long: `Permanently delete a personal or network-template device config.

Only cloud sources are valid. The row is removed from RelayMesh;
this cannot be undone.

Personal rows (` + "`cloud:mine/<label>`" + `) require you to be the owner.
Network templates (` + "`cloud:<n>/template/<label>`" + ` or bare
` + "`cloud:<n>/<label>`" + `) require an elevated network role
(admin, operator, or deployer).

Interactive terminals prompt for confirmation unless ` + "`--yes`" + ` is
passed. Non-interactive shells must pass ` + "`--yes`" + `.

Examples:

  rmesh device config delete --from cloud:mine/eu-868
  rmesh device config delete --from cloud:home/template/eu-868-default --yes
  rmesh device config delete --from cloud:home/a46229d5-0f27-44e8-9350-5737260cfb25`,
	RunE: runDeviceConfigDelete,
}

func runDeviceConfigDelete(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(deleteFlags.from) == "" {
		return errors.New("--from is required (e.g. cloud:mine/eu-868)")
	}
	src, err := deviceconfigs.ParseSource(deleteFlags.from)
	if err != nil {
		return err
	}
	if src.Kind != deviceconfigs.SourceCloud {
		return fmt.Errorf("--from must be a cloud source (got %q); delete only applies to saved cloud configs", deleteFlags.from)
	}

	_, client, err := requireSession()
	if err != nil {
		return err
	}
	c := concrete(client)
	if c == nil {
		return errors.New("delete requires the concrete *apiclient.Client")
	}

	// Personal vs template dispatch. Personal rows live under /me
	// (D-219) and have no network ref.
	isPersonal := src.Owner == deviceconfigs.CloudOwnerMine
	var (
		netID string
		cfgID string
	)
	if isPersonal {
		cfgID, err = c.ResolveDeviceConfigID(ctx, "", src.Label, apiclient.OwnerMine)
		if err != nil {
			return err
		}
	} else {
		netID, err = resolveCloudNetworkID(ctx, client, src.Network)
		if err != nil {
			return err
		}
		cfgID, err = c.ResolveDeviceConfigID(ctx, netID, src.Label, ownerHintFromCloudSource(src))
		if err != nil {
			return err
		}
	}

	path := src.String()
	warn := "This permanently deletes the saved cloud config."
	if isPersonal {
		if detail, gerr := c.GetMyDeviceConfig(ctx, cfgID); gerr == nil {
			_ = detail
			warn = "This permanently deletes your personal cloud config."
		}
	} else if detail, gerr := c.GetDeviceConfig(ctx, netID, cfgID, false); gerr == nil {
		warn = "This permanently deletes a network template for everyone with access to this network."
		if detail.IsFeatured {
			warn += " It is currently featured on the network overview."
		}
	}
	if err := confirmDeviceConfigDelete(cmd, warn, path, src.Label); err != nil {
		return err
	}

	if isPersonal {
		if err := c.DeleteMyDeviceConfig(ctx, cfgID); err != nil {
			return err
		}
	} else {
		if err := c.DeleteDeviceConfig(ctx, netID, cfgID); err != nil {
			return err
		}
	}
	return cliui.New(cmd.OutOrStdout()).DeletedCloudConfig(src.Label, path, cfgID)
}

func ownerHintFromCloudSource(src deviceconfigs.Source) apiclient.OwnerHint {
	switch src.Owner {
	case deviceconfigs.CloudOwnerMine:
		return apiclient.OwnerMine
	case deviceconfigs.CloudOwnerTemplate:
		return apiclient.OwnerTemplate
	default:
		return apiclient.OwnerEither
	}
}

func confirmDeviceConfigDelete(cmd *cobra.Command, warn, path, label string) error {
	if deleteFlags.yes {
		return nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return errors.New("refusing to delete without --yes (stdin is not a terminal)")
	}
	ui := cliui.New(cmd.OutOrStdout())
	if err := ui.Warn(warn); err != nil {
		return err
	}
	if err := ui.Details(
		cliui.Field{Key: "path", Value: path},
		cliui.Field{Key: "label", Value: label},
	); err != nil {
		return err
	}
	if _, err := fmt.Fprint(cmd.OutOrStdout(), "Type yes to confirm: "); err != nil {
		return err
	}
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return fmt.Errorf("read confirmation: %w", err)
	}
	if strings.TrimSpace(strings.ToLower(line)) != "yes" {
		return errors.New("delete cancelled")
	}
	return nil
}

func init() {
	deviceConfigCmd.AddCommand(deviceConfigDeleteCmd)
	f := deviceConfigDeleteCmd.Flags()
	f.StringVar(&deleteFlags.from, "from", "", "Cloud source: cloud:mine/<label> or cloud:<network>/<label> (or cloud:<network>/template/<label>)")
	f.BoolVar(&deleteFlags.yes, "yes", false, "Skip confirmation prompt (required when stdin is not a terminal)")
	_ = deviceConfigDeleteCmd.MarkFlagRequired("from")
	deviceConfigDeleteCmd.SilenceUsage = true
}
