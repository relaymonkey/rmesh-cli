package device

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/exepirit/meshtastic-go/pkg/meshtastic"
	"github.com/exepirit/meshtastic-go/pkg/meshtastic/proto"
)

// State is the device configuration returned by `WantConfigId`. Beyond
// what meshtastic-go's stock `GetState` collects (MyInfo, NodeInfo,
// Channels, Metadata, Network), this records *every* `Config.*` and
// `ModuleConfig.*` submessage the firmware emits during the config
// sweep. We need the full surface for `rmesh device config get` and
// for the diff that powers the apply pre-check.
type State struct {
	meshtastic.DeviceState

	// Config.* submessages keyed by canonical name (matches the
	// CanonicalPayload shape in internal/deviceconfigs). Values are
	// the strongly-typed wire protos so the caller can both render
	// them via protojson and reason about specific fields (e.g.
	// LoRa.Region for the region-change safety check).
	LoRa         *proto.Config_LoRaConfig
	DeviceConfig *proto.Config_DeviceConfig // Config.Device submessage; distinct from the embedded DeviceState.Device (DeviceMetadata)
	Position     *proto.Config_PositionConfig
	Power        *proto.Config_PowerConfig
	Display      *proto.Config_DisplayConfig
	Bluetooth    *proto.Config_BluetoothConfig
	Security     *proto.Config_SecurityConfig

	// ModuleConfig.* submessages. Forward-tolerant: only the ones
	// the firmware emits will be non-nil; the canonicalisation step
	// skips nil fields rather than writing them as empty objects.
	ModuleMQTT            *proto.ModuleConfig_MQTTConfig
	ModuleSerial          *proto.ModuleConfig_SerialConfig
	ModuleExtNotification *proto.ModuleConfig_ExternalNotificationConfig
	ModuleStoreForward    *proto.ModuleConfig_StoreForwardConfig
	ModuleRangeTest       *proto.ModuleConfig_RangeTestConfig
	ModuleTelemetry       *proto.ModuleConfig_TelemetryConfig
	ModuleCannedMessage   *proto.ModuleConfig_CannedMessageConfig
	ModuleAudio           *proto.ModuleConfig_AudioConfig
	ModuleRemoteHardware  *proto.ModuleConfig_RemoteHardwareConfig
	ModuleNeighborInfo    *proto.ModuleConfig_NeighborInfoConfig
	ModuleAmbientLighting *proto.ModuleConfig_AmbientLightingConfig
	ModuleDetectionSensor *proto.ModuleConfig_DetectionSensorConfig
	ModulePaxcounter      *proto.ModuleConfig_PaxcounterConfig
}

// GetState requests the full device state from the radio. Beyond
// meshtastic-go's stock GetState (which only collects Network + LoRa
// from the Config.* variants), this captures every Config.* and
// ModuleConfig.* submessage the firmware emits during the
// WantConfigId sweep.
//
// Unknown submessages from newer firmware round-trip through the
// canonical JSON path in internal/deviceconfigs, so a firmware
// update is *not* a hard requirement for `rmesh` to keep working —
// only newly-named submessages need a typed field added here, and
// only if a CLI flag wants to special-case them (e.g. region check).
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
			absorbConfig(&state, payload.Config)
		case *proto.FromRadio_ModuleConfig:
			absorbModuleConfig(&state, payload.ModuleConfig)
		case *proto.FromRadio_ConfigCompleteId:
			if payload.ConfigCompleteId == configID {
				return state, nil
			}
		}
	}
}

// absorbConfig stores the wire-side Config.* variant into the right
// typed slot on State. Unknown variants are silently dropped — the
// canonicalisation step picks up only the ones it knows about, and
// the protojson path catches the rest if we add a typed accessor
// later.
func absorbConfig(s *State, cfg *proto.Config) {
	if cfg == nil {
		return
	}
	switch cfg.PayloadVariant.(type) {
	case *proto.Config_Network:
		s.NetworkConfig = cfg.GetNetwork()
	case *proto.Config_Lora:
		s.LoRa = cfg.GetLora()
	case *proto.Config_Device:
		s.DeviceConfig = cfg.GetDevice()
	case *proto.Config_Position:
		s.Position = cfg.GetPosition()
	case *proto.Config_Power:
		s.Power = cfg.GetPower()
	case *proto.Config_Display:
		s.Display = cfg.GetDisplay()
	case *proto.Config_Bluetooth:
		s.Bluetooth = cfg.GetBluetooth()
	case *proto.Config_Security:
		s.Security = cfg.GetSecurity()
	}
}

// absorbModuleConfig is the parallel sink for the ModuleConfig.*
// fan-out.
func absorbModuleConfig(s *State, m *proto.ModuleConfig) {
	if m == nil {
		return
	}
	switch m.PayloadVariant.(type) {
	case *proto.ModuleConfig_Mqtt:
		s.ModuleMQTT = m.GetMqtt()
	case *proto.ModuleConfig_Serial:
		s.ModuleSerial = m.GetSerial()
	case *proto.ModuleConfig_ExternalNotification:
		s.ModuleExtNotification = m.GetExternalNotification()
	case *proto.ModuleConfig_StoreForward:
		s.ModuleStoreForward = m.GetStoreForward()
	case *proto.ModuleConfig_RangeTest:
		s.ModuleRangeTest = m.GetRangeTest()
	case *proto.ModuleConfig_Telemetry:
		s.ModuleTelemetry = m.GetTelemetry()
	case *proto.ModuleConfig_CannedMessage:
		s.ModuleCannedMessage = m.GetCannedMessage()
	case *proto.ModuleConfig_Audio:
		s.ModuleAudio = m.GetAudio()
	case *proto.ModuleConfig_RemoteHardware:
		s.ModuleRemoteHardware = m.GetRemoteHardware()
	case *proto.ModuleConfig_NeighborInfo:
		s.ModuleNeighborInfo = m.GetNeighborInfo()
	case *proto.ModuleConfig_AmbientLighting:
		s.ModuleAmbientLighting = m.GetAmbientLighting()
	case *proto.ModuleConfig_DetectionSensor:
		s.ModuleDetectionSensor = m.GetDetectionSensor()
	case *proto.ModuleConfig_Paxcounter:
		s.ModulePaxcounter = m.GetPaxcounter()
	}
}
