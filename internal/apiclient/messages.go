package apiclient

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// MessageList is GET /api/v1/networks/{id}/messages.
type MessageList struct {
	Items      []MessageEnvelope `json:"items"`
	NextCursor *string           `json:"next_cursor"`
}

// ListMessagesQuery holds GET /messages query parameters (passed through raw).
type ListMessagesQuery struct {
	From            string
	To              string
	Q               string
	Filters         []string
	NodeFilters     []string
	GatewayFilters  []string
	Sort            []string
	Cursor          string
	Limit           int
	Order           string
}

// ListMessages returns historical message envelopes for a network.
func (c *Client) ListMessages(ctx context.Context, networkID string, q ListMessagesQuery) (MessageList, error) {
	params := url.Values{}
	if q.From != "" {
		params.Set("from", q.From)
	}
	if q.To != "" {
		params.Set("to", q.To)
	}
	if q.Q != "" {
		params.Set("q", q.Q)
	}
	if q.Limit > 0 {
		params.Set("limit", strconv.Itoa(q.Limit))
	}
	for _, f := range q.Filters {
		params.Add("filter", f)
	}
	for _, f := range q.NodeFilters {
		params.Add("node_filter", f)
	}
	for _, f := range q.GatewayFilters {
		params.Add("gateway_filter", f)
	}
	for _, s := range q.Sort {
		params.Add("sort", s)
	}
	if q.Cursor != "" {
		params.Set("cursor", q.Cursor)
	}
	if q.Order != "" {
		params.Set("order", q.Order)
	}
	path := fmt.Sprintf("/api/v1/networks/%s/messages", url.PathEscape(networkID))
	var list MessageList
	if err := c.getJSONQuery(ctx, path, params, &list); err != nil {
		return MessageList{}, err
	}
	return list, nil
}

// ListMessageFields returns the Traffic field catalog for a network/range.
func (c *Client) ListMessageFields(ctx context.Context, networkID, from, to string) (MessageFieldCatalog, error) {
	params := url.Values{}
	params.Set("from", from)
	params.Set("to", to)
	path := fmt.Sprintf("/api/v1/networks/%s/messages/fields", url.PathEscape(networkID))
	var cat MessageFieldCatalog
	if err := c.getJSONQuery(ctx, path, params, &cat); err != nil {
		return MessageFieldCatalog{}, err
	}
	return cat, nil
}

// ResolveNetworkRef finds a network by UUID, slug, short_id, or exact name.
func (c *Client) ResolveNetworkRef(ctx context.Context, ref string) (Network, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return Network{}, fmt.Errorf("network reference is empty")
	}
	list, err := c.ListNetworks(ctx)
	if err != nil {
		return Network{}, err
	}
	var nameMatches []Network
	for _, n := range list.Items {
		if n.ID == ref || n.Slug == ref || n.ShortID == ref {
			return n, nil
		}
		if strings.EqualFold(strings.TrimSpace(n.Name), ref) {
			nameMatches = append(nameMatches, n)
		}
	}
	switch len(nameMatches) {
	case 1:
		return nameMatches[0], nil
	case 0:
		return Network{}, fmt.Errorf("network %q not found", ref)
	default:
		return Network{}, fmt.Errorf("network %q is ambiguous (%d name matches)", ref, len(nameMatches))
	}
}
