// Package clienv resolves RelayMesh cloud URLs from environment variables.
package clienv

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"strings"
)

const (
	EnvAPIURL    = "RMESH_API_URL"
	EnvAuthURL   = "RMESH_AUTH_URL"
	EnvStreamURL = "RMESH_STREAM_URL"

	DefaultAPIURL  = "https://mesh.relaymonkey.com"
	DefaultAuthURL = "https://auth.relaymonkey.com"
)

// APIURL is the RelayMesh REST API origin (paths are /api/v1/...).
func APIURL() string {
	return getenv(EnvAPIURL, DefaultAPIURL)
}

// AuthURL is the Ory Kratos public origin used for CLI login.
func AuthURL() string {
	return getenv(EnvAuthURL, DefaultAuthURL)
}

// StreamURL is the origin for cmd/streamd (WebSocket live).
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
	u.Path = path.Join(u.Path, "/api/v1/networks", networkID, "live")
	return u.String(), nil
}

func getenv(name, defaultVal string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return strings.TrimRight(v, "/")
	}
	return defaultVal
}
