package cliconfig

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDefaultAgentConfigPath(t *testing.T) {
	got := DefaultAgentConfigPath()
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatal(err)
		}
		want := filepath.Join(home, userDataDirName, macosAgentConfigFile)
		if got != want {
			t.Fatalf("DefaultAgentConfigPath() = %q, want %q", got, want)
		}
	default:
		if got != linuxDefaultAgentCfg {
			t.Fatalf("DefaultAgentConfigPath() = %q, want %q", got, linuxDefaultAgentCfg)
		}
	}
}
