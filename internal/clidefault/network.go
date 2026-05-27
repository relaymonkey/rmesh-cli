package clidefault

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/relaymonkey/relaymesh-edge/internal/cliconfig"
)

// Network is the saved default network for cloud CLI commands.
type Network struct {
	NetworkID string    `json:"network_id"`
	Name      string    `json:"name,omitempty"`
	Slug      string    `json:"slug,omitempty"`
	ShortID   string    `json:"short_id,omitempty"`
	SetAt     time.Time `json:"set_at"`
}

func path() (string, error) {
	return cliconfig.DefaultNetworkPath()
}

// Load returns the saved default network.
func Load() (Network, error) {
	p, err := path()
	if err != nil {
		return Network{}, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return Network{}, ErrNotSet
		}
		return Network{}, fmt.Errorf("read default network: %w", err)
	}
	var n Network
	if err := json.Unmarshal(data, &n); err != nil {
		return Network{}, fmt.Errorf("parse default network: %w", err)
	}
	if n.NetworkID == "" {
		return Network{}, ErrNotSet
	}
	return n, nil
}

// Save persists the default network (mode 0600).
func Save(n Network) error {
	p, err := path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return fmt.Errorf("mkdir default network dir: %w", err)
	}
	n.SetAt = time.Now().UTC()
	data, err := json.MarshalIndent(n, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}

// Clear removes the saved default network.
func Clear() error {
	p, err := path()
	if err != nil {
		return err
	}
	err = os.Remove(p)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove default network: %w", err)
	}
	return nil
}

// ErrNotSet is returned when no default network is configured.
var ErrNotSet = errors.New("no default network — run: rmesh network use <id>")
