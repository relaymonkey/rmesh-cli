package config

import (
	"os"
	"path/filepath"
	"runtime"
)

const userDataDirName = ".rmesh"

// UserDataDir returns the operator-local rmesh directory (~/.rmesh).
func UserDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, userDataDirName), nil
}

// SessionPath returns ~/.rmesh/session.json (override with RMESH_SESSION_FILE).
func SessionPath() (string, error) {
	if p := os.Getenv("RMESH_SESSION_FILE"); p != "" {
		return p, nil
	}
	dir, err := UserDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "session.json"), nil
}

// DefaultPath returns the platform default agent config file path.
// macOS: ~/.rmesh/config.yaml. Linux and others: /etc/rmesh/config.yaml.
func DefaultPath() string {
	if runtime.GOOS == "darwin" {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, userDataDirName, macosConfigFile)
		}
	}
	return linuxDefaultPath
}
