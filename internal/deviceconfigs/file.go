package deviceconfigs

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadFromFile reads a canonical payload from disk. The format is
// inferred from the extension (.yaml/.yml → YAML, .json or anything
// else → JSON); the body is passed through JSON for canonicalisation
// so YAML and JSON files round-trip identically.
func LoadFromFile(path string) (CanonicalPayload, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return CanonicalPayload{}, fmt.Errorf("read %s: %w", path, err)
	}
	return ParseBytes(raw, path)
}

// ParseBytes parses a payload from raw bytes. The `hint` argument may
// be a filename (used only to pick JSON vs YAML by extension); when
// empty, JSON is tried first and YAML on fallback.
func ParseBytes(raw []byte, hint string) (CanonicalPayload, error) {
	if isYAML(hint, raw) {
		var any interface{}
		if err := yaml.Unmarshal(raw, &any); err != nil {
			return CanonicalPayload{}, fmt.Errorf("parse yaml: %w", err)
		}
		// Round-trip through JSON to apply the json struct tags on
		// CanonicalPayload (yaml.Unmarshal would otherwise expect
		// snake_case → CamelCase via reflection).
		jb, err := json.Marshal(toJSONCompatible(any))
		if err != nil {
			return CanonicalPayload{}, fmt.Errorf("round-trip yaml: %w", err)
		}
		var p CanonicalPayload
		if err := json.Unmarshal(jb, &p); err != nil {
			return CanonicalPayload{}, fmt.Errorf("decode yaml-derived json: %w", err)
		}
		return p, nil
	}
	var p CanonicalPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return CanonicalPayload{}, fmt.Errorf("parse json: %w", err)
	}
	return p, nil
}

// WriteToFile renders to disk. JSON is pretty-printed; YAML uses two-
// space indent.
func WriteToFile(path string, p CanonicalPayload) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()
	format := FormatJSON
	if isYAML(path, nil) {
		format = FormatYAML
	}
	return Render(f, p, format)
}

func isYAML(hint string, body []byte) bool {
	h := strings.ToLower(hint)
	if strings.HasSuffix(h, ".yaml") || strings.HasSuffix(h, ".yml") {
		return true
	}
	if strings.HasSuffix(h, ".json") {
		return false
	}
	// Body-sniff: JSON files start with `{` or `[` after whitespace.
	for _, b := range body {
		switch b {
		case ' ', '\t', '\r', '\n':
			continue
		case '{', '[':
			return false
		default:
			return true
		}
	}
	return false
}

// toJSONCompatible coerces yaml.Unmarshal's map[interface{}]interface{}
// values into map[string]interface{} so json.Marshal accepts them.
// yaml.v3's interface decoder produces the latter for top-level maps
// but the former for nested maps in some cases — handle both.
func toJSONCompatible(v interface{}) interface{} {
	switch x := v.(type) {
	case map[interface{}]interface{}:
		out := make(map[string]interface{}, len(x))
		for k, vv := range x {
			out[fmt.Sprintf("%v", k)] = toJSONCompatible(vv)
		}
		return out
	case map[string]interface{}:
		out := make(map[string]interface{}, len(x))
		for k, vv := range x {
			out[k] = toJSONCompatible(vv)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(x))
		for i, vv := range x {
			out[i] = toJSONCompatible(vv)
		}
		return out
	}
	return v
}
