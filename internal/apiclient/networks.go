package apiclient

import "context"

// ListNetworks returns networks the authenticated principal can access.
func (c *Client) ListNetworks(ctx context.Context) (NetworkList, error) {
	var list NetworkList
	if err := c.getJSONQuery(ctx, "/api/v1/networks", nil, &list); err != nil {
		return NetworkList{}, err
	}
	return list, nil
}
