package clicomplete

import (
	"context"
	"sync"
	"time"

	rmtransport "github.com/relaymonkey/relaymesh-edge/internal/transport"
)

const bleDiscoverTimeout = 20 * time.Second // hard cap (adaptive window decides actual duration)

var (
	bleDeviceCacheMu  sync.Mutex
	bleDeviceCache    []rmtransport.BLEDevice
	bleDeviceCacheAt  time.Time
	bleDeviceCacheTTL = 15 * time.Second
)

func cachedBLEDevices(ctx context.Context) []rmtransport.BLEDevice {
	if ctx.Err() != nil {
		return nil
	}
	bleDeviceCacheMu.Lock()
	if time.Since(bleDeviceCacheAt) < bleDeviceCacheTTL && len(bleDeviceCache) > 0 {
		cached := append([]rmtransport.BLEDevice(nil), bleDeviceCache...)
		bleDeviceCacheMu.Unlock()
		return cached
	}
	bleDeviceCacheMu.Unlock()

	scanCtx, cancel := context.WithTimeout(ctx, bleDiscoverTimeout)
	defer cancel()

	devices, err := rmtransport.DiscoverBLE(scanCtx)
	if err != nil || len(devices) == 0 {
		return nil
	}

	bleDeviceCacheMu.Lock()
	bleDeviceCache = append([]rmtransport.BLEDevice(nil), devices...)
	bleDeviceCacheAt = time.Now()
	cached := append([]rmtransport.BLEDevice(nil), bleDeviceCache...)
	bleDeviceCacheMu.Unlock()
	return cached
}
