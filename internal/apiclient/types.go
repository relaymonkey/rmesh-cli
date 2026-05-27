package apiclient

// Shapes match the RelayMesh REST API for the endpoints rmesh calls.
// rmesh calls. Update here when those response schemas change.

// Me is GET /api/v1/me (subset used by the CLI).
type Me struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// Network is GET /api/v1/networks item shape (list fields).
type Network struct {
	ID         string `json:"id"`
	Slug       string `json:"slug"`
	ShortID    string `json:"short_id"`
	Name       string `json:"name"`
	Visibility string `json:"visibility"`
	JoinPolicy string `json:"join_policy"`
	CreatedAt  string `json:"created_at"`
}

// NetworkList is GET /api/v1/networks.
type NetworkList struct {
	Items []Network `json:"items"`
}

// MessageFieldDescriptor is one entry from GET /messages/fields (Traffic catalog).
type MessageFieldDescriptor struct {
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	Kind       string   `json:"kind"`
	Indexed    bool     `json:"indexed"`
	AllowedOps []string `json:"allowed_ops"`
	Sortable   bool     `json:"sortable"`
	Coverage   float64  `json:"coverage"`
}

// MessageFieldCatalog is GET /api/v1/networks/{id}/messages/fields.
type MessageFieldCatalog struct {
	Fields []MessageFieldDescriptor `json:"fields"`
}
