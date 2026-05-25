package apiclient

import "context"

// Network is GET /api/v1/networks item shape (list fields).
type Network struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	ShortID     string `json:"short_id"`
	Name        string `json:"name"`
	Visibility  string `json:"visibility"`
	JoinPolicy  string `json:"join_policy"`
	CreatedAt   string `json:"created_at"`
}

// NetworkList is GET /api/v1/networks.
type NetworkList struct {
	Items []Network `json:"items"`
}

// ListNetworks returns networks the authenticated principal can access.
func (c *Client) ListNetworks(ctx context.Context) (NetworkList, error) {
	var list NetworkList
	if err := c.getJSONQuery(ctx, "/api/v1/networks", nil, &list); err != nil {
		return NetworkList{}, err
	}
	return list, nil
}
