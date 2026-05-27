package clicomplete

import "testing"

func TestTransportCompletionStage(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "scheme"},
		{"s", "scheme"},
		{"serial", "scheme"},
		{"serial:", "serial"},
		{"serial:/dev/ttyUSB0", "serial"},
		{"ble", "scheme"},
		{"ble://", "ble"},
		{"ble://Meshtastic_ab12", "ble"},
		{"http://127.0.0.1:4403", "http"},
		{"https://host", "http"},
	}
	for _, tc := range tests {
		if got := transportCompletionStage(tc.input); got != tc.want {
			t.Errorf("transportCompletionStage(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestTransportURLProviderEmptyIsSchemeOnly(t *testing.T) {
	items, _, err := TransportURLProvider(t.Context(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) == 0 {
		t.Fatal("expected scheme hints")
	}
	for _, it := range items {
		switch it.Value {
		case "serial:", "ble://", "http://", "https://":
			continue
		default:
			if len(it.Value) > 0 && it.Value != "" {
				// config URL is allowed
				if it.Description == "from agent config" {
					continue
				}
				t.Fatalf("unexpected enumerated device %q on empty completion", it.Value)
			}
		}
	}
}
