package transport

import (
	"io"

	"github.com/exepirit/meshtastic-go/pkg/meshtastic"
)

// Close closes the transport when supported.
func Close(t meshtastic.HardwareTransport) error {
	if closer, ok := t.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}
