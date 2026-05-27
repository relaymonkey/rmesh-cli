package cmd

import "github.com/spf13/cobra"

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Local node ingest — radio to RelayMesh cloud",
	Long:  "Connect to a local mesh radio and forward observed packets to RelayMesh.",
}

func init() {
	rootCmd.AddCommand(agentCmd)

	agentCmd.AddCommand(runCmd)
	agentCmd.AddCommand(observeCmd)
	agentCmd.AddCommand(doctorCmd)
	agentCmd.AddCommand(pairCmd)

	runCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "log each MQTT publish (topic, source, portnum)")
	runCmd.Flags().BoolVar(&resetCadence, "reset-cadence", false, "force synthetic re-emission on next tick")
	observeCmd.Flags().BoolVar(&resetCadence, "reset-cadence", false, "force synthetic re-emission on next tick")
}
