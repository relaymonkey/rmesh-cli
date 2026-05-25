package synthesise_test

import (
	"testing"

	"github.com/exepirit/meshtastic-go/pkg/meshtastic/proto"

	"github.com/relaymonkey/relaymesh-edge/internal/synthesise"
)

func TestBuildNodeInfoDeterministicID(t *testing.T) {
	node := &proto.NodeInfo{
		Num: 0x1a2b3c4d,
		User: &proto.User{
			Id:        "!1a2b3c4d",
			LongName:  "Test Node",
			ShortName: "TN",
		},
	}
	a, err := synthesise.BuildNodeInfo(node, 0)
	if err != nil {
		t.Fatal(err)
	}
	b, err := synthesise.BuildNodeInfo(node, 0)
	if err != nil {
		t.Fatal(err)
	}
	if a.Packet.GetId() != b.Packet.GetId() {
		t.Fatalf("packet id not deterministic: %d vs %d", a.Packet.GetId(), b.Packet.GetId())
	}
	if a.Packet.GetFrom() != node.Num {
		t.Fatalf("from = %d", a.Packet.GetFrom())
	}
	if !a.Packet.GetViaMqtt() {
		t.Fatal("expected via_mqtt on synthetic packet")
	}
}
