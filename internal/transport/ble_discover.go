package transport

import (
	"context"
	"sort"
	"sync"
	"time"
)

// Adaptive BLE discovery: scan for BLEDiscoveryMinWindow, and every time a
// previously-unseen Meshtastic radio appears, extend the deadline by
// BLEDiscoveryExtendWindow. Stops when the window expires without any new
// device, or when the hard cap BLEDiscoveryMaxWindow is reached. Exposed as
// vars (not consts) so tab-completion / doctor / future flags can tune them.
var (
	BLEDiscoveryMinWindow    = 2 * time.Second
	BLEDiscoveryExtendWindow = 2 * time.Second
	BLEDiscoveryMaxWindow    = 10 * time.Second
)

// DiscoverBLE scans for nearby Meshtastic BLE devices using an adaptive
// window. It returns when the inactivity window expires, the hard cap is
// reached, or ctx is cancelled.
func DiscoverBLE(ctx context.Context) ([]BLEDevice, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil
	}

	var (
		mu     sync.Mutex
		byKey  = make(map[string]bleAdvertisement)
		newHit = make(chan struct{}, 1)
	)

	scanCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	scanErr := make(chan error, 1)
	go func() {
		scanErr <- runBLEDiscoveryScan(scanCtx, func(adv bleAdvertisement) {
			if !isMeshtasticAdvertisement(adv) || adv.url == "" {
				return
			}
			key := adv.addressKey
			if key == "" {
				key = adv.url
			}
			mu.Lock()
			prev, existed := byKey[key]
			if existed {
				byKey[key] = mergeAdvertisement(prev, adv)
			} else {
				byKey[key] = adv
			}
			mu.Unlock()
			if !existed {
				select {
				case newHit <- struct{}{}:
				default:
				}
			}
		})
	}()

	hardDeadline := time.NewTimer(BLEDiscoveryMaxWindow)
	defer hardDeadline.Stop()
	inactivity := time.NewTimer(BLEDiscoveryMinWindow)
	defer inactivity.Stop()

loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case <-hardDeadline.C:
			break loop
		case <-inactivity.C:
			break loop
		case <-newHit:
			if !inactivity.Stop() {
				select {
				case <-inactivity.C:
				default:
				}
			}
			inactivity.Reset(BLEDiscoveryExtendWindow)
		}
	}

	cancel()
	// Drain the goroutine so we don't leak it; the actual scan-stop error is
	// uninteresting for discovery (cancel-driven shutdown).
	select {
	case <-scanErr:
	case <-time.After(2 * time.Second):
	}

	mu.Lock()
	out := make([]BLEDevice, 0, len(byKey))
	for _, adv := range byKey {
		out = append(out, adv.device())
	}
	mu.Unlock()

	sort.Slice(out, func(i, j int) bool {
		if out[i].RSSI != out[j].RSSI {
			return out[i].RSSI > out[j].RSSI
		}
		return out[i].URL < out[j].URL
	})

	return out, nil
}

func discoverBLEAdvertisement(ctx context.Context, match bleMatcher) (bleAdvertisement, error) {
	var (
		mu    sync.Mutex
		found bleAdvertisement
		hit   bool
	)

	scanCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	err := runBLEConnectScan(scanCtx, func(adv bleAdvertisement) {
		if !match(adv) {
			return
		}
		mu.Lock()
		if !hit || adv.rssi > found.rssi {
			found = mergeAdvertisement(found, adv)
			hit = true
		}
		mu.Unlock()
		cancel()
	})

	mu.Lock()
	defer mu.Unlock()
	if hit {
		return found, nil
	}
	if err != nil && err != context.Canceled {
		return bleAdvertisement{}, err
	}
	return bleAdvertisement{}, err
}

func waitScanDone(scanDone <-chan struct{}, fallback time.Duration) {
	select {
	case <-scanDone:
	case <-time.After(fallback):
	}
}
