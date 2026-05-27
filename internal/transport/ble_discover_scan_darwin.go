//go:build darwin

package transport

import (
	"context"
	"sync"
	"time"

	"github.com/tinygo-org/cbgo"
)

type bleDiscoverCentralDelegate struct {
	cbgo.CentralManagerDelegateBase

	onResult func(bleAdvertisement)
	powered  chan struct{}
}

func (d *bleDiscoverCentralDelegate) CentralManagerDidUpdateState(cmgr cbgo.CentralManager) {
	if cmgr.State() == cbgo.ManagerStatePoweredOn {
		select {
		case <-d.powered:
		default:
			close(d.powered)
		}
	}
}

func (d *bleDiscoverCentralDelegate) DidDiscoverPeripheral(_ cbgo.CentralManager, prph cbgo.Peripheral, advFields cbgo.AdvFields, rssi int) {
	if d.onResult == nil {
		return
	}
	d.onResult(advertisementFromPeripheral(prph, advFields, rssi))
}

// runBLEDiscoveryScan uses CoreBluetooth duplicate advertisements so scan
// responses can merge a local name (e.g. R001_4e3d) with the mesh service UUID.
// Tab completion and device listing only — connect scans on DefaultAdapter.
func runBLEDiscoveryScan(ctx context.Context, onResult func(bleAdvertisement)) error {
	delegate := &bleDiscoverCentralDelegate{
		onResult: onResult,
		powered:  make(chan struct{}),
	}

	cm := cbgo.NewCentralManager(nil)
	cm.SetDelegate(delegate)
	if cm.State() != cbgo.ManagerStatePoweredOn {
		select {
		case <-delegate.powered:
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
			return context.DeadlineExceeded
		}
	}

	meshUUID, err := cbgo.ParseUUID(meshServiceUUID.String())
	if err != nil {
		return err
	}

	var stopOnce sync.Once
	stopScan := func() {
		stopOnce.Do(func() { cm.StopScan() })
	}

	scanDone := make(chan struct{})
	cm.Scan([]cbgo.UUID{meshUUID}, &cbgo.CentralManagerScanOpts{
		AllowDuplicates: true,
	})

	go func() {
		<-ctx.Done()
		stopScan()
		close(scanDone)
	}()

	<-ctx.Done()
	stopScan()
	waitScanDone(scanDone, 2*time.Second)
	return ctx.Err()
}

func advertisementFromPeripheral(prph cbgo.Peripheral, advFields cbgo.AdvFields, rssi int) bleAdvertisement {
	id := prph.Identifier().String()
	out := bleAdvertisement{
		addressKey: id,
		name:       advFields.LocalName,
		rssi:       int16(rssi),
	}
	out.address.Set(id)

	mesh := meshServiceUUID.String()
	for _, u := range advFields.ServiceUUIDs {
		if u.String() == mesh {
			out.meshService = true
			break
		}
	}
	out.url = bleURLFromAdvertisement(out)
	return out
}

func bleURLFromAdvertisement(adv bleAdvertisement) string {
	if adv.name != "" {
		return "ble://" + adv.name
	}
	if adv.addressKey != "" {
		return "ble://" + adv.addressKey
	}
	return ""
}
