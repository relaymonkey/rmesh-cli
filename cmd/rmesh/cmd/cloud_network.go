package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/relaymonkey/relaymesh-edge/internal/apiclient"
	"github.com/relaymonkey/relaymesh-edge/internal/clinetwork"
)

// resolveNetworkID returns the network UUID from --network or the saved default.
func resolveNetworkID(_ *cobra.Command, client apiclient.CloudClient, flagValue string) (string, error) {
	return clinetwork.ResolveID(context.Background(), client, flagValue)
}
