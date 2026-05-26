package device

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/exepirit/meshtastic-go/pkg/meshtastic"
	"github.com/exepirit/meshtastic-go/pkg/meshtastic/proto"
	gproto "google.golang.org/protobuf/proto"

	"github.com/relaymonkey/relaymesh-edge/internal/deviceconfigs"
)

// ApplyResult summarises a `set --to device` operation. Returned to
// the CLI so it can render the post-apply status line:
//
//	applied 1 sections, 0 channels, 0 drift, 1 reboot
type ApplyResult struct {
	// Sections is the ordered list of canonical submessage keys
	// the apply path sent to the device, in send order.
	Sections []string
	// ChannelsSent is the number of `SetChannel` admin messages
	// emitted (typically 0..8).
	ChannelsSent int
	// Rebooted is true when the device emitted FromRadio_Rebooted
	// after at least one of the per-section Set* writes. Useful
	// primarily for the post-apply re-read: when a reboot was
	// observed, the radio may still be reconnecting.
	Rebooted bool
	// DriftCount is the count of submessages that still differ
	// after the apply settled (re-read vs intended payload). Zero
	// is the happy case.
	DriftCount int
	// Drift carries the post-apply diff so the CLI can show *which*
	// submessages the firmware refused. When DriftCount == 0 this
	// is empty.
	Drift []deviceconfigs.SubmessageDiff
	// RereadInconclusive is true when the post-commit re-read came
	// back with a payload too sparse to compare against (radio still
	// reconnecting after the reboot). When true, DriftCount and
	// Drift are not populated and the CLI should print a "drift not
	// verified" line instead of a confident drift report.
	RereadInconclusive bool
}

// ApplyOptions tune the apply session.
type ApplyOptions struct {
	// RebootWait is no longer the post-commit wait window — there
	// is no commit step on this path. It's retained as the per-section
	// reboot grace (clamped internally to a small constant) so
	// existing callers stay source-compatible. Set 0 to use the
	// default.
	RebootWait time.Duration
	// SkipReread, when true, skips the post-commit re-read and
	// drift calculation. Useful for `--dry-run`.
	SkipReread bool
	// PreApplyState, if non-nil, is the canonical payload captured
	// before the apply session started. Used as a sanity reference
	// for the post-apply re-read: if the re-read returns a payload
	// with substantially fewer sections than were observed before
	// the commit, the radio almost certainly hadn't finished
	// reconnecting after the reboot. In that case Apply marks the
	// drift result as inconclusive (`ApplyResult.RereadInconclusive`)
	// rather than reporting every section the device actually has
	// as "missing".
	PreApplyState *deviceconfigs.CanonicalPayload
	// OnProgress, if non-nil, is invoked at every milestone in the
	// apply session. It is called from the same goroutine as Apply,
	// so the callback should be quick (a render to stderr is fine;
	// network I/O is not). Use this to drive a CLI progress UI —
	// per-section lines, a spinner during the wait windows, etc.
	// When nil, Apply emits no progress signals.
	OnProgress func(ApplyEvent)
}

// ApplyEvent describes one step in the apply state machine. The CLI
// translates these into operator-visible progress lines and spinners;
// the agent (Phase 2) will turn them into structured logs.
//
// Stages, in observation order:
//
//	"send_section"     — about to send SetConfig{Detail} (Detail = "config.lora", "module_config.mqtt", …).
//	"wire_payload"     — verbose dump of the protojson about to leave the host.
//	"section_sent"     — admin message acknowledged by the transport (NOT a firmware ack — see CLI rendering).
//	"section_reboot"   — firmware emitted FromRadio_Rebooted within the per-section grace window after this Set*.
//	"send_channel"     — about to send SetChannel; Index carries the slot.
//	"channel_sent"     — channel admin message acknowledged by the transport.
//	"channel_reboot"   — firmware rebooted after this SetChannel.
//	"reread_start"     — beginning the post-apply state re-read.
//	"reread_done"      — re-read returned a usable payload.
//	"reread_partial"   — re-read returned a sparse payload (radio still reconnecting).
//	"reread_failed"    — re-read errored entirely.
type ApplyEvent struct {
	Stage  string
	Detail string
	// Index / Total are populated for "send_section" / "section_sent"
	// and "send_channel" / "channel_sent" so the UI can render
	// "[2/3] config.position".
	Index, Total int
}

