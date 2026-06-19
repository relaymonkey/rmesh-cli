package transport

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"syscall"
	"time"

	"github.com/exepirit/meshtastic-go/pkg/meshtastic"
)

const defaultReconnectPoll = 500 * time.Millisecond

// IsDisconnect reports whether err likely means the radio dropped the local
// transport (USB serial vanished mid-write, BLE link lost, etc.). macOS
// surfaces this as ENXIO ("device not configured") on serial writes.
func IsDisconnect(err error) bool {
	if err == nil {
		return false
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		switch errno {
		case syscall.ENXIO, syscall.ENODEV, syscall.EIO, syscall.ECONNRESET, syscall.EPIPE:
			return true
		}
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "device not configured") ||
		strings.Contains(msg, "not connected") ||
		strings.Contains(msg, "port has been closed") ||
		strings.Contains(msg, "no such file or directory")
}

// WaitReady polls Open until a transport connects or the deadline expires.
// Typical use: the firmware scheduled a reboot (~7 s after a SetConfig that
// requires one); the USB CDC port disappears and re-enumerates on the same
// path a few seconds later.
func WaitReady(
	ctx context.Context,
	connectionString string,
	wait time.Duration,
	opts ...Options,
) (meshtastic.HardwareTransport, error) {
	if wait <= 0 {
		wait = 15 * time.Second
	}
	deadline, cancel := context.WithTimeout(ctx, wait)
	defer cancel()

	var lastErr error
	ticker := time.NewTicker(defaultReconnectPoll)
	defer ticker.Stop()

	for {
		t, err := Open(connectionString, opts...)
		if err == nil {
			return t, nil
		}
		lastErr = err
		select {
		case <-deadline.Done():
			if lastErr != nil {
				return nil, fmt.Errorf("%w (last open error: %v)", deadline.Err(), lastErr)
			}
			return nil, deadline.Err()
		case <-ticker.C:
		}
	}
}
