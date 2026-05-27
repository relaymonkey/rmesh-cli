package clicomplete

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/relaymonkey/relaymesh-edge/internal/apiclient"
	"github.com/relaymonkey/relaymesh-edge/internal/cliconfig"
	"github.com/relaymonkey/relaymesh-edge/internal/config"
	"github.com/relaymonkey/relaymesh-edge/internal/deviceconfigs"
	"github.com/relaymonkey/relaymesh-edge/internal/session"
)

// ConfigEndpointKind selects which `--from` / `--to` tokens are valid
// for a given flag on `rmesh device config` (D-209).
type ConfigEndpointKind int

const (
	// ConfigEndpointSource completes read-side sources: device, file,
	// cloud.
	ConfigEndpointSource ConfigEndpointKind = iota
	// ConfigEndpointDestGet completes `get --to`: stdout or a file path.
	// (Deprecated alias surface — D-216.)
	ConfigEndpointDestGet
	// ConfigEndpointDestSet completes `set --to`: device or cloud upload.
	// (Deprecated alias surface — D-216.)
	ConfigEndpointDestSet
	// ConfigEndpointDestCopy completes `copy --to`: every destination
	// (device, cloud, file, stdout) per D-216.
	ConfigEndpointDestCopy
)

const deviceConfigCacheTTL = 30 * time.Second

var (
	deviceConfigCacheMu sync.Mutex
	deviceConfigCache   = map[string]deviceConfigCacheEntry{}
)

type deviceConfigCacheEntry struct {
	items []Item
	at    time.Time
}

// ConfigEndpointProvider completes the D-209 `--from` / `--to` grammar.
func ConfigEndpointProvider(kind ConfigEndpointKind) DirectiveProvider {
	return func(ctx context.Context, toComplete string) ([]Item, cobra.ShellCompDirective, error) {
		tc := strings.TrimSpace(toComplete)

		// Scheme prefixes take precedence over the path heuristic —
		// `cloud:UUID/` contains a `/` but is not a file path.
		switch {
		case strings.HasPrefix(tc, "cloud:"):
			items, err := completeCloudEndpoint(ctx, tc)
			return items, cobra.ShellCompDirectiveNoFileComp, err
		case strings.HasPrefix(tc, "file:"):
			pathPart := strings.TrimPrefix(tc, "file:")
			if pathPart == "" {
				return []Item{{Value: "file:", Description: "local JSON/YAML file"}},
					cobra.ShellCompDirectiveDefault, nil
			}
			return nil, cobra.ShellCompDirectiveDefault, nil
		case strings.HasPrefix(tc, "device:"):
			items, err := completeDeviceURL(tc)
			return items, cobra.ShellCompDirectiveNoFileComp, err
		}

		if looksLikeConfigPath(tc) {
			return nil, cobra.ShellCompDirectiveDefault, nil
		}

		items := topLevelConfigEndpoints(kind, tc)
		return items, cobra.ShellCompDirectiveNoFileComp, nil
	}
}

// SectionKeysProvider completes `--section` keys for config verbs.
func SectionKeysProvider(_ context.Context, toComplete string) ([]Item, error) {
	prefix := strings.ToLower(toComplete)
	var items []Item
	for _, k := range append(deviceconfigs.AllSubmessageKeys(), "channels") {
		if prefix != "" && !strings.HasPrefix(k, prefix) {
			continue
		}
		items = append(items, Item{Value: k})
	}
	return items, nil
}

func topLevelConfigEndpoints(kind ConfigEndpointKind, toComplete string) []Item {
	var candidates []Item
	add := func(value, desc string) {
		if !prefixMatch(toComplete, value) {
			return
		}
		candidates = append(candidates, Item{Value: value, Description: desc})
	}

	switch kind {
	case ConfigEndpointSource:
		add("device", "live Meshtastic device")
		add("file:", "local JSON/YAML file")
		add("cloud:", "saved cloud config")
	case ConfigEndpointDestGet:
		add("-", "stdout")
	case ConfigEndpointDestSet:
		add("device", "apply to live device")
		add("cloud:", "upload to cloud")
	case ConfigEndpointDestCopy:
		add("device", "apply to live device")
		add("cloud:", "upload to cloud")
		add("file:", "local JSON/YAML file")
		add("-", "stdout")
	}
	return candidates
}

func completeCloudEndpoint(ctx context.Context, toComplete string) ([]Item, error) {
	rest := strings.TrimPrefix(toComplete, "cloud:")
	if rest == "" {
		return prefixCloudNetworks(ctx, toComplete, "")
	}
	if slash := strings.Index(rest, "/"); slash >= 0 {
		netRef := rest[:slash]
		labelPrefix := rest[slash+1:]
		return completeCloudLabels(ctx, netRef, labelPrefix, toComplete[:len("cloud:")+slash+1])
	}
	return prefixCloudNetworks(ctx, toComplete, rest)
}

