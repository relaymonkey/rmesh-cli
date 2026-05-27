//go:build darwin

package transport

import (
	"context"
	"sync"
	"time"

	"tinygo.org/x/bluetooth"
)

func runBLEConnectScan(ctx context.Context, onResult func(bleAdvertisement)) error {
	if err := enableBLEAdapter(); err != nil {
		return err
	}

	adapter := bluetooth.DefaultAdapter
	scanDone := make(chan error, 1)
	go func() {
		scanDone <- adapter.Scan(func(_ *bluetooth.Adapter, result bluetooth.ScanResult) {
			onResult(advertisementFromScan(result))
		})
	}()

	var stopOnce sync.Once
	stopScan := func() {
		stopOnce.Do(func() { _ = adapter.StopScan() })
	}

	go func() {
		<-ctx.Done()
		stopScan()
	}()

	<-ctx.Done()
	stopScan()

	var scanErr error
	select {
	case scanErr = <-scanDone:
	case <-time.After(2 * time.Second):
		scanErr = ctx.Err()
	}
	return scanErr
}
