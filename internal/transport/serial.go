package transport

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/exepirit/meshtastic-go/pkg/meshtastic"
	"github.com/exepirit/meshtastic-go/pkg/meshtastic/proto"
	goserial "go.bug.st/serial"
	protobuf "google.golang.org/protobuf/proto"
)

const serialReadTimeout = 250 * time.Millisecond

type serialTransport struct {
	port goserial.Port
	mu   sync.Mutex
}

func openSerial(path string) (*serialTransport, error) {
	mode := &goserial.Mode{BaudRate: 115200}
	port, err := goserial.Open(path, mode)
	if err != nil {
		return nil, fmt.Errorf("open serial port %q: %w", path, err)
	}
	if err := port.SetReadTimeout(serialReadTimeout); err != nil {
		_ = port.Close()
		return nil, fmt.Errorf("set serial read timeout: %w", err)
	}
	return &serialTransport{port: port}, nil
}

func (st *serialTransport) ReceiveFromRadio(ctx context.Context) (*proto.FromRadio, error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		st.mu.Lock()
		buf, err := st.readFrame(ctx)
		st.mu.Unlock()

		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		if errors.Is(err, errSerialReadTimeout) {
			continue
		}
		if err != nil {
			return nil, err
		}

		packet := new(proto.FromRadio)
		if err := protobuf.Unmarshal(buf, packet); err != nil {
			return nil, meshtastic.ErrInvalidPacketFormat
		}
		return packet, nil
	}
}

var errSerialReadTimeout = errors.New("serial: read timeout")

func (st *serialTransport) readFrame(ctx context.Context) ([]byte, error) {
	header := make([]byte, 4)

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if err := st.readExact(ctx, header[:1]); err != nil {
			return nil, err
		}
		if header[0] != 0x94 {
			continue
		}

		if err := st.readExact(ctx, header[1:2]); err != nil {
			return nil, err
		}
		if header[1] != 0xc3 {
			continue
		}

		if err := st.readExact(ctx, header[2:]); err != nil {
			return nil, err
		}

		pduLen := int(binary.BigEndian.Uint16(header[2:4]))
		if pduLen > 512 {
			continue
		}

		data := make([]byte, pduLen)
		if err := st.readExact(ctx, data); err != nil {
			return nil, err
		}
		return data, nil
	}
}

func (st *serialTransport) readExact(ctx context.Context, buf []byte) error {
	for len(buf) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, err := st.port.Read(buf)
		if err != nil {
			return err
		}
		if n == 0 {
			return errSerialReadTimeout
		}
		buf = buf[n:]
	}
	return nil
}

func (st *serialTransport) SendToRadio(ctx context.Context, packet *proto.ToRadio) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	buf, err := protobuf.Marshal(packet)
	if err != nil {
		return fmt.Errorf("marshal toradio: %w", err)
	}
	if len(buf) > 512 {
		return errors.New("packet too long")
	}

	header := []byte{0x94, 0xc3, 0, 0}
	binary.BigEndian.PutUint16(header[2:4], uint16(len(buf)))

	st.mu.Lock()
	defer st.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := st.port.Write(header); err != nil {
		return err
	}
	_, err = st.port.Write(buf)
	return err
}

func (st *serialTransport) Close() error {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.port == nil {
		return nil
	}
	err := st.port.Close()
	st.port = nil
	return err
}

var _ meshtastic.HardwareTransport = (*serialTransport)(nil)
var _ io.Closer = (*serialTransport)(nil)