// defaultApplyOptions returns the recommended defaults.
func defaultApplyOptions() ApplyOptions {
	return ApplyOptions{RebootWait: 15 * time.Second}
}

// Apply ships per-section admin messages directly to the connected
// device — no `BeginEditSettings` / `CommitEditSettings` wrapping.
//
// Why no edit transaction:
//
//   - In firmware (`AdminModule::saveChanges`), when `hasOpenEditTransaction`
//     is true, the code path that persists *and* fires the
//     `configChanged` observer is skipped. The new value sits in RAM;
//     the radio's `reconfigure()` does not run; the result on next
//     read is racy and version-dependent.
//   - With NO open transaction, each `handleSetConfig` ends with
//     `saveChanges(SEGMENT_*, false)`, which calls `service->reloadConfig`,
//     which fires the `configChanged` observer, which makes
//     `RadioInterface::applyModemConfig()` actually re-clock the radio
//     with the new lora settings. This is what `meshtastic --set
//     lora.tx_power 20` does, and it's what makes the value stick.
//   - For sections that *do* require a reboot (region, role, security),
//     the firmware schedules one after the per-section save. We watch
//     for `FromRadio_Rebooted` in a short grace window after each
//     send and surface it in the progress stream so the operator
//     sees what's happening, and so the post-apply re-read knows the
//     radio is mid-reconnect.
//
// MeshPacket framing is deliberately matched to what every working
// Meshtastic client uses:
//
//   - `to = MyNodeInfo.my_node_num` — the device's own node number,
//     captured during the pre-apply state read. The "to=0" loopback
//     pattern works for reads but is not the canonical pattern for
//     local admin writes; addressing by node number is what the
//     official Python `meshtastic` CLI and your `enterprise_config.py`
//     both do.
//   - `want_ack = true` — the firmware acks the admin packet up to
//     AdminModule, which tells us the write reached the handler
//     instead of being dropped by some earlier filter.
//
// No `session_passkey`. Firmware (`AdminModule::handleAdminMessage`)
// only enforces the passkey when `mp.from != 0`; local Phone API
// admin writes carry `from = 0`, so the field is intentionally
// unused on this path.
func Apply(
	ctx context.Context,
	transport meshtastic.HardwareTransport,
	intended deviceconfigs.CanonicalPayload,
	opts ApplyOptions,
) (ApplyResult, error) {
	if opts.RebootWait == 0 {
		opts = defaultApplyOptions()
	}
	res := ApplyResult{}

	// emit fans events out to the optional OnProgress callback. A
	// nil callback turns every emit into a no-op, so Apply's hot path
	// stays untouched when no observer is attached.
	emit := func(ev ApplyEvent) {
		if opts.OnProgress != nil {
			opts.OnProgress(ev)
		}
	}

	// Address admin packets at the local node's actual node number.
	// PreApplyState carries a fresh full read; we extract my_node_num
	// from the saved snapshot so Apply doesn't need a second read.
	var localNodeNum uint32
	if opts.PreApplyState != nil {
		localNodeNum = preApplyNodeNum(*opts.PreApplyState)
	}
	if localNodeNum == 0 {
		// Defensive: pre-apply might not be populated in some
		// callers. Probe with a short metadata request — falls back
		// to to=0 if the device doesn't answer.
		localNodeNum = probeLocalNodeNum(ctx, transport)
	}

	// Pre-count the total number of section + channel sends so the
	// progress UI can render "[2/3]" without us having to send the
	// admin messages first.
	totalSections := 0
	for _, k := range deviceconfigs.ConfigKeys {
		if v, ok := intended.Config[k]; ok && len(v) > 0 {
			totalSections++
		}
	}
	for _, k := range deviceconfigs.ModuleConfigKeys {
		if v, ok := intended.ModuleConfig[k]; ok && len(v) > 0 {
			totalSections++
		}
	}
	totalChannels := 0
	for _, c := range intended.Channels {
		if len(c) > 0 {
			totalChannels++
		}
	}

	// observeReboot watches the FromRadio stream for a short window
	// after each Set*. If the firmware emits FromRadio_Rebooted, we
	// surface it via the progress callback and propagate the boolean
	// to the result so the CLI can render "device rebooting…" and
	// the re-read can wait until the radio is back. A timeout is the
	// happy path for sections like lora that apply live.
	const perSectionRebootGrace = 3 * time.Second
	observeReboot := func(label string, stage string) {
		if waitForReboot(ctx, transport, perSectionRebootGrace) {
			res.Rebooted = true
			emit(ApplyEvent{Stage: stage, Detail: label})
		}
	}

	// Send Config.* submessages.
	sectionIdx := 0
	for _, key := range deviceconfigs.ConfigKeys {
		raw, ok := intended.Config[key]
		if !ok || len(raw) == 0 {
			continue
		}
		sectionIdx++
		emit(ApplyEvent{Stage: "send_section", Detail: "config." + key, Index: sectionIdx, Total: totalSections})
		cfg, err := parseConfigSubmessage(key, raw)
		if err != nil {
			return res, fmt.Errorf("parse config.%s: %w", key, err)
		}
		emit(ApplyEvent{Stage: "wire_payload", Detail: "config." + key + ": " + protoJSONOneLine(cfg)})
		if err := sendAdmin(ctx, transport, &proto.AdminMessage{
			PayloadVariant: &proto.AdminMessage_SetConfig{SetConfig: cfg},
		}, localNodeNum); err != nil {
			return res, fmt.Errorf("set config.%s: %w", key, err)
		}
		res.Sections = append(res.Sections, "config."+key)
		emit(ApplyEvent{Stage: "section_sent", Detail: "config." + key, Index: sectionIdx, Total: totalSections})
		observeReboot("config."+key, "section_reboot")
	}

	// Send ModuleConfig.* submessages.
	for _, key := range deviceconfigs.ModuleConfigKeys {
		raw, ok := intended.ModuleConfig[key]
		if !ok || len(raw) == 0 {
			continue
		}
		sectionIdx++
		emit(ApplyEvent{Stage: "send_section", Detail: "module_config." + key, Index: sectionIdx, Total: totalSections})
		mod, err := parseModuleConfigSubmessage(key, raw)
		if err != nil {
			return res, fmt.Errorf("parse module_config.%s: %w", key, err)
		}
		emit(ApplyEvent{Stage: "wire_payload", Detail: "module_config." + key + ": " + protoJSONOneLine(mod)})
		if err := sendAdmin(ctx, transport, &proto.AdminMessage{
			PayloadVariant: &proto.AdminMessage_SetModuleConfig{SetModuleConfig: mod},
		}, localNodeNum); err != nil {
			return res, fmt.Errorf("set module_config.%s: %w", key, err)
		}
		res.Sections = append(res.Sections, "module_config."+key)
		emit(ApplyEvent{Stage: "section_sent", Detail: "module_config." + key, Index: sectionIdx, Total: totalSections})
		observeReboot("module_config."+key, "section_reboot")
	}

	// Send Channel rows. Each channel in the canonical payload is a
	// fully-formed Meshtastic `Channel` protojson; we hand it
	// straight to AdminMessage_SetChannel.
	channelIdx := 0
	for i, rawCh := range intended.Channels {
		if len(rawCh) == 0 {
			continue
		}
		ch := &proto.Channel{}
		if err := unmarshalOpts.Unmarshal(rawCh, ch); err != nil {
			return res, fmt.Errorf("parse channels[%d]: %w", i, err)
		}
		// The `index` field is authoritative when present;
		// fall back to the array position if not.
		if ch.Index == 0 && i != 0 {
			ch.Index = int32(i)
		}
		channelIdx++
		emit(ApplyEvent{
			Stage: "send_channel", Detail: fmt.Sprintf("channels[%d]", ch.Index),
			Index: channelIdx, Total: totalChannels,
		})
		if err := sendAdmin(ctx, transport, &proto.AdminMessage{
			PayloadVariant: &proto.AdminMessage_SetChannel{SetChannel: ch},
		}, localNodeNum); err != nil {
			return res, fmt.Errorf("set channel %d: %w", ch.Index, err)
		}
		res.ChannelsSent++
		emit(ApplyEvent{
			Stage: "channel_sent", Detail: fmt.Sprintf("channels[%d]", ch.Index),
			Index: channelIdx, Total: totalChannels,
		})
		observeReboot(fmt.Sprintf("channels[%d]", ch.Index), "channel_reboot")
	}

	if !opts.SkipReread {
		emit(ApplyEvent{Stage: "reread_start"})
		readCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		after, err := GetState(readCtx, transport)
		if err != nil {
			emit(ApplyEvent{Stage: "reread_failed", Detail: err.Error()})
		} else {
			actual, err := ToCanonicalPayload(after)
			if err != nil {
				emit(ApplyEvent{Stage: "reread_failed", Detail: err.Error()})
			} else if isRereadInconclusive(actual, opts.PreApplyState) {
				// The radio came back to us with a partial state
				// — almost always because it's still mid-reconnect
				// after CommitEditSettings. Reporting "29 drift"
				// in this case is just noise; the operator's
				// next read will show the real state. Mark the
				// drift unverified and let the CLI print a
				// short hint.
				res.RereadInconclusive = true
				emit(ApplyEvent{Stage: "reread_partial"})
			} else {
				// Drift is "for the sections we actually wrote,
				// did the device end up where we asked?". Project
				// the full re-read onto intended's sparse shape
				// before diffing — otherwise every section the
				// device has that we didn't ship reads as "added"
				// noise, which is exactly the 29-drift footgun
				// from the user-visible apply session.
				projected := actual.ProjectOnto(intended)
				drift := deviceconfigs.Diff(intended, projected)
				res.DriftCount = deviceconfigs.CountChanges(drift)
				res.Drift = drift
				emit(ApplyEvent{Stage: "reread_done"})
			}
		}
	}
	return res, nil
}

