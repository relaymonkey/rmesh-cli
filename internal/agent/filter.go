package agent

import "github.com/exepirit/meshtastic-go/pkg/meshtastic/proto"

// bitfieldOkToMqttMask mirrors BITFIELD_OK_TO_MQTT_MASK in Meshtastic firmware
// (src/mesh/Router.h): bit 0 of the decoded Data.bitfield carries the sender's
// MQTT-upload consent.
const bitfieldOkToMqttMask = 1

// okToMqtt reports whether a passthrough packet consents to MQTT upload.
//
// Consent lives in the decoded Data.bitfield. A packet that explicitly cleared
// the bit ("not willing") is the only thing we treat as a refusal. A packet
// with no decoded bitfield — older firmware, relayed traffic, or a packet this
// gateway cannot decrypt — makes no statement and is treated as ok.
func okToMqtt(packet *proto.MeshPacket) bool {
	decoded := packet.GetDecoded()
	if decoded == nil || decoded.Bitfield == nil {
		return true
	}
	return decoded.GetBitfield()&bitfieldOkToMqttMask != 0
}
