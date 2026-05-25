package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	configPath   string
	resetCadence bool
	verbose      bool
)

// Execute runs the rmesh CLI.
func Execute() error {
	return rootCmd.Execute()
}

var rootCmd = &cobra.Command{
	Use:   "rmesh",
	Short: "RelayMesh edge agent — Phone API to MQTT gateway",
	Long:  "rmesh connects to a local Meshtastic node and forwards observed packets to RelayMesh over MQTT.",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configPath, "config", configDefaultPath(), "path to config.yaml")

	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(observeCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(pairCmd)

	runCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "log each MQTT publish (topic, source, portnum)")
	runCmd.Flags().BoolVar(&resetCadence, "reset-cadence", false, "force synthetic re-emission on next tick")
	observeCmd.Flags().BoolVar(&resetCadence, "reset-cadence", false, "force synthetic re-emission on next tick")
}

func configDefaultPath() string {
	if p := os.Getenv("RMESH_CONFIG"); p != "" {
		return p
	}
	return "/etc/rmesh/config.yaml"
}

func loadConfig() (cfgPath string, err error) {
	if configPath == "" {
		return "", fmt.Errorf("config path is empty")
	}
	if _, err := os.Stat(configPath); err != nil {
		return configPath, fmt.Errorf("config %s: %w", configPath, err)
	}
	return configPath, nil
}
