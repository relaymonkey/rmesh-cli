package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

var pairCmd = &cobra.Command{
	Use:   "pair",
	Short: "Pair this agent with a RelayMesh network (stub)",
	Long:  "Pairing mints scoped MQTT credentials via the RelayMesh API. Requires `rmesh auth login`. Full handshake lands in a follow-up release.",
	RunE: func(cmd *cobra.Command, args []string) error {
		networkID, _ := cmd.Flags().GetString("network")

		_, client, err := requireSession()
		if err != nil {
			return err
		}
		me, err := client.GetMe(context.Background())
		if err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "signed in as %s\n", me.Email)
		fmt.Fprintln(cmd.OutOrStdout(), "")
		fmt.Fprintln(cmd.OutOrStdout(), "Cloud pairing API is not wired yet.")
		fmt.Fprintln(cmd.OutOrStdout(), "")
		fmt.Fprintln(cmd.OutOrStdout(), "Manual pairing for now:")
		fmt.Fprintln(cmd.OutOrStdout(), "  1. Open your network in RelayMesh → Credentials → Issue MQTT credential")
		fmt.Fprintln(cmd.OutOrStdout(), "  2. Copy broker_url, username, password, topic_prefix into config (rmesh config edit)")
		fmt.Fprintln(cmd.OutOrStdout(), "  3. Run: rmesh agent doctor && rmesh agent observe")
		fmt.Fprintln(cmd.OutOrStdout(), "  4. When satisfied: rmesh agent run")
		if networkID != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "\n(network flag reserved for future API pairing: %s)\n", networkID)
		}
		return nil
	},
}

func init() {
	pairCmd.Flags().String("network", "", "RelayMesh network UUID (future)")
	pairCmd.SilenceUsage = true
}
