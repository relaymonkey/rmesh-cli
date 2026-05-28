package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/relaymonkey/relaymesh-edge/internal/clidefault"
	"github.com/relaymonkey/relaymesh-edge/internal/cliconfig"
	"github.com/relaymonkey/relaymesh-edge/internal/cliui"
)

var (
	configPath   string
	resetCadence bool
	verbose      bool
	debug        bool
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

	err := rootCmd.ExecuteContext(ctx)
	if err != nil {
		renderRootError(os.Stderr, err)
	}
	return err
}

// renderRootError is the single sink for command-return errors. It
// replaces cobra's default `Error: <chain>` line with a cliui ✗
// headline (per D-217) and, for well-known sentinels, attaches an
// actionable hint. Keeping this centralized means individual commands
// don't have to remember to print errors through cliui themselves —
// `return err` from any RunE is enough.
func renderRootError(w *os.File, err error) {
	ui := cliui.New(w)
	switch {
	case errors.Is(err, clidefault.ErrNotSet):
		_ = ui.Fail("No default network set")
		_ = ui.Hint("run: rmesh network use <id> (see `rmesh network list`)")
	default:
		_ = ui.Fail(err.Error())
	}
}

var rootCmd = &cobra.Command{
	Use:   "rmesh",
	Short: "RelayMesh CLI",
	Long:  "rmesh is the RelayMesh CLI — auth, networks, traffic and config against the cloud API. Use `rmesh agent` for local radio ingest.",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configPath, "config", cliconfig.AgentConfigPath(), "path to config.yaml")
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "enable debug logging (transport, BLE, internal state)")
	// Suppress cobra's default `Error: <chain>` line and the auto
	// usage dump on RunE failures. `renderRootError` is the single
	// stderr sink for command-return errors so every failure goes
	// through the cliui framework (D-217). Subcommands no longer
	// need their own `SilenceUsage = true`.
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true
	cobra.OnInitialize(applyDebugLogging)
}

func applyDebugLogging() {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})))
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

// SetVersion sets the version of the CLI.
func SetVersion(v string) {
	rootCmd.Version = v
}
