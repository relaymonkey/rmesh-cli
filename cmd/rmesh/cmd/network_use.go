package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/relaymonkey/relaymesh-edge/internal/clidefault"
	"github.com/relaymonkey/relaymesh-edge/internal/cliui"
)

var networkUseCmd = &cobra.Command{
	Use:   "use NETWORK",
	Short: "Set the default network for commands that accept --network",
	Long:  "NETWORK may be a UUID, slug, short_id, or exact name from `rmesh network list`.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, client, err := requireSession()
		if err != nil {
			return err
		}
		n, err := client.ResolveNetworkRef(context.Background(), args[0])
		if err != nil {
			return err
		}
		if err := clidefault.Save(clidefault.Network{
			NetworkID: n.ID,
			Name:      n.Name,
			Slug:      n.Slug,
			ShortID:   n.ShortID,
		}); err != nil {
			return err
		}
		ui := cliui.New(cmd.OutOrStdout())
		return ui.Success("Default network · "+n.Name,
			cliui.Field{Key: "id", Value: n.ID},
			cliui.Field{Key: "slug", Value: n.Slug},
			cliui.Field{Key: "short_id", Value: n.ShortID},
		)
	},
}

var networkCurrentCmd = &cobra.Command{
	Use:     "current",
	Aliases: []string{"default", "show"},
	Short:   "Show the default network",
	RunE: func(cmd *cobra.Command, args []string) error {
		def, err := clidefault.Load()
		if err != nil {
			return err
		}
		ui := cliui.New(cmd.OutOrStdout())
		headline := "Default network · " + def.Name
		if def.Name == "" {
			headline = "Default network"
		}
		return ui.Status(headline,
			cliui.Field{Key: "id", Value: def.NetworkID},
			cliui.Field{Key: "slug", Value: def.Slug},
			cliui.Field{Key: "short_id", Value: def.ShortID},
			cliui.Field{Key: "set at", Value: def.SetAt.Format("2006-01-02 15:04:05 UTC")},
		)
	},
}

func init() {
	networkCmd.AddCommand(networkUseCmd)
	networkCmd.AddCommand(networkCurrentCmd)
	networkUseCmd.SilenceUsage = true
	networkCurrentCmd.SilenceUsage = true
}
