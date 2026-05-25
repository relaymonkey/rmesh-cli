package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/relaymonkey/relaymesh-edge/internal/clidefault"
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
		fmt.Fprintf(cmd.OutOrStdout(), "default network: %s (%s)\n", n.Name, n.ID)
		return nil
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
		fmt.Fprintf(cmd.OutOrStdout(), "default network: %s\n", def.Name)
		fmt.Fprintf(cmd.OutOrStdout(), "  id:        %s\n", def.NetworkID)
		if def.Slug != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "  slug:      %s\n", def.Slug)
		}
		if def.ShortID != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "  short_id:  %s\n", def.ShortID)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  set at:    %s\n", def.SetAt.Format("2006-01-02 15:04:05 UTC"))
		return nil
	},
}

func init() {
	networkCmd.AddCommand(networkUseCmd)
	networkCmd.AddCommand(networkCurrentCmd)
	networkUseCmd.SilenceUsage = true
	networkCurrentCmd.SilenceUsage = true
}
