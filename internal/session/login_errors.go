package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	ory "github.com/ory/client-go"
)

func loginError(err error, httpResp *http.Response) error {
	if httpResp != nil && httpResp.Body != nil {
		defer httpResp.Body.Close()
	}
	if apiErr, ok := err.(*ory.GenericOpenAPIError); ok && len(apiErr.Body()) > 0 {
		if msg := kratosErrorMessage(apiErr.Body()); msg != "" {
			return errors.New(msg)
		}
	}
	return fmt.Errorf("login failed: %w", err)
}

func kratosErrorMessage(body []byte) string {
	var parsed struct {
		UI struct {
			Messages []ory.UiText `json:"messages"`
			Nodes    []ory.UiNode `json:"nodes"`
		} `json:"ui"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ""
	}
	return joinUIMessages(parsed.UI.Messages, collectNodeMessages(parsed.UI.Nodes))
}

func collectNodeMessages(nodes []ory.UiNode) []ory.UiText {
	var out []ory.UiText
	for _, node := range nodes {
		out = append(out, node.Messages...)
	}
	return out
}

func joinUIMessages(groups ...[]ory.UiText) string {
	seen := make(map[string]struct{})
	var parts []string
	for _, msgs := range groups {
		for _, m := range msgs {
			if m.Type != "error" {
				continue
			}
			text := strings.TrimSpace(m.Text)
			if text == "" {
				continue
			}
			if _, ok := seen[text]; ok {
				continue
			}
			seen[text] = struct{}{}
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, " ")
}
