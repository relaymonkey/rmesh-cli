package cmd

import (
	"github.com/spf13/cobra"

	"github.com/relaymonkey/relaymesh-edge/internal/config"
)

// loadAgentConfig loads config.yaml, applies `--set` dotted-path overrides,
// then applies the section-scoped typed overrides registered for cmd, then
// finalizes (defaults + validation).
//
// Order: file → --set (general) → typed flags (specific, wins on conflict) →
// Finalize. Typed flags win because they're the more focused expression; --set
// is the long-tail escape hatch.
func loadAgentConfig(cmd *cobra.Command) (string, config.Config, error) {
	path, err := loadConfig()
	if err != nil {
		return "", config.Config{}, err
	}
	cfg, err := config.LoadRaw(path)
	if err != nil {
		return "", config.Config{}, err
	}

	if cmd.Flags().Changed("set") {
		pairs, _ := cmd.Flags().GetStringArray("set")
		if err := config.ApplySetPaths(&cfg, pairs); err != nil {
			return "", config.Config{}, err
		}
	}

	overrides, err := overridesFromSections(cmd)
	if err != nil {
		return "", config.Config{}, err
	}
	overrides.Apply(&cfg)

	if err := cfg.Finalize(); err != nil {
		return "", config.Config{}, err
	}
	return path, cfg, nil
}

func overridesFromSections(cmd *cobra.Command) (config.Overrides, error) {
	var o config.Overrides
	for _, s := range cmdSections[cmd] {
		if err := s.read(cmd.Flags(), &o); err != nil {
			return config.Overrides{}, err
		}
	}
	return o, nil
}
