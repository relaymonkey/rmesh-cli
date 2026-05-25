package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var pairCmd = &cobra.Command{
	Use:   "pair",
	Short: "Pair this agent with a RelayMesh network (stub)",
	Long:  "Pairing mints scoped MQTT credentials via the RelayMesh web UI. Full handshake lands in a follow-up release.",
	RunE: func(cmd *cobra.Command, args []string) error {
		networkID, _ := cmd.Flags().GetString("network")
		fmt.Fprintln(cmd.OutOrStdout(), "rmesh pair is not wired to the cloud API yet.")
		fmt.Fprintln(cmd.OutOrStdout(), "")
		fmt.Fprintln(cmd.OutOrStdout(), "Manual pairing for now:")
		fmt.Fprintln(cmd.OutOrStdout(), "  1. Open your network in RelayMesh → Credentials → Issue MQTT credential")
		fmt.Fprintln(cmd.OutOrStdout(), "  2. Copy broker_url, username, password, topic_prefix into /etc/rmesh/config.yaml")
		fmt.Fprintln(cmd.OutOrStdout(), "  3. Run: rmesh doctor && rmesh observe")
		fmt.Fprintln(cmd.OutOrStdout(), "  4. When satisfied: rmesh run")
		if networkID != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "\n(network flag reserved for future UI pairing: %s)\n", networkID)
		}
		return nil
	},
}

func init() {
	pairCmd.Flags().String("network", "", "RelayMesh network UUID (future)")
	pairCmd.SilenceUsage = true
}
