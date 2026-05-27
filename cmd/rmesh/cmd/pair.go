package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/relaymonkey/relaymesh-edge/internal/cliui"
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

		ui := cliui.New(cmd.OutOrStdout())
		if err := ui.Status("Signed in · " + me.Email); err != nil {
			return err
		}
		if err := ui.Blank(); err != nil {
			return err
		}
		if err := ui.Warn("Cloud pairing API is not wired yet."); err != nil {
			return err
		}
		if err := ui.Blank(); err != nil {
			return err
		}
		if err := ui.Line("Manual pairing for now:"); err != nil {
			return err
		}
		if err := ui.Steps(
			"Open your network in RelayMesh → Credentials → Issue MQTT credential",
			"Copy broker_url, username, password, topic_prefix into config (rmesh config edit)",
			"Run: rmesh agent doctor && rmesh agent observe",
			"When satisfied: rmesh agent run",
		); err != nil {
			return err
		}
		if networkID != "" {
			if err := ui.Blank(); err != nil {
				return err
			}
			return ui.Note("network flag reserved for future API pairing: " + networkID)
		}
		return nil
	},
}

func init() {
	pairCmd.Flags().String("network", "", "RelayMesh network UUID (future)")
	pairCmd.SilenceUsage = true
}
