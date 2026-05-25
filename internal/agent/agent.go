package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/exepirit/meshtastic-go/pkg/meshtastic"
	"github.com/exepirit/meshtastic-go/pkg/meshtastic/proto"

	"github.com/relaymonkey/relaymesh-edge/internal/config"
	rmdevice "github.com/relaymonkey/relaymesh-edge/internal/device"
	"github.com/relaymonkey/relaymesh-edge/internal/envelope"
	"github.com/relaymonkey/relaymesh-edge/internal/forwarder"
	"github.com/relaymonkey/relaymesh-edge/internal/labels"
	"github.com/relaymonkey/relaymesh-edge/internal/nodeid"
	"github.com/relaymonkey/relaymesh-edge/internal/observe"
	"github.com/relaymonkey/relaymesh-edge/internal/synthesise"
	rmtransport "github.com/relaymonkey/relaymesh-edge/internal/transport"
)

// Options tune runtime behaviour.
type Options struct {
	Observe      bool
	Verbose      bool
	ResetCadence bool
	ObserveOut   io.Writer
}

// Run connects to the local node and forwards traffic upstream.
func Run(ctx context.Context, cfg config.Config, opts Options) error {
	transport, err := rmtransport.Open(cfg.Transport.URL)
	if err != nil {
		return fmt.Errorf("transport: %w", err)
	}
	defer rmtransport.Close(transport)

	device, err := meshtastic.NewConfiguredDevice(ctx, transport)
	if err != nil {
		return fmt.Errorf("device: %w", err)
	}

	state, err := rmdevice.GetState(ctx, transport)
	if err != nil {
		return fmt.Errorf("device state: %w", err)
	}
	gatewayID := gatewayFromState(state.DeviceState)
	channel := envelope.PrimaryChannel(state.Channels, state.LoRa)

	a := &runtime{
		cfg:       cfg,
		opts:      opts,
		device:    device,
		transport: transport,
		gatewayID: gatewayID,
		channel:   channel,
		nodes:     indexNodes(state.Nodes),
		seen:      make(map[uint32]struct{}),
		emitState: make(map[emitKey]time.Time),
	}
	if opts.ResetCadence {
		a.emitState = make(map[emitKey]time.Time)
	}

	if opts.Observe {
		out := opts.ObserveOut
		if out == nil {
			out = os.Stdout
		}
		a.sink = observe.New(out)
		slog.Info("observe mode: MQTT publish disabled")
	} else {
		pub, err := forwarder.Connect(ctx, cfg.MQTT, cfg.AgentID, cfg.Labels)
		if err != nil {
			return fmt.Errorf("mqtt: %w", err)
		}
		defer pub.Close(ctx)
		a.pub = pub
	}

	slog.Info("rmesh agent started",
		"gateway_id", gatewayID,
		"channel", channel.ChannelID,
		"nodes", len(a.nodes),
		"observe", opts.Observe,
	)

	errCh := make(chan error, 1)
	go func() {
		errCh <- a.readLoop(ctx)
	}()

	// Emit once on startup so operators can validate synthesis without waiting.
	if err := a.emitSynthetic(ctx); err != nil {
		slog.Warn("initial synthesise failed", "err", err)
	}

	ticker := time.NewTicker(cfg.Synthesise.NodeDBPoll)
	defer ticker.Stop()

	shutdownWait := 3 * time.Second

	for {
		select {
		case <-ctx.Done():
			ticker.Stop()
			_ = rmtransport.Close(transport)
			select {
			case err := <-errCh:
				if err != nil && !errors.Is(err, context.Canceled) {
					slog.Debug("read loop stopped", "err", err)
				}
			case <-time.After(shutdownWait):
				slog.Warn("read loop did not stop promptly after shutdown")
			}
			return ctx.Err()
		case err := <-errCh:
			if err != nil && !errors.Is(err, context.Canceled) {
				return err
			}
			return nil
		case <-ticker.C:
			if ctx.Err() != nil {
				continue
			}
			if err := a.refreshAndSynthesise(ctx); err != nil {
				if errors.Is(err, context.Canceled) {
					continue
				}
				slog.Warn("synthesise tick failed", "err", err)
			}
		}
	}
}

type runtime struct {
	cfg       config.Config
	opts      Options
	device    *meshtastic.Device
	transport meshtastic.HardwareTransport
	pub       *forwarder.Publisher
	sink      *observe.Sink
	gatewayID string
	channel   envelope.ChannelMeta
	mu        sync.Mutex
	nodes     map[uint32]*proto.NodeInfo
	seen      map[uint32]struct{}
	emitState map[emitKey]time.Time
}

type emitKey struct {
	node uint32
	kind synthesise.Kind
}

func (a *runtime) readLoop(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		frame, err := a.transport.ReceiveFromRadio(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}
			return err
		}
		switch payload := frame.PayloadVariant.(type) {
		case *proto.FromRadio_Packet:
			if err := a.handlePacket(ctx, payload.Packet, false); err != nil {
				slog.Warn("passthrough failed", "err", err)
			}
		case *proto.FromRadio_NodeInfo:
			a.storeNode(payload.NodeInfo)
		default:
		}
	}
}

func (a *runtime) refreshAndSynthesise(ctx context.Context) error {
	state, err := rmdevice.GetState(ctx, a.transport)
	if err != nil {
		return err
	}
	for _, n := range state.Nodes {
		a.storeNode(n)
	}
	return a.emitSynthetic(ctx)
}

