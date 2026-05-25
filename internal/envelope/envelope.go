package envelope

import (
	"fmt"

	"github.com/exepirit/meshtastic-go/pkg/meshtastic"
	"github.com/exepirit/meshtastic-go/pkg/meshtastic/proto"
)

// ChannelMeta holds publish metadata for a Meshtastic channel.
type ChannelMeta struct {
	Index     uint32
	Name      string
	ChannelID string
}

// WrapPassthrough builds a ServiceEnvelope for a packet heard over Phone API.
func WrapPassthrough(packet *proto.MeshPacket, gatewayID, channelID string) *proto.ServiceEnvelope {
	return &proto.ServiceEnvelope{
		Packet:    packet,
		ChannelId: channelID,
		GatewayId: gatewayID,
	}
}

// WrapSynthetic builds a ServiceEnvelope for NodeDB-synthesised traffic.
func WrapSynthetic(packet *proto.MeshPacket, gatewayID, channelID string) *proto.ServiceEnvelope {
	packet.ViaMqtt = true
	if packet.To == 0 {
		packet.To = meshtastic.BroadcastNodenum
	}
	return &proto.ServiceEnvelope{
		Packet:    packet,
		ChannelId: channelID,
		GatewayId: gatewayID,
	}
}

// PublishTopic returns the RelayMesh MQTT publish topic for an envelope.
func PublishTopic(topicPrefix, channelID, gatewayID string) string {
	return fmt.Sprintf("%s/2/e/%s/%s", topicPrefix, channelID, gatewayID)
}