// isRereadInconclusive returns true when the post-apply payload looks
// like a radio that hasn't finished a clean WantConfigId sweep. The
// signal we trust most is "channels": every Meshtastic device we've
// seen reports at least one channel (the PRIMARY); a re-read that
// comes back with zero is the radio still reconnecting. As a second
// guard, if we have a pre-apply snapshot, a >50% drop in observed
// section count is treated as inconclusive too — apply only mutates
// what the operator asked for, so a healthy re-read has roughly the
// same population as the pre-apply snapshot.
func isRereadInconclusive(actual deviceconfigs.CanonicalPayload, pre *deviceconfigs.CanonicalPayload) bool {
	if len(actual.Channels) == 0 {
		return true
	}
	if pre == nil {
		return false
	}
	preCount := len(pre.Channels) + len(pre.Config) + len(pre.ModuleConfig)
	postCount := len(actual.Channels) + len(actual.Config) + len(actual.ModuleConfig)
	if preCount == 0 {
		return false
	}
	// Trip if the post-apply read kept fewer than half the sections
	// we saw a moment ago. A real apply changes a handful of fields,
	// not the population of present sections.
	return postCount*2 < preCount
}

// protoJSONOneLine renders a protobuf message as compact protojson
// (one line, snake_case keys, default values omitted). Used by the
// "wire_payload" progress event so the operator sees the exact
// SetConfig / SetModuleConfig contents that will be transmitted, in
// the same shape they appear in `rmesh device config get`. Any
// marshalling error falls back to a stub string — the diagnostic
// can't be allowed to fail the apply.
func protoJSONOneLine(msg gproto.Message) string {
	if msg == nil {
		return "<nil>"
	}
	b, err := marshalOpts.Marshal(msg)
	if err != nil {
		return fmt.Sprintf("<marshal error: %v>", err)
	}
	return string(b)
}

