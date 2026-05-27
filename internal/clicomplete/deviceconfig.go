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

	// The cloud splits configs into two collections: network
	// templates live under /networks/{n}/device-configs, personal rows
	// under /me/device-configs. Operators rarely care which side a
	// label lives on when *completing* it — they just want every label
	// they can `show --from cloud:<n>/<label>` against. Union both
	// lists and tag the description so the per-row "mine" vs "template"
	// distinction stays visible.
	bgCtx := context.Background()
	templates, terr := client.ListDeviceConfigs(bgCtx, networkID)
	mine, merr := client.ListMyDeviceConfigs(bgCtx, networkID)
	// Tolerate partial failures — if one side errors (e.g. the operator
	// isn't a member, or backend transient), still offer the other.
	if terr != nil && merr != nil {
		return nil, terr
	}

	lp := strings.ToLower(labelPrefix)
	var items []Item
	seenLabel := map[string]struct{}{}
	emit := func(cfg apiclient.DeviceConfigSummary, ownerTag string) {
		if lp != "" && !strings.HasPrefix(strings.ToLower(cfg.Label), lp) &&
			!strings.HasPrefix(strings.ToLower(cfg.ID), lp) {
			return
		}
		// Personal + template can share a label (the backend's unique
		// indexes are per-ownership). Disambiguate with the explicit
		// `mine/` / `template/` prefix so completion never offers two
		// rows that produce different payloads when expanded.
		value := valuePrefix + ownerTag + "/" + cfg.Label
		if _, ok := seenLabel[value]; ok {
			return
		}
		seenLabel[value] = struct{}{}
		desc := cfg.Region
		if cfg.ModemPreset != "" {
			desc = cfg.Region + " · " + cfg.ModemPreset
		}
		if desc == "" {
			desc = ownerTag
		} else {
			desc = ownerTag + " · " + desc
		}
		items = append(items, Item{Value: value, Description: desc})
	}
	for _, cfg := range mine.Items {
		emit(cfg, "mine")
	}
	for _, cfg := range templates.Items {
		emit(cfg, "template")
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
