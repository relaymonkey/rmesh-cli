package decode

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/exepirit/meshtastic-go/pkg/meshtastic/proto"
	protobuf "google.golang.org/protobuf/proto"

	"github.com/relaymonkey/relaymesh-edge/internal/decrypt"
)

func TestDecodeNeighborInfoCleartext(t *testing.T) {
	ni := &proto.NeighborInfo{
		NodeId: 0x12345678,
		Neighbors: []*proto.Neighbor{
			{NodeId: 0xabcdef01, Snr: 8.5},
			{NodeId: 0xabcdef02, Snr: -2.0},
		},
	}
	payload, err := protobuf.Marshal(ni)
	if err != nil {
		t.Fatal(err)
	}
	packet := &proto.MeshPacket{
		From: 0x12345678,
		Id:   42,
		PayloadVariant: &proto.MeshPacket_Decoded{
			Decoded: &proto.Data{
				Portnum: proto.PortNum_NEIGHBORINFO_APP,
				Payload: payload,
			},
		},
	}
	got, err := DecodeMeshPacket(packet, decrypt.DefaultPSKInput)
	if err != nil {
		t.Fatal(err)
	}
	if got.PortnumName != "NEIGHBORINFO_APP" {
		t.Fatalf("portnum_name = %q", got.PortnumName)
	}
	if got.DecoderKind != decoderKindProtobuf {
		t.Fatalf("decoder_kind = %q", got.DecoderKind)
	}
	neighbors, ok := got.Value["neighbors"].([]any)
	if !ok || len(neighbors) != 2 {
		t.Fatalf("neighbors = %v", got.Value["neighbors"])
	}
}

func TestEnrichObserveLine(t *testing.T) {
	ni := &proto.NeighborInfo{NodeId: 1, Neighbors: []*proto.Neighbor{{NodeId: 2, Snr: 1}}}
	payload, _ := protobuf.Marshal(ni)
	packet := &proto.MeshPacket{
		From: 1,
		Id:   9,
		PayloadVariant: &proto.MeshPacket_Decoded{
			Decoded: &proto.Data{
				Portnum: proto.PortNum_NEIGHBORINFO_APP,
				Payload: payload,
			},
		},
	}
	wire, _ := protobuf.Marshal(packet)
	line := []byte(`{"kind":"packet","portnum":71,"mesh_packet_b64":"` + base64.StdEncoding.EncodeToString(wire) + `"}`)
	out, err := EnrichObserveLine(line, decrypt.DefaultPSKInput, false)
	if err != nil {
		t.Fatal(err)
	}
	var row map[string]any
	if err := json.Unmarshal(out, &row); err != nil {
		t.Fatal(err)
	}
	dec, ok := row["decoded"].(map[string]any)
	if !ok {
		t.Fatalf("decoded missing: %v", row)
	}
	if dec["portnum_name"] != "NEIGHBORINFO_APP" {
		t.Fatalf("portnum_name = %v", dec["portnum_name"])
	}
	if _, ok := row["mesh_packet_b64"]; !ok {
		t.Fatal("mesh_packet_b64 should remain without --strip")
	}
}

func TestEnrichObserveLineStrip(t *testing.T) {
	ni := &proto.NeighborInfo{NodeId: 1, Neighbors: []*proto.Neighbor{{NodeId: 2, Snr: 1}}}
	payload, _ := protobuf.Marshal(ni)
	packet := &proto.MeshPacket{
		From: 1,
		Id:   9,
		PayloadVariant: &proto.MeshPacket_Decoded{
			Decoded: &proto.Data{
				Portnum: proto.PortNum_NEIGHBORINFO_APP,
				Payload: payload,
			},
		},
	}
	wire, _ := protobuf.Marshal(packet)
	line := []byte(`{"kind":"packet","portnum":71,"mesh_packet_b64":"` + base64.StdEncoding.EncodeToString(wire) + `"}`)
	out, err := EnrichObserveLine(line, decrypt.DefaultPSKInput, true)
	if err != nil {
		t.Fatal(err)
	}
	var row map[string]any
	if err := json.Unmarshal(out, &row); err != nil {
		t.Fatal(err)
	}
	if _, ok := row["mesh_packet_b64"]; ok {
		t.Fatalf("mesh_packet_b64 should be stripped: %v", row)
	}
	if row["decoded"] == nil {
		t.Fatal("decoded missing after strip")
	}
}

func TestEnrichObserveLinePassthrough(t *testing.T) {
	line := []byte(`{"kind":"packet","portnum":71}`)
	out, err := EnrichObserveLine(line, decrypt.DefaultPSKInput, false)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(line) {
		t.Fatalf("expected passthrough, got %s", out)
	}
}

func TestDecodeEncryptedPosition(t *testing.T) {
	encrypted, err := hex.DecodeString("d1febe296f05a105167914b799c2291118d156d29160721491fdc503cc290295abdc")
	if err != nil {
		t.Fatal(err)
	}
	packet := &proto.MeshPacket{
		From: 0x9ee784a0,
		Id:   0x3ceb474f,
		PayloadVariant: &proto.MeshPacket_Encrypted{
			Encrypted: encrypted,
		},
	}
	got, err := DecodeMeshPacket(packet, decrypt.DefaultPSKInput)
	if err != nil {
		t.Fatal(err)
	}
	if !got.WasEncrypted {
		t.Fatal("expected was_encrypted")
	}
	if got.PortnumName != "POSITION_APP" {
		t.Fatalf("portnum_name = %q", got.PortnumName)
	}
	if got.Value["latitude_i"] == nil {
		t.Fatalf("value = %v", got.Value)
	}
}
