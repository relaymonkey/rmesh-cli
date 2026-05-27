package synthesise

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/exepirit/meshtastic-go/pkg/meshtastic"
	"github.com/exepirit/meshtastic-go/pkg/meshtastic/proto"
	protobuf "google.golang.org/protobuf/proto"
)

// Kind identifies a synthetic packet type emitted from NodeDB.
type Kind string

const (
	KindNodeInfo  Kind = "nodeinfo"
	KindPosition  Kind = "position"
	KindMapReport Kind = "mapreport"
)

// Outbound is a synthetic envelope ready for publish.
type Outbound struct {
	Kind    Kind
	Packet  *proto.MeshPacket
	Content []byte
}

// BuildNodeInfo emits NODEINFO_APP for a NodeDB entry.
func BuildNodeInfo(node *proto.NodeInfo, channelIndex uint32) (*Outbound, error) {
	if node == nil {
		return nil, fmt.Errorf("nodeinfo: nil node")
	}
	if node.User == nil {
		return nil, fmt.Errorf("nodeinfo: missing user for node %d", node.Num)
	}
	payload, err := protobuf.Marshal(node.User)
	if err != nil {
		return nil, fmt.Errorf("nodeinfo: marshal user: %w", err)
	}
	return build(node.Num, channelIndex, proto.PortNum_NODEINFO_APP, payload, KindNodeInfo)
}

// BuildPosition emits POSITION_APP when coordinates exist.
func BuildPosition(node *proto.NodeInfo, channelIndex uint32, respectPrecision bool) (*Outbound, error) {
	if node == nil || node.Position == nil {
		return nil, fmt.Errorf("position: missing position for node %d", node.GetNum())
	}
	pos := protobuf.Clone(node.Position).(*proto.Position)
	if respectPrecision && pos.LocationSource != proto.Position_LOC_MANUAL {
		// Keep firmware-reported precision bits when configured.
	}
	payload, err := protobuf.Marshal(pos)
	if err != nil {
		return nil, fmt.Errorf("position: marshal: %w", err)
	}
	return build(node.Num, channelIndex, proto.PortNum_POSITION_APP, payload, KindPosition)
}

// BuildMapReport emits MAP_REPORT_APP from NodeDB fields.
func BuildMapReport(node *proto.NodeInfo, channelIndex uint32, onlineLocal uint32) (*Outbound, error) {
	if node == nil || node.User == nil {
		return nil, fmt.Errorf("mapreport: missing user for node %d", node.GetNum())
	}
	report := &proto.MapReport{
		LongName:            node.User.LongName,
		ShortName:           node.User.ShortName,
		Role:                node.User.Role,
		HwModel:             node.User.HwModel,
		NumOnlineLocalNodes: onlineLocal,
		HasOptedReportLocation: node.Position != nil &&
			node.Position.GetLatitudeI() != 0 && node.Position.GetLongitudeI() != 0,
	}
	if node.Position != nil {
		if node.Position.LatitudeI != nil {
			report.LatitudeI = *node.Position.LatitudeI
		}
		if node.Position.LongitudeI != nil {
			report.LongitudeI = *node.Position.LongitudeI
		}
		if node.Position.Altitude != nil {
			report.Altitude = *node.Position.Altitude
		}
		if node.Position.PrecisionBits != 0 {
			report.PositionPrecision = node.Position.PrecisionBits
		}
	}
	payload, err := protobuf.Marshal(report)
	if err != nil {
		return nil, fmt.Errorf("mapreport: marshal: %w", err)
	}
	return build(node.Num, channelIndex, proto.PortNum_MAP_REPORT_APP, payload, KindMapReport)
}

func build(from, channelIndex uint32, port proto.PortNum, payload []byte, kind Kind) (*Outbound, error) {
	contentKey := contentHash(from, port, payload)
	packet := &proto.MeshPacket{
		From:     from,
		To:       meshtastic.BroadcastNodenum,
		Channel:  channelIndex,
		Id:       deterministicPacketID(contentKey),
		HopLimit: 3,
		ViaMqtt:  true,
		PayloadVariant: &proto.MeshPacket_Decoded{
			Decoded: &proto.Data{
				Portnum: port,
				Payload: payload,
			},
		},
	}
	return &Outbound{Kind: kind, Packet: packet, Content: contentKey}, nil
}

func contentHash(from uint32, port proto.PortNum, payload []byte) []byte {
	h := sha256.New()
	var buf [8]byte
	binary.LittleEndian.PutUint32(buf[:4], from)
	binary.LittleEndian.PutUint32(buf[4:8], uint32(port))
	h.Write(buf[:])
	h.Write(payload)
	return h.Sum(nil)
}

func deterministicPacketID(contentKey []byte) uint32 {
	// Use lower 32 bits of content hash; set Meshtastic random bits in upper range.
	v := binary.LittleEndian.Uint32(contentKey[:4])
	v = v & 0x3ff
	v |= 0x40000000
	return v
}

// Due reports whether a cadence window has elapsed, including optional jitter.
func Due(last time.Time, interval, jitter time.Duration, now time.Time) bool {
	if last.IsZero() {
		return true
	}
	wait := interval
	if jitter > 0 {
		offset := time.Duration(int64(jitter) * int64(contentKeyToJitter(last.UnixNano())) / 100)
		wait += offset
	}
	return now.Sub(last) >= wait
}

func contentKeyToJitter(seed int64) int64 {
	if seed < 0 {
		seed = -seed
	}
	return seed % 100
}
