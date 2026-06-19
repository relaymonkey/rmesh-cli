package cmd

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"github.com/exepirit/meshtastic-go/pkg/meshtastic"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	rmdevice "github.com/relaymonkey/relaymesh-edge/internal/device"
	"github.com/relaymonkey/relaymesh-edge/internal/deviceconfigs"
	rmtransport "github.com/relaymonkey/relaymesh-edge/internal/transport"
)

// maxApplyPasses caps reconnect/resume cycles during one copy-to-device.
// Each pass applies the remaining diff; a typical full-surface import with
// one mid-apply reboot finishes in 2–3 passes.
const maxApplyPasses = 10

// maxForcedReboots caps how many times the apply will force a device reboot
// to push through a band switch (LR11x0 dual-band live-reconfigure failure).
// Two covers the worst case (one to save the staged config, one to apply it
// from a clean boot) while still failing fast on a genuinely rejected lora.
const maxForcedReboots = 2

// deviceReadTimeout bounds a single WantConfigId state read.
const deviceReadTimeout = 30 * time.Second

// rebootPollInterval is the gap between post-reboot readiness probes when we
// poll the device for its actual state rather than sleeping a fixed time.
const rebootPollInterval = time.Second

// applyPayloadToDevice runs a reboot-aware apply session. When the firmware
// reboots mid-apply (common for config.device, config.position, security, …)
// or the USB serial port drops, rmesh waits for the transport to come back and
// resumes with a fresh diff until the device matches the intended payload or
// a non-recoverable error occurs.
func applyPayloadToDevice(
	ctx context.Context,
	cmd *cobra.Command,
	dst deviceconfigs.Source,
	payload deviceconfigs.CanonicalPayload,
	opts applyOptions,
) error {
	url, err := resolveDeviceURL(dst.URL)
	if err != nil {
		return err
	}

	rebootWait := opts.RebootWait
	if rebootWait <= 0 {
		rebootWait = 20 * time.Second
	}

	progressW := cmd.ErrOrStderr()
	tty := term.IsTerminal(int(os.Stdout.Fd()))
	progress := newApplyProgress(progressW, tty, opts.Verbose)
	defer progress.Close()

	var (
		aggregated    rmdevice.ApplyResult
		preApply      *deviceconfigs.CanonicalPayload
		regionChecked bool
		printedDiff   bool
		prevSections  = -1
		stall         int
		localNodeNum  uint32
		forcedReboots int
		regionPrimed  bool
	)

	for pass := 1; pass <= maxApplyPasses; pass++ {
		transport, currentPayload, nodeNum, readLogs, err := openAndReadDevice(ctx, url, pass, progress, rebootWait)
		if err != nil {
			return err
		}
		if nodeNum != 0 {
			localNodeNum = nodeNum
		}
		// Boot-time firmware logs from the reconnect read (e.g. a radio
		// reconfigure failure that reverted the apply) feed the stall /
		// drift diagnosis so "device reported:" carries the real reason.
		aggregated.Notifications = append(aggregated.Notifications, readLogs...)

		if !regionChecked {
			if err := checkRegionChange(cmd, currentPayload, payload, opts.AllowRegionChange); err != nil {
				rmtransport.Close(transport)
				return err
			}
			regionChecked = true
		}
		if preApply == nil {
			cp := currentPayload
			preApply = &cp
		}

		// Diff only over the sections the intended payload actually
		// carries. A section the device has but `intended` omits — e.g.
		// one dropped via `--exclude`, or simply absent from a sparse
		// source — would otherwise show as a "removal" the apply can never
		// reconcile (there is no delete-section op on the device), so the
		// resume loop would spin forever sending empty transactions.
		// Projecting the device onto intended's shape elides those.
		projectedCurrent := currentPayload.ProjectOnto(payload)

		if opts.DryRun {
			diff := deviceconfigs.Diff(projectedCurrent, payload)
			fmt.Fprintln(cmd.OutOrStdout(), "dry-run — pending changes:")
			deviceconfigs.RenderDiff(cmd.OutOrStdout(), diff, deviceconfigs.DiffRenderOptions{
				FromLabel: "device (current)",
				ToLabel:   "intended",
				Color:     tty,
			})
			rmtransport.Close(transport)
			return nil
		}

		// Band-cross priming. A dual-band radio (LR11x0) won't move the LoRa
		// region across the 2.4 GHz ↔ sub-GHz boundary on a transactional
		// apply — the live reconfigure's setFrequency fails, the flash write
		// aborts, nothing persists, and a full apply loops. So when the region
		// is changing, first switch the band with a get-modify-set of the
		// CURRENT lora (region + use_preset only), exactly as meshtastic does;
		// the device reboots itself onto the new band. The full intended
		// config — including any custom use_preset=false lora — then applies on
		// the next pass within the band.
		if !regionPrimed && regionCrossing(currentPayload, payload) {
			region := deviceconfigs.HintsFromPayload(payload).Region
			progress.handle(rmdevice.ApplyEvent{
				Stage:  "send_section",
				Detail: fmt.Sprintf("switching to %s band (preset) before applying full config", region),
			})
			if err := rmdevice.PrimeRegion(ctx, transport, currentPayload.Config["lora"], region, localNodeNum); err != nil {
				rmtransport.Close(transport)
				return fmt.Errorf("prime region: %w", err)
			}
			// Wait for the band switch to actually complete — keyed on the
			// device's real state (it drops the link to reboot, then comes
			// back reporting the new region), not a fixed sleep. Consumes and
			// closes `transport`.
			if err := awaitBandSwitch(ctx, transport, url, region, rebootWait, progress); err != nil {
				return fmt.Errorf("band switch to %s: %w", region, err)
			}
			regionPrimed = true
			aggregated.Rebooted = true
			// The band switch is a phase change: reset progress tracking so
			// the within-band apply that follows gets a fresh convergence
			// budget (the no-progress guard must not count the pre-prime pass,
			// whose section count is unchanged by a region-only prime).
			// Use a high sentinel, not -1: the guard tests `len(diff) >= prev`,
			// and any real count is >= -1, which would false-trip immediately.
			prevSections = math.MaxInt
			stall = 0
			preApply = nil
			continue
		}

		diff := deviceconfigs.Diff(projectedCurrent, payload)
		if len(diff) == 0 {
			rmtransport.Close(transport)
			if pass == 1 {
				fmt.Fprintln(cmd.OutOrStdout(), "✓ device matches intended — nothing to apply.")
				return nil
			}
			return writeApplySummary(cmd, aggregated)
		}

		// Stall guard: if a resume pass re-reads the same (or larger)
		// pending set the firmware is reverting what we send — re-sending
		// it again just re-triggers the disconnect. Two consecutive
		// no-progress passes (one tolerated for reboot settling) abort
		// with a diagnosis instead of burning every pass.
		if pass > 1 {
			if len(diff) >= prevSections {
				stall++
			} else {
				stall = 0
			}
		}
		prevSections = len(diff)
		if stall >= 2 {
			rmtransport.Close(transport)
			return stalledApplyError(diff, aggregated.Notifications)
		}

		if !printedDiff {
			fmt.Fprintln(progressW, "applying changes:")
			deviceconfigs.RenderDiff(progressW, diff, deviceconfigs.DiffRenderOptions{
				FromLabel: "device (current)",
				ToLabel:   "intended",
				Color:     tty,
			})
			fmt.Fprintln(progressW)
			printedDiff = true
		} else {
			fmt.Fprintf(progressW, "\nresuming apply (pass %d/%d) — %d section(s) remaining: %s\n\n",
				pass, maxApplyPasses, len(diff), strings.Join(diffSectionNames(diff), ", "))
		}

		filtered := deviceconfigs.PayloadFromDiff(payload, diff)
		res, applyErr := rmdevice.Apply(ctx, transport, filtered, rmdevice.ApplyOptions{
			RebootWait:    rebootWait,
			PreApplyState: preApply,
			LocalNodeNum:  localNodeNum,
			OnProgress:    progress.handle,
		})
		rmtransport.Close(transport)

		mergeApplyResult(&aggregated, res)

		if applyErr != nil {
			if !rmtransport.IsDisconnect(applyErr) {
				return fmt.Errorf("apply: %w", applyErr)
			}
			// Link dropped — almost always a scheduled reboot after a
			// section like config.device / config.lora. Don't open the
			// port here; the next pass's openAndReadDevice owns the single
			// reconnect (opening it here would leak the handle and make the
			// reopen fail with "serial port busy").
			aggregated.Rebooted = true // the link drop is the reboot
			progress.handle(rmdevice.ApplyEvent{Stage: "reconnect_wait", Detail: applyErr.Error()})
			continue
		}

		if res.RereadInconclusive {
			aggregated.RereadInconclusive = true
			// A post-commit re-read that fails / comes back partial means
			// the device dropped the link to reboot — record that so the
			// summary reports the reboot even when the FromRadio_Rebooted
			// frame was lost to the USB teardown.
			aggregated.Rebooted = true
			progress.handle(rmdevice.ApplyEvent{Stage: "reconnect_wait", Detail: "partial re-read after apply"})
			continue
		}

		if res.DriftCount > 0 {
			// Drift right after a reboot can be the radio still settling;
			// one more read pass disambiguates.
			if res.Rebooted && pass < maxApplyPasses {
				continue
			}
			// Drift with no reboot where the stuck section is config.lora
			// is the dual-band live-reconfigure case (LR11x0): the radio
			// can't switch 2.4 GHz ↔ sub-GHz on a live reconfigure, so the
			// change only takes after a reboot. Force one and retry before
			// giving up. Bounded so a genuinely-rejected lora still reports.
			if driftHasLoRa(res.Drift) && forcedReboots < maxForcedReboots && pass < maxApplyPasses {
				if err := forceRebootAndWait(ctx, url, localNodeNum, rebootWait, progress); err != nil {
					return reportApplyDrift(cmd, aggregated, res, tty)
				}
				forcedReboots++
				aggregated.Rebooted = true
				continue
			}
			return reportApplyDrift(cmd, aggregated, res, tty)
		}

		aggregated.DriftCount = 0
		aggregated.RereadInconclusive = false
		return writeApplySummary(cmd, aggregated)
	}

	return fmt.Errorf("apply did not complete after %d reconnect passes — device may still be rebooting; wait and re-run copy", maxApplyPasses)
}

