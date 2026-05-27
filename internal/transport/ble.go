package transport

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/exepirit/meshtastic-go/pkg/meshtastic"
	"tinygo.org/x/bluetooth"
)

const bleScanTimeout = 30 * time.Second

var (
	meshServiceUUID    = mustUUID("6ba1b218-15a8-461f-9fa8-5dcae273eafd")
	fromRadioCharUUID  = mustUUID("2c55e69e-4993-11ed-b878-0242ac120002")
	toRadioCharUUID    = mustUUID("f75c76d2-129e-4dad-a1dd-7866124401e7")
	fromNumCharUUID    = mustUUID("ed9da18c-a800-4f66-a670-aa7547e34453")
)

func mustUUID(s string) bluetooth.UUID {
	u, err := bluetooth.ParseUUID(s)
	if err != nil {
		panic(err)
	}
	return u
}

func openBLE(u *url.URL, opts Options) (meshtastic.HardwareTransport, error) {
	mac, name, err := bleTarget(u)
	if err != nil {
		return nil, err
	}
	slog.Info("ble: connecting over Bluetooth LE (scan + GATT handshake + full NodeDB sync — expect 30-60s on first connect; serial is faster if available)")
	if opts.BLEPin != "" {
		slog.Debug("ble: passkey configured (informational — pairing is OS-brokered)", "pin_digits", len(opts.BLEPin))
	}
	ctx, cancel := context.WithTimeout(context.Background(), bleScanTimeout)
	defer cancel()
	return connectBLE(ctx, mac, name)
}

// bleTarget extracts a BLE MAC address or advertised device name from a transport URL.
//
// Supported forms:
//   - ble://AA:BB:CC:DD:EE:FF
//   - ble:/AA:BB:CC:DD:EE:FF
//   - ble:AA:BB:CC:DD:EE:FF
//   - ble://Meshtastic_ab12  (name when the host is not a MAC)
//   - ble://AA11BB22-CC33-DD44-EE55-FF6677889900  (CoreBluetooth peripheral UUID on macOS)
func bleTarget(u *url.URL) (mac, name string, err error) {
	candidates := []string{
		u.Host,
		strings.TrimPrefix(u.Path, "/"),
		u.Opaque,
	}
	for _, raw := range candidates {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if isBLEMAC(raw) {
			return raw, "", nil
		}
		if name == "" {
			name = raw
		}
	}
	if name != "" {
		return "", name, nil
	}
	return "", "", fmt.Errorf("ble URL missing MAC or device name")
}

func isBLEMAC(s string) bool {
	_, err := bluetooth.ParseMAC(s)
	return err == nil
}

// meshtasticAdvertisedName returns the BLE local name Meshtastic firmware derives
// from the radio MAC (last two bytes of the wire-order address, lowercase hex).
func meshtasticAdvertisedName(mac string) (string, error) {
	parsed, err := bluetooth.ParseMAC(mac)
	if err != nil {
		return "", err
	}
	// tinygo stores MAC little-endian; indices 0/1 are the last two wire-order bytes.
	return fmt.Sprintf("Meshtastic_%02x%02x", parsed[1], parsed[0]), nil
}
