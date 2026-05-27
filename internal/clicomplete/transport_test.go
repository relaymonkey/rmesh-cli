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
	if len(items) == 0 {
		t.Fatal("expected at least one serial: candidate")
	}
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
