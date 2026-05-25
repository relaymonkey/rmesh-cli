package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/relaymonkey/relaymesh-edge/internal/apiclient"
	"github.com/relaymonkey/relaymesh-edge/internal/clidefault"
)

// resolveNetworkID returns the network UUID from --network or the saved default.
func resolveNetworkID(cmd *cobra.Command, client *apiclient.Client, flagValue string) (string, error) {
	ref := strings.TrimSpace(flagValue)
	if ref != "" {
		n, err := client.ResolveNetworkRef(context.Background(), ref)
		if err != nil {
			return "", err
		}
		return n.ID, nil
	}
	def, err := clidefault.Load()
	if err != nil {
		if errors.Is(err, clidefault.ErrNotSet) {
			return "", fmt.Errorf("no network specified — use --network or: rmesh network use <id|slug>")
		}
		return "", err
	}
	return def.NetworkID, nil
}
