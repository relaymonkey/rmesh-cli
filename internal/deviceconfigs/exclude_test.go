package deviceconfigs

import (
	"encoding/json"
	"testing"
)

func TestExcludeFieldsAcceptsSectionForms(t *testing.T) {
	t.Parallel()
	base := func() CanonicalPayload {
		return CanonicalPayload{
			Config: map[string]json.RawMessage{
				"lora":   json.RawMessage(`{"region":"EU_868","tx_power":20}`),
				"device": json.RawMessage(`{"tzdef":"X"}`),
			},
			ModuleConfig: map[string]json.RawMessage{
				"mqtt": json.RawMessage(`{"address":"localhost","username":"admin"}`),
			},
		}
	}

	cases := []struct {
		name        string
		exclude     string
		wantLoraOut bool // true if config.lora should be gone
	}{
		{"bare key", "lora", true},
		{"group-prefixed (as shown in diff)", "config.lora", true},
		{"unrelated key keeps lora", "device", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := base().ExcludeFields([]string{tc.exclude})
			_, loraPresent := out.Config["lora"]
			if loraPresent == tc.wantLoraOut {
				t.Fatalf("exclude %q: lora present=%v, want gone=%v", tc.exclude, loraPresent, tc.wantLoraOut)
			}
		})
	}
}

func TestExcludeFieldsFieldPathForms(t *testing.T) {
	t.Parallel()
	p := CanonicalPayload{
		Config: map[string]json.RawMessage{
			"lora": json.RawMessage(`{"region":"EU_868","tx_power":20}`),
		},
		ModuleConfig: map[string]json.RawMessage{
			"mqtt": json.RawMessage(`{"address":"localhost","username":"admin"}`),
		},
	}
	// Both the bare and group-prefixed field paths drop just the field.
	for _, path := range []string{"lora.region", "config.lora.region"} {
		out := p.ExcludeFields([]string{path})
		raw, ok := out.Config["lora"]
		if !ok {
			t.Fatalf("%s: lora section should remain", path)
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("%s: unmarshal: %v", path, err)
		}
		if _, present := m["region"]; present {
			t.Fatalf("%s: region should have been removed, got %v", path, m)
		}
		if _, present := m["tx_power"]; !present {
			t.Fatalf("%s: tx_power should remain", path)
		}
	}
}
