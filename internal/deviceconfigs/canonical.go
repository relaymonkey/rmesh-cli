// Package deviceconfigs implements the canonical Meshtastic
// device-configuration shape used by `rmesh device config`.
//
// The canonical shape is intentionally protojson-flavoured: each
// submessage is stored as raw JSON so unknown fields from newer
// firmware round-trip losslessly through `rmesh device config get`
// and back through `set` without an `rmesh` rebuild.
//
// Cloud and edge agree on this shape — the backend stores it sealed
// under the platform KEK and the CLI reads / writes it byte-for-byte.
package deviceconfigs

import (
	"encoding/json"
	"errors"
)

// SchemaVersion is the major version of the canonical JSON shape.
// Bumps when the layout changes in a way a reader cannot infer from
// the field names. Stays in lockstep with the backend's
// `CanonicalPayloadSchemaVersion`.
const SchemaVersion = 1

// CanonicalPayload is the canonical wire / file shape. Each
// submessage is stored as raw JSON so the editor / renderer can
// inspect known sub-fields while unknown fields round-trip
// untouched.
type CanonicalPayload struct {
	Channels      []json.RawMessage          `json:"channels,omitempty"`
	Config        map[string]json.RawMessage `json:"config,omitempty"`
	ModuleConfig  map[string]json.RawMessage `json:"module_config,omitempty"`
	Owner         json.RawMessage            `json:"owner,omitempty"`
	FixedPosition json.RawMessage            `json:"fixed_position,omitempty"`
}

// ConfigKeys is the ordered list of `Config.*` submessage keys the
// canonical shape recognises. Render / diff / section-filter loop in
// this order so output is stable.
var ConfigKeys = []string{
	"lora", "device", "position", "power",
	"network", "display", "bluetooth", "security",
}

// ModuleConfigKeys is the ordered list of `ModuleConfig.*` submessage
// keys. Forward-tolerant — unknown keys still round-trip via
// CanonicalPayload.ModuleConfig.
var ModuleConfigKeys = []string{
	"mqtt", "serial", "ext_notification", "store_forward",
	"range_test", "telemetry", "canned_message", "audio",
	"remote_hardware", "neighbor_info", "ambient_lighting",
	"detection_sensor", "paxcounter",
}

// AllSubmessageKeys returns the union ordered for stable iteration:
// `config.<k>` first, then `module_config.<k>`. Used by `--section`
// filter validation in the CLI.
func AllSubmessageKeys() []string {
	out := make([]string, 0, len(ConfigKeys)+len(ModuleConfigKeys))
	out = append(out, ConfigKeys...)
	out = append(out, ModuleConfigKeys...)
	return out
}

// LoRaHints captures the two denormalised hints the backend stores
// alongside the sealed blob — region and modem preset. The CLI uses
// them for region-change safety checks.
type LoRaHints struct {
	Region      string
	ModemPreset string
}

// HintsFromPayload extracts the LoRa region and modem preset names.
// Missing values come back empty strings.
func HintsFromPayload(p CanonicalPayload) LoRaHints {
	out := LoRaHints{}
	if raw, ok := p.Config["lora"]; ok && len(raw) > 0 {
		var lora struct {
			Region      string `json:"region"`
			ModemPreset string `json:"modem_preset"`
			UsePreset   bool   `json:"use_preset"`
		}
		_ = json.Unmarshal(raw, &lora)
		out.Region = lora.Region
		// protojson with EmitUnpopulated=false strips `use_preset:
		// false`, so absence == custom (proto3 default). Only an
		// explicit `use_preset: true` makes the modem_preset enum
		// meaningful — otherwise the radio is running custom
		// bandwidth / spread_factor / coding_rate.
		if lora.UsePreset {
			out.ModemPreset = lora.ModemPreset
		} else {
			out.ModemPreset = "Custom"
		}
	}
	return out
}

// IsEmpty reports whether the payload carries no submessages. Used by
// `set` to refuse a no-op apply.
func (p CanonicalPayload) IsEmpty() bool {
	return len(p.Channels) == 0 && len(p.Config) == 0 && len(p.ModuleConfig) == 0
}

