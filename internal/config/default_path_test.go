package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDefaultPath(t *testing.T) {
	got := DefaultPath()
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatal(err)
		}
		want := filepath.Join(home, userDataDirName, macosConfigFile)
		if got != want {
			t.Fatalf("DefaultPath() = %q, want %q", got, want)
		}
	default:
		if got != linuxDefaultPath {
			t.Fatalf("DefaultPath() = %q, want %q", got, linuxDefaultPath)
		}
	}
}
