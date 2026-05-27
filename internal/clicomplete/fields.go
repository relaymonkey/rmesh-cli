package clicomplete

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/relaymonkey/relaymesh-edge/internal/apiclient"
	"github.com/relaymonkey/relaymesh-edge/internal/clidefault"
	"github.com/relaymonkey/relaymesh-edge/internal/session"
)

const fieldsCatalogTTL = 30 * time.Second

var (
	fieldsCacheMu sync.Mutex
	fieldsCache   []string
	fieldsCacheAt time.Time
)

// MessageFieldsProvider completes --fields from GET /messages/fields (Traffic catalog).
func MessageFieldsProvider(ctx context.Context, toComplete string) ([]Item, error) {
	names, err := cachedFieldNames(ctx)
	if err != nil {
		return nil, err
	}
	// summary is a UI column but not returned by /messages/fields.
	names = append([]string{"summary"}, names...)
	prefix := strings.ToLower(toComplete)
	var items []Item
	seen := map[string]struct{}{}
	for _, name := range names {
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		if prefix != "" && !strings.HasPrefix(strings.ToLower(name), prefix) {
			continue
		}
		items = append(items, Item{Value: name})
	}
	return items, nil
}

func cachedFieldNames(ctx context.Context) ([]string, error) {
	fieldsCacheMu.Lock()
	if time.Since(fieldsCacheAt) < fieldsCatalogTTL && len(fieldsCache) > 0 {
		cached := append([]string(nil), fieldsCache...)
		fieldsCacheMu.Unlock()
		return cached, nil
	}
	fieldsCacheMu.Unlock()

	saved, err := session.Load()
	if err != nil {
		return nil, err
	}
	def, err := clidefault.Load()
	if err != nil {
		return nil, fmt.Errorf("fields completion needs a default network (rmesh network use …): %w", err)
	}
	client := apiclient.New(saved)
	to := time.Now().UTC()
	from := to.Add(-24 * time.Hour)
	cat, err := client.ListMessageFields(ctx, def.NetworkID, from.Format(time.RFC3339Nano), to.Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(cat.Fields))
	for _, f := range cat.Fields {
		if f.Name != "" {
			names = append(names, f.Name)
		}
	}

	fieldsCacheMu.Lock()
	fieldsCache = names
	fieldsCacheAt = time.Now()
	cached := append([]string(nil), fieldsCache...)
	fieldsCacheMu.Unlock()
	return cached, nil
}
