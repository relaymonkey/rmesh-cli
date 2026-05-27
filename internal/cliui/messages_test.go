package cliui

import (
	"bytes"
	"strings"
	"testing"
)

func TestSavedCloudConfigPlain(t *testing.T) {
	u, buf := plainUI()
	if err := u.SavedCloudConfig("eu-868", "cloud:home/mine/eu-868", "uuid-1"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "ok Saved cloud config · eu-868") {
		t.Fatalf("out = %q", out)
	}
	if !strings.Contains(out, "path") || !strings.Contains(out, "cloud:home/mine/eu-868") {
		t.Fatalf("out = %q", out)
	}
}

func TestPromotedCloudConfigPlain(t *testing.T) {
	var buf bytes.Buffer
	u := &UI{w: &buf, sty: styler{on: false}}
	if err := u.PromotedCloudConfig("demo", "cloud:n/mine/x", "cloud:n/template/demo", "id-1", "members"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "Promoted to network template · demo") {
		t.Fatalf("out = %q", out)
	}
	if !strings.Contains(out, "visibility") || !strings.Contains(out, "members") {
		t.Fatalf("out = %q", out)
	}
}

func TestDeletedCloudConfigPlain(t *testing.T) {
	u, buf := plainUI()
	if err := u.DeletedCloudConfig("eu-868", "cloud:home/mine/eu-868", "uuid-1"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "ok Deleted cloud config · eu-868") {
		t.Fatalf("out = %q", out)
	}
}

func TestDryRunPlain(t *testing.T) {
	u, buf := plainUI()
	if err := u.DryRun("PATCH not sent"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "Dry run · PATCH not sent") {
		t.Fatalf("out = %q", buf.String())
	}
}
