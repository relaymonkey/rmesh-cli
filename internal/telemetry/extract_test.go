package telemetry_test

import (
	"testing"

	meshtasticproto "github.com/exepirit/meshtastic-go/pkg/meshtastic/proto"
	"google.golang.org/protobuf/proto"

	"github.com/relaymonkey/relaymesh-edge/internal/telemetry"
)

func TestExtractDeviceMetrics(t *testing.T) {
	chu := float32(28.5)
	tx := float32(4.2)
	payload, err := proto.Marshal(&meshtasticproto.Telemetry{
		Variant: &meshtasticproto.Telemetry_DeviceMetrics{
			DeviceMetrics: &meshtasticproto.DeviceMetrics{
				ChannelUtilization: &chu,
				AirUtilTx:            &tx,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	packet := &meshtasticproto.MeshPacket{
		From: 0x12345678,
		PayloadVariant: &meshtasticproto.MeshPacket_Decoded{
			Decoded: &meshtasticproto.Data{
				Portnum: meshtasticproto.PortNum_TELEMETRY_APP,
				Payload: payload,
			},
		},
	}

	num, dm, ok := telemetry.ExtractDeviceMetrics(packet)
	if !ok {
		t.Fatal("expected ok")
	}
	if num != 0x12345678 {
		t.Fatalf("from = %#x", num)
	}
	if dm.GetChannelUtilization() != chu || dm.GetAirUtilTx() != tx {
		t.Fatalf("dm = %+v", dm)
	}
}

func TestExtractDeviceMetricsSkipsNonTelemetry(t *testing.T) {
	packet := &meshtasticproto.MeshPacket{
		From: 1,
		PayloadVariant: &meshtasticproto.MeshPacket_Decoded{
			Decoded: &meshtasticproto.Data{
				Portnum: meshtasticproto.PortNum_TEXT_MESSAGE_APP,
			},
		},
	}
	if _, _, ok := telemetry.ExtractDeviceMetrics(packet); ok {
		t.Fatal("expected false for text message")
	}
}

func TestDeviceMetricsFromNodeInfo(t *testing.T) {
	chu := float32(30.0)
	node := &meshtasticproto.NodeInfo{
		Num: 42,
		DeviceMetrics: &meshtasticproto.DeviceMetrics{
			ChannelUtilization: &chu,
		},
	}
	num, dm, ok := telemetry.DeviceMetricsFromNodeInfo(node)
	if !ok || num != 42 || dm.GetChannelUtilization() != chu {
		t.Fatalf("got num=%d dm=%+v ok=%v", num, dm, ok)
	}
}
