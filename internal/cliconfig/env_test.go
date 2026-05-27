package cliconfig

import (
	"os"
	"testing"
)

func TestAPIURLDefault(t *testing.T) {
	os.Unsetenv(EnvAPIURL)
	if got := APIURL(); got != DefaultAPIURL {
		t.Fatalf("APIURL() = %q, want %q", got, DefaultAPIURL)
	}
}

func TestAPIURLOverride(t *testing.T) {
	t.Setenv(EnvAPIURL, "http://localhost:8090/")
	if got := APIURL(); got != "http://localhost:8090" {
		t.Fatalf("APIURL() = %q", got)
	}
}

func TestAuthURLOverride(t *testing.T) {
	t.Setenv(EnvAuthURL, "http://localhost:4433")
	if got := AuthURL(); got != "http://localhost:4433" {
		t.Fatalf("AuthURL() = %q", got)
	}
}

func TestStreamURLLocalBackend(t *testing.T) {
	t.Setenv(EnvStreamURL, "")
	t.Setenv(EnvAPIURL, "http://localhost:8090")
	if got := StreamURL(); got != "http://localhost:8091" {
		t.Fatalf("StreamURL() = %q", got)
	}
	ws, err := LiveWSURL("net-uuid")
	if err != nil {
		t.Fatal(err)
	}
	if ws != "ws://localhost:8091/api/v1/networks/net-uuid/live" {
		t.Fatalf("LiveWSURL() = %q", ws)
	}
}

func TestAgentConfigPathOverride(t *testing.T) {
	t.Setenv(EnvAgentConfig, "/tmp/rmesh.yaml")
	if got := AgentConfigPath(); got != "/tmp/rmesh.yaml" {
		t.Fatalf("AgentConfigPath() = %q", got)
	}
}

func TestSessionPathOverride(t *testing.T) {
	t.Setenv(EnvSessionFile, "/tmp/session.json")
	got, err := SessionPath()
	if err != nil {
		t.Fatal(err)
	}
	if got != "/tmp/session.json" {
		t.Fatalf("SessionPath() = %q", got)
	}
}

func TestEditorFallback(t *testing.T) {
	t.Setenv("EDITOR", "")
	t.Setenv("VISUAL", "")
	if got := Editor(); got != "nano" {
		t.Fatalf("Editor() = %q", got)
	}
	t.Setenv("EDITOR", "vim")
	if got := Editor(); got != "vim" {
		t.Fatalf("Editor() = %q", got)
	}
}
