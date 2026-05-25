package cliui

import (
	"bytes"
	"strings"
	"testing"
)

func plainUI() (*UI, *bytes.Buffer) {
	var buf bytes.Buffer
	u := &UI{w: &buf, sty: styler{on: false}}
	return u, &buf
}

func TestSuccessPlain(t *testing.T) {
	u, buf := plainUI()
	if err := u.Success("Default network · EU", Field{Key: "id", Value: "uuid-1"}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "ok Default network · EU") {
		t.Fatalf("out = %q", out)
	}
	if !strings.Contains(out, "id  uuid-1") {
		t.Fatalf("out = %q", out)
	}
}

func TestFailWithHint(t *testing.T) {
	u, buf := plainUI()
	if err := u.Fail("Not logged in", Field{Key: "session", Value: "/tmp/session (missing)"}); err != nil {
		t.Fatal(err)
	}
	if err := u.Hint("rmesh auth login"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "error Not logged in") {
		t.Fatalf("out = %q", out)
	}
	if !strings.Contains(out, "-> rmesh auth login") {
		t.Fatalf("out = %q", out)
	}
}

func TestStreamOmitsEmptyFields(t *testing.T) {
	u, buf := plainUI()
	if err := u.Stream("Live stream connected", Field{Key: "network", Value: "net-1"}, Field{Key: "server", Value: ""}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Contains(out, "server") {
		t.Fatalf("out = %q", out)
	}
}

func TestSteps(t *testing.T) {
	u, buf := plainUI()
	if err := u.Steps("one", "two"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "1. one") || !strings.Contains(out, "2. two") {
		t.Fatalf("out = %q", out)
	}
}
