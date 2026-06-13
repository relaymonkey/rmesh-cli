package metrics

import (
	"fmt"
	"net"
)

// CheckListenAddr verifies that listenAddr can be bound for the /metrics server.
// Used by agent doctor before a long run.
func CheckListenAddr(listenAddr string) error {
	if listenAddr == "" {
		return fmt.Errorf("listen address is empty")
	}
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("metrics listen %s: %w", listenAddr, err)
	}
	return ln.Close()
}