// preApplyNodeNum extracts the local node number from a CanonicalPayload
// snapshot if it carries one. Today the canonical payload doesn't
// surface MyNodeInfo (it intentionally omits the per-device identity
// off the radio config wire), so this returns 0 and we fall back to
// probeLocalNodeNum. Kept as a hook so a future schema bump can
// thread node_num through cheaply.
func preApplyNodeNum(_ deviceconfigs.CanonicalPayload) uint32 { return 0 }

// probeLocalNodeNum sends a GetDeviceMetadata request and reads back
// the My-Node value out of the next FromRadio_Packet that comes from
// the device. Total budget: ~3 s. On any failure we return 0, which
// makes Apply fall back to to=0 framing — same as before this change.
func probeLocalNodeNum(ctx context.Context, transport meshtastic.HardwareTransport) uint32 {
	reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := sendAdmin(reqCtx, transport, &proto.AdminMessage{
		PayloadVariant: &proto.AdminMessage_GetDeviceMetadataRequest{
			GetDeviceMetadataRequest: true,
		},
	}, 0); err != nil {
		return 0
	}
	for {
		pkt, err := transport.ReceiveFromRadio(reqCtx)
		if err != nil {
			return 0
		}
		// The response carries the firmware's `from = my_node_num`
		// on the MeshPacket envelope, regardless of whether the
		// AdminMessage payload itself decodes for us. That's exactly
		// the value we want to address admin writes at.
		if mesh := pkt.GetPacket(); mesh != nil && mesh.GetFrom() != 0 {
			return mesh.GetFrom()
		}
	}
}

