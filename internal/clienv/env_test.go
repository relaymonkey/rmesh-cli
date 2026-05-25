package clienv

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
