package metrics_test

import (
	"net"
	"testing"

	"github.com/relaymonkey/relaymesh-edge/internal/metrics"
)

func TestCheckListenAddrAvailable(t *testing.T) {
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := probe.Addr().String()
	probe.Close()

	if err := metrics.CheckListenAddr(addr); err != nil {
		t.Fatalf("expected available port: %v", err)
	}
}

func TestCheckListenAddrOccupied(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()

	if err := metrics.CheckListenAddr(occupied.Addr().String()); err == nil {
		t.Fatal("expected bind error for occupied port")
	}
}
