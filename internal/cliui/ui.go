package cliui

import (
	"fmt"
	"io"
	"strings"
)

// Field is one indented detail line beneath a headline.
type Field struct {
	Key   string
	Value string
}

// UI renders human-facing CLI messages — success confirmations, status panels,
// hints, and stream notices. Structured list/get output belongs in clioutput
// (-o table|json|yaml|id), not here.
type UI struct {
	w   io.Writer
	sty styler
}

// New builds a UI for stdout or stderr. Color and symbols are enabled on TTY
// unless NO_COLOR or CLICOLOR=0 is set.
func New(w io.Writer) *UI {
	return &UI{w: w, sty: styler{on: styledWriter(w)}}
}

// Success prints a ✓ headline and optional aligned detail fields.
func (u *UI) Success(headline string, fields ...Field) error {
	return u.headline("success", headline, fields)
}

// Fail prints a ✗ headline and optional detail fields (status screens).
func (u *UI) Fail(headline string, fields ...Field) error {
	return u.headline("fail", headline, fields)
}

// Status prints a neutral ● headline for read-only status (no mutation).
func (u *UI) Status(headline string, fields ...Field) error {
	return u.headline("status", headline, fields)
}

// Hint prints a dim → next-step line.
func (u *UI) Hint(text string) error {
	prefix := "-> "
	if u.sty.on {
		prefix = u.sty.dim("→ ")
	}
	_, err := fmt.Fprintf(u.w, "%s%s\n", prefix, text)
	return err
}

// Warn prints a ! warning line.
func (u *UI) Warn(text string) error {
	prefix := "warn: "
	if u.sty.on {
		prefix = u.sty.yellow("! ")
	}
	_, err := fmt.Fprintf(u.w, "%s%s\n", prefix, text)
	return err
}

// Note prints an indented dim note.
func (u *UI) Note(text string) error {
	line := "  " + text
	if u.sty.on {
		line = u.sty.dim("  " + text)
	}
	_, err := fmt.Fprintln(u.w, line)
	return err
}

// Line prints a plain line (numbered steps, prose).
func (u *UI) Line(text string) error {
	_, err := fmt.Fprintln(u.w, text)
	return err
}

// Blank prints an empty line.
func (u *UI) Blank() error {
	_, err := fmt.Fprintln(u.w)
	return err
}

// Prompt writes a label without a trailing newline (login prompts).
func (u *UI) Prompt(label string) error {
	_, err := fmt.Fprint(u.w, label)
	return err
}

// Steps prints a numbered list.
func (u *UI) Steps(items ...string) error {
	for i, item := range items {
		if err := u.Line(fmt.Sprintf("  %d. %s", i+1, item)); err != nil {
			return err
		}
	}
	return nil
}

// Details prints aligned fields without a headline (endpoint banners, etc.).
func (u *UI) Details(fields ...Field) error {
	return u.writeFields(fields, "  ")
}

// Stream prints a stderr status line for long-running output (live traffic).
func (u *UI) Stream(headline string, fields ...Field) error {
	prefix := "-> "
	if u.sty.on {
		prefix = u.sty.dim("→ ")
	}
	if _, err := fmt.Fprintf(u.w, "%s%s\n", prefix, headline); err != nil {
		return err
	}
	return u.writeFields(fields, "  ")
}

func (u *UI) headline(kind, headline string, fields []Field) error {
	var prefix string
	switch kind {
	case "success":
		prefix = "ok "
		if u.sty.on {
			prefix = u.sty.green("✓ ")
		}
	case "fail":
		prefix = "error "
		if u.sty.on {
			prefix = u.sty.red("✗ ")
		}
	case "status":
		prefix = ""
		if u.sty.on {
			prefix = u.sty.cyan("● ")
		}
	}
	if _, err := fmt.Fprintf(u.w, "%s%s\n", prefix, headline); err != nil {
		return err
	}
	return u.writeFields(fields, "  ")
}

func (u *UI) writeFields(fields []Field, indent string) error {
	fields = omitEmptyFields(fields)
	if len(fields) == 0 {
		return nil
	}
	maxKey := 0
	for _, f := range fields {
		if len(f.Key) > maxKey {
			maxKey = len(f.Key)
		}
	}
	for _, f := range fields {
		key := f.Key
		if u.sty.on {
			key = u.sty.dim(f.Key)
		}
		sep := "  "
		if u.sty.on {
			sep = u.sty.dim(" · ")
		}
		padding := strings.Repeat(" ", maxKey-len(f.Key))
		if _, err := fmt.Fprintf(u.w, "%s%s%s%s%s\n", indent, key, padding, sep, f.Value); err != nil {
			return err
		}
	}
	return nil
}

func omitEmptyFields(fields []Field) []Field {
	out := make([]Field, 0, len(fields))
	for _, f := range fields {
		if strings.TrimSpace(f.Value) == "" {
			continue
		}
		out = append(out, f)
	}
	return out
}
