package cmd

import (
	"github.com/spf13/cobra"

	"github.com/relaymonkey/relaymesh-edge/internal/cliconfig"
	"github.com/relaymonkey/relaymesh-edge/internal/cliui"
)

var configEditShort bool

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage rmesh agent configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		if configEditShort {
			return runConfigEdit(cmd)
		}
		return cmd.Help()
	},
}

var configEditCmd = &cobra.Command{
	Use:     "edit",
	Aliases: []string{"e"},
	Short:   "Open config in $EDITOR",
	Long:    "Creates the config file from a minimal template when missing, then opens it in $EDITOR or $VISUAL (default: nano).",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runConfigEdit(cmd)
	},
}

func runConfigEdit(cmd *cobra.Command) error {
	path := configPath
	if path == "" {
		path = cliconfig.AgentConfigPath()
	}
	if err := cliconfig.EditAgentConfig(path); err != nil {
		return err
	}
	return cliui.New(cmd.OutOrStdout()).Success("Config saved", cliui.Field{Key: "path", Value: path})
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configEditCmd)

	configCmd.Flags().BoolVarP(&configEditShort, "edit", "e", false, "open config in $EDITOR")
	configEditCmd.SilenceUsage = true
}
