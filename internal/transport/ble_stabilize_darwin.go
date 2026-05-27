//go:build darwin

package transport

import "time"

func stabilizeBLEConnection() {
	// CoreBluetooth needs a beat after connect before GATT writes succeed.
	time.Sleep(250 * time.Millisecond)
}
