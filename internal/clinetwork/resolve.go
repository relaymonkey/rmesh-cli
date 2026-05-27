package clinetwork

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/relaymonkey/relaymesh-edge/internal/apiclient"
	"github.com/relaymonkey/relaymesh-edge/internal/clidefault"
)

// ResolveID returns the network UUID from an explicit ref or the saved default.
func ResolveID(ctx context.Context, client apiclient.CloudClient, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref != "" {
		n, err := client.ResolveNetworkRef(ctx, ref)
		if err != nil {
			return "", err
		}
		return n.ID, nil
	}
	def, err := clidefault.Load()
	if err != nil {
		if errors.Is(err, clidefault.ErrNotSet) {
			return "", fmt.Errorf("no network specified — use --network or: rmesh network use <id>")
		}
		return "", err
	}
	return def.NetworkID, nil
}