func openAndReadDevice(
	ctx context.Context,
	url string,
	pass int,
	progress *applyProgress,
	rebootWait time.Duration,
) (meshtastic.HardwareTransport, deviceconfigs.CanonicalPayload, uint32, []rmdevice.DeviceNotification, error) {
	var (
		transport meshtastic.HardwareTransport
		err       error
	)
	// A freshly re-enumerated USB port can accept Open() while still
	// failing the first config-sweep read (port present, firmware not
	// ready). Bound the open→read retries so a flapping port can't spin
	// forever; each WaitReady call already has its own deadline.
	const maxReadAttempts = 5
	for attempt := 1; ; attempt++ {
		if pass == 1 && attempt == 1 {
			transport, err = rmtransport.Open(url)
		} else {
			progress.handle(rmdevice.ApplyEvent{Stage: "reconnect_wait"})
			transport, err = rmtransport.WaitReady(ctx, url, rebootWait)
			if err == nil {
				progress.handle(rmdevice.ApplyEvent{Stage: "reconnect_done"})
			}
		}
		if err != nil {
			if pass == 1 && attempt == 1 {
				return nil, deviceconfigs.CanonicalPayload{}, 0, nil, fmt.Errorf("open transport %s: %w", url, err)
			}
			return nil, deviceconfigs.CanonicalPayload{}, 0, nil, fmt.Errorf("reconnect to %s: %w", url, err)
		}

		readCtx, cancel := context.WithTimeout(ctx, deviceReadTimeout)
		state, readErr := rmdevice.GetState(readCtx, transport)
		cancel()
		if readErr == nil {
			payload, convErr := rmdevice.ToCanonicalPayload(state)
			if convErr != nil {
				rmtransport.Close(transport)
				return nil, deviceconfigs.CanonicalPayload{}, 0, nil, convErr
			}
			// Surface any WARN+ firmware logs from this read — after a
			// commit reboot this is where a boot-time revert / radio
			// reconfigure failure is reported (it's logged, not sent as a
			// ClientNotification).
			for _, n := range state.Logs {
				progress.handle(rmdevice.ApplyEvent{Stage: "notification", Detail: n.Level + ": " + n.Message})
			}
			// MyInfo.my_node_num is the authoritative local node number;
			// admin writes must be addressed to it.
			return transport, payload, state.MyInfo.GetMyNodeNum(), state.Logs, nil
		}

		rmtransport.Close(transport)
		if rmtransport.IsDisconnect(readErr) && attempt < maxReadAttempts {
			// Port vanished again mid-read — wait for the next
			// re-enumeration and retry the read.
			continue
		}
		return nil, deviceconfigs.CanonicalPayload{}, 0, nil, fmt.Errorf("read current device state: %w", readErr)
	}
}

