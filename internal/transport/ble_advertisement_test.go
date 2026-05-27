package transport

import "testing"

func TestIsMeshtasticAdvertisement(t *testing.T) {
	tests := []struct {
		name string
		adv  bleAdvertisement
		want bool
	}{
		{
			name: "mesh service uuid",
			adv:  bleAdvertisement{meshService: true},
			want: true,
		},
		{
			name: "meshtastic name",
			adv:  bleAdvertisement{name: "Meshtastic_ab12"},
			want: true,
		},
		{
			name: "custom short name",
			adv:  bleAdvertisement{name: "R001_4e3d"},
			want: true,
		},
		{
			name: "unrelated",
			adv:  bleAdvertisement{name: "soundcoreSpaceQ45"},
			want: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isMeshtasticAdvertisement(tc.adv); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestMergeAdvertisement(t *testing.T) {
	cur := bleAdvertisement{
		addressKey: "id-1",
		url:        "ble://id-1",
		rssi:       -70,
		meshService: true,
	}
	next := bleAdvertisement{
		addressKey: "id-1",
		name:       "Meshtastic_ab12",
		url:        "ble://Meshtastic_ab12",
		rssi:       -55,
		meshService: true,
	}
	merged := mergeAdvertisement(cur, next)
	if merged.name != "Meshtastic_ab12" || merged.url != "ble://Meshtastic_ab12" || merged.rssi != -55 {
		t.Fatalf("merged = %#v", merged)
	}
}
