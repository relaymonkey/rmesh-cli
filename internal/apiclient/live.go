package apiclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/relaymonkey/relaymesh-edge/internal/cliconfig"
)

// StreamLive connects to the network live WebSocket and invokes fn for each frame.
// Hello frames go to onHello; envelopes to onMessage. Runs until ctx is cancelled
// or the connection closes.
func (c *Client) StreamLive(
	ctx context.Context,
	networkID string,
	onHello func(map[string]any),
	onMessage func(MessageEnvelope),
) error {
	wsURL, err := cliconfig.LiveWSURL(networkID)
	if err != nil {
		return err
	}

	header := http.Header{}
	header.Set("X-Session-Token", c.sessionToken)

	dialer := websocket.Dialer{
		HandshakeTimeout: 30 * time.Second,
		Subprotocols:     []string{"v1.relaymesh"},
	}
	conn, resp, err := dialer.DialContext(ctx, wsURL, header)
	if err != nil {
		return fmt.Errorf("websocket %s: %w", wsURL, handshakeError(wsURL, resp, err))
	}
	defer conn.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var probe struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal(data, &probe); err != nil {
				continue
			}
			if probe.Type == "hello" {
				if onHello != nil {
					var h map[string]any
					_ = json.Unmarshal(data, &h)
					onHello(h)
				}
				continue
			}
			var env MessageEnvelope
			if err := json.Unmarshal(data, &env); err != nil {
				continue
			}
			if onMessage != nil {
				onMessage(env)
			}
		}
	}()

	select {
	case <-ctx.Done():
		_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		return ctx.Err()
	case <-done:
		return nil
	}
}

func handshakeError(wsURL string, resp *http.Response, dialErr error) error {
	if resp == nil {
		return dialErr
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	detail := strings.TrimSpace(string(body))
	msg := fmt.Sprintf("%s (%s)", resp.Status, detail)
	if resp.StatusCode == http.StatusUnauthorized {
		if strings.Contains(detail, "missing session cookie") {
			msg += " — live stream rejected the CLI session; run `rmesh auth login` or check RMESH_STREAM_URL"
		} else {
			msg += " — run `rmesh auth login` or check RMESH_STREAM_URL"
		}
	}
	return fmt.Errorf("%s", msg)
}
