package transport

import (
	"strings"
	"time"

	"tinygo.org/x/bluetooth"
)

// BLEDevice is a Meshtastic radio discovered during a BLE scan.
type BLEDevice struct {
	URL    string
	AltURL string // optional second connect form (macOS peripheral UUID)
	Name   string
	RSSI   int16
}

const bleDiscoverScanTimeout = 10 * time.Second

type bleAdvertisement struct {
	addressKey  string
	address     bluetooth.Address
	url         string
	name        string
	rssi        int16
	meshService bool
}

func (a bleAdvertisement) device() BLEDevice {
	name := a.name
	if name == "" {
		name = a.addressKey
	}
	out := BLEDevice{
		URL:  a.url,
		Name: name,
		RSSI: a.rssi,
	}
	// Prefer CoreBluetooth UUID on macOS when both name and UUID are known — matches meshtastic --ble-scan.
	if a.name != "" && a.addressKey != "" && a.addressKey != a.name && strings.Contains(a.addressKey, "-") {
		out.AltURL = "ble://" + a.addressKey
	}
	return out
}

func advertisementFromScan(result bluetooth.ScanResult) bleAdvertisement {
	return bleAdvertisement{
		addressKey:  result.Address.String(),
		address:     result.Address,
		url:         bleURLFromScan(result),
		name:        result.LocalName(),
		rssi:        result.RSSI,
		meshService: result.HasServiceUUID(meshServiceUUID),
	}
}

func isMeshtasticAdvertisement(adv bleAdvertisement) bool {
	if adv.meshService {
		return true
	}
	name := strings.ToLower(adv.name)
	if strings.HasPrefix(name, "meshtastic_") {
		return true
	}
	// Custom short names (e.g. R001_4e3d) ride in the scan response alongside the mesh service.
	return adv.name != "" && strings.Contains(adv.name, "_")
}

func mergeAdvertisement(cur, next bleAdvertisement) bleAdvertisement {
	if cur.addressKey == "" {
		return next
	}
	out := cur
	if next.name != "" {
		out.name = next.name
	}
	if next.url != "" {
		out.url = next.url
	}
	if next.rssi > out.rssi {
		out.rssi = next.rssi
	}
	out.meshService = out.meshService || next.meshService
	return out
}
