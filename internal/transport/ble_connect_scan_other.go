//go:build !darwin

package transport

import "context"

func runBLEConnectScan(ctx context.Context, onResult func(bleAdvertisement)) error {
	return runBLEDiscoveryScan(ctx, onResult)
}
