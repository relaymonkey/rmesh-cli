package cmd

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/relaymonkey/relaymesh-edge/internal/agent"
	"github.com/relaymonkey/relaymesh-edge/internal/config"
)

var observeCmd = &cobra.Command{
	Use:   "observe",
	Short: "Dry-run: Phone API only, print JSONL to stdout (no MQTT)",
	Long:  "Validate transport, NodeDB synthesis, and envelope shape before enabling production publish.",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := loadConfig()
		if err != nil {
			return err
		}
		cfg, err := config.Load(path)
		if err != nil {
			return err
		}

		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()

		slog.Info("starting rmesh agent observe", "config", path, "agent_id", cfg.AgentID)
		return shutdownErr(agent.Run(ctx, cfg, agent.Options{
			Observe:      true,
			ResetCadence: resetCadence,
			ObserveOut:   os.Stdout,
		}))
	},
}

func init() {
	observeCmd.SilenceUsage = true
}
