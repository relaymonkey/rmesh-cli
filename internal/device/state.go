package device

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/exepirit/meshtastic-go/pkg/meshtastic"
	"github.com/exepirit/meshtastic-go/pkg/meshtastic/proto"
)

// State is device configuration returned by WantConfigId, including LoRa settings
// required to resolve MQTT channel_id the same way Meshtastic firmware does.
type State struct {
	meshtastic.DeviceState
	LoRa *proto.Config_LoRaConfig
}

// GetState requests full device state from the radio. meshtastic-go's GetState omits LoRa config.
func GetState(ctx context.Context, transport meshtastic.HardwareTransport) (State, error) {
	configID := uint32(rand.Int())
	if err := transport.SendToRadio(ctx, &proto.ToRadio{
		PayloadVariant: &proto.ToRadio_WantConfigId{
			WantConfigId: configID,
		},
	}); err != nil {
		return State{}, fmt.Errorf("want config: %w", err)
	}

	var state State
	for {
		packet, err := transport.ReceiveFromRadio(ctx)
		if err != nil {
			return state, fmt.Errorf("read config: %w", err)
		}

		switch payload := packet.PayloadVariant.(type) {
		case *proto.FromRadio_MyInfo:
			state.MyInfo = payload.MyInfo
		case *proto.FromRadio_NodeInfo:
			state.Nodes = append(state.Nodes, payload.NodeInfo)
		case *proto.FromRadio_Channel:
			state.Channels = append(state.Channels, payload.Channel)
		case *proto.FromRadio_Metadata:
			state.Device = payload.Metadata
		case *proto.FromRadio_Config:
			switch payload.Config.PayloadVariant.(type) {
			case *proto.Config_Network:
				state.NetworkConfig = payload.Config.GetNetwork()
			case *proto.Config_Lora:
				state.LoRa = payload.Config.GetLora()
			}
		case *proto.FromRadio_ConfigCompleteId:
			if payload.ConfigCompleteId == configID {
				return state, nil
			}
		}
	}
}
