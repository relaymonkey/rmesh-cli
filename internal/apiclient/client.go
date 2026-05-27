package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/relaymonkey/relaymesh-edge/internal/cliconfig"
	"github.com/relaymonkey/relaymesh-edge/internal/session"
)

// Client calls RelayMesh REST APIs with a stored session.
type Client struct {
	baseURL      string
	sessionToken string
	http         *http.Client
}

// New builds a client from a saved session, preferring stored APIURL when set.
func New(sess session.Saved) *Client {
	base := strings.TrimRight(sess.APIURL, "/")
	if base == "" {
		base = cliconfig.APIURL()
	}
	return &Client{
		baseURL:      base,
		sessionToken: sess.SessionToken,
		http:         &http.Client{Timeout: 30 * time.Second},
	}
}

// GetMe verifies the session against RelayMesh.
func (c *Client) GetMe(ctx context.Context) (Me, error) {
	var me Me
	if err := c.getJSONQuery(ctx, "/api/v1/me", nil, &me); err != nil {
		return Me{}, err
	}
	return me, nil
}

func (c *Client) getJSONQuery(ctx context.Context, path string, query url.Values, dest any) error {
	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	return c.getJSON(ctx, u, dest)
}

func (c *Client) getJSON(ctx context.Context, url string, dest any) error {
	return c.doJSON(ctx, http.MethodGet, url, nil, dest)
}

// PostJSON sends a JSON body and unmarshals the response into `dest`.
// Empty body is allowed (pass nil).
func (c *Client) PostJSON(ctx context.Context, path string, body, dest any) error {
	return c.doJSON(ctx, http.MethodPost, c.baseURL+path, body, dest)
}

// PatchJSON sends a PATCH with a JSON body and unmarshals the
// response into `dest`. Empty body is allowed (pass nil).
func (c *Client) PatchJSON(ctx context.Context, path string, body, dest any) error {
	return c.doJSON(ctx, http.MethodPatch, c.baseURL+path, body, dest)
}

// doJSON is the shared HTTP plumbing for GET/POST/PATCH/etc.
//
// All RelayMesh endpoints today are JSON-in / JSON-out; we lean on
// that uniformity rather than building a richer typed surface. dest
// may be nil when the caller does not need to decode (204 responses
// or fire-and-forget POSTs).
func (c *Client) doJSON(ctx context.Context, method, url string, body, dest any) error {
	var rdr io.Reader
	if body != nil {
		bs, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal body: %w", err)
		}
		rdr = bytes.NewReader(bs)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("X-Session-Token", c.sessionToken)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request %s: %w", url, err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if res.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("session expired or invalid — run: rmesh auth login")
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("%s returned %s: %s", url, res.Status, strings.TrimSpace(string(raw)))
	}
	if dest == nil || len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		return fmt.Errorf("decode %s: %w", url, err)
	}
	return nil
}
