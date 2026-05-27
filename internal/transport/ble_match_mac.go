//go:build !darwin

package transport

import (
	"fmt"

	"tinygo.org/x/bluetooth"
)

func newBLEMatcher(mac, name string) (bleMatcher, error) {
	switch {
	case mac != "":
		parsed, err := bluetooth.ParseMAC(mac)
		if err != nil {
			return nil, fmt.Errorf("invalid BLE MAC %q: %w", mac, err)
		}
		return func(adv bleAdvertisement) bool {
			return adv.address.MAC == parsed
		}, nil
	case name != "":
		return matchName(name), nil
	default:
		return nil, fmt.Errorf("ble URL missing MAC or device name")
	}
}
