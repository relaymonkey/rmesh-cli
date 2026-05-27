package transport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"sync"
	"time"

	"github.com/exepirit/meshtastic-go/pkg/meshtastic"
	"github.com/exepirit/meshtastic-go/pkg/meshtastic/proto"
	protobuf "google.golang.org/protobuf/proto"
	"tinygo.org/x/bluetooth"
)

const (
	blePacketBuffer = 120
	// blePollInterval drains fromRadio when the fromNum notification doesn't fire
	// (observed on macOS / CoreBluetooth). Matches the Meshtastic Python CLI cadence.
	blePollInterval = 200 * time.Millisecond
)

var errBLEEmptyQueue = errors.New("ble: no data in queue")

type bleTransport struct {
	device    bluetooth.Device
	fromRadio bluetooth.DeviceCharacteristic
	fromNum   bluetooth.DeviceCharacteristic
	toRadio   bluetooth.DeviceCharacteristic

	packets chan *proto.FromRadio
	closeMu sync.Mutex
	closed  bool
	stop    chan struct{}
	pulled  chan struct{}
}

func (t *bleTransport) ReceiveFromRadio(ctx context.Context) (*proto.FromRadio, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case p, ok := <-t.packets:
		if !ok {
			return nil, errors.New("ble: transport closed")
		}
		return p, nil
	}
}

func (t *bleTransport) SendToRadio(ctx context.Context, packet *proto.ToRadio) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	buf, err := protobuf.Marshal(packet)
	if err != nil {
		return fmt.Errorf("marshal toradio: %w", err)
	}
	slog.Debug("ble: SendToRadio", "bytes", len(buf), "payload", reflect.TypeOf(packet.PayloadVariant))
	if err := t.writeToRadio(buf); err != nil {
		slog.Debug("ble: SendToRadio failed", "error", err)
		return err
	}
	// Nudge the poller so a response that arrives between ticks is drained immediately.
	t.kickPoller()
	return nil
}

func (t *bleTransport) Close() error {
	t.closeMu.Lock()
	defer t.closeMu.Unlock()
	if t.closed {
		return nil
	}
	t.closed = true
	if t.stop != nil {
		close(t.stop)
	}
	_ = t.fromNum.EnableNotifications(nil)
	close(t.packets)
	return t.device.Disconnect()
}

func (t *bleTransport) start() error {
	t.stop = make(chan struct{})
	t.pulled = make(chan struct{}, 1)

	buf, err := protobuf.Marshal(&proto.ToRadio{
		PayloadVariant: &proto.ToRadio_WantConfigId{},
	})
	if err != nil {
		return fmt.Errorf("marshal WantConfigId: %w", err)
	}
	slog.Info("ble: loading device state (channels, modules, nodes — can take 15-30s)")
	if err := t.writeToRadio(buf); err != nil {
		return fmt.Errorf("send WantConfigId: %w", err)
	}

	drained := 0
	var readErr error
	for {
		pkt, err := t.readPacket()
		if err != nil {
			readErr = err
			break
		}
		drained++
		slog.Debug("ble: initial drain packet", "index", drained, "payload", reflect.TypeOf(pkt.PayloadVariant))
		if drained%25 == 0 {
			slog.Info("ble: loading device state", "packets", drained)
		}
	}
	slog.Info("ble: device state loaded", "packets", drained)
	if !errors.Is(readErr, errBLEEmptyQueue) {
		return fmt.Errorf("initial BLE read: %w", readErr)
	}

	if err := t.fromNum.EnableNotifications(func(_ []byte) {
		slog.Debug("ble: fromNum notification fired")
		t.kickPoller()
	}); err != nil {
		slog.Debug("ble: EnableNotifications failed; relying on poller", "error", err)
	} else {
		slog.Debug("ble: notifications enabled")
	}

	go t.pollLoop()
	return nil
}

func (t *bleTransport) kickPoller() {
	if t.pulled == nil {
		return
	}
	select {
	case t.pulled <- struct{}{}:
	default:
	}
}

func (t *bleTransport) pollLoop() {
	ticker := time.NewTicker(blePollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-t.stop:
			return
		case <-ticker.C:
			t.pullPackets()
		case <-t.pulled:
			t.pullPackets()
		}
	}
}

func (t *bleTransport) pullPackets() {
	for {
		packet, err := t.readPacket()
		switch {
		case errors.Is(err, errBLEEmptyQueue):
			return
		case err != nil:
			slog.Debug("ble: readPacket error", "error", err)
			return
		default:
			slog.Debug("ble: packet received", "payload", reflect.TypeOf(packet.PayloadVariant))
			t.closeMu.Lock()
			closed := t.closed
			t.closeMu.Unlock()
			if closed {
				return
			}
			select {
			case t.packets <- packet:
			default:
				select {
				case <-t.packets:
				default:
				}
				select {
				case t.packets <- packet:
				default:
				}
			}
		}
	}
}

func (t *bleTransport) readPacket() (*proto.FromRadio, error) {
	buf := make([]byte, 512)
	n, err := t.fromRadio.Read(buf)
	if err != nil {
		return nil, err
	}
	if n < 1 {
		return nil, errBLEEmptyQueue
	}

	packet := new(proto.FromRadio)
	if err := protobuf.Unmarshal(buf[:n], packet); err != nil {
		slog.Debug("ble: unmarshal failed", "bytes", n, "error", err)
		return nil, meshtastic.ErrInvalidPacketFormat
	}
	return packet, nil
}

var (
	_ meshtastic.HardwareTransport = (*bleTransport)(nil)
	_ io.Closer                    = (*bleTransport)(nil)
)
