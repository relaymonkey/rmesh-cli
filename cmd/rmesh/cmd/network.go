package cmd

import "github.com/spf13/cobra"

var networkOutput string

var networkCmd = &cobra.Command{
	Use:     "network",
	Aliases: []string{"networks"},
	Short:   "RelayMesh networks (cloud API)",
	Long:    "List and manage networks via the RelayMesh REST API. Requires `rmesh auth login`.",
}

func init() {
	rootCmd.AddCommand(networkCmd)
	networkCmd.AddCommand(networkListCmd)

	networkCmd.PersistentFlags().StringVarP(
		&networkOutput,
		"output", "o", "table",
		"Output format: table, json, yaml, id",
	)
	networkListCmd.SilenceUsage = true
}