func checkRegionChange(
	cmd *cobra.Command,
	current deviceconfigs.CanonicalPayload,
	intended deviceconfigs.CanonicalPayload,
	allow bool,
) error {
	currentHints := deviceconfigs.HintsFromPayload(current)
	intendedHints := deviceconfigs.HintsFromPayload(intended)
	if intendedHints.Region != "" && intendedHints.Region != currentHints.Region && currentHints.Region != "" {
		if !allow {
			fmt.Fprintf(cmd.ErrOrStderr(),
				"error: region change %s → %s would alter regulatory band; pass --allow-region-change to proceed\n",
				currentHints.Region, intendedHints.Region)
			os.Exit(2)
		}
	}
	return nil
}

func mergeApplyResult(dst *rmdevice.ApplyResult, src rmdevice.ApplyResult) {
	dst.Sections = append(dst.Sections, src.Sections...)
	dst.ChannelsSent += src.ChannelsSent
	dst.Rebooted = dst.Rebooted || src.Rebooted
	if src.DriftCount > 0 {
		dst.DriftCount = src.DriftCount
		dst.Drift = src.Drift
	}
	// Keep only the latest pass's notifications — they describe the
	// state of the section(s) still failing, which is what a stall or
	// drift report wants to show.
	if len(src.Notifications) > 0 {
		dst.Notifications = src.Notifications
	}
}

