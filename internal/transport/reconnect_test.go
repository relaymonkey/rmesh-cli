package transport

import (
	"errors"
	"syscall"
	"testing"
)

func TestIsDisconnect(t *testing.T) {
	t.Parallel()
	if IsDisconnect(nil) {
		t.Fatal("nil should not be disconnect")
	}
	if !IsDisconnect(syscall.ENXIO) {
		t.Fatal("ENXIO should be disconnect")
	}
	if !IsDisconnect(errors.New("write /dev/cu.usbmodem2101: device not configured")) {
		t.Fatal("macOS serial message should match")
	}
	if IsDisconnect(errors.New("parse config.position: invalid JSON")) {
		t.Fatal("semantic errors should not match")
	}
}
