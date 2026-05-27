package climessages

import (
	"fmt"
	"strings"

	"github.com/relaymonkey/relaymesh-edge/internal/apiclient"
)

// DefaultListFields matches Traffic defaultColumnIds when --fields is omitted.
// Canonical list: docs/traffic-columns.md (keep in sync with the RelayMesh web UI).
var DefaultListFields = []string{
	"ingest_ts",
	"source_node_id",
	"packet_type",
	"channel_index",
	"payload_size",
	"gateway_id",
	"summary",
	"encrypted",
}

// DefaultTextFields when --fields is omitted on traffic text.
// Canonical list: docs/traffic-columns.md (keep in sync with Traffic text view).
var DefaultTextFields = []string{
	"ingest_ts",
	"source_node_id",
	"dest_node_id",
	"decoded.value.text",
}

// ParseFields splits --fields verbatim (Traffic column / filter ids, no aliasing).
func ParseFields(raw string, textMode bool) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		if textMode {
			return append([]string(nil), DefaultTextFields...), nil
		}
		return append([]string(nil), DefaultListFields...), nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, p := range parts {
		id := strings.TrimSpace(p)
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("fields list is empty")
	}
	return out, nil
}

// FieldHeader returns the column header (Traffic columnLabel = id verbatim).
func FieldHeader(field string) string {
	return field
}

// FieldValue extracts one cell for a Traffic column id from the raw envelope.
// The only non-JSON column id is "summary" (computed in the UI the same way).
func FieldValue(m apiclient.MessageEnvelope, field string) string {
	if field == "summary" {
		s := SummarizeDecoded(m)
		if s == "" {
			return "—"
		}
		return s
	}
	if field == "encrypted" {
		if v, ok := m.At("encrypted"); ok {
			if b, ok := v.(bool); ok {
				if b {
					return "encrypted"
				}
				return "cleartext"
			}
		}
		return "—"
	}
	v, ok := m.At(field)
	if !ok {
		return "—"
	}
	return apiclient.FormatLeaf(v)
}

// ProjectJSON returns rows keyed by the requested Traffic column ids.
func ProjectJSON(items []apiclient.MessageEnvelope, fields []string) []map[string]any {
	out := make([]map[string]any, len(items))
	for i, m := range items {
		row := map[string]any{}
		for _, f := range fields {
			if f == "summary" {
				s := SummarizeDecoded(m)
				if s != "" {
					row[f] = s
				} else {
					row[f] = nil
				}
				continue
			}
			v, ok := m.At(f)
			if !ok {
				row[f] = nil
				continue
			}
			row[f] = v
		}
		out[i] = row
	}
	return out
}

// ProjectTable builds headers/rows for the selected column ids.
func ProjectTable(items []apiclient.MessageEnvelope, fields []string) (headers []string, rows [][]string) {
	for _, f := range fields {
		headers = append(headers, FieldHeader(f))
	}
	for _, m := range items {
		row := make([]string, len(fields))
		for i, f := range fields {
			row[i] = FieldValue(m, f)
		}
		rows = append(rows, row)
	}
	return headers, rows
}

// FormatLiveLine renders one live row for table-style streaming output.
func FormatLiveLine(m apiclient.MessageEnvelope, fields []string) string {
	parts := make([]string, len(fields))
	for i, f := range fields {
		parts[i] = FieldValue(m, f)
	}
	return strings.Join(parts, " ")
}

// ExtractText reads decoded.value.text from a raw envelope.
func ExtractText(m apiclient.MessageEnvelope) string {
	return m.StringField("decoded.value.text")
}

// IsText returns true when packet_type is TEXT_MESSAGE_APP (portnum 1).
func IsText(m apiclient.MessageEnvelope) bool {
	n, ok := m.IntField("packet_type")
	return ok && n == TextPacketType
}