// regionCrossing reports whether the apply moves the device to a different
// LoRa region than it currently reports (both known).
func regionCrossing(current, intended deviceconfigs.CanonicalPayload) bool {
	c := deviceconfigs.HintsFromPayload(current).Region
	i := deviceconfigs.HintsFromPayload(intended).Region
	return c != "" && i != "" && c != i
}

// driftHasLoRa reports whether the post-apply drift includes config.lora —
// the section whose band changes need a reboot to take on dual-band radios.
func driftHasLoRa(drift []deviceconfigs.SubmessageDiff) bool {
	for _, d := range drift {
		if d.Group == "config" && d.Key == "lora" {
			return true
		}
	}
	return false
}

// awaitReboot waits for a device reboot to complete without a fixed sleep: it
// reads the open transport until the link drops (the reboot tears the port
// down) or the window elapses, then reconnects. Keying off the actual port
// drop avoids reconnecting to the still-up pre-reboot session. Consumes and
// closes `transport`.
func awaitReboot(ctx context.Context, transport meshtastic.HardwareTransport, url string, wait time.Duration) error {
	watch, cancel := context.WithTimeout(ctx, wait)
	for {
		// Drain until a read error — that's the link dropping for the reboot
		// (or the window expiring, in which case we reconnect anyway).
		if _, err := transport.ReceiveFromRadio(watch); err != nil {
			break
		}
	}
	cancel()
	rmtransport.Close(transport)
	t, err := rmtransport.WaitReady(ctx, url, wait)
	if err != nil {
		return err
	}
	rmtransport.Close(t)
	return nil
}

// awaitBandSwitch waits for a region prime to take effect, dynamically: it
// watches for the reboot (via awaitReboot), then polls the device until it
// reports the target region — the signal that it has booted on the new band
// and is ready for the within-band apply. No fixed settle; bounded by `wait`.
func awaitBandSwitch(ctx context.Context, transport meshtastic.HardwareTransport, url, targetRegion string, wait time.Duration, progress *applyProgress) error {
	progress.handle(rmdevice.ApplyEvent{Stage: "reconnect_wait", Detail: "waiting for band switch reboot"})
	if err := awaitReboot(ctx, transport, url, wait); err != nil {
		return fmt.Errorf("reconnect after reboot: %w", err)
	}
	deadline := time.Now().Add(wait)
	for {
		t, err := rmtransport.WaitReady(ctx, url, wait)
		if err != nil {
			return fmt.Errorf("reconnect: %w", err)
		}
		readCtx, c := context.WithTimeout(ctx, deviceReadTimeout)
		state, rerr := rmdevice.GetState(readCtx, t)
		c()
		rmtransport.Close(t)
		if rerr == nil {
			cp, convErr := rmdevice.ToCanonicalPayload(state)
			if convErr == nil && deviceconfigs.HintsFromPayload(cp).Region == targetRegion {
				progress.handle(rmdevice.ApplyEvent{Stage: "reconnect_done"})
				return nil
			}
		}
		if !time.Now().Before(deadline) {
			// Couldn't confirm in time; proceed — the within-band apply and
			// its resume loop will reconcile (or stall with a diagnosis).
			progress.handle(rmdevice.ApplyEvent{Stage: "reconnect_done"})
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(rebootPollInterval):
		}
	}
}

