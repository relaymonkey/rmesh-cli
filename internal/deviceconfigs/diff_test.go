package deviceconfigs

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func mustRaw(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

// When two payloads are identical, RenderDiff prints a single
// confirmation line with the labels embedded.
func TestRenderDiff_NoDifferences(t *testing.T) {
	p := CanonicalPayload{
		Config: map[string]json.RawMessage{
			"lora": mustRaw(t, map[string]any{"region": "EU_868", "tx_power": 22}),
		},
	}
	d := Diff(p, p)
	if len(d) != 0 {
		t.Fatalf("expected no section diffs, got %+v", d)
	}
	var buf bytes.Buffer
	RenderDiff(&buf, d, DiffRenderOptions{FromLabel: "device", ToLabel: "cloud:home/eu"})
	out := buf.String()
	if !strings.Contains(out, "device") || !strings.Contains(out, "cloud:home/eu") {
		t.Fatalf("expected labels in confirmation, got %q", out)
	}
	if !strings.Contains(out, "no differences") {
		t.Fatalf("expected 'no differences' in confirmation, got %q", out)
	}
}

// Field-level walk: a tx_power change inside config.lora should
// surface as a single ~ leaf, not a whole-section flag.
func TestRenderDiff_FieldLevelChange(t *testing.T) {
	from := CanonicalPayload{
		Config: map[string]json.RawMessage{
			"lora": mustRaw(t, map[string]any{"region": "EU_868", "tx_power": 22}),
		},
	}
	to := CanonicalPayload{
		Config: map[string]json.RawMessage{
			"lora": mustRaw(t, map[string]any{"region": "EU_868", "tx_power": 27}),
		},
	}
	d := Diff(from, to)
	if len(d) != 1 || d[0].Group != "config" || d[0].Key != "lora" {
		t.Fatalf("expected one config.lora diff, got %+v", d)
	}
	fields := FieldChanges(d[0])
	if len(fields) != 1 {
		t.Fatalf("expected 1 leaf change, got %+v", fields)
	}
	if fields[0].Path != "tx_power" || fields[0].Status != "changed" {
		t.Fatalf("expected ~ tx_power, got %+v", fields[0])
	}
	var buf bytes.Buffer
	RenderDiff(&buf, d, DiffRenderOptions{FromLabel: "device", ToLabel: "cloud"})
	out := buf.String()
	if !strings.Contains(out, "config.lora") {
		t.Fatalf("expected section header, got %q", out)
	}
	if !strings.Contains(out, "tx_power") || !strings.Contains(out, "22") || !strings.Contains(out, "27") {
		t.Fatalf("expected 'tx_power 22 → 27', got %q", out)
	}
}

// Channels are aligned by index; a settings.name swap on slot 1
// should drill into channels[1].settings.name.
func TestRenderDiff_ChannelFieldChange(t *testing.T) {
	from := CanonicalPayload{
		Channels: []json.RawMessage{
			mustRaw(t, map[string]any{"role": "PRIMARY"}),
			mustRaw(t, map[string]any{"index": 1, "settings": map[string]any{"name": "old"}}),
		},
	}
	to := CanonicalPayload{
		Channels: []json.RawMessage{
			mustRaw(t, map[string]any{"role": "PRIMARY"}),
			mustRaw(t, map[string]any{"index": 1, "settings": map[string]any{"name": "enterprise"}}),
		},
	}
	d := Diff(from, to)
	if len(d) != 1 {
		t.Fatalf("expected 1 channel diff, got %+v", d)
	}
	var buf bytes.Buffer
	RenderDiff(&buf, d, DiffRenderOptions{FromLabel: "a", ToLabel: "b"})
	out := buf.String()
	if !strings.Contains(out, "channels[1]") {
		t.Fatalf("expected 'channels[1]' header, got %q", out)
	}
	if !strings.Contains(out, "settings.name") || !strings.Contains(out, `"old"`) || !strings.Contains(out, `"enterprise"`) {
		t.Fatalf("expected settings.name diff, got %q", out)
	}
}

// Adding a brand-new section (mqtt only on the to side) should walk
// every leaf as `+`.
func TestRenderDiff_AddedSection(t *testing.T) {
	from := CanonicalPayload{}
	to := CanonicalPayload{
		ModuleConfig: map[string]json.RawMessage{
			"mqtt": mustRaw(t, map[string]any{
				"address":  "mqtt.meshtastic.org",
				"username": "meshdev",
				"enabled":  true,
			}),
		},
	}
	d := Diff(from, to)
	if len(d) != 1 || d[0].Status != "added" {
		t.Fatalf("expected one added module_config.mqtt diff, got %+v", d)
	}
	fields := FieldChanges(d[0])
	if len(fields) != 3 {
		t.Fatalf("expected 3 leaf adds, got %+v", fields)
	}
	for _, f := range fields {
		if f.Status != "added" {
			t.Fatalf("expected all added, got %+v", f)
		}
	}
}

// CountChanges still counts SECTION-level changes (admin.go drift
// reporting depends on this; field-level expansion is presentation
// only).
func TestCountChanges_SectionGranular(t *testing.T) {
	from := CanonicalPayload{
		Config: map[string]json.RawMessage{
			"lora":   mustRaw(t, map[string]any{"tx_power": 22}),
			"device": mustRaw(t, map[string]any{"role": "CLIENT"}),
		},
	}
	to := CanonicalPayload{
		Config: map[string]json.RawMessage{
			"lora":   mustRaw(t, map[string]any{"tx_power": 27}),
			"device": mustRaw(t, map[string]any{"role": "ROUTER"}),
		},
	}
	if got := CountChanges(Diff(from, to)); got != 2 {
		t.Fatalf("expected 2 section changes, got %d", got)
	}
}
