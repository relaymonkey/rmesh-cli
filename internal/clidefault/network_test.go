package clidefault

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadClear(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("RMESH_DEFAULT_NETWORK_FILE", filepath.Join(dir, "default-network.json"))

	if err := Clear(); err != nil {
		t.Fatal(err)
	}
	_, err := Load()
	if err != ErrNotSet {
		t.Fatalf("Load() = %v, want ErrNotSet", err)
	}

	want := Network{NetworkID: "uuid-1", Name: "alpha", Slug: "alpha-abc", ShortID: "abcd1234"}
	if err := Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.NetworkID != want.NetworkID || got.Name != want.Name {
		t.Fatalf("Load() = %+v", got)
	}
	if err := Clear(); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultNetworkPath(t *testing.T) {
	t.Setenv("RMESH_DEFAULT_NETWORK_FILE", "")
	p, err := path()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(p) != "default-network.json" {
		t.Fatalf("path = %q", p)
	}
	if _, err := os.Stat(filepath.Dir(p)); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
}