// sendAdmin wraps an AdminMessage in a ToRadio packet on the ADMIN_APP
// portnum and ships it to the firmware.
//
// `localNodeNum` is the connected device's actual node number,
// captured during the pre-apply state read. Addressing admin writes
// at the real node number (rather than the legacy `to=0` loopback)
// matches the canonical pattern used by the official Python
// `meshtastic` CLI and your `enterprise_config.py`. When 0 is passed
// we fall back to `to=0`, which still works on read paths and on
// older firmware builds that didn't differentiate.
//
// `WantAck` is true so the firmware acknowledges the admin packet up
// to AdminModule. Without it we couldn't tell a silently-dropped
// write from one that reached the handler — exactly the ambiguity
// the user-facing "applied N sections, K drift" line was muddling.
func sendAdmin(ctx context.Context, transport meshtastic.HardwareTransport, msg *proto.AdminMessage, localNodeNum uint32) error {
	payload, err := gproto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal admin: %w", err)
	}
	pkt := &proto.MeshPacket{
		To: localNodeNum,
		PayloadVariant: &proto.MeshPacket_Decoded{
			Decoded: &proto.Data{
				Portnum: proto.PortNum_ADMIN_APP,
				Payload: payload,
			},
		},
		Id:       rand.Uint32(),
		WantAck:  true,
		HopLimit: 0,
	}
	return transport.SendToRadio(ctx, &proto.ToRadio{
		PayloadVariant: &proto.ToRadio_Packet{Packet: pkt},
	})
}

