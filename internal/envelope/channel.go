package envelope

import (
	"github.com/exepirit/meshtastic-go/pkg/meshtastic/proto"
)

// ModemPresetDisplayName mirrors Meshtastic firmware DisplayFormatters::getModemPresetDisplayName.
func ModemPresetDisplayName(preset proto.Config_LoRaConfig_ModemPreset, usePreset bool) string {
	if !usePreset {
		return "Custom"
	}
	switch preset {
	case proto.Config_LoRaConfig_SHORT_TURBO:
		return "ShortTurbo"
	case proto.Config_LoRaConfig_SHORT_SLOW:
		return "ShortSlow"
	case proto.Config_LoRaConfig_SHORT_FAST:
		return "ShortFast"
	case proto.Config_LoRaConfig_MEDIUM_SLOW:
		return "MediumSlow"
	case proto.Config_LoRaConfig_MEDIUM_FAST:
		return "MediumFast"
	case proto.Config_LoRaConfig_LONG_SLOW:
		return "LongSlow"
	case proto.Config_LoRaConfig_LONG_FAST:
		return "LongFast"
	case proto.Config_LoRaConfig_LONG_TURBO:
		return "LongTurbo"
	case proto.Config_LoRaConfig_LONG_MODERATE:
		return "LongMod"
	default:
		return "Invalid"
	}
}

// ChannelName resolves the MQTT channel_id for a channel index, matching firmware Channels::getName.
func ChannelName(ch *proto.Channel, lora *proto.Config_LoRaConfig) string {
	if ch == nil || ch.Settings == nil {
		return defaultChannelName(lora)
	}
	if name := ch.Settings.Name; name != "" {
		return name
	}
	return defaultChannelName(lora)
}

func defaultChannelName(lora *proto.Config_LoRaConfig) string {
	usePreset := true
	preset := proto.Config_LoRaConfig_LONG_FAST
	if lora != nil {
		usePreset = lora.GetUsePreset()
		preset = lora.GetModemPreset()
	}
	return ModemPresetDisplayName(preset, usePreset)
}

// PrimaryChannel picks the primary channel and resolves its MQTT channel_id.
func PrimaryChannel(channels []*proto.Channel, lora *proto.Config_LoRaConfig) ChannelMeta {
	ch, index := primaryChannelEntry(channels)
	name := ChannelName(ch, lora)
	return ChannelMeta{
		Index:     index,
		Name:      storedChannelName(ch),
		ChannelID: name,
	}
}

func primaryChannelEntry(channels []*proto.Channel) (*proto.Channel, uint32) {
	for _, ch := range channels {
		if ch != nil && ch.GetRole() == proto.Channel_PRIMARY {
			return ch, uint32(ch.GetIndex())
		}
	}
	if len(channels) > 0 && channels[0] != nil {
		return channels[0], uint32(channels[0].GetIndex())
	}
	return nil, 0
}

func storedChannelName(ch *proto.Channel) string {
	if ch == nil || ch.Settings == nil || ch.Settings.Name == "" {
		return ""
	}
	return ch.Settings.Name
}
