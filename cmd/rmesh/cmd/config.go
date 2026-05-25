package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/relaymonkey/relaymesh-edge/internal/config"
)

var configEditShort bool

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage rmesh configuration",
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
		path = config.DefaultPath()
	}
	if err := config.EditInTerminal(path); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "edited %s\n", path)
	return nil
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configEditCmd)

	configCmd.Flags().BoolVarP(&configEditShort, "edit", "e", false, "open config in $EDITOR")
	configEditCmd.SilenceUsage = true
}
