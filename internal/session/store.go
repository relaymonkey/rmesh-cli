package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/relaymonkey/relaymesh-edge/internal/cliconfig"
)

const sessionCookieName = "ory_kratos_session"

// Saved holds a RelayMesh CLI session for API calls.
type Saved struct {
	SessionToken string    `json:"session_token"`
	APIURL       string    `json:"api_url"`
	AuthURL      string    `json:"auth_url"`
	Email        string    `json:"email,omitempty"`
	SavedAt      time.Time `json:"saved_at"`
}

// SessionTokenHeader returns the session token sent on API requests.
func (s Saved) SessionTokenHeader() string {
	return s.SessionToken
}

// CookieHeader returns a browser-style Cookie header for legacy flows.
func (s Saved) CookieHeader() string {
	return sessionCookieName + "=" + s.SessionToken
}

// FilePath returns the resolved session file path (for status output).
func FilePath() (string, error) {
	return cliconfig.SessionPath()
}

func legacyStorePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "rmesh", "session.json"), nil
}

func storePath() (string, error) {
	return cliconfig.SessionPath()
}

// Load reads the saved session. Returns ErrNotLoggedIn when absent.
func Load() (Saved, error) {
	path, err := storePath()
	if err != nil {
		return Saved{}, err
	}
	s, err := loadFile(path)
	if err == nil {
		return s, nil
	}
	if !errors.Is(err, ErrNotLoggedIn) {
		return Saved{}, err
	}
	legacy, lerr := legacyStorePath()
	if lerr != nil {
		return Saved{}, err
	}
	s, lerr = loadFile(legacy)
	if lerr != nil {
		return Saved{}, err
	}
	if saveErr := Save(s); saveErr == nil {
		_ = os.Remove(legacy)
	}
	return s, nil
}

func loadFile(path string) (Saved, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Saved{}, ErrNotLoggedIn
		}
		return Saved{}, fmt.Errorf("read session: %w", err)
	}
	var s Saved
	if err := json.Unmarshal(data, &s); err != nil {
		return Saved{}, fmt.Errorf("parse session: %w", err)
	}
	if s.SessionToken == "" {
		return Saved{}, ErrNotLoggedIn
	}
	return s, nil
}

// Save persists the session with mode 0600.
func Save(s Saved) error {
	path, err := storePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("mkdir session dir: %w", err)
	}
	s.SavedAt = time.Now().UTC()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// Clear removes the saved session file.
func Clear() error {
	path, err := storePath()
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove session: %w", err)
	}
	legacy, lerr := legacyStorePath()
	if lerr == nil {
		_ = os.Remove(legacy)
	}
	return nil
}

// ErrNotLoggedIn is returned when no CLI session is stored.
var ErrNotLoggedIn = errors.New("not logged in")
