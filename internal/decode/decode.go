package decode

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"unicode/utf8"

	"github.com/exepirit/meshtastic-go/pkg/meshtastic/proto"
	"google.golang.org/protobuf/encoding/protojson"
	protobuf "google.golang.org/protobuf/proto"

	"github.com/relaymonkey/relaymesh-edge/internal/decrypt"
)

const (
	decoderKindProtobuf = "protobuf_ruleset"
	decoderKindUTF8     = "ruleset_utf8"
	decryptScheme       = "meshtastic-aes-ctr-psk"
)

// Payload mirrors RelayMesh cloud envelope `decoded` (messages.DecodedPayload).
type Payload struct {
	DecoderKind   string         `json:"decoder_kind"`
	PortnumName   string         `json:"portnum_name,omitempty"`
	Value         map[string]any `json:"value,omitempty"`
	Error         string         `json:"error,omitempty"`
	PacketType    uint16         `json:"packet_type,omitempty"`
	WasEncrypted  bool           `json:"was_encrypted,omitempty"`
	DecryptScheme string         `json:"decrypt_scheme,omitempty"`
}

var utf8Portnums = map[uint32]struct{}{
	1:  {},
	11: {},
	66: {},
}

// EnrichObserveLine adds a `decoded` field to one observe JSONL object when
// `mesh_packet_b64` is present. Lines without wire bytes pass through unchanged.
// When stripWire is true, mesh_packet_b64 is removed after a successful enrich.
func EnrichObserveLine(line []byte, pskInput string, stripWire bool) ([]byte, error) {
	var row map[string]json.RawMessage
	if err := json.Unmarshal(line, &row); err != nil {
		return nil, err
	}
	raw, ok := row["mesh_packet_b64"]
	if !ok || string(raw) == "null" || len(raw) == 0 {
		return line, nil
	}
	var b64 string
	if err := json.Unmarshal(raw, &b64); err != nil || b64 == "" {
		return line, nil
	}
	wire, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("mesh_packet_b64: %w", err)
	}
	packet := &proto.MeshPacket{}
	if err := protobuf.Unmarshal(wire, packet); err != nil {
		return nil, fmt.Errorf("mesh packet: %w", err)
	}
	decoded, err := DecodeMeshPacket(packet, pskInput)
	if err != nil {
		return nil, err
	}
	enc, err := json.Marshal(decoded)
	if err != nil {
		return nil, err
	}
	row["decoded"] = enc
	if stripWire {
		delete(row, "mesh_packet_b64")
	}
	return json.Marshal(row)
}

// DecodeMeshPacket produces a cloud-shaped decoded payload from a MeshPacket.
func DecodeMeshPacket(packet *proto.MeshPacket, pskInput string) (*Payload, error) {
	if packet == nil {
		return &Payload{Error: "nil_packet"}, nil
	}
	data, wasEncrypted, err := extractData(packet, pskInput)
	if err != nil {
		return &Payload{
			DecoderKind:   decoderKindProtobuf,
			Error:         "decrypt_failed",
			WasEncrypted:  true,
			DecryptScheme: decryptScheme,
		}, nil
	}
	if data == nil {
		return &Payload{Error: "no_payload"}, nil
	}
	portnum := uint32(data.GetPortnum())
	name := portnumName(portnum)
	out := &Payload{
		PortnumName:  name,
		PacketType:   uint16(portnum),
		WasEncrypted: wasEncrypted,
	}
	if wasEncrypted {
		out.DecryptScheme = decryptScheme
	}
	payload := data.GetPayload()
	if _, ok := utf8Portnums[portnum]; ok {
		if !utf8.Valid(payload) {
			out.DecoderKind = decoderKindUTF8
			out.Error = "invalid_utf8"
			return out, nil
		}
		out.DecoderKind = decoderKindUTF8
		out.Value = map[string]any{"text": string(payload)}
		return out, nil
	}
	msg, ok := newProtobufMessage(portnum)
	if !ok {
		out.DecoderKind = decoderKindProtobuf
		out.Error = "unsupported_portnum"
		return out, nil
	}
	if err := protobuf.Unmarshal(payload, msg); err != nil {
		out.DecoderKind = decoderKindProtobuf
		out.Error = "protobuf_unmarshal_failed"
		return out, nil
	}
	value, err := protoToMap(msg)
	if err != nil {
		return nil, err
	}
	normalizeNodeInfoRole(portnum, value)
	out.DecoderKind = decoderKindProtobuf
	out.Value = value
	return out, nil
}

func extractData(packet *proto.MeshPacket, pskInput string) (*proto.Data, bool, error) {
	if d := packet.GetDecoded(); d != nil {
		return d, false, nil
	}
	enc := packet.GetEncrypted()
	if len(enc) == 0 {
		return nil, false, nil
	}
	key, err := decrypt.NormaliseMeshtasticPSK(pskInput)
	if err != nil {
		return nil, false, err
	}
	plain, err := decrypt.DecryptAESCTRPSK(enc, key, decrypt.Context{
		PacketID:     uint64(packet.GetId()),
		SourceNodeID: packet.GetFrom(),
	})
	if err != nil {
		return nil, true, err
	}
	data := &proto.Data{}
	if err := protobuf.Unmarshal(plain, data); err != nil {
		return nil, true, err
	}
	return data, true, nil
}

func newProtobufMessage(portnum uint32) (protobuf.Message, bool) {
	switch portnum {
	case 3:
		return &proto.Position{}, true
	case 4:
		return &proto.User{}, true
	case 5:
		return &proto.Routing{}, true
	case 67:
		return &proto.Telemetry{}, true
	case 70:
		return &proto.RouteDiscovery{}, true
	case 71:
		return &proto.NeighborInfo{}, true
	case 73:
		return &proto.MapReport{}, true
	default:
		return nil, false
	}
}

func protoToMap(msg protobuf.Message) (map[string]any, error) {
	b, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(msg)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func normalizeNodeInfoRole(portnum uint32, value map[string]any) {
	if portnum != 4 {
		return
	}
	if _, ok := value["role"]; ok {
		if n, ok := asFloat(value["role"]); ok {
			value["role"] = roleName(int(n))
		}
		return
	}
	value["role"] = "CLIENT"
}

func roleName(n int) string {
	switch n {
	case 0:
		return "CLIENT"
	case 1:
		return "CLIENT_MUTE"
	case 2:
		return "ROUTER"
	case 3:
		return "ROUTER_CLIENT"
	case 4:
		return "REPEATER"
	case 5:
		return "TRACKER"
	case 6:
		return "SENSOR"
	case 7:
		return "TAK"
	case 8:
		return "CLIENT_HIDDEN"
	case 9:
		return "LOST_AND_FOUND"
	case 10:
		return "TAK_TRACKER"
	case 11:
		return "ROUTER_LATE"
	case 12:
		return "CLIENT_BASE"
	default:
		return strconv.Itoa(n)
	}
}

func portnumName(n uint32) string {
	switch n {
	case 0:
		return "UNKNOWN_APP"
	case 1:
		return "TEXT_MESSAGE_APP"
	case 3:
		return "POSITION_APP"
	case 4:
		return "NODEINFO_APP"
	case 5:
		return "ROUTING_APP"
	case 11:
		return "ALERT_APP"
	case 66:
		return "RANGE_TEST_APP"
	case 67:
		return "TELEMETRY_APP"
	case 70:
		return "TRACEROUTE_APP"
	case 71:
		return "NEIGHBORINFO_APP"
	case 73:
		return "MAP_REPORT_APP"
	default:
		return "PORT_" + strconv.FormatUint(uint64(n), 10)
	}
}

func asFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}
