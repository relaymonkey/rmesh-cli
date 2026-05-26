package deviceconfigs

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// RedactedPlaceholder is the string substituted for PSK / admin-key
// fields when `--reveal-secrets` was not passed. Matches the backend
// redactor (relaymesh-backend/internal/deviceconfigs/redact.go).
const RedactedPlaceholder = "***"

// Redact returns a copy of `p` with PSK / admin-key material replaced
// with RedactedPlaceholder. Used by `rmesh device config get` for
// CLI output and by the file-write path when the source is a device
// read and the operator did not pass `--reveal-secrets`.
func Redact(p CanonicalPayload) CanonicalPayload {
	out := CanonicalPayload{
		Owner:         p.Owner,
		FixedPosition: p.FixedPosition,
	}
	if len(p.Channels) > 0 {
		out.Channels = make([]json.RawMessage, len(p.Channels))
		for i, ch := range p.Channels {
			out.Channels[i] = redactChannel(ch)
		}
	}
	if len(p.Config) > 0 {
		out.Config = map[string]json.RawMessage{}
		for k, v := range p.Config {
			if k == "security" {
				out.Config[k] = redactJSONFields(v, "private_key", "admin_key", "public_key")
				continue
			}
			out.Config[k] = v
		}
	}
	if len(p.ModuleConfig) > 0 {
		out.ModuleConfig = map[string]json.RawMessage{}
		for k, v := range p.ModuleConfig {
			if k == "mqtt" {
				out.ModuleConfig[k] = redactJSONFields(v, "password", "root_topic_password")
				continue
			}
			out.ModuleConfig[k] = v
		}
	}
	return out
}

func redactChannel(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return raw
	}
	if v, ok := m["settings"]; ok {
		m["settings"] = redactJSONFields(v, "psk")
	}
	out, err := json.Marshal(m)
	if err != nil {
		return raw
	}
	return out
}

func redactJSONFields(raw json.RawMessage, fields ...string) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return raw
	}
	for _, f := range fields {
		if _, ok := m[f]; ok {
			b, _ := json.Marshal(RedactedPlaceholder)
			m[f] = b
		}
	}
	out, err := json.Marshal(m)
	if err != nil {
		return raw
	}
	return out
}

// RenderFormat is the output format for `rmesh device config get`.
type RenderFormat string

const (
	FormatJSON RenderFormat = "json"
	FormatYAML RenderFormat = "yaml"
	FormatTree RenderFormat = "tree"
)

// Render writes the payload to `w` in the chosen format. The tree
// format is human-oriented (grouped by submessage, one section per
// chunk); json / yaml are machine-oriented and reflect the canonical
// shape byte-for-byte.
func Render(w io.Writer, p CanonicalPayload, format RenderFormat) error {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(p)
	case FormatYAML:
		// Round-trip through json.Marshal first so the YAML output
		// uses the canonical field names (the json tags), not the
		// Go field names.
		raw, err := json.Marshal(p)
		if err != nil {
			return err
		}
		var any interface{}
		if err := json.Unmarshal(raw, &any); err != nil {
			return err
		}
		enc := yaml.NewEncoder(w)
		enc.SetIndent(2)
		return enc.Encode(any)
	case FormatTree:
		return renderTree(w, p)
	default:
		return fmt.Errorf("unknown format %q", format)
	}
}

func renderTree(w io.Writer, p CanonicalPayload) error {
	hints := HintsFromPayload(p)
	chips := []string{}
	if hints.Region != "" {
		chips = append(chips, "region="+hints.Region)
	}
	if hints.ModemPreset != "" {
		chips = append(chips, "modem_preset="+hints.ModemPreset)
	}
	fmt.Fprintf(w, "device-config (schema=%d)", SchemaVersion)
	if len(chips) > 0 {
		fmt.Fprintf(w, "  [%s]", strings.Join(chips, ", "))
	}
	fmt.Fprintln(w)

	if len(p.Channels) > 0 {
		fmt.Fprintf(w, "\nchannels (%d)\n", len(p.Channels))
		for i, ch := range p.Channels {
			fmt.Fprintf(w, "  [%d]\n", i)
			writeIndentedJSON(w, ch, "    ")
		}
	}

	if len(p.Config) > 0 {
		fmt.Fprintln(w, "\nconfig")
		for _, k := range orderedKeys(p.Config, ConfigKeys) {
			fmt.Fprintf(w, "  %s\n", k)
			writeIndentedJSON(w, p.Config[k], "    ")
		}
	}

	if len(p.ModuleConfig) > 0 {
		fmt.Fprintln(w, "\nmodule_config")
		for _, k := range orderedKeys(p.ModuleConfig, ModuleConfigKeys) {
			fmt.Fprintf(w, "  %s\n", k)
			writeIndentedJSON(w, p.ModuleConfig[k], "    ")
		}
	}

	if len(p.Owner) > 0 {
		fmt.Fprintln(w, "\nowner")
		writeIndentedJSON(w, p.Owner, "  ")
	}
	if len(p.FixedPosition) > 0 {
		fmt.Fprintln(w, "\nfixed_position")
		writeIndentedJSON(w, p.FixedPosition, "  ")
	}
	return nil
}

// orderedKeys returns keys present in `m` ordered by `preferred`
// first, with any unknown trailing keys sorted alphabetically.
func orderedKeys(m map[string]json.RawMessage, preferred []string) []string {
	out := make([]string, 0, len(m))
	seen := map[string]bool{}
	for _, k := range preferred {
		if _, ok := m[k]; ok {
			out = append(out, k)
			seen[k] = true
		}
	}
	trail := []string{}
	for k := range m {
		if !seen[k] {
			trail = append(trail, k)
		}
	}
	sort.Strings(trail)
	return append(out, trail...)
}

// writeIndentedJSON pretty-prints `raw` indented under `prefix`. Best-
// effort: malformed JSON is written verbatim.
func writeIndentedJSON(w io.Writer, raw json.RawMessage, prefix string) {
	if len(raw) == 0 {
		fmt.Fprintf(w, "%s(empty)\n", prefix)
		return
	}
	var any interface{}
	if err := json.Unmarshal(raw, &any); err != nil {
		fmt.Fprintf(w, "%s%s\n", prefix, string(raw))
		return
	}
	out, err := json.MarshalIndent(any, prefix, "  ")
	if err != nil {
		fmt.Fprintf(w, "%s%s\n", prefix, string(raw))
		return
	}
	fmt.Fprintf(w, "%s%s\n", prefix, out)
}

// ParseFormat normalises the user-supplied format string.
func ParseFormat(s string) (RenderFormat, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "json":
		return FormatJSON, nil
	case "yaml", "yml":
		return FormatYAML, nil
	case "", "tree":
		return FormatTree, nil
	}
	return "", fmt.Errorf("unknown format %q (want json, yaml, or tree)", s)
}
