package apiclient

import (
	"encoding/json"
	"strconv"
	"strings"
)

// MessageEnvelope holds one GET /messages item or WebSocket frame as raw JSON
// so --fields and output stay 1:1 with the Traffic UI / API without a local
// field registry.
type MessageEnvelope []byte

func (m *MessageEnvelope) UnmarshalJSON(data []byte) error {
	*m = append((*m)[0:0], data...)
	return nil
}

func (m MessageEnvelope) MarshalJSON() ([]byte, error) {
	if len(m) == 0 {
		return []byte("null"), nil
	}
	return m, nil
}

// At walks a dotted Traffic column / filter field id on the envelope JSON.
func (m MessageEnvelope) At(path string) (any, bool) {
	path = strings.TrimSpace(path)
	if path == "" || len(m) == 0 {
		return nil, false
	}
	var root any
	if err := json.Unmarshal(m, &root); err != nil {
		return nil, false
	}
	cur := root
	for _, seg := range strings.Split(path, ".") {
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = obj[seg]
		if !ok || cur == nil {
			return nil, false
		}
	}
	return cur, true
}

// IntField reads a top-level or nested numeric field as int.
func (m MessageEnvelope) IntField(path string) (int, bool) {
	v, ok := m.At(path)
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case json.Number:
		i, err := n.Int64()
		return int(i), err == nil
	default:
		return 0, false
	}
}

// StringField reads a string leaf at path.
func (m MessageEnvelope) StringField(path string) string {
	v, ok := m.At(path)
	if !ok {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// FormatLeaf renders a JSON leaf for table output.
func FormatLeaf(v any) string {
	if v == nil {
		return "—"
	}
	switch x := v.(type) {
	case string:
		if strings.TrimSpace(x) == "" {
			return "—"
		}
		return x
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		if x {
			return "true"
		}
		return "false"
	default:
		b, err := json.Marshal(x)
		if err != nil {
			return "—"
		}
		return string(b)
	}
}
