package observe

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/exepirit/meshtastic-go/pkg/meshtastic/proto"
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
	At           time.Time `json:"at"`
	Kind         string    `json:"kind"`
	IngestSource string    `json:"ingest_source"`
	Topic        string    `json:"topic"`
	GatewayID    string    `json:"gateway_id"`
	From         uint32    `json:"from"`
	To           uint32    `json:"to"`
	PacketID     uint32    `json:"packet_id"`
	Portnum      uint32    `json:"portnum,omitempty"`
	Synthetic    bool      `json:"synthetic"`
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
