package agent

import (
	"testing"

	"github.com/exepirit/meshtastic-go/pkg/meshtastic/proto"
)

func TestOkToMqtt(t *testing.T) {
	bits := func(v uint32) *proto.MeshPacket {
		return &proto.MeshPacket{
			PayloadVariant: &proto.MeshPacket_Decoded{
				Decoded: &proto.Data{Bitfield: &v},
			},
		}
	}

	cases := []struct {
		name   string
		packet *proto.MeshPacket
		want   bool
	}{
		{
			name:   "consent bit set",
			packet: bits(bitfieldOkToMqttMask),
			want:   true,
		},
		{
			name:   "consent bit set among other flags",
			packet: bits(0b11),
			want:   true,
		},
		{
			name:   "consent bit cleared",
			packet: bits(0),
			want:   false,
		},
		{
			name: "no bitfield is treated as ok",
			packet: &proto.MeshPacket{
				PayloadVariant: &proto.MeshPacket_Decoded{Decoded: &proto.Data{}},
			},
			want: true,
		},
		{
			name: "encrypted packet is treated as ok",
			packet: &proto.MeshPacket{
				PayloadVariant: &proto.MeshPacket_Encrypted{Encrypted: []byte{0x01}},
			},
			want: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := okToMqtt(tc.packet); got != tc.want {
				t.Errorf("okToMqtt() = %v, want %v", got, tc.want)
			}
		})
	}
}
