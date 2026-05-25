// Package cliconfig resolves operator-local paths and cloud URLs from environment
// variables. All RMESH_* overrides are read here — not scattered across cmd/ or
// internal packages.
package cliconfig

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	EnvAPIURL             = "RMESH_API_URL"
	EnvAuthURL            = "RMESH_AUTH_URL"
	EnvStreamURL          = "RMESH_STREAM_URL"
	EnvAgentConfig        = "RMESH_CONFIG"
	EnvSessionFile        = "RMESH_SESSION_FILE"
	EnvDefaultNetworkFile = "RMESH_DEFAULT_NETWORK_FILE"

	DefaultAPIURL  = "https://mesh.relaymonkey.com"
	DefaultAuthURL = "https://auth.relaymonkey.com"

	userDataDirName      = ".rmesh"
	linuxDefaultAgentCfg = "/etc/rmesh/config.yaml"
	macosAgentConfigFile = "config.yaml"
)

// UserDataDir returns the operator-local rmesh directory (~/.rmesh).
func UserDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, userDataDirName), nil
}

// DefaultAgentConfigPath returns the platform default agent config.yaml path
// (without RMESH_CONFIG override).
func DefaultAgentConfigPath() string {
	if runtime.GOOS == "darwin" {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, userDataDirName, macosAgentConfigFile)
		}
	}
	return linuxDefaultAgentCfg
}

// AgentConfigPath returns the agent config.yaml path (RMESH_CONFIG or platform default).
func AgentConfigPath() string {
	if p := strings.TrimSpace(os.Getenv(EnvAgentConfig)); p != "" {
		return p
	}
	return DefaultAgentConfigPath()
}

// APIURL is the RelayMesh REST API origin (paths are /api/v1/...).
func APIURL() string {
	return getenv(EnvAPIURL, DefaultAPIURL)
}

// AuthURL is the Ory Kratos public origin used for CLI login.
func AuthURL() string {
	return getenv(EnvAuthURL, DefaultAuthURL)
}

// StreamURL is the origin for live traffic (WebSocket).
// Defaults to APIURL, except :8090 (local backend) → :8091 (streamd).
func StreamURL() string {
	if v := strings.TrimSpace(os.Getenv(EnvStreamURL)); v != "" {
		return strings.TrimRight(v, "/")
	}
	api := APIURL()
	if strings.Contains(api, ":8090") {
		return strings.Replace(api, ":8090", ":8091", 1)
	}
	return api
}

// LiveWSURL builds ws(s)://…/api/v1/networks/{id}/live for streamd.
func LiveWSURL(networkID string) (string, error) {
	base := StreamURL()
	u, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("parse stream url: %w", err)
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	case "ws", "wss":
	default:
		return "", fmt.Errorf("unsupported stream url scheme %q", u.Scheme)
	}
	u.Path = pathJoin(u.Path, "/api/v1/networks", networkID, "live")
	return u.String(), nil
}

// SessionPath returns ~/.rmesh/session.json (override with RMESH_SESSION_FILE).
func SessionPath() (string, error) {
	if p := strings.TrimSpace(os.Getenv(EnvSessionFile)); p != "" {
		return p, nil
	}
	dir, err := UserDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "session.json"), nil
}

// DefaultNetworkPath returns ~/.rmesh/default-network.json (override with RMESH_DEFAULT_NETWORK_FILE).
func DefaultNetworkPath() (string, error) {
	if p := strings.TrimSpace(os.Getenv(EnvDefaultNetworkFile)); p != "" {
		return p, nil
	}
	dir, err := UserDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "default-network.json"), nil
}

// Editor returns $EDITOR, then $VISUAL, then "nano".
func Editor() string {
	if e := strings.TrimSpace(os.Getenv("EDITOR")); e != "" {
		return e
	}
	if e := strings.TrimSpace(os.Getenv("VISUAL")); e != "" {
		return e
	}
	return "nano"
}

func getenv(name, defaultVal string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return strings.TrimRight(v, "/")
	}
	return defaultVal
}

func pathJoin(basePath string, elems ...string) string {
	p := strings.TrimSuffix(basePath, "/")
	for _, e := range elems {
		e = strings.Trim(e, "/")
		if e == "" {
			continue
		}
		p += "/" + e
	}
	if p == "" {
		return "/"
	}
	return p
}
