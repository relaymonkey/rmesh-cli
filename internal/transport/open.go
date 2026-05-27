// Package transport opens local radio connections without pulling BLE deps.
package transport

import (
	"fmt"
	"net/url"

	"github.com/exepirit/meshtastic-go/pkg/meshtastic"
	"github.com/exepirit/meshtastic-go/pkg/meshtastic/http"
)

// Open creates a hardware transport from a URL string.
//
// Supported schemes:
//   - serial:/dev/ttyUSB0
//   - http://192.168.1.10:4403
//   - https://meshtastic.local:4403
func Open(connectionString string) (meshtastic.HardwareTransport, error) {
	u, err := url.Parse(connectionString)
	if err != nil {
		return nil, fmt.Errorf("invalid connection string: %w", err)
	}
	switch u.Scheme {
	case "serial":
		return openSerial(u.Path)
	case "http", "https":
		return &http.Transport{URL: u.String()}, nil
	default:
		return nil, fmt.Errorf("unsupported transport scheme %q (MVP: serial, http, https)", u.Scheme)
	}
}
