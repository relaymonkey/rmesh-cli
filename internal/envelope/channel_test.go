package envelope

import (
	"testing"

	"github.com/exepirit/meshtastic-go/pkg/meshtastic/proto"
)

func TestChannelNameEmptyPreset(t *testing.T) {
	ch := &proto.Channel{
		Settings: &proto.ChannelSettings{Name: ""},
	}
	lora := &proto.Config_LoRaConfig{
		UsePreset:   true,
		ModemPreset: proto.Config_LoRaConfig_LONG_FAST,
	}
	if got := ChannelName(ch, lora); got != "LongFast" {
		t.Fatalf("got %q want LongFast", got)
	}
}

func TestChannelNameEmptyCustomLoRa(t *testing.T) {
	ch := &proto.Channel{
		Settings: &proto.ChannelSettings{Name: ""},
	}
	lora := &proto.Config_LoRaConfig{UsePreset: false}
	if got := ChannelName(ch, lora); got != "Custom" {
		t.Fatalf("got %q want Custom", got)
	}
}

func TestChannelNameExplicit(t *testing.T) {
	ch := &proto.Channel{
		Settings: &proto.ChannelSettings{Name: "MyMesh"},
	}
	if got := ChannelName(ch, nil); got != "MyMesh" {
		t.Fatalf("got %q want MyMesh", got)
	}
}

func TestPrimaryChannelUsesRole(t *testing.T) {
	channels := []*proto.Channel{
		{Index: 0, Role: proto.Channel_SECONDARY, Settings: &proto.ChannelSettings{Name: "secondary"}},
		{Index: 1, Role: proto.Channel_PRIMARY, Settings: &proto.ChannelSettings{Name: ""}},
	}
	lora := &proto.Config_LoRaConfig{UsePreset: false}
	meta := PrimaryChannel(channels, lora)
	if meta.Index != 1 {
		t.Fatalf("index %d want 1", meta.Index)
	}
	if meta.ChannelID != "Custom" {
		t.Fatalf("channel_id %q want Custom", meta.ChannelID)
	}
}
