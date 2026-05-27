package clicomplete

// Package clicomplete provides a small framework for shell tab completion in rmesh.
//
// Register static choices:
//
//	clicomplete.RegisterFlag(cmd, "output", clicomplete.StaticStrings("table", "json"))
//
// Register API-backed positional args:
//
//	clicomplete.RegisterArgs(myCmd, clicomplete.NetworksProvider)
//
// Add a new dynamic source by implementing Provider and wiring it from
// cmd/rmesh/cmd/zz_complete_register.go (must run after command flags exist).

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/relaymonkey/relaymesh-edge/internal/apiclient"
	"github.com/relaymonkey/relaymesh-edge/internal/session"
)

// Item is one completion candidate with an optional description (shown after \t in zsh).
type Item struct {
	Value       string
	Description string
}

// Provider loads completion candidates. Implementations should filter by toComplete.
type Provider func(ctx context.Context, toComplete string) ([]Item, error)

const networkCacheTTL = 30 * time.Second

var (
	networkCacheMu sync.Mutex
	networkCache   []apiclient.Network
	networkCacheAt time.Time
)

// RegisterArgs wires dynamic positional completion onto cmd.
func RegisterArgs(cmd *cobra.Command, provider Provider) {
	cmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return runProvider(provider, toComplete)
	}
}

// RegisterFlag wires dynamic flag-value completion (e.g. --network).
func RegisterFlag(cmd *cobra.Command, flagName string, provider Provider) {
	RegisterFlagDirective(cmd, flagName, func(ctx context.Context, toComplete string) ([]Item, cobra.ShellCompDirective, error) {
		items, err := provider(ctx, toComplete)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError, err
		}
		return items, cobra.ShellCompDirectiveNoFileComp, nil
	})
}

// DirectiveProvider loads completion candidates and chooses whether the
// shell may fall through to file-path completion.
type DirectiveProvider func(ctx context.Context, toComplete string) ([]Item, cobra.ShellCompDirective, error)

// RegisterFlagDirective wires flag-value completion with an explicit
// shell directive (e.g. allow file completion after `file:`).
func RegisterFlagDirective(cmd *cobra.Command, flagName string, provider DirectiveProvider) {
	if err := cmd.RegisterFlagCompletionFunc(flagName, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		items, directive, err := provider(context.Background(), toComplete)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		return formatItems(items), directive
	}); err != nil {
		panic(fmt.Sprintf("clicomplete: register %q on %q: %v", flagName, cmd.Name(), err))
	}
}

func runProvider(provider Provider, toComplete string) ([]string, cobra.ShellCompDirective) {
	items, err := provider(context.Background(), toComplete)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	return formatItems(items), cobra.ShellCompDirectiveNoFileComp
}

func formatItems(items []Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		if it.Description != "" {
			out[i] = it.Value + "\t" + it.Description
		} else {
			out[i] = it.Value
		}
	}
	return out
}

// NetworksProvider completes network UUIDs.
func NetworksProvider(ctx context.Context, toComplete string) ([]Item, error) {
	nets, err := cachedNetworks(ctx)
	if err != nil {
		return nil, err
	}
	prefix := strings.ToLower(toComplete)
	var items []Item
	for _, n := range nets {
		if n.ID == "" {
			continue
		}
		if prefix != "" && !strings.HasPrefix(strings.ToLower(n.ID), prefix) {
			continue
		}
		items = append(items, Item{
			Value:       n.ID,
			Description: fmt.Sprintf("%s (%s)", n.Name, n.ID),
		})
	}
	return items, nil
}

func cachedNetworks(ctx context.Context) ([]apiclient.Network, error) {
	networkCacheMu.Lock()
	if time.Since(networkCacheAt) < networkCacheTTL && len(networkCache) > 0 {
		cached := append([]apiclient.Network(nil), networkCache...)
		networkCacheMu.Unlock()
		return cached, nil
	}
	networkCacheMu.Unlock()

	saved, err := session.Load()
	if err != nil {
		return nil, err
	}
	client := apiclient.New(saved)
	list, err := client.ListNetworks(ctx)
	if err != nil {
		return nil, err
	}

	networkCacheMu.Lock()
	networkCache = list.Items
	networkCacheAt = time.Now()
	cached := append([]apiclient.Network(nil), networkCache...)
	networkCacheMu.Unlock()
	return cached, nil
}

// StaticStrings completes fixed string choices (output formats, etc.).
func StaticStrings(values ...string) Provider {
	set := append([]string(nil), values...)
	return func(_ context.Context, toComplete string) ([]Item, error) {
		prefix := strings.ToLower(toComplete)
		var items []Item
		for _, v := range set {
			if prefix != "" && !strings.HasPrefix(strings.ToLower(v), prefix) {
				continue
			}
			items = append(items, Item{Value: v})
		}
		return items, nil
	}
}
