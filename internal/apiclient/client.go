package apiclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/relaymonkey/relaymesh-edge/internal/clienv"
	"github.com/relaymonkey/relaymesh-edge/internal/session"
)

// Client calls RelayMesh REST APIs with a stored Kratos session.
type Client struct {
	baseURL      string
	sessionToken string
	http         *http.Client
}

// New builds a client from a saved session, preferring stored APIURL when set.
func New(sess session.Saved) *Client {
	base := strings.TrimRight(sess.APIURL, "/")
	if base == "" {
		base = clienv.APIURL()
	}
	return &Client{
		baseURL:      base,
		sessionToken: sess.SessionToken,
		http:         &http.Client{Timeout: 30 * time.Second},
	}
}

// Me is the GET /api/v1/me response shape (subset).
type Me struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// GetMe verifies the session against RelayMesh.
func (c *Client) GetMe(ctx context.Context) (Me, error) {
	var me Me
	if err := c.getJSON(ctx, "/api/v1/me", &me); err != nil {
		return Me{}, err
	}
	return me, nil
}

func (c *Client) getJSON(ctx context.Context, path string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	// Native Kratos login returns a session token (ory_st_*), not a browser cookie.
	req.Header.Set("X-Session-Token", c.sessionToken)
	req.Header.Set("Accept", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request %s: %w", path, err)
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("session expired or invalid — run: rmesh auth login")
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("%s returned %s: %s", path, res.Status, strings.TrimSpace(string(body)))
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}
