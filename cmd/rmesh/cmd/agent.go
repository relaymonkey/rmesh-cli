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

	// Per-verb section binding. Each verb advertises only the flags it
	// actually reads — see `D-220`.
	//
	// `pair` deliberately gets no agent-config flags: it talks to the
	// cloud API, not the radio.
	bindConfigSections(runCmd,
		sectionIdentity, sectionTransport, sectionMQTT, sectionForward, sectionSynthesise, sectionLabels, sectionMetrics)
	bindConfigSections(observeCmd,
		sectionIdentity, sectionTransport, sectionForward, sectionSynthesise, sectionLabels, sectionMetrics)
	bindConfigSections(doctorCmd,
		sectionIdentity, sectionTransport, sectionMQTT, sectionSynthesise, sectionLabels, sectionMetrics)

	// `-v / --verbose` is scoped to `agent *` so it doesn't collide
	// with the per-section `-v` flags on `device-config set/edit/copy`
	// (which print the exact SetConfig payload). Single flag, max
	// output: debug-level slog (transport, BLE, internal state) plus
	// per-publish MQTT lines on `agent run`.
	agentCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose logging (debug level: MQTT publishes, transport, BLE, internal state)")
	runCmd.Flags().BoolVar(&resetCadence, "reset-cadence", false, "force synthetic re-emission on next tick")
	observeCmd.Flags().BoolVar(&resetCadence, "reset-cadence", false, "force synthetic re-emission on next tick")
}
