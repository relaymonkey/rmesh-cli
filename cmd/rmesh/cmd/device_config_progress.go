package cmd

import (
	"fmt"
	"io"
	"sync"
	"time"

	rmdevice "github.com/relaymonkey/relaymesh-edge/internal/device"
)

// applyProgress translates `rmdevice.ApplyEvent`s into operator-visible
// progress on stderr.
//
// Two visual modes:
//
//   - **TTY**: per-section ✓ lines stream into the terminal as the
//     admin messages go out, and the long waits ("waiting for device
//     to reboot…", "re-reading device state…") are accompanied by an
//     inline ASCII-braille spinner that overwrites itself with `\r`.
//   - **Non-TTY** (CI, tee, redirect): plain log lines, one per event,
//     no animation. Spinners would just litter the output with control
//     codes.
//
// The reporter is single-use: pass `r.handle` to `ApplyOptions.OnProgress`
// for one apply session, then call `r.Close()` so any in-flight spinner
// goroutine winds down and the cursor goes back to the next line.
type applyProgress struct {
	w       io.Writer
	tty     bool
	verbose bool

	mu      sync.Mutex
	spinner *spinner
}

func newApplyProgress(w io.Writer, tty, verbose bool) *applyProgress {
	return &applyProgress{w: w, tty: tty, verbose: verbose}
}

func (r *applyProgress) handle(ev rmdevice.ApplyEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Any new event line stops a running spinner first so the spinner's
	// `\r` re-paint doesn't clobber the next print.
	r.stopSpinnerLocked()

	switch ev.Stage {
	case "send_section":
		if ev.Total > 0 {
			fmt.Fprintf(r.w, "  [%d/%d] %s\n", ev.Index, ev.Total, ev.Detail)
		} else {
			fmt.Fprintf(r.w, "  %s\n", ev.Detail)
		}
	case "section_sent":
		// "sent" = the admin packet went out on the transport with
		// WantAck. The firmware will save + fire its configChanged
		// observer (no edit transaction; lora applies live, others
		// reboot per their own rules). We watch for FromRadio_Rebooted
		// in the next 3 s and surface "section_reboot" if it fires.
		fmt.Fprintln(r.w, "    sent")
	case "section_reboot":
		fmt.Fprintf(r.w, "    ↻ device rebooted after %s\n", ev.Detail)
	case "wire_payload":
		// Verbose mode prints the exact SetConfig / SetModuleConfig
		// JSON about to leave the host. Useful for confirming "yes,
		// the value from the diff is on the wire" when post-apply
		// drift looks suspicious.
		if r.verbose {
			fmt.Fprintf(r.w, "    wire: %s\n", ev.Detail)
		}
	case "send_channel":
		if ev.Total > 0 {
			fmt.Fprintf(r.w, "  [%d/%d] %s\n", ev.Index, ev.Total, ev.Detail)
		} else {
			fmt.Fprintf(r.w, "  %s\n", ev.Detail)
		}
	case "channel_sent":
		fmt.Fprintln(r.w, "    sent")
	case "channel_reboot":
		fmt.Fprintf(r.w, "    ↻ device rebooted after %s\n", ev.Detail)
	case "reread_start":
		r.startSpinnerLocked("verifying applied state")
	case "reread_done":
		fmt.Fprintln(r.w, "✓ device state re-read")
	case "reread_partial":
		fmt.Fprintln(r.w, "… re-read returned a partial state")
	case "reread_failed":
		if ev.Detail != "" {
			fmt.Fprintf(r.w, "… re-read failed: %s\n", ev.Detail)
		} else {
			fmt.Fprintln(r.w, "… re-read failed")
		}
	}
}

func (r *applyProgress) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopSpinnerLocked()
}

func (r *applyProgress) startSpinnerLocked(label string) {
	if !r.tty {
		// Non-TTY: emit a single line and don't animate.
		fmt.Fprintf(r.w, "→ %s…\n", label)
		return
	}
	r.spinner = newSpinner(r.w, label)
	r.spinner.start()
}

func (r *applyProgress) stopSpinnerLocked() {
	if r.spinner == nil {
		return
	}
	r.spinner.stop()
	r.spinner = nil
}

// spinner animates a small braille cycle prefixed to a label, painted
// in place with `\r` and a trailing space-pad to scrub leftover glyphs
// from longer previous frames. Runs on its own goroutine; `stop()`
// blocks until that goroutine has cleared the line so the next caller
// can print without racing the animator.
type spinner struct {
	w     io.Writer
	label string
	done  chan struct{}
	wg    sync.WaitGroup
}

var spinnerFrames = []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}

func newSpinner(w io.Writer, label string) *spinner {
	return &spinner{w: w, label: label, done: make(chan struct{})}
}

func (s *spinner) start() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		i := 0
		for {
			select {
			case <-s.done:
				return
			case <-ticker.C:
				fmt.Fprintf(s.w, "\r%s %s   ", string(spinnerFrames[i%len(spinnerFrames)]), s.label)
				i++
			}
		}
	}()
}

func (s *spinner) stop() {
	close(s.done)
	s.wg.Wait()
	// Wipe the spinner line so the next print starts at column 0
	// without overlapping leftover characters from the longest frame.
	fmt.Fprintf(s.w, "\r%s\r", strRepeat(" ", len(s.label)+8))
}

// strRepeat is a tiny helper to avoid importing strings just for one
// call (and to make this file's surface area self-contained).
func strRepeat(s string, n int) string {
	if n <= 0 {
		return ""
	}
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}
