//go:build darwin

package transport

import (
	"tinygo.org/x/bluetooth"
)

func bleURLFromScan(result bluetooth.ScanResult) string {
	if name := result.LocalName(); name != "" {
		return "ble://" + name
	}
	if id := result.Address.String(); id != "" {
		return "ble://" + id
	}
	return ""
}

func isBLEPeripheralID(s string) bool {
	_, err := bluetooth.ParseUUID(s)
	return err == nil
}
