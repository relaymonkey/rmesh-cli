package observe

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/exepirit/meshtastic-go/pkg/meshtastic/proto"
	protobuf "google.golang.org/protobuf/proto"
)

// Sink prints envelopes to a writer for dry-run validation.
type Sink struct {
	out io.Writer
}

// New returns a TTY/log sink.
func New(out io.Writer) *Sink {
	return &Sink{out: out}
}

// Event is a serialised observation for observe mode.
type Event struct {
	At           time.Time         `json:"at"`
	Kind         string            `json:"kind"`
	IngestSource string            `json:"ingest_source"`
	IngestLabels map[string]string `json:"ingest_labels,omitempty"`
	Topic        string            `json:"topic"`
	GatewayID    string            `json:"gateway_id"`
	From         uint32            `json:"from"`
	To           uint32            `json:"to"`
	PacketID     uint32            `json:"packet_id"`
	Portnum      uint32            `json:"portnum,omitempty"`
	Synthetic    bool              `json:"synthetic"`
	// Dropped is the reason a passthrough packet would not be published
	// (currently only "ok_to_mqtt"). Empty when the packet is forwarded.
	Dropped string `json:"dropped,omitempty"`
	// MeshPacketB64 is the serialised meshtastic.MeshPacket protobuf
	// (standard base64). Consumed by `rmesh decode` for local payload
	// inspection without cloud ingest.
	MeshPacketB64 string `json:"mesh_packet_b64,omitempty"`
}

// Write logs one envelope observation as JSONL.
func (s *Sink) Write(ev Event) error {
	ev.At = time.Now().UTC()
	b, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(s.out, "%s\n", b)
	return err
}

// MeshPacketB64 encodes a MeshPacket for observe JSONL / rmesh decode.
func MeshPacketB64(packet *proto.MeshPacket) string {
	if packet == nil {
		return ""
	}
	b, err := protobuf.Marshal(packet)
	if err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(b)
}

// Portnum extracts the application port from a mesh packet when decoded.
func Portnum(packet *proto.MeshPacket) uint32 {
	if packet == nil {
		return 0
	}
	if decoded := packet.GetDecoded(); decoded != nil {
		return uint32(decoded.GetPortnum())
	}
	return 0
}