// CloneSections returns a copy of p restricted to the supplied
// section keys (canonical names). Empty keys returns p as-is.
// Unknown keys are silently dropped; the CLI validates the list
// before calling this.
func (p CanonicalPayload) CloneSections(keep []string) CanonicalPayload {
	if len(keep) == 0 {
		return p
	}
	set := map[string]struct{}{}
	for _, k := range keep {
		set[k] = struct{}{}
	}
	out := CanonicalPayload{
		Owner:         p.Owner,
		FixedPosition: p.FixedPosition,
	}
	if _, ok := set["channels"]; ok {
		out.Channels = p.Channels
	}
	if len(p.Config) > 0 {
		out.Config = map[string]json.RawMessage{}
		for _, k := range ConfigKeys {
			if _, ok := set[k]; ok {
				if v, has := p.Config[k]; has {
					out.Config[k] = v
				}
			}
		}
	}
	if len(p.ModuleConfig) > 0 {
		out.ModuleConfig = map[string]json.RawMessage{}
		for _, k := range ModuleConfigKeys {
			if _, ok := set[k]; ok {
				if v, has := p.ModuleConfig[k]; has {
					out.ModuleConfig[k] = v
				}
			}
		}
	}
	return out
}

// ExcludeFields removes individual fields from the payload by their
// dotted path (e.g. `lora.region`, `owner`). Only top-level submessage
// keys are recognised today; nested-field exclusion is a TODO that
// can grow incrementally without breaking callers.
func (p CanonicalPayload) ExcludeFields(paths []string) CanonicalPayload {
	if len(paths) == 0 {
		return p
	}
	out := p
	for _, raw := range paths {
		switch raw {
		case "owner":
			out.Owner = nil
		case "fixed_position":
			out.FixedPosition = nil
		case "channels":
			out.Channels = nil
		default:
			// Accept the forms the diff / stall output actually shows,
			// plus bare keys and field paths:
			//   "lora", "config.lora", "module_config.mqtt"   → drop section
			//   "lora.region", "config.lora.region"           → drop field
			// An optional `config.` / `module_config.` group prefix is
			// stripped first so an operator can paste a section name from
			// the diff straight into `--exclude`.
			head, tail := splitFirst(raw, '.')
			if head == "config" || head == "module_config" {
				head, tail = splitFirst(tail, '.')
			}
			if head == "" {
				continue
			}
			if tail == "" {
				delete(out.Config, head)
				delete(out.ModuleConfig, head)
				continue
			}
			if v, ok := out.Config[head]; ok {
				out.Config[head] = removeJSONField(v, tail)
			}
			if v, ok := out.ModuleConfig[head]; ok {
				out.ModuleConfig[head] = removeJSONField(v, tail)
			}
		}
	}
	return out
}

func splitFirst(s string, sep byte) (head, tail string) {
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			return s[:i], s[i+1:]
		}
	}
	return s, ""
}

// removeJSONField deletes a top-level field from a JSON object. If
// `raw` isn't a JSON object, returns it unchanged.
func removeJSONField(raw json.RawMessage, field string) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return raw
	}
	delete(m, field)
	out, err := json.Marshal(m)
	if err != nil {
		return raw
	}
	return out
}

// ErrInvalidSection is returned by ValidateSections when a key isn't
// in CanonicalConfigKeys ∪ CanonicalModuleConfigKeys ∪ {"channels"}.
var ErrInvalidSection = errors.New("unknown section key")

// ValidateSections checks that every supplied section key is known.
// Returns the first unknown key wrapped in ErrInvalidSection.
func ValidateSections(keys []string) error {
	known := map[string]bool{"channels": true}
	for _, k := range ConfigKeys {
		known[k] = true
	}
	for _, k := range ModuleConfigKeys {
		known[k] = true
	}
	for _, k := range keys {
		if !known[k] {
			return errors.New(string(ErrInvalidSection.Error()) + ": " + k)
		}
	}
	return nil
}
