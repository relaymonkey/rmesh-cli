package cmd

import (
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with RelayMesh",
	Long:  "Sign in for cloud commands such as networks, traffic, and pairing. Set RMESH_API_URL and RMESH_AUTH_URL for local testing.",
}

func init() {
	rootCmd.AddCommand(authCmd)

	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authLogoutCmd)
	authCmd.AddCommand(authStatusCmd)
}
