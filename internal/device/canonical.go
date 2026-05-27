package device

import (
	"encoding/json"
	"fmt"
	"reflect"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/relaymonkey/relaymesh-edge/internal/deviceconfigs"
)

// marshalOpts is shared across this package's protojson conversions.
// `UseProtoNames` keeps the canonical snake_case keys that match the
// .proto field names (and our canonical shape); the default
// camelCase would diverge from the backend's `region` / `modem_preset`
// hint extraction.
var marshalOpts = protojson.MarshalOptions{
	UseProtoNames:   true,
	EmitUnpopulated: false,
	Multiline:       false,
}

// unmarshalOpts is the parallel set for parsing protojson back into
// the strongly-typed wire protos. `DiscardUnknown: true` lets us
// round-trip payloads produced by a newer firmware whose protos this
// `rmesh` build does not yet know about.
var unmarshalOpts = protojson.UnmarshalOptions{
	DiscardUnknown: true,
}

// ToCanonicalPayload converts a fully-populated `State` into the
// canonical payload shape. Submessages the firmware did not
// emit (nil pointers) are simply omitted from the result.
func ToCanonicalPayload(s State) (deviceconfigs.CanonicalPayload, error) {
	out := deviceconfigs.CanonicalPayload{
		Config:       map[string]json.RawMessage{},
		ModuleConfig: map[string]json.RawMessage{},
	}

	if len(s.Channels) > 0 {
		out.Channels = make([]json.RawMessage, 0, len(s.Channels))
		for _, ch := range s.Channels {
			raw, err := protoToJSON(ch)
			if err != nil {
				return deviceconfigs.CanonicalPayload{}, fmt.Errorf("marshal channel: %w", err)
			}
			out.Channels = append(out.Channels, raw)
		}
	}

	cfgFields := []struct {
		key string
		msg proto.Message
	}{
		{"lora", maybeMsg(s.LoRa)},
		{"device", maybeMsg(s.DeviceConfig)},
		{"position", maybeMsg(s.Position)},
		{"power", maybeMsg(s.Power)},
		{"network", maybeMsg(s.NetworkConfig)},
		{"display", maybeMsg(s.Display)},
		{"bluetooth", maybeMsg(s.Bluetooth)},
		{"security", maybeMsg(s.Security)},
	}
	for _, f := range cfgFields {
		if f.msg == nil {
			continue
		}
		raw, err := protoToJSON(f.msg)
		if err != nil {
			return deviceconfigs.CanonicalPayload{}, fmt.Errorf("marshal config.%s: %w", f.key, err)
		}
		out.Config[f.key] = raw
	}

	modFields := []struct {
		key string
		msg proto.Message
	}{
		{"mqtt", maybeMsg(s.ModuleMQTT)},
		{"serial", maybeMsg(s.ModuleSerial)},
		{"ext_notification", maybeMsg(s.ModuleExtNotification)},
		{"store_forward", maybeMsg(s.ModuleStoreForward)},
		{"range_test", maybeMsg(s.ModuleRangeTest)},
		{"telemetry", maybeMsg(s.ModuleTelemetry)},
		{"canned_message", maybeMsg(s.ModuleCannedMessage)},
		{"audio", maybeMsg(s.ModuleAudio)},
		{"remote_hardware", maybeMsg(s.ModuleRemoteHardware)},
		{"neighbor_info", maybeMsg(s.ModuleNeighborInfo)},
		{"ambient_lighting", maybeMsg(s.ModuleAmbientLighting)},
		{"detection_sensor", maybeMsg(s.ModuleDetectionSensor)},
		{"paxcounter", maybeMsg(s.ModulePaxcounter)},
	}
	for _, f := range modFields {
		if f.msg == nil {
			continue
		}
		raw, err := protoToJSON(f.msg)
		if err != nil {
			return deviceconfigs.CanonicalPayload{}, fmt.Errorf("marshal module_config.%s: %w", f.key, err)
		}
		out.ModuleConfig[f.key] = raw
	}

	if len(out.Config) == 0 {
		out.Config = nil
	}
	if len(out.ModuleConfig) == 0 {
		out.ModuleConfig = nil
	}
	return out, nil
}

// FirmwareVersion returns the firmware version captured in the
// device's `Metadata` packet, if any. Used by `rmesh device config
// set --to cloud` to populate the denormalised hint.
func (s State) FirmwareVersion() string {
	if s.Device == nil {
		return ""
	}
	return s.Device.GetFirmwareVersion()
}

// protoToJSON marshals a single proto message into the canonical JSON
// shape (snake_case, unpopulated fields omitted).
func protoToJSON(m proto.Message) (json.RawMessage, error) {
	if m == nil {
		return nil, nil
	}
	b, err := marshalOpts.Marshal(m)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

// maybeMsg returns nil when the typed pointer is nil. Works around
// the fact that a typed-nil pointer compares non-nil when stored in
// an interface (proto.Message) — using it directly in the field
// table would short-circuit the nil check.
func maybeMsg[T proto.Message](v T) proto.Message {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr && rv.IsNil() {
		return nil
	}
	return v
}
