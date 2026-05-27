//go:build !darwin

package transport

import (
	"strings"

	"tinygo.org/x/bluetooth"
)

func bleURLFromScan(result bluetooth.ScanResult) string {
	if name := result.LocalName(); strings.HasPrefix(name, "Meshtastic_") {
		return "ble://" + name
	}
	return "ble://" + result.Address.MAC.String()
}

func isBLEPeripheralID(string) bool {
	return false
}
