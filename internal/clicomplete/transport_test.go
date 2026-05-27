package clicomplete

import (
	"context"
	"strings"
	"testing"
)

func TestTransportURLProvider(t *testing.T) {
	items, _, err := TransportURLProvider(t.Context(), "serial:")
	if err != nil {
		t.Fatal(err)
	}
	// CI runners have no serial ports; tolerate an empty result, but if any
	// candidates come back they must all be serial: candidates.
	for _, it := range items {
		if !strings.HasPrefix(it.Value, "serial:") {
			t.Fatalf("unexpected value %q", it.Value)
		}
	}
}

func TestTransportURLProviderSchemeHints(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	items, _, err := TransportURLProvider(ctx, "ble")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, it := range items {
		if it.Value == "ble://" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected ble:// hint, got %#v", items)
	}
}
