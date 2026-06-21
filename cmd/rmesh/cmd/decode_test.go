package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/exepirit/meshtastic-go/pkg/meshtastic/proto"
	protobuf "google.golang.org/protobuf/proto"

	"github.com/relaymonkey/relaymesh-edge/internal/decrypt"
	"github.com/relaymonkey/relaymesh-edge/internal/observe"
)

func TestRunDecodePipeline(t *testing.T) {
	ni := &proto.NeighborInfo{
		NodeId:    1,
		Neighbors: []*proto.Neighbor{{NodeId: 2, Snr: 3.5}},
	}
	payload, _ := protobuf.Marshal(ni)
	packet := &proto.MeshPacket{
		From: 1,
		Id:   7,
		PayloadVariant: &proto.MeshPacket_Decoded{
			Decoded: &proto.Data{
				Portnum: proto.PortNum_NEIGHBORINFO_APP,
				Payload: payload,
			},
		},
	}
	ev := observe.Event{
		Kind:          "packet",
		Portnum:       71,
		MeshPacketB64: observe.MeshPacketB64(packet),
	}
	var in bytes.Buffer
	enc := observe.New(&in)
	if err := enc.Write(ev); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := streamDecode(&in, &out, decrypt.DefaultPSKInput, false); err != nil {
		t.Fatal(err)
	}
	line := strings.TrimSpace(out.String())
	if !strings.Contains(line, `"portnum_name":"NEIGHBORINFO_APP"`) &&
		!strings.Contains(line, `"portnum_name": "NEIGHBORINFO_APP"`) {
		t.Fatalf("missing decoded neighbor info: %s", line)
	}
}

func TestRunDecodePipelineStrip(t *testing.T) {
	ni := &proto.NeighborInfo{NodeId: 1, Neighbors: []*proto.Neighbor{{NodeId: 2, Snr: 3.5}}}
	payload, _ := protobuf.Marshal(ni)
	packet := &proto.MeshPacket{
		From: 1,
		Id:   7,
		PayloadVariant: &proto.MeshPacket_Decoded{
			Decoded: &proto.Data{
				Portnum: proto.PortNum_NEIGHBORINFO_APP,
				Payload: payload,
			},
		},
	}
	ev := observe.Event{
		Kind:          "packet",
		Portnum:       71,
		MeshPacketB64: observe.MeshPacketB64(packet),
	}
	var in bytes.Buffer
	enc := observe.New(&in)
	if err := enc.Write(ev); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := streamDecode(&in, &out, decrypt.DefaultPSKInput, true); err != nil {
		t.Fatal(err)
	}
	line := strings.TrimSpace(out.String())
	if strings.Contains(line, "mesh_packet_b64") {
		t.Fatalf("mesh_packet_b64 should be stripped: %s", line)
	}
	if !strings.Contains(line, "NEIGHBORINFO_APP") {
		t.Fatalf("missing decoded neighbor info: %s", line)
	}
}
