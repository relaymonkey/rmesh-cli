package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/relaymonkey/relaymesh-edge/internal/session"
)

func runAuthStatus(cmd *cobra.Command) error {
	path, err := session.FilePath()
	if err != nil {
		return err
	}

	saved, err := session.Load()
	if err != nil {
		if errors.Is(err, session.ErrNotLoggedIn) {
			fmt.Fprintf(cmd.OutOrStdout(), "RelayMesh CLI: not logged in\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  session file: %s (missing)\n", path)
			fmt.Fprintln(cmd.OutOrStdout(), "  run: rmesh auth login")
			return fmt.Errorf("not logged in")
		}
		return err
	}

	client := apiclientFromSession(saved)
	me, err := client.GetMe(context.Background())
	if err != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "RelayMesh CLI: session invalid\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  session file: %s\n", path)
		fmt.Fprintf(cmd.OutOrStdout(), "  api:          %s\n", saved.APIURL)
		fmt.Fprintf(cmd.OutOrStdout(), "  saved email:  %s\n", saved.Email)
		fmt.Fprintf(cmd.OutOrStdout(), "  saved at:     %s\n", saved.SavedAt.Format("2006-01-02 15:04:05 UTC"))
		fmt.Fprintf(cmd.OutOrStdout(), "  error:        %v\n", err)
		fmt.Fprintln(cmd.OutOrStdout(), "  run: rmesh auth login")
		return err
	}

	email := me.Email
	if email == "" {
		email = saved.Email
	}

	fmt.Fprintf(cmd.OutOrStdout(), "RelayMesh CLI: logged in to %s as %s\n", saved.APIURL, email)
	fmt.Fprintf(cmd.OutOrStdout(), "  session file: %s\n", path)
	fmt.Fprintf(cmd.OutOrStdout(), "  auth:         %s\n", saved.AuthURL)
	fmt.Fprintf(cmd.OutOrStdout(), "  user id:      %s\n", me.ID)
	fmt.Fprintf(cmd.OutOrStdout(), "  saved at:     %s\n", saved.SavedAt.Format("2006-01-02 15:04:05 UTC"))
	return nil
}

var authStatusCmd = &cobra.Command{
	Use:     "status",
	Aliases: []string{"st"},
	Short:   "Show login state and verify the session against the API",
	Long:    "Like `gh auth status`: checks the saved session file and calls GET /api/v1/me.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAuthStatus(cmd)
	},
}

var authWhoamiCmd = &cobra.Command{
	Use:     "whoami",
	Aliases: []string{"me"},
	Short:   "Print the current RelayMesh user (alias for auth status)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAuthStatus(cmd)
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show CLI login status",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAuthStatus(cmd)
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
	authCmd.AddCommand(authStatusCmd)
	authStatusCmd.SilenceUsage = true
	authWhoamiCmd.SilenceUsage = true
	statusCmd.SilenceUsage = true
}
