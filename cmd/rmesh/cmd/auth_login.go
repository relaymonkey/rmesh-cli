package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/relaymonkey/relaymesh-edge/internal/cliconfig"
	"github.com/relaymonkey/relaymesh-edge/internal/cliui"
	"github.com/relaymonkey/relaymesh-edge/internal/session"
)

var (
	authLoginEmail    string
	authLoginPassword string
)

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Sign in to RelayMesh and save a session",
	RunE: func(cmd *cobra.Command, args []string) error {
		ui := cliui.New(cmd.OutOrStdout())
		email := authLoginEmail
		password := authLoginPassword

		if email == "" {
			if err := ui.Prompt("Email: "); err != nil {
				return err
			}
			var err error
			email, err = readLine()
			if err != nil {
				return err
			}
		}
		if password == "" {
			if err := ui.Prompt("Password: "); err != nil {
				return err
			}
			b, err := term.ReadPassword(int(syscall.Stdin))
			fmt.Fprintln(cmd.OutOrStdout())
			if err != nil {
				return fmt.Errorf("read password: %w", err)
			}
			password = string(b)
		}

		if err := ui.Details(
			cliui.Field{Key: "api", Value: cliconfig.APIURL()},
			cliui.Field{Key: "auth", Value: cliconfig.AuthURL()},
		); err != nil {
			return err
		}

		saved, err := session.Login(context.Background(), email, password)
		if err != nil {
			return err
		}
		if err := ui.Success("Logged in · " + saved.Email); err != nil {
			return err
		}
		return ui.Hint("rmesh auth status")
	},
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove the saved RelayMesh session",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := session.Clear(); err != nil {
			return err
		}
		return cliui.New(cmd.OutOrStdout()).Success("Logged out")
	},
}

func init() {
	authLoginCmd.Flags().StringVar(&authLoginEmail, "email", "", "account email (prompted when omitted)")
	authLoginCmd.Flags().StringVar(&authLoginPassword, "password", "", "password (prompted when omitted; avoid on shared shells)")
	authLoginCmd.SilenceUsage = true
	authLogoutCmd.SilenceUsage = true
}

func readLine() (string, error) {
	sc := bufio.NewScanner(os.Stdin)
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return "", err
		}
		return "", fmt.Errorf("empty input")
	}
	return sc.Text(), nil
}