func (a *runtime) handlePacket(ctx context.Context, packet *proto.MeshPacket, synthetic bool) error {
	if packet == nil {
		return nil
	}
	var env *proto.ServiceEnvelope
	if synthetic {
		env = envelope.WrapSynthetic(packet, a.gatewayID, a.channel.ChannelID)
	} else {
		env = envelope.WrapPassthrough(packet, a.gatewayID, a.channel.ChannelID)
	}
	topic := envelope.PublishTopic(a.cfg.MQTT.TopicPrefix, a.channel.ChannelID, a.gatewayID)
	source := labels.IngestSource(a.cfg.AgentID)
	if synthetic {
		source = labels.IngestSourceNodeDB(a.cfg.AgentID)
	}

	if a.opts.Observe {
		return a.sink.Write(observe.Event{
			Kind:         "packet",
			IngestSource: source,
			IngestLabels: a.cfg.Labels,
			Topic:        topic,
			GatewayID:    a.gatewayID,
			From:         packet.GetFrom(),
			To:           packet.GetTo(),
			PacketID:     packet.GetId(),
			Portnum:      observe.Portnum(packet),
			Synthetic:    synthetic,
		})
	}
	var err error
	if synthetic {
		err = a.pub.PublishSynthetic(ctx, env)
	} else {
		err = a.pub.PublishPassthrough(ctx, env)
	}
	if err != nil {
		return err
	}
	if a.opts.Verbose {
		slog.Info("mqtt published",
			"topic", topic,
			"ingest_source", source,
			"from", nodeid.FromNum(packet.GetFrom()),
			"portnum", observe.Portnum(packet),
			"packet_id", packet.GetId(),
			"synthetic", synthetic,
		)
	}
	return nil
}

func (a *runtime) emitSynthetic(ctx context.Context) error {
	a.mu.Lock()
	nodes := make([]*proto.NodeInfo, 0, len(a.nodes))
	for _, n := range a.nodes {
		nodes = append(nodes, n)
	}
	a.mu.Unlock()

	onlineLocal := uint32(len(nodes))
	for _, node := range nodes {
		if node == nil {
			continue
		}
		a.mu.Lock()
		_, first := a.seen[node.Num]
		if !first {
			a.seen[node.Num] = struct{}{}
		}
		a.mu.Unlock()

		if err := a.maybeEmit(ctx, node, synthesise.KindNodeInfo, a.cfg.Synthesise.NodeInfo, first, func() (*synthesise.Outbound, error) {
			return synthesise.BuildNodeInfo(node, a.channel.Index)
		}); err != nil {
			if isSkipErr(err) {
				slog.Debug("skip nodeinfo synthesise", "node", nodeid.FromNum(node.Num), "err", err)
			} else {
				return err
			}
		}
		if err := a.maybeEmit(ctx, node, synthesise.KindPosition, a.cfg.Synthesise.Position, first, func() (*synthesise.Outbound, error) {
			return synthesise.BuildPosition(node, a.channel.Index, a.cfg.Synthesise.Position.RespectPositionPrecision)
		}); err != nil {
			if isSkipErr(err) {
				slog.Debug("skip position synthesise", "node", nodeid.FromNum(node.Num), "err", err)
			} else {
				return err
			}
		}
		if err := a.maybeEmit(ctx, node, synthesise.KindMapReport, a.cfg.Synthesise.MapReport, first, func() (*synthesise.Outbound, error) {
			return synthesise.BuildMapReport(node, a.channel.Index, onlineLocal)
		}); err != nil {
			if isSkipErr(err) {
				slog.Debug("skip mapreport synthesise", "node", nodeid.FromNum(node.Num), "err", err)
			} else {
				return err
			}
		}
	}
	return nil
}

func (a *runtime) maybeEmit(
	ctx context.Context,
	node *proto.NodeInfo,
	kind synthesise.Kind,
	policy config.CadenceConfig,
	firstSeen bool,
	build func() (*synthesise.Outbound, error),
) error {
	if !policy.Enabled {
		return nil
	}
	key := emitKey{node: node.Num, kind: kind}
	a.mu.Lock()
	last := a.emitState[key]
	a.mu.Unlock()

	if !(firstSeen && policy.OnFirstSeen) && !synthesise.Due(last, policy.Interval, policy.Jitter, time.Now()) {
		return nil
	}

	out, err := build()
	if err != nil {
		return err
	}
	if err := a.handlePacket(ctx, out.Packet, true); err != nil {
		return err
	}
	a.mu.Lock()
	a.emitState[key] = time.Now()
	a.mu.Unlock()
	slog.Debug("synthetic emitted", "kind", kind, "node", nodeid.FromNum(node.Num), "packet_id", out.Packet.GetId())
	return nil
}

func (a *runtime) storeNode(node *proto.NodeInfo) {
	if node == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.nodes[node.Num] = node
}

func gatewayFromState(state meshtastic.DeviceState) string {
	if info, ok := state.CurrentNodeInfo(); ok && info.GetUser() != nil && info.User.GetId() != "" {
		return info.User.GetId()
	}
	if state.MyInfo != nil {
		return nodeid.FromNum(state.MyInfo.MyNodeNum)
	}
	return ""
}

func indexNodes(nodes []*proto.NodeInfo) map[uint32]*proto.NodeInfo {
	out := make(map[uint32]*proto.NodeInfo, len(nodes))
	for _, n := range nodes {
		if n != nil {
			out[n.Num] = n
		}
	}
	return out
}

func isSkipErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "missing position") ||
		strings.Contains(msg, "missing user") ||
		strings.Contains(msg, "nil node")
}
