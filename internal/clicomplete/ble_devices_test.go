package clicomplete

import "testing"

func TestShouldDiscoverBLE(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"ble", false},
		{"ble://", true},
		{"ble://Meshtastic_ab12", true},
		{"serial:", false},
		{"serial:/dev/ttyUSB0", false},
		{"http://", false},
	}
	for _, tc := range tests {
		if got := shouldDiscoverBLE(tc.in); got != tc.want {
			t.Fatalf("shouldDiscoverBLE(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
