package clicomplete

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/relaymonkey/relaymesh-edge/internal/apiclient"
	"github.com/relaymonkey/relaymesh-edge/internal/deviceconfigs"
	"github.com/relaymonkey/relaymesh-edge/internal/session"
)

// ConfigEndpointKind selects which `--from` / `--to` tokens are valid
// for a given flag on `rmesh device config`.
type ConfigEndpointKind int

const (
	// ConfigEndpointSource completes read-side sources: device, file,
	// cloud.
	ConfigEndpointSource ConfigEndpointKind = iota
	// ConfigEndpointDestGet completes `get --to`: stdout or a file path.
	// (Deprecated alias surface.)
	ConfigEndpointDestGet
	// ConfigEndpointDestSet completes `set --to`: device or cloud upload.
	// (Deprecated alias surface.)
	ConfigEndpointDestSet
	// ConfigEndpointDestCopy completes `copy --to`: every destination
	// (device, cloud, file, stdout).
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

// ConfigEndpointProvider completes the `--from` / `--to` grammar.
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
		add("device", "live mesh radio")
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
		// Top-level: offer `mine/` (personal library) plus every
		// network the caller belongs to. Per D-219 personal is a
		// peer of the networks at this level.
		items := []Item{{Value: "cloud:mine/", Description: "your personal library"}}
		nets, err := prefixCloudNetworks(ctx, toComplete, "")
		return append(items, nets...), err
	}
	// `cloud:mine/...` — complete personal labels.
	if strings.HasPrefix(rest, "mine/") {
		labelPrefix := strings.TrimPrefix(rest, "mine/")
		return completePersonalLabels(ctx, labelPrefix, "cloud:mine/")
	}
	if rest == "mine" {
		return []Item{{Value: "cloud:mine/", Description: "your personal library"}}, nil
	}
	if slash := strings.Index(rest, "/"); slash >= 0 {
		netRef := rest[:slash]
		labelPrefix := rest[slash+1:]
		return completeCloudLabels(ctx, netRef, labelPrefix, toComplete[:len("cloud:")+slash+1])
	}
	// Partial network ref (no `/` yet). Match against networks and
	// also offer `mine/` when the prefix matches.
	out, err := prefixCloudNetworks(ctx, toComplete, rest)
	if err != nil {
		return out, err
	}
	if strings.HasPrefix("mine", strings.ToLower(rest)) {
		out = append([]Item{{Value: "cloud:mine/", Description: "your personal library"}}, out...)
	}
	return out, nil
}

// completePersonalLabels completes labels in the caller's
// cross-network personal library (D-219). No network ref.
func completePersonalLabels(_ context.Context, labelPrefix, valuePrefix string) ([]Item, error) {
	saved, err := session.Load()
	if err != nil {
		return nil, err
	}
	client := apiclient.New(saved)
	bgCtx := context.Background()
	list, err := client.ListMyDeviceConfigs(bgCtx)
	if err != nil {
		return nil, err
	}
	lp := strings.ToLower(labelPrefix)
	out := make([]Item, 0, len(list.Items))
	for _, cfg := range list.Items {
		if lp != "" && !strings.HasPrefix(strings.ToLower(cfg.Label), lp) &&
			!strings.HasPrefix(strings.ToLower(cfg.ID), lp) {
			continue
		}
		desc := cfg.Region
		if cfg.ModemPreset != "" {
			if desc == "" {
				desc = cfg.ModemPreset
			} else {
				desc = desc + " · " + cfg.ModemPreset
			}
		}
		if desc == "" {
			desc = "personal"
		}
		out = append(out, Item{Value: valuePrefix + cfg.Label, Description: desc})
	}
	return out, nil
}

func prefixCloudNetworks(ctx context.Context, toComplete, netPrefix string) ([]Item, error) {
	nets, err := cachedNetworks(ctx)
	if err != nil {
		return nil, err
	}
	prefix := strings.ToLower(toComplete)
	var items []Item
	// Only offer the UUID — one row per network. short_id
	// exists specifically because the firmware's MQTTConfig.root buffer
	// is limited to 32 bytes (which a full UUID overflows), so short_id is
	// dedicated to the MQTT topic prefix. Promoting it in the
	// CLI URI grammar is scope creep on that role. Slug is the human
	// URL handle for the frontend; name is display-only and shell-
	// unfriendly. The UUID is the canonical key the backend uses
	// everywhere, paste-safe and unambiguous. ResolveNetworkRef stays
	// permissive (UUID + slug + short_id + name) so operators with
	// alternate forms in their shell history don't break — this only
	// changes what we *suggest*, not what we accept.
	for _, n := range nets {
		if n.ID == "" {
			continue
		}
		if netPrefix != "" && !strings.HasPrefix(strings.ToLower(n.ID), strings.ToLower(netPrefix)) {
			continue
		}
		value := "cloud:" + n.ID + "/"
		if prefix != "" && !strings.HasPrefix(strings.ToLower(value), prefix) {
			continue
		}
		items = append(items, Item{
			Value:       value,
			Description: fmt.Sprintf("%s (%s)", n.Name, n.ID),
		})
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

	// Per D-219 personal rows are completed under `cloud:mine/`
	// (`completePersonalLabels`), not here. This path completes
	// network templates only.
	bgCtx := context.Background()
	templates, err := client.ListDeviceConfigs(bgCtx, networkID)
	if err != nil {
		return nil, err
	}

	lp := strings.ToLower(labelPrefix)
	items := make([]Item, 0, len(templates.Items))
	for _, cfg := range templates.Items {
		if lp != "" && !strings.HasPrefix(strings.ToLower(cfg.Label), lp) &&
			!strings.HasPrefix(strings.ToLower(cfg.ID), lp) {
			continue
		}
		desc := cfg.Region
		if cfg.ModemPreset != "" {
			if desc == "" {
				desc = cfg.ModemPreset
			} else {
				desc = desc + " · " + cfg.ModemPreset
			}
		}
		if desc == "" {
			desc = "template"
		} else {
			desc = "template · " + desc
		}
		items = append(items, Item{Value: valuePrefix + cfg.Label, Description: desc})
	}

	deviceConfigCacheMu.Lock()
	deviceConfigCache[cacheKey] = deviceConfigCacheEntry{items: items, at: time.Now()}
	deviceConfigCacheMu.Unlock()
	return items, nil
}

func completeDeviceURL(toComplete string) ([]Item, error) {
	url, err := AgentConfigTransportURL()
	if err != nil || url == "" {
		return nil, err
	}
	value := "device:" + url
	if !prefixMatch(toComplete, value) && !prefixMatch(toComplete, "device:") {
		return nil, nil
	}
	return []Item{{Value: value, Description: "agent config transport.url"}}, nil
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