func prefixCloudNetworks(ctx context.Context, toComplete, netPrefix string) ([]Item, error) {
	nets, err := cachedNetworks(ctx)
	if err != nil {
		return nil, err
	}
	prefix := strings.ToLower(toComplete)
	var items []Item
	seen := map[string]struct{}{}
	for _, n := range nets {
		desc := fmt.Sprintf("%s (%s)", n.Name, n.ID)
		for _, ref := range []string{n.Slug, n.ShortID, n.ID, n.Name} {
			if ref == "" {
				continue
			}
			if netPrefix != "" && !strings.HasPrefix(strings.ToLower(ref), strings.ToLower(netPrefix)) {
				continue
			}
			value := "cloud:" + ref + "/"
			if _, ok := seen[value]; ok {
				continue
			}
			if prefix != "" && !strings.HasPrefix(strings.ToLower(value), prefix) &&
				!strings.HasPrefix(strings.ToLower("cloud:"+ref), prefix) {
				continue
			}
			seen[value] = struct{}{}
			items = append(items, Item{Value: value, Description: desc})
		}
	}
	return items, nil
}

func completeCloudLabels(ctx context.Context, netRef, labelPrefix, valuePrefix string) ([]Item, error) {
	saved, err := session.Load()
	if err != nil {
		return nil, err
	}
	client := apiclient.New(saved)
	n, err := client.ResolveNetworkRef(ctx, netRef)
	if err != nil {
		return nil, err
	}
	items, err := cachedDeviceConfigLabels(client, n.ID, labelPrefix, valuePrefix)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 && labelPrefix == "" {
		// New upload target: keep the trailing slash so the operator can
		// type a fresh label after tab-completing the network.
		items = []Item{{Value: valuePrefix, Description: "type a label after /"}}
	}
	return items, nil
}

func cachedDeviceConfigLabels(client *apiclient.Client, networkID, labelPrefix, valuePrefix string) ([]Item, error) {
	cacheKey := networkID + "\x00" + labelPrefix
	deviceConfigCacheMu.Lock()
	if ent, ok := deviceConfigCache[cacheKey]; ok && time.Since(ent.at) < deviceConfigCacheTTL {
		cached := append([]Item(nil), ent.items...)
		deviceConfigCacheMu.Unlock()
		return cached, nil
	}
	deviceConfigCacheMu.Unlock()

	list, err := client.ListDeviceConfigs(context.Background(), networkID)
	if err != nil {
		return nil, err
	}
	lp := strings.ToLower(labelPrefix)
	var items []Item
	for _, cfg := range list.Items {
		if lp != "" && !strings.HasPrefix(strings.ToLower(cfg.Label), lp) &&
			!strings.HasPrefix(strings.ToLower(cfg.ID), lp) {
			continue
		}
		desc := cfg.Region
		if cfg.ModemPreset != "" {
			desc = cfg.Region + " · " + cfg.ModemPreset
		}
		items = append(items, Item{
			Value:       valuePrefix + cfg.Label,
			Description: desc,
		})
	}

	deviceConfigCacheMu.Lock()
	deviceConfigCache[cacheKey] = deviceConfigCacheEntry{items: items, at: time.Now()}
	deviceConfigCacheMu.Unlock()
	return items, nil
}

func completeDeviceURL(toComplete string) ([]Item, error) {
	url, err := agentTransportURL()
	if err != nil || url == "" {
		return nil, err
	}
	value := "device:" + url
	if !prefixMatch(toComplete, value) && !prefixMatch(toComplete, "device:") {
		return nil, nil
	}
	return []Item{{Value: value, Description: "agent config transport.url"}}, nil
}

func agentTransportURL() (string, error) {
	cfg, err := config.Load(cliconfig.AgentConfigPath())
	if err != nil {
		return "", nil
	}
	return cfg.Transport.URL, nil
}

func looksLikeConfigPath(s string) bool {
	if s == "" {
		return false
	}
	if strings.ContainsRune(s, '/') || strings.HasPrefix(s, ".") {
		return true
	}
	for _, ext := range []string{".yaml", ".yml", ".json"} {
		if strings.HasSuffix(strings.ToLower(s), ext) {
			return true
		}
	}
	return false
}

func prefixMatch(toComplete, candidate string) bool {
	if toComplete == "" {
		return true
	}
	return strings.HasPrefix(strings.ToLower(candidate), strings.ToLower(toComplete))
}