// waitForReboot blocks until a `FromRadio_Rebooted` packet arrives,
// the configured `wait` deadline expires, or a short grace window
// elapses with no traffic from the device.
//
// Most config changes do *not* trigger a firmware reboot — only a
// handful of fields (region, primary-channel rotation, …) do. The
// firmware emits `FromRadio_Rebooted` within ~1s of CommitEditSettings
// when a reboot is happening; if no traffic at all arrives in the
// grace window after Commit, no reboot is coming. Sitting through
// the full `wait` window in that case made every apply look hung
// for 15 seconds when in reality the device was already done.
//
// Behaviour:
//
//   - Reboot signal seen in either window  → return true.
//   - No packets at all in grace window     → return false (fast path,
//     "no reboot needed"). This is the dominant case.
//   - Some packets but no Rebooted by `wait` deadline → return false
//     (timeout — covers the rare case where the firmware emits other
//     packets before the actual Rebooted, or USB drops the link).
func waitForReboot(ctx context.Context, transport meshtastic.HardwareTransport, wait time.Duration) bool {
	const grace = 3 * time.Second
	graceWindow := grace
	if graceWindow > wait {
		graceWindow = wait
	}

	graceCtx, gCancel := context.WithTimeout(ctx, graceWindow)
	first, err := transport.ReceiveFromRadio(graceCtx)
	gCancel()
	if err != nil {
		// Grace window elapsed in silence → no reboot is coming.
		return false
	}
	if _, ok := first.PayloadVariant.(*proto.FromRadio_Rebooted); ok {
		return true
	}
	// Got *some* packet. The firmware is talking — keep an ear on the
	// stream up to the full wait in case Rebooted arrives behind a
	// few telemetry / position packets.
	remainder := wait - graceWindow
	if remainder <= 0 {
		return false
	}
	deadline, cancel := context.WithTimeout(ctx, remainder)
	defer cancel()
	for {
		packet, err := transport.ReceiveFromRadio(deadline)
		if err != nil {
			return false
		}
		if _, ok := packet.PayloadVariant.(*proto.FromRadio_Rebooted); ok {
			return true
		}
	}
}

// parseConfigSubmessage turns one canonical submessage into a Config
// envelope addressed at the right oneof variant.
func parseConfigSubmessage(key string, raw json.RawMessage) (*proto.Config, error) {
	switch key {
	case "device":
		x := &proto.Config_DeviceConfig{}
		if err := unmarshalOpts.Unmarshal(raw, x); err != nil {
			return nil, err
		}
		return &proto.Config{PayloadVariant: &proto.Config_Device{Device: x}}, nil
	case "position":
		x := &proto.Config_PositionConfig{}
		if err := unmarshalOpts.Unmarshal(raw, x); err != nil {
			return nil, err
		}
		return &proto.Config{PayloadVariant: &proto.Config_Position{Position: x}}, nil
	case "power":
		x := &proto.Config_PowerConfig{}
		if err := unmarshalOpts.Unmarshal(raw, x); err != nil {
			return nil, err
		}
		return &proto.Config{PayloadVariant: &proto.Config_Power{Power: x}}, nil
	case "network":
		x := &proto.Config_NetworkConfig{}
		if err := unmarshalOpts.Unmarshal(raw, x); err != nil {
			return nil, err
		}
		return &proto.Config{PayloadVariant: &proto.Config_Network{Network: x}}, nil
	case "display":
		x := &proto.Config_DisplayConfig{}
		if err := unmarshalOpts.Unmarshal(raw, x); err != nil {
			return nil, err
		}
		return &proto.Config{PayloadVariant: &proto.Config_Display{Display: x}}, nil
	case "lora":
		x := &proto.Config_LoRaConfig{}
		if err := unmarshalOpts.Unmarshal(raw, x); err != nil {
			return nil, err
		}
		return &proto.Config{PayloadVariant: &proto.Config_Lora{Lora: x}}, nil
	case "bluetooth":
		x := &proto.Config_BluetoothConfig{}
		if err := unmarshalOpts.Unmarshal(raw, x); err != nil {
			return nil, err
		}
		return &proto.Config{PayloadVariant: &proto.Config_Bluetooth{Bluetooth: x}}, nil
	case "security":
		x := &proto.Config_SecurityConfig{}
		if err := unmarshalOpts.Unmarshal(raw, x); err != nil {
			return nil, err
		}
		return &proto.Config{PayloadVariant: &proto.Config_Security{Security: x}}, nil
	}
	return nil, errors.New("unknown config key: " + key)
}

