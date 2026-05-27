//go:build darwin

package transport

import (
	"fmt"

	"tinygo.org/x/bluetooth"
)

func newBLEMatcher(mac, name string) (bleMatcher, error) {
	switch {
	case mac != "":
		if isBLEPeripheralID(mac) {
			return matchPeripheralID(mac), nil
		}
		advName, err := meshtasticAdvertisedName(mac)
		if err != nil {
			return nil, fmt.Errorf("invalid BLE MAC %q: %w", mac, err)
		}
		return matchName(advName), nil
	case name != "":
		if isBLEPeripheralID(name) {
			return matchPeripheralID(name), nil
		}
		return matchName(name), nil
	default:
		return nil, fmt.Errorf("ble URL missing MAC or device name")
	}
}

func matchPeripheralID(id string) bleMatcher {
	parsed, err := bluetooth.ParseUUID(id)
	if err != nil {
		return matchNoDevice
	}
	return func(adv bleAdvertisement) bool {
		if adv.address.UUID == parsed {
			return true
		}
		advUUID, err := bluetooth.ParseUUID(adv.addressKey)
		return err == nil && advUUID == parsed
	}
}
