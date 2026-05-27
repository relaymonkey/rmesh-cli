package transport

import (
	"net/url"
	"testing"
)

func TestMeshtasticAdvertisedName(t *testing.T) {
	name, err := meshtasticAdvertisedName("00:1A:7D:DA:71:13")
	if err != nil {
		t.Fatal(err)
	}
	if name != "Meshtastic_7113" {
		t.Fatalf("got %q, want Meshtastic_7113", name)
	}
}

func TestBLETarget(t *testing.T) {
	tests := []struct {
		raw      string
		wantMAC  string
		wantName string
		wantErr  bool
	}{
		{raw: "ble://00:1A:7D:DA:71:13", wantMAC: "00:1A:7D:DA:71:13"},
		{raw: "ble:/00:1A:7D:DA:71:13", wantMAC: "00:1A:7D:DA:71:13"},
		{raw: "ble:00:1A:7D:DA:71:13", wantMAC: "00:1A:7D:DA:71:13"},
		{raw: "ble://Meshtastic_ab12", wantName: "Meshtastic_ab12"},
		{raw: "ble://", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.raw, func(t *testing.T) {
			u, err := url.Parse(normalizeBLEMACURL(tc.raw))
			if err != nil {
				t.Fatal(err)
			}
			mac, name, err := bleTarget(u)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if mac != tc.wantMAC || name != tc.wantName {
				t.Fatalf("got mac=%q name=%q, want mac=%q name=%q", mac, name, tc.wantMAC, tc.wantName)
			}
		})
	}
}

func TestNormalizeBLEMACURL(t *testing.T) {
	tests := []struct{ in, want string }{
		{"ble://00:1A:7D:DA:71:13", "ble:00:1A:7D:DA:71:13"},
		{"BLE://00:1A:7D:DA:71:13", "ble:00:1A:7D:DA:71:13"},
		{"ble://Meshtastic_ab12", "ble://Meshtastic_ab12"},
		{"ble://A890DB11-E4B8-ADD4-D2A1-060A4F2E5FA9", "ble://A890DB11-E4B8-ADD4-D2A1-060A4F2E5FA9"},
		{"serial:/dev/ttyUSB0", "serial:/dev/ttyUSB0"},
		{"ble:00:1A:7D:DA:71:13", "ble:00:1A:7D:DA:71:13"},
	}
	for _, tc := range tests {
		if got := normalizeBLEMACURL(tc.in); got != tc.want {
			t.Errorf("normalizeBLEMACURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