// parseModuleConfigSubmessage is the parallel ModuleConfig fan-out.
func parseModuleConfigSubmessage(key string, raw json.RawMessage) (*proto.ModuleConfig, error) {
	switch key {
	case "mqtt":
		x := &proto.ModuleConfig_MQTTConfig{}
		if err := unmarshalOpts.Unmarshal(raw, x); err != nil {
			return nil, err
		}
		return &proto.ModuleConfig{PayloadVariant: &proto.ModuleConfig_Mqtt{Mqtt: x}}, nil
	case "serial":
		x := &proto.ModuleConfig_SerialConfig{}
		if err := unmarshalOpts.Unmarshal(raw, x); err != nil {
			return nil, err
		}
		return &proto.ModuleConfig{PayloadVariant: &proto.ModuleConfig_Serial{Serial: x}}, nil
	case "ext_notification":
		x := &proto.ModuleConfig_ExternalNotificationConfig{}
		if err := unmarshalOpts.Unmarshal(raw, x); err != nil {
			return nil, err
		}
		return &proto.ModuleConfig{PayloadVariant: &proto.ModuleConfig_ExternalNotification{ExternalNotification: x}}, nil
	case "store_forward":
		x := &proto.ModuleConfig_StoreForwardConfig{}
		if err := unmarshalOpts.Unmarshal(raw, x); err != nil {
			return nil, err
		}
		return &proto.ModuleConfig{PayloadVariant: &proto.ModuleConfig_StoreForward{StoreForward: x}}, nil
	case "range_test":
		x := &proto.ModuleConfig_RangeTestConfig{}
		if err := unmarshalOpts.Unmarshal(raw, x); err != nil {
			return nil, err
		}
		return &proto.ModuleConfig{PayloadVariant: &proto.ModuleConfig_RangeTest{RangeTest: x}}, nil
	case "telemetry":
		x := &proto.ModuleConfig_TelemetryConfig{}
		if err := unmarshalOpts.Unmarshal(raw, x); err != nil {
			return nil, err
		}
		return &proto.ModuleConfig{PayloadVariant: &proto.ModuleConfig_Telemetry{Telemetry: x}}, nil
	case "canned_message":
		x := &proto.ModuleConfig_CannedMessageConfig{}
		if err := unmarshalOpts.Unmarshal(raw, x); err != nil {
			return nil, err
		}
		return &proto.ModuleConfig{PayloadVariant: &proto.ModuleConfig_CannedMessage{CannedMessage: x}}, nil
	case "audio":
		x := &proto.ModuleConfig_AudioConfig{}
		if err := unmarshalOpts.Unmarshal(raw, x); err != nil {
			return nil, err
		}
		return &proto.ModuleConfig{PayloadVariant: &proto.ModuleConfig_Audio{Audio: x}}, nil
	case "remote_hardware":
		x := &proto.ModuleConfig_RemoteHardwareConfig{}
		if err := unmarshalOpts.Unmarshal(raw, x); err != nil {
			return nil, err
		}
		return &proto.ModuleConfig{PayloadVariant: &proto.ModuleConfig_RemoteHardware{RemoteHardware: x}}, nil
	case "neighbor_info":
		x := &proto.ModuleConfig_NeighborInfoConfig{}
		if err := unmarshalOpts.Unmarshal(raw, x); err != nil {
			return nil, err
		}
		return &proto.ModuleConfig{PayloadVariant: &proto.ModuleConfig_NeighborInfo{NeighborInfo: x}}, nil
	case "ambient_lighting":
		x := &proto.ModuleConfig_AmbientLightingConfig{}
		if err := unmarshalOpts.Unmarshal(raw, x); err != nil {
			return nil, err
		}
		return &proto.ModuleConfig{PayloadVariant: &proto.ModuleConfig_AmbientLighting{AmbientLighting: x}}, nil
	case "detection_sensor":
		x := &proto.ModuleConfig_DetectionSensorConfig{}
		if err := unmarshalOpts.Unmarshal(raw, x); err != nil {
			return nil, err
		}
		return &proto.ModuleConfig{PayloadVariant: &proto.ModuleConfig_DetectionSensor{DetectionSensor: x}}, nil
	case "paxcounter":
		x := &proto.ModuleConfig_PaxcounterConfig{}
		if err := unmarshalOpts.Unmarshal(raw, x); err != nil {
			return nil, err
		}
		return &proto.ModuleConfig{PayloadVariant: &proto.ModuleConfig_Paxcounter{Paxcounter: x}}, nil
	}
	return nil, errors.New("unknown module_config key: " + key)
}
