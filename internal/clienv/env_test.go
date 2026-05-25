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
