package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion script",
	Long: `Write a completion script to stdout.

Add tab completion in zsh — put this in ~/.zshrc:

  eval "$(rmesh completion zsh)"

Re-run after upgrading rmesh so new subcommands and dynamic completers
(network list for "network use", --network, --fields) are picked up.

Dynamic completion requires an active session (rmesh auth login).

Other shells: bash, fish, powershell (pass as the first argument).`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		shell := "zsh"
		if len(args) == 1 {
			shell = args[0]
		}
		switch shell {
		case "bash":
			return rootCmd.GenBashCompletion(cmd.OutOrStdout())
		case "zsh":
			return rootCmd.GenZshCompletion(cmd.OutOrStdout())
		case "fish":
			return rootCmd.GenFishCompletion(cmd.OutOrStdout(), true)
		case "powershell":
			return rootCmd.GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
		default:
			return fmt.Errorf("unknown shell %q (try bash, zsh, fish, powershell)", shell)
		}
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
