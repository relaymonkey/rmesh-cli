package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/relaymonkey/relaymesh-edge/internal/cliconfig"
)

var (
	configPath   string
	resetCadence bool
	verbose      bool
)

// Execute runs the rmesh CLI.
//
// We install a process-wide SIGINT / SIGTERM handler at the root and
// pass its context to cobra via `ExecuteContext`. Every sub-command's
// `cmd.Context()` then returns that cancellable context, so a single
// Ctrl-C aborts in-flight blocking operations (serial reads, BLE
// round-trips, HTTP polls) instead of the process appearing to hang.
//
// On the second SIGINT we hard-exit — covers the case where the
// in-flight goroutine sits in a CGO call that doesn't honour ctx
// cancellation (e.g. some serial libraries block in a syscall that
// only returns on actual data). One Ctrl-C asks nicely; two Ctrl-Cs
// kill the process regardless.
func Execute() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		// First signal already cancelled `ctx`. Wait for a second
		// one and force-exit if the command hasn't already returned.
		hard := make(chan os.Signal, 1)
		signal.Notify(hard, os.Interrupt, syscall.SIGTERM)
		<-hard
		fmt.Fprintln(os.Stderr, "\nrmesh: forcing exit on second interrupt")
		os.Exit(130)
	}()

	return rootCmd.ExecuteContext(ctx)
}

var rootCmd = &cobra.Command{
	Use:   "rmesh",
	Short: "RelayMesh CLI",
	Long:  "rmesh is the RelayMesh CLI — auth, networks, traffic, and config against the cloud API. Use `rmesh agent` for local Meshtastic node ingest (Phone API → MQTT).",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configPath, "config", cliconfig.AgentConfigPath(), "path to config.yaml")
}

func loadConfig() (cfgPath string, err error) {
	if configPath == "" {
		return "", fmt.Errorf("config path is empty")
	}
	if _, err := os.Stat(configPath); err != nil {
		return configPath, fmt.Errorf("config %s: %w", configPath, err)
	}
	return configPath, nil
}