// forceRebootAndWait asks the device to reboot and waits for it to come back,
// dynamically. Used for the within-band drift-without-reboot lora case where a
// change only takes after a reboot.
func forceRebootAndWait(
	ctx context.Context,
	url string,
	localNodeNum uint32,
	rebootWait time.Duration,
	progress *applyProgress,
) error {
	progress.handle(rmdevice.ApplyEvent{Stage: "reconnect_wait", Detail: "change needs a reboot — rebooting device"})
	// WaitReady (not a bare Open): the port was just closed by Apply and a
	// macOS serial reopen can transiently fail with "resource busy".
	transport, err := rmtransport.WaitReady(ctx, url, rebootWait)
	if err != nil {
		return fmt.Errorf("open for reboot: %w", err)
	}
	if err := rmdevice.Reboot(ctx, transport, localNodeNum, 1); err != nil {
		rmtransport.Close(transport)
		return fmt.Errorf("send reboot: %w", err)
	}
	// awaitReboot watches the link drop (the reboot) and reconnects — no fixed
	// sleep, no reconnecting to the pre-reboot session.
	if err := awaitReboot(ctx, transport, url, rebootWait); err != nil {
		return fmt.Errorf("reconnect after reboot: %w", err)
	}
	progress.handle(rmdevice.ApplyEvent{Stage: "reconnect_done"})
	return nil
}

// diffSectionNames renders the dotted section labels of a diff
// ("config.lora", "module_config.mqtt", "channels[1]") for compact
// progress / error lines.
func diffSectionNames(diff []deviceconfigs.SubmessageDiff) []string {
	out := make([]string, 0, len(diff))
	for _, d := range diff {
		switch {
		case d.Group == "channels":
			out = append(out, fmt.Sprintf("channels[%s]", d.Key))
		case d.Key == "":
			out = append(out, d.Group)
		default:
			out = append(out, d.Group+"."+d.Key)
		}
	}
	return out
}

// stalledApplyError builds the no-progress diagnosis. When the firmware
// emitted a ClientNotification explaining the rejection we surface its
// verbatim text (the device's own words); we do not guess a cause.
func stalledApplyError(diff []deviceconfigs.SubmessageDiff, notes []rmdevice.DeviceNotification) error {
	sections := diffSectionNames(diff)
	var b strings.Builder
	fmt.Fprintf(&b,
		"apply stalled — the firmware keeps reverting %d section(s) (%s); no progress across passes",
		len(sections), strings.Join(sections, ", "))

	if len(notes) > 0 {
		b.WriteString("\ndevice reported:")
		for _, n := range notes {
			fmt.Fprintf(&b, "\n  %s: %s", n.Level, n.Message)
		}
	}
	if driftHasLoRa(diff) {
		// Band crossings are primed automatically, so a lingering config.lora
		// stall is the radio refusing these specific modem settings (e.g. a
		// custom bandwidth/channel the hardware can't tune) — not a missing
		// band switch.
		b.WriteString("\nconfig.lora was rejected by the radio — likely a custom bandwidth/spread-factor/channel" +
			" the hardware can't tune. Use `use_preset: true` with a modem_preset, or adjust the custom values.")
	}
	fmt.Fprintf(&b,
		"\nto apply everything else, re-run excluding the rejected section(s): `--exclude %s` (or `--section <list>`).",
		strings.Join(sections, ","))
	return errors.New(b.String())
}

func rebootLabel(rebooted bool) string {
	if rebooted {
		return "1 reboot"
	}
	return "0 reboot"
}

func writeApplySummary(cmd *cobra.Command, res rmdevice.ApplyResult) error {
	fmt.Fprintf(cmd.OutOrStdout(),
		"applied %d sections, %d channels, %d drift, %s\n",
		len(res.Sections), res.ChannelsSent, res.DriftCount, rebootLabel(res.Rebooted))
	return nil
}

func reportApplyDrift(cmd *cobra.Command, aggregated rmdevice.ApplyResult, last rmdevice.ApplyResult, tty bool) error {
	fmt.Fprintf(cmd.OutOrStdout(),
		"applied %d sections, %d channels, %d drift, %s\n",
		len(aggregated.Sections), aggregated.ChannelsSent, last.DriftCount, rebootLabel(aggregated.Rebooted))
	fmt.Fprintln(cmd.ErrOrStderr(), "\npost-apply drift — firmware did not accept:")
	deviceconfigs.RenderDiff(cmd.ErrOrStderr(), last.Drift, deviceconfigs.DiffRenderOptions{
		FromLabel: "intended",
		ToLabel:   "device (after)",
		Color:     tty,
	})
	if len(last.Notifications) > 0 {
		fmt.Fprintln(cmd.ErrOrStderr(), "\ndevice reported:")
		for _, n := range last.Notifications {
			fmt.Fprintf(cmd.ErrOrStderr(), "  %s: %s\n", n.Level, n.Message)
		}
	}
	return errors.New("post-apply drift detected (see above)")
}
