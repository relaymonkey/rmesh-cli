// Package clienv resolves RelayMesh cloud URLs from the environment.
package clienv

import (
	"os"
	"strings"
)

const (
	EnvAPIURL  = "RMESH_API_URL"
	EnvAuthURL = "RMESH_AUTH_URL"

	DefaultAPIURL  = "https://mesh.relaymonkey.com"
	DefaultAuthURL = "https://auth.relaymonkey.com"
)

// APIURL is the RelayMesh REST API origin (paths are /api/v1/...).
func APIURL() string {
	return strings.TrimRight(firstNonEmpty(os.Getenv(EnvAPIURL), DefaultAPIURL), "/")
}

// AuthURL is the Ory Kratos public origin used for CLI login.
func AuthURL() string {
	return strings.TrimRight(firstNonEmpty(os.Getenv(EnvAuthURL), DefaultAuthURL), "/")
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
