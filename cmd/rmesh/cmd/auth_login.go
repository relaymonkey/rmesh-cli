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
		email := authLoginEmail
		password := authLoginPassword

		if email == "" {
			fmt.Fprint(cmd.OutOrStdout(), "Email: ")
			var err error
			email, err = readLine()
			if err != nil {
				return err
			}
		}
		if password == "" {
			fmt.Fprint(cmd.OutOrStdout(), "Password: ")
			b, err := term.ReadPassword(int(syscall.Stdin))
			fmt.Fprintln(cmd.OutOrStdout())
			if err != nil {
				return fmt.Errorf("read password: %w", err)
			}
			password = string(b)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "API:  %s\n", cliconfig.APIURL())
		fmt.Fprintf(cmd.OutOrStdout(), "Auth: %s\n", cliconfig.AuthURL())

		saved, err := session.Login(context.Background(), email, password)
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "logged in as %s\n", saved.Email)
		fmt.Fprintln(cmd.OutOrStdout(), "run `rmesh auth status` to verify")
		return nil
	},
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove the saved RelayMesh session",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := session.Clear(); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "logged out")
		return nil
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
