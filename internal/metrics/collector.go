package metrics

import (
	"time"

	"github.com/exepirit/meshtastic-go/pkg/meshtastic/proto"

	"github.com/relaymonkey/relaymesh-edge/internal/nodeid"
)

const (
	SourceTelemetry = "telemetry"
	SourceNodeDB    = "nodedb"
)

// UpdateDeviceMetrics records RF utilization gauges for one node.
func (r *Registry) UpdateDeviceMetrics(nodeNum uint32, source string, dm *proto.DeviceMetrics) {
	if r == nil || dm == nil || nodeNum == 0 {
		return
	}
	nodeID := nodeid.FromNum(nodeNum)
	labels := prometheusLabels(r.agentID, r.gatewayID, nodeID, source)

	if dm.ChannelUtilization != nil {
		r.channelUtil.WithLabelValues(labels...).Set(float64(dm.GetChannelUtilization()) / 100.0)
	}
	if dm.AirUtilTx != nil {
		r.airUtilTx.WithLabelValues(labels...).Set(float64(dm.GetAirUtilTx()) / 100.0)
	}

	r.updatedAt.WithLabelValues(r.agentID, r.gatewayID, nodeID).Set(float64(time.Now().Unix()))
}

func prometheusLabels(agentID, gatewayID, nodeID, source string) []string {
	return []string{agentID, gatewayID, nodeID, source}
}
