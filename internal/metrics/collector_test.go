package metrics_test

import (
	"strings"
	"testing"

	"github.com/exepirit/meshtastic-go/pkg/meshtastic/proto"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/relaymonkey/relaymesh-edge/internal/metrics"
)

func TestUpdateDeviceMetrics(t *testing.T) {
	reg := metrics.NewRegistry("agent-a", "!gateway01")
	chu := float32(25.0)
	tx := float32(3.5)
	reg.UpdateDeviceMetrics(0x01020304, metrics.SourceTelemetry, &proto.DeviceMetrics{
		ChannelUtilization: &chu,
		AirUtilTx:            &tx,
	})

	const chuMetric = `
		# HELP rmesh_node_channel_utilization_ratio Observed channel utilization for a mesh node (0-1; Meshtastic reports percent).
		# TYPE rmesh_node_channel_utilization_ratio gauge
		rmesh_node_channel_utilization_ratio{agent_id="agent-a",gateway_id="!gateway01",node_id="!01020304",source="telemetry"} 0.25
	`
	if err := testutil.CollectAndCompare(reg.Prometheus(), strings.NewReader(chuMetric), "rmesh_node_channel_utilization_ratio"); err != nil {
		t.Fatal(err)
	}
}
