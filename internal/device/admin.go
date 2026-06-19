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
	// Notifications carries any `ClientNotification` messages the
	// firmware emitted while processing this apply — e.g. the exact
	// reason a section was rejected ("Region EU_868 needs sub-GHz,
	// which this radio does not support"). The CLI surfaces these
	// verbatim so a rejected apply shows the device's own words
	// instead of an inferred cause.
	Notifications []DeviceNotification
}

// DeviceNotification is one `ClientNotification` the firmware emitted
// during an apply (validation warnings/errors, region rejections, …).
type DeviceNotification struct {
	Level   string
	Message string
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
	// LocalNodeNum is the connected device's own node number, taken
	// from the authoritative MyNodeInfo captured during the pre-apply
	// GetState sweep. Admin packets must be addressed to this node
	// (`to == my_node_num`) or the firmware won't process them as local
	// admin — it treats them as mesh traffic for some other node and
	// silently ignores them locally (no apply, no reboot, no NAK). When
	// 0, Apply falls back to a best-effort metadata probe, which is
	// unreliable on a busy mesh (it can latch a neighbor's node number),
	// so callers should always set this from the read.
	LocalNodeNum uint32
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
	return ApplyOptions{RebootWait: 20 * time.Second}
}

// Apply wraps all per-section admin writes in a single
// `BeginEditSettings` … `CommitEditSettings` transaction — the same
// protocol the official Meshtastic app and Python CLI use for bulk
// config.
//
// Why the edit transaction (this replaced an earlier per-section,
// no-transaction approach that could not survive a multi-section apply):
//
//   - While `hasOpenEditTransaction` is true, firmware
//     (`AdminModule::saveChanges`) *stages* each `Set*` in RAM: it does
//     not call `service->reloadConfig`, does not fire the `configChanged`
//     observer, and does not reboot. Crucially, a region / modem-preset
//     change is therefore NOT applied live mid-stream — so the radio
//     isn't re-clocked and the USB-CDC / BLE link stays up through every
//     section. The previous approach applied the first reboot- or
//     reconfigure-triggering section immediately, dropping the link
//     before the remaining sections were sent; the apply could never
//     converge.
//   - `CommitEditSettings` clears the transaction and runs one
//     `saveChanges(CONFIG|MODULECONFIG|DEVICESTATE|CHANNELS|NODEDATABASE)`
//     — a single persist of every staged section — then schedules one
//     reboot. On reboot the firmware loads the saved config from disk
//     and `reconfigure()` runs at boot, so the values stick atomically.
//   - The caller owns reconnect + re-read after the commit reboot (see
//     cmd/rmesh/cmd/device_config_apply.go); a partial re-read while the
//     radio is still reconnecting is reported as inconclusive rather than
//     as drift.
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
// only enforces the passkey when `mp.from != 0`; local device sessions
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

	// Address admin packets at the local node's actual node number. The
	// caller passes the authoritative my_node_num from the GetState read;
	// only when that's absent do we fall back to the (unreliable) probe.
	localNodeNum := opts.LocalNodeNum
	if localNodeNum == 0 && opts.PreApplyState != nil {
		localNodeNum = preApplyNodeNum(*opts.PreApplyState)
	}
	if localNodeNum == 0 {
		// Defensive: node number not supplied. Probe with a short
		// metadata request — unreliable on a busy mesh (can latch a
		// neighbor's node), falls back to to=0 if the device is silent.
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

	rebootObserve := opts.RebootWait
	if rebootObserve <= 0 {
		rebootObserve = defaultApplyOptions().RebootWait
	}

	// Open an edit transaction. This is how every working Meshtastic
	// client (the official app, the Python CLI) applies bulk config:
	// while `hasOpenEditTransaction` is true the firmware *stages* each
	// Set* in RAM without saving, without firing the `configChanged`
	// observer, and without rebooting. That keeps the serial/BLE link
	// up through every section — critical when one of them is a region
	// or modem-preset change that would otherwise re-clock the radio
	// (dropping USB) the instant it was applied live. CommitEditSettings
	// then persists everything in one shot and reboots once.
	noteEmit := func(n DeviceNotification) {
		emit(ApplyEvent{Stage: "notification", Detail: n.Level + ": " + n.Message})
	}

	// Per-admin-packet ACK budget. Sending the whole surface as one
	// unpaced burst overruns the firmware's serial RX buffer: later
	// packets (including CommitEditSettings) are dropped, the transaction
	// never commits, and the apply silently no-ops. Waiting for each
	// packet's routing ACK before sending the next paces the stream (the
	// firmware drains its buffer between packets) and lets a NAK be
	// attributed to the exact section. On ACK timeout we proceed — pacing
	// is still achieved and the post-commit observe is a backstop.
	const ackTimeout = 3 * time.Second

	emit(ApplyEvent{Stage: "send_section", Detail: "begin edit transaction"})
	if _, err := sendAndAwaitAck(ctx, transport, &proto.AdminMessage{
		PayloadVariant: &proto.AdminMessage_BeginEditSettings{BeginEditSettings: true},
	}, localNodeNum, "begin edit transaction", ackTimeout, noteEmit); err != nil {
		return res, fmt.Errorf("begin edit transaction: %w", err)
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
		notes, err := sendAndAwaitAck(ctx, transport, &proto.AdminMessage{
			PayloadVariant: &proto.AdminMessage_SetConfig{SetConfig: cfg},
		}, localNodeNum, "config."+key, ackTimeout, noteEmit)
		if err != nil {
			return res, fmt.Errorf("set config.%s: %w", key, err)
		}
		res.Notifications = append(res.Notifications, notes...)
		res.Sections = append(res.Sections, "config."+key)
		emit(ApplyEvent{Stage: "section_sent", Detail: "config." + key, Index: sectionIdx, Total: totalSections})
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
		notes, err := sendAndAwaitAck(ctx, transport, &proto.AdminMessage{
			PayloadVariant: &proto.AdminMessage_SetModuleConfig{SetModuleConfig: mod},
		}, localNodeNum, "module_config."+key, ackTimeout, noteEmit)
		if err != nil {
			return res, fmt.Errorf("set module_config.%s: %w", key, err)
		}
		res.Notifications = append(res.Notifications, notes...)
		res.Sections = append(res.Sections, "module_config."+key)
		emit(ApplyEvent{Stage: "section_sent", Detail: "module_config." + key, Index: sectionIdx, Total: totalSections})
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
		label := fmt.Sprintf("channels[%d]", ch.Index)
		emit(ApplyEvent{Stage: "send_channel", Detail: label, Index: channelIdx, Total: totalChannels})
		notes, err := sendAndAwaitAck(ctx, transport, &proto.AdminMessage{
			PayloadVariant: &proto.AdminMessage_SetChannel{SetChannel: ch},
		}, localNodeNum, label, ackTimeout, noteEmit)
		if err != nil {
			return res, fmt.Errorf("set channel %d: %w", ch.Index, err)
		}
		res.Notifications = append(res.Notifications, notes...)
		res.ChannelsSent++
		emit(ApplyEvent{Stage: "channel_sent", Detail: label, Index: channelIdx, Total: totalChannels})
	}

	// Commit: firmware clears the transaction, persists CONFIG +
	// MODULECONFIG + DEVICESTATE + CHANNELS + NODEDATABASE in one write,
	// and schedules a single reboot (~DEFAULT_REBOOT_SECONDS). After this
	// the link drops; the caller's reconnect loop waits for the device to
	// come back and re-reads to verify.
	emit(ApplyEvent{Stage: "send_section", Detail: "commit edit transaction"})
	commitID, err := sendAdmin(ctx, transport, &proto.AdminMessage{
		PayloadVariant: &proto.AdminMessage_CommitEditSettings{CommitEditSettings: true},
	}, localNodeNum)
	if err != nil {
		return res, fmt.Errorf("commit edit transaction: %w", err)
	}
	rebooted, notes := observeCommit(ctx, transport, rebootObserve, map[uint32]string{commitID: "commit edit transaction"}, noteEmit)
	res.Notifications = append(res.Notifications, notes...)
	if rebooted {
		res.Rebooted = true
		emit(ApplyEvent{Stage: "section_reboot", Detail: "commit"})
	}

	if !opts.SkipReread {
		emit(ApplyEvent{Stage: "reread_start"})
		readCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		after, err := GetState(readCtx, transport)
		if err != nil {
			// The commit reboot is tearing the link down. We have not
			// verified the result, so mark it inconclusive — the caller
			// reconnects and re-reads on the next pass rather than
			// reporting an unverified success.
			res.RereadInconclusive = true
			emit(ApplyEvent{Stage: "reread_failed", Detail: err.Error()})
		} else {
			// Surface any WARN+ firmware logs seen during the re-read —
			// e.g. a boot-time radio reconfigure failure that reverted the
			// config. These are logged, never sent as a ClientNotification.
			for _, n := range after.Logs {
				res.Notifications = append(res.Notifications, n)
				emit(ApplyEvent{Stage: "notification", Detail: n.Level + ": " + n.Message})
			}
			actual, err := ToCanonicalPayload(after)
			if err != nil {
				res.RereadInconclusive = true
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

	if _, err := sendAdmin(reqCtx, transport, &proto.AdminMessage{
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
func sendAdmin(ctx context.Context, transport meshtastic.HardwareTransport, msg *proto.AdminMessage, localNodeNum uint32) (uint32, error) {
	payload, err := gproto.Marshal(msg)
	if err != nil {
		return 0, fmt.Errorf("marshal admin: %w", err)
	}
	id := rand.Uint32()
	pkt := &proto.MeshPacket{
		To: localNodeNum,
		PayloadVariant: &proto.MeshPacket_Decoded{
			Decoded: &proto.Data{
				Portnum: proto.PortNum_ADMIN_APP,
				Payload: payload,
			},
		},
		Id:       id,
		WantAck:  true,
		HopLimit: 0,
	}
	return id, transport.SendToRadio(ctx, &proto.ToRadio{
		PayloadVariant: &proto.ToRadio_Packet{Packet: pkt},
	})
}

// PrimeRegion crosses the LoRa band (2.4 GHz ↔ sub-GHz) on a dual-band radio
// (LR11x0), the one change the radio won't make on a live transactional apply.
//
// It mirrors exactly what `meshtastic --set lora.region <R> --set lora.use_preset true`
// does (captured from its debug log): a get-modify-set — take the device's
// CURRENT lora, change only `region` and `use_preset=true`, and send it as a
// single standalone SetConfig (no edit transaction, no explicit reboot; the
// device reboots itself onto the new band). Keeping the current `modem_preset`
// is essential: it's already valid for the radio, whereas substituting the
// target profile's preset (e.g. LONG_SLOW) can be invalid for the new region
// and the firmware reverts. After this, the caller applies the full intended
// config (including any custom `use_preset=false` lora) within the new band.
//
// `currentLoRa` is the device's current lora submessage (canonical protojson);
// `region` is the target region name (e.g. "EU_868").
func PrimeRegion(ctx context.Context, transport meshtastic.HardwareTransport, currentLoRa json.RawMessage, region string, localNodeNum uint32) error {
	code, ok := proto.Config_LoRaConfig_RegionCode_value[region]
	if !ok {
		return fmt.Errorf("unknown LoRa region %q", region)
	}
	lora := &proto.Config_LoRaConfig{}
	if len(currentLoRa) > 0 {
		if err := unmarshalOpts.Unmarshal(currentLoRa, lora); err != nil {
			return fmt.Errorf("parse current lora: %w", err)
		}
	}
	// Change only region + use_preset; keep every other current field so the
	// preset config stays valid for the radio (the get-modify-set meshtastic does).
	lora.Region = proto.Config_LoRaConfig_RegionCode(code)
	lora.UsePreset = true
	if _, err := sendAdmin(ctx, transport, &proto.AdminMessage{
		PayloadVariant: &proto.AdminMessage_SetConfig{
			SetConfig: &proto.Config{PayloadVariant: &proto.Config_Lora{Lora: lora}},
		},
	}, localNodeNum); err != nil {
		return fmt.Errorf("set region: %w", err)
	}
	return nil
}

// Reboot asks the device to reboot in `seconds`. The apply path uses it to
// force a reboot for changes the firmware only applies at boot (a within-band
// lora change that fails a live reconfigure), rather than relying on the
// firmware's own fault-reset to fire.
func Reboot(ctx context.Context, transport meshtastic.HardwareTransport, localNodeNum uint32, seconds int32) error {
	_, err := sendAdmin(ctx, transport, &proto.AdminMessage{
		PayloadVariant: &proto.AdminMessage_RebootSeconds{RebootSeconds: seconds},
	}, localNodeNum)
	return err
}

// sendAndAwaitAck sends one admin message and blocks until its routing
// ACK/NAK (correlated by packet id) arrives or `timeout` elapses, then
// returns. This is deliberate flow control: a Meshtastic radio cannot
// absorb the whole config surface as one unpaced burst over serial
// (its RX buffer overruns and later packets — including the commit —
// are silently dropped), so we send one packet at a time and wait for
// the firmware to acknowledge it before sending the next, exactly as
// the official clients do.
//
// While waiting it captures any device-reported condition (a NAK for
// this packet, or a ClientNotification emitted during processing) via
// packetToNotification and forwards it to onNote. On timeout it returns
// no error — the firmware may simply not ack a given message; the wait
// still paced the stream, and the post-commit observe is a backstop.
func sendAndAwaitAck(
	ctx context.Context,
	transport meshtastic.HardwareTransport,
	msg *proto.AdminMessage,
	localNodeNum uint32,
	label string,
	timeout time.Duration,
	onNote func(DeviceNotification),
) ([]DeviceNotification, error) {
	id, err := sendAdmin(ctx, transport, msg, localNodeNum)
	if err != nil {
		return nil, err
	}
	ids := map[uint32]string{id: label}
	var notes []DeviceNotification

	deadline, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for {
		pkt, rerr := transport.ReceiveFromRadio(deadline)
		if rerr != nil {
			return notes, nil // ack timed out — proceed; the wait still paced the send
		}
		if n, ok := packetToNotification(pkt, ids); ok {
			notes = append(notes, n)
			if onNote != nil {
				onNote(n)
			}
		}
		if isRoutingResponseFor(pkt, id) {
			// The routing ACK can arrive just before the firmware's
			// ClientNotification for the same write, so don't return the
			// instant we see it — drain a short trailing window for a
			// notification that follows the ack.
			notes = append(notes, drainNotifications(ctx, transport, 400*time.Millisecond, ids, onNote)...)
			return notes, nil
		}
	}
}

// isRoutingResponseFor reports whether pkt is the ROUTING_APP ack/nak the
// firmware sent in response to the admin packet with the given id.
func isRoutingResponseFor(pkt *proto.FromRadio, id uint32) bool {
	mp, ok := pkt.PayloadVariant.(*proto.FromRadio_Packet)
	if !ok {
		return false
	}
	d := mp.Packet.GetDecoded()
	return d != nil && d.GetPortnum() == proto.PortNum_ROUTING_APP && d.GetRequestId() == id
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
	rebooted, _ := observeCommit(ctx, transport, wait, nil, nil)
	return rebooted
}

// packetToNotification turns a FromRadio packet into an operator-facing
// notification when it carries a device-reported condition: either a
// `ClientNotification` (firmware validation warning/error) or a Routing
// NAK (allocErrorResponse) acking one of our admin packets with a
// non-NONE error. `sentIDs` maps admin packet ids → section labels so a
// NAK names the section the firmware rejected. Returns ok=false for
// anything else (live packets, plain ACKs, …).
func packetToNotification(pkt *proto.FromRadio, sentIDs map[uint32]string) (DeviceNotification, bool) {
	switch v := pkt.PayloadVariant.(type) {
	case *proto.FromRadio_ClientNotification:
		if v.ClientNotification == nil {
			return DeviceNotification{}, false
		}
		return DeviceNotification{
			Level:   logLevelName(v.ClientNotification.Level),
			Message: v.ClientNotification.Message,
		}, true
	case *proto.FromRadio_LogRecord:
		// The firmware's own debug log. Surface WARN/ERROR/CRITICAL lines
		// only — these carry the real reason a config write was rejected,
		// clamped, or failed to reconfigure ("Channel number invalid for
		// EU_868", "Invalid LoRa config", "Reconfigure failed, rebooting",
		// …), which the admin-layer ClientNotification / Routing channels
		// don't always relay. INFO/DEBUG/TRACE are dropped as noise.
		lr := v.LogRecord
		// INFO and above: the firmware narrates config handling at INFO
		// ("Set config: LoRa", region swaps, initRegion), which is what
		// explains a config that reverts without a validation error.
		// DEBUG/TRACE are dropped as noise. Only emitted at all when the
		// device has security.debug_log_api_enabled set.
		if lr == nil || lr.Level < proto.LogRecord_INFO {
			return DeviceNotification{}, false
		}
		msg := lr.Message
		if lr.Source != "" {
			msg = lr.Source + ": " + msg
		}
		return DeviceNotification{Level: logLevelName(lr.Level), Message: msg}, true
	case *proto.FromRadio_Packet:
		d := v.Packet.GetDecoded()
		if d == nil || d.GetPortnum() != proto.PortNum_ROUTING_APP {
			return DeviceNotification{}, false
		}
		var routing proto.Routing
		if err := gproto.Unmarshal(d.GetPayload(), &routing); err != nil {
			return DeviceNotification{}, false
		}
		if routing.GetErrorReason() == proto.Routing_NONE {
			return DeviceNotification{}, false // plain ACK
		}
		label, ours := sentIDs[d.GetRequestId()]
		if !ours {
			return DeviceNotification{}, false
		}
		name := proto.Routing_Error_name[int32(routing.GetErrorReason())]
		if name == "" {
			name = fmt.Sprintf("error %d", routing.GetErrorReason())
		}
		return DeviceNotification{
			Level:   "error",
			Message: fmt.Sprintf("%s rejected by firmware (%s)", label, name),
		}, true
	}
	return DeviceNotification{}, false
}

// observeCommit reads the FromRadio stream after CommitEditSettings,
// returning whether the firmware rebooted and any ClientNotification
// messages it emitted while validating the staged sections. Those
// notifications are the device's own rejection reasons (e.g. an
// unsupported region), captured so the CLI can show them verbatim
// rather than guessing.
//
// It stops at the first of: a FromRadio_Rebooted, `wait` elapsing, or
// an initial grace window passing in silence (the firmware isn't
// talking, so no reboot is coming and the buffered notifications, if
// any, have already been drained).
func observeCommit(
	ctx context.Context,
	transport meshtastic.HardwareTransport,
	wait time.Duration,
	sentIDs map[uint32]string,
	onNote func(DeviceNotification),
) (bool, []DeviceNotification) {
	const grace = 3 * time.Second
	graceWindow := grace
	if graceWindow > wait {
		graceWindow = wait
	}

	var notes []DeviceNotification
	collect := func(pkt *proto.FromRadio) {
		n, ok := packetToNotification(pkt, sentIDs)
		if !ok {
			return
		}
		notes = append(notes, n)
		if onNote != nil {
			onNote(n)
		}
	}

	graceCtx, gCancel := context.WithTimeout(ctx, graceWindow)
	first, err := transport.ReceiveFromRadio(graceCtx)
	gCancel()
	if err != nil {
		// Grace window elapsed in silence → no reboot is coming.
		return false, notes
	}
	collect(first)
	if _, ok := first.PayloadVariant.(*proto.FromRadio_Rebooted); ok {
		return true, notes
	}
	// Firmware is talking — keep reading up to the full wait so a
	// Rebooted (or a notification) arriving behind a few telemetry /
	// position packets is still seen.
	remainder := wait - graceWindow
	if remainder <= 0 {
		return false, notes
	}
	deadline, cancel := context.WithTimeout(ctx, remainder)
	defer cancel()
	for {
		packet, err := transport.ReceiveFromRadio(deadline)
		if err != nil {
			return false, notes
		}
		collect(packet)
		if _, ok := packet.PayloadVariant.(*proto.FromRadio_Rebooted); ok {
			return true, notes
		}
	}
}

// drainNotifications reads the FromRadio stream for up to `window`,
// collecting device-reported conditions (ClientNotifications and Routing
// NAKs for our admin packets, via packetToNotification) and discarding
// everything else. Used after staging Set* messages but before
// CommitEditSettings, while the link is still up, so firmware rejections
// (unsupported region, invalid params) are captured reliably instead of
// being lost to the commit reboot's USB teardown.
//
// It reads until the window elapses (a transport read timeout / silence
// returns an error, which ends the drain). Non-notification packets in
// the buffer are skipped.
func drainNotifications(
	ctx context.Context,
	transport meshtastic.HardwareTransport,
	window time.Duration,
	sentIDs map[uint32]string,
	onNote func(DeviceNotification),
) []DeviceNotification {
	var notes []DeviceNotification
	deadline, cancel := context.WithTimeout(ctx, window)
	defer cancel()
	for {
		pkt, err := transport.ReceiveFromRadio(deadline)
		if err != nil {
			return notes
		}
		n, ok := packetToNotification(pkt, sentIDs)
		if !ok {
			continue
		}
		notes = append(notes, n)
		if onNote != nil {
			onNote(n)
		}
	}
}

// logLevelName maps the firmware LogRecord_Level enum to a short label
// for operator-facing notification lines.
func logLevelName(l proto.LogRecord_Level) string {
	switch l {
	case proto.LogRecord_CRITICAL:
		return "critical"
	case proto.LogRecord_ERROR:
		return "error"
	case proto.LogRecord_WARNING:
		return "warning"
	case proto.LogRecord_INFO:
		return "info"
	case proto.LogRecord_DEBUG:
		return "debug"
	case proto.LogRecord_TRACE:
		return "trace"
	default:
		return "info"
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
