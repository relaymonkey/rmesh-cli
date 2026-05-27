// Package transport opens local radio connections (serial, HTTP, BLE).
package transport

import (
	"fmt"
	"net/url"

	"github.com/exepirit/meshtastic-go/pkg/meshtastic"
	"github.com/exepirit/meshtastic-go/pkg/meshtastic/http"
)

// Options holds optional knobs that apply to specific transports.
// Unknown fields are ignored by transports that don't use them.
type Options struct {
	// BLEPin is the FIXED_PIN passkey for Meshtastic BLE pairing. Currently
	// informational — pairing is brokered by the host OS (CoreBluetooth /
	// BlueZ); rmesh logs the configured PIN but cannot inject it yet.
	BLEPin string
}

// Open creates a hardware transport from a URL string.
//
// Supported schemes:
//   - serial:/dev/ttyUSB0
//   - http://192.168.1.10:4403
//   - https://meshtastic.local:4403
//   - ble://AA:BB:CC:DD:EE:FF  (or ble:/… / ble:…)
//   - ble://Meshtastic_ab12    (advertised name when host is not a MAC)
func Open(connectionString string, opts ...Options) (meshtastic.HardwareTransport, error) {
	var o Options
	if len(opts) > 0 {
		o = opts[0]
	}
	u, err := url.Parse(connectionString)
	if err != nil {
		return nil, fmt.Errorf("invalid connection string: %w", err)
	}
	switch u.Scheme {
	case "serial":
		return openSerial(u.Path)
	case "http", "https":
		return &http.Transport{URL: u.String()}, nil
	case "ble":
		return openBLE(u, o)
	default:
		return nil, fmt.Errorf("unsupported transport scheme %q (use serial, http, https, or ble)", u.Scheme)
	}
}
