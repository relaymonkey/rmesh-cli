package apiclient

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// DeleteDeviceConfig removes a network template row. Personal rows
// have moved to `/me/device-configs/{cfg_id}` per D-219; the CLI
// dispatches on Source.Owner before calling either this method or
// `DeleteMyDeviceConfig`.
func (c *Client) DeleteDeviceConfig(ctx context.Context, networkID, configID string) error {
	path := fmt.Sprintf("/api/v1/networks/%s/device-configs/%s",
		url.PathEscape(networkID), url.PathEscape(configID))
	return c.doJSON(ctx, http.MethodDelete, c.baseURL+path, nil, nil)
}
