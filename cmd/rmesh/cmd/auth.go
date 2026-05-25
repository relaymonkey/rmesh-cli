package cmd

import (
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with RelayMesh (Kratos session)",
	Long:  "Login stores a Kratos session for pair and other cloud commands. Set RMESH_API_URL and RMESH_AUTH_URL for local testing.",
}

func init() {
	rootCmd.AddCommand(authCmd)

	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authLogoutCmd)
	authCmd.AddCommand(authStatusCmd)
	authCmd.AddCommand(authWhoamiCmd)
}
