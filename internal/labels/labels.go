package labels

import (
	"encoding/json"
	"fmt"
)

const (
	PropIngestSource = "relaymesh_ingest_source"
	PropLabels       = "relaymesh_labels"
)

// MarshalJSON encodes operator labels for MQTT user properties.
func MarshalJSON(labels map[string]string) (string, error) {
	if len(labels) == 0 {
		return "{}", nil
	}
	b, err := json.Marshal(labels)
	if err != nil {
		return "", fmt.Errorf("marshal labels: %w", err)
	}
	return string(b), nil
}

// IngestSource returns the provenance token for passthrough packets.
func IngestSource(agentID string) string {
	return "edge:" + agentID
}

// IngestSourceNodeDB returns the provenance token for synthetic NodeDB packets.
func IngestSourceNodeDB(agentID string) string {
	return "edge:" + agentID + ":nodedb"
}
