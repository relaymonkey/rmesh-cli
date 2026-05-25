package cliui

import (
	"io"
	"os"

	"golang.org/x/term"
)

const (
	ansiReset  = "\033[0m"
	ansiDim    = "\033[2m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiCyan   = "\033[36m"
	ansiRed    = "\033[31m"
)

func styledWriter(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("CLICOLOR") == "0" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

type styler struct {
	on bool
}

func (s styler) green(v string) string {
	if !s.on {
		return v
	}
	return ansiGreen + v + ansiReset
}

func (s styler) red(v string) string {
	if !s.on {
		return v
	}
	return ansiRed + v + ansiReset
}

func (s styler) yellow(v string) string {
	if !s.on {
		return v
	}
	return ansiYellow + v + ansiReset
}

func (s styler) cyan(v string) string {
	if !s.on {
		return v
	}
	return ansiCyan + v + ansiReset
}

func (s styler) dim(v string) string {
	if !s.on {
		return v
	}
	return ansiDim + v + ansiReset
}

func (s styler) bold(v string) string {
	if !s.on {
		return v
	}
	return "\033[1m" + v + ansiReset
}
