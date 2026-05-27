package cmd

import (
	"context"
	"log/slog"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/relaymonkey/relaymesh-edge/internal/agent"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the agent and publish to RelayMesh",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, cfg, err := loadAgentConfig(cmd)
		if err != nil {
			return err
		}

		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()

		slog.Info("starting rmesh agent run", "config", path, "agent_id", cfg.AgentID)
		return shutdownErr(agent.Run(ctx, cfg, agent.Options{
			Observe:      false,
			Verbose:      verbose,
			ResetCadence: resetCadence,
		}))
	},
}

func init() {
	runCmd.SilenceUsage = true
}
