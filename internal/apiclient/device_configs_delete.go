package apiclient

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// DeleteDeviceConfig removes a personal or network-template row.
// Returns nil on 204. Authorisation is enforced server-side (personal:
// owner-only; templates: elevated network role).
func (c *Client) DeleteDeviceConfig(ctx context.Context, networkID, configID string) error {
	path := fmt.Sprintf("/api/v1/networks/%s/device-configs/%s",
		url.PathEscape(networkID), url.PathEscape(configID))
	return c.doJSON(ctx, http.MethodDelete, c.baseURL+path, nil, nil)
}
