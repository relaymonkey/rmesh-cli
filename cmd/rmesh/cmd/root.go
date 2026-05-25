package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/relaymonkey/relaymesh-edge/internal/cliconfig"
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
	Short: "RelayMesh CLI",
	Long:  "rmesh is the RelayMesh CLI — auth, networks, traffic, and config against the cloud API. Use `rmesh agent` for local Meshtastic node ingest (Phone API → MQTT).",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configPath, "config", cliconfig.AgentConfigPath(), "path to config.yaml")
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
