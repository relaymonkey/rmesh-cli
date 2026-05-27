package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/relaymonkey/relaymesh-edge/internal/cliui"
	"github.com/relaymonkey/relaymesh-edge/internal/session"
)

func runAuthStatus(cmd *cobra.Command) error {
	ui := cliui.New(cmd.OutOrStdout())
	path, err := session.FilePath()
	if err != nil {
		return err
	}

	saved, err := session.Load()
	if err != nil {
		if errors.Is(err, session.ErrNotLoggedIn) {
			_ = ui.Fail("Not logged in", cliui.Field{Key: "session", Value: path + " (missing)"})
			_ = ui.Hint("rmesh auth login")
			return fmt.Errorf("not logged in")
		}
		return err
	}

	client := apiclientFromSession(saved)
	me, err := client.GetMe(context.Background())
	if err != nil {
		_ = ui.Fail("Session invalid",
			cliui.Field{Key: "session", Value: path},
			cliui.Field{Key: "api", Value: saved.APIURL},
			cliui.Field{Key: "email", Value: saved.Email},
			cliui.Field{Key: "saved", Value: saved.SavedAt.Format("2006-01-02 15:04:05 UTC")},
			cliui.Field{Key: "error", Value: err.Error()},
		)
		_ = ui.Hint("rmesh auth login")
		return err
	}

	email := me.Email
	if email == "" {
		email = saved.Email
	}

	return ui.Status("Session · logged in",
		cliui.Field{Key: "email", Value: email},
		cliui.Field{Key: "user id", Value: me.ID},
		cliui.Field{Key: "api", Value: saved.APIURL},
		cliui.Field{Key: "auth", Value: saved.AuthURL},
		cliui.Field{Key: "session", Value: path},
		cliui.Field{Key: "saved", Value: saved.SavedAt.Format("2006-01-02 15:04:05 UTC")},
	)
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

func init() {
	authStatusCmd.SilenceUsage = true
}
