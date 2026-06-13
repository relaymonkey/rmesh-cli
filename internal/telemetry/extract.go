// Package telemetry decodes Meshtastic TELEMETRY_APP payloads for local observability.
package telemetry

import (
	meshtasticproto "github.com/exepirit/meshtastic-go/pkg/meshtastic/proto"
	"google.golang.org/protobuf/proto"
)

// ExtractDeviceMetrics returns device RF metrics from a TELEMETRY_APP mesh packet.
func ExtractDeviceMetrics(packet *meshtasticproto.MeshPacket) (nodeNum uint32, dm *meshtasticproto.DeviceMetrics, ok bool) {
	if packet == nil {
		return 0, nil, false
	}
	decoded := packet.GetDecoded()
	if decoded == nil || decoded.GetPortnum() != meshtasticproto.PortNum_TELEMETRY_APP {
		return 0, nil, false
	}
	var tel meshtasticproto.Telemetry
	if err := proto.Unmarshal(decoded.GetPayload(), &tel); err != nil {
		return 0, nil, false
	}
	dm = tel.GetDeviceMetrics()
	if dm == nil {
		return 0, nil, false
	}
	if dm.ChannelUtilization == nil && dm.AirUtilTx == nil {
		return 0, nil, false
	}
	return packet.GetFrom(), dm, true
}

// DeviceMetricsFromNodeInfo returns device_metrics embedded in a NodeInfo row.
func DeviceMetricsFromNodeInfo(node *meshtasticproto.NodeInfo) (nodeNum uint32, dm *meshtasticproto.DeviceMetrics, ok bool) {
	if node == nil {
		return 0, nil, false
	}
	dm = node.GetDeviceMetrics()
	if dm == nil {
		return 0, nil, false
	}
	if dm.ChannelUtilization == nil && dm.AirUtilTx == nil {
		return 0, nil, false
	}
	return node.GetNum(), dm, true
}
