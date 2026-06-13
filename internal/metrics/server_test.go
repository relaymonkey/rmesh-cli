package metrics_test

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	meshtasticproto "github.com/exepirit/meshtastic-go/pkg/meshtastic/proto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/relaymonkey/relaymesh-edge/internal/metrics"
)

func TestMetricsHTTPHandler(t *testing.T) {
	reg := metrics.NewRegistry("agent-a", "!gw")
	chu := float32(12.0)
	reg.UpdateDeviceMetrics(1, metrics.SourceNodeDB, &meshtasticproto.DeviceMetrics{
		ChannelUtilization: &chu,
	})
	handler := promhttp.HandlerFor(reg.Prometheus(), promhttp.HandlerOpts{Registry: reg.Prometheus()})

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "rmesh_node_channel_utilization_ratio") {
		t.Fatalf("missing series: %s", body)
	}
	if !strings.Contains(body, "go_goroutines") {
		t.Fatal("expected process collectors on /metrics")
	}
}

func TestStartBindFailure(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()

	reg := metrics.NewRegistry("agent-a", "!gw")
	errCh := make(chan error, 1)
	go func() {
		errCh <- metrics.Start(context.Background(), occupied.Addr().String(), reg)
	}()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected bind error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for bind error")
	}
}

func TestStartServesMetrics(t *testing.T) {
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := probe.Addr().String()
	probe.Close()

	reg := metrics.NewRegistry("agent-a", "!gw")
	chu := float32(8.5)
	reg.UpdateDeviceMetrics(2, metrics.SourceTelemetry, &meshtasticproto.DeviceMetrics{
		ChannelUtilization: &chu,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- metrics.Start(ctx, addr, reg)
	}()

	resp, err := http.Get("http://" + addr + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d body = %s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), "rmesh_node_channel_utilization_ratio") {
		t.Fatalf("missing series: %s", body)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil && err != context.Canceled {
			t.Fatalf("unexpected stop error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for server shutdown")
	}
}
