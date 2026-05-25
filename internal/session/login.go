package session

import (
	"context"
	"fmt"
	"strings"
	"time"

	ory "github.com/ory/client-go"

	"github.com/relaymonkey/relaymesh-edge/internal/clienv"
)

// Login performs a Kratos native (API) login and stores the session.
func Login(ctx context.Context, email, password string) (Saved, error) {
	email = strings.TrimSpace(email)
	if email == "" || password == "" {
		return Saved{}, fmt.Errorf("email and password are required")
	}

	authURL := clienv.AuthURL()
	apiURL := clienv.APIURL()

	client := ory.NewAPIClient(&ory.Configuration{
		Servers: ory.ServerConfigurations{{URL: authURL}},
	})

	flow, _, err := client.FrontendAPI.CreateNativeLoginFlow(ctx).Execute()
	if err != nil {
		return Saved{}, fmt.Errorf("create login flow: %w", err)
	}

	csrf := uiNodeValue(flow.Ui.Nodes, "csrf_token")
	var csrfPtr *string
	if csrf != "" {
		csrfPtr = &csrf
	}
	body := ory.UpdateLoginFlowWithPasswordMethodAsUpdateLoginFlowBody(&ory.UpdateLoginFlowWithPasswordMethod{
		Method:     "password",
		Identifier: email,
		Password:   password,
		CsrfToken:  csrfPtr,
	})

	resp, httpResp, err := client.FrontendAPI.UpdateLoginFlow(ctx).
		Flow(flow.Id).
		UpdateLoginFlowBody(body).
		Execute()
	if err != nil {
		return Saved{}, loginError(err, httpResp)
	}
	if resp.SessionToken == nil || *resp.SessionToken == "" {
		return Saved{}, fmt.Errorf("login succeeded but no session token returned")
	}

	saved := Saved{
		SessionToken: *resp.SessionToken,
		APIURL:       apiURL,
		AuthURL:      authURL,
		Email:        email,
		SavedAt:      time.Now().UTC(),
	}
	if err := Save(saved); err != nil {
		return Saved{}, err
	}
	return saved, nil
}

func uiNodeValue(nodes []ory.UiNode, name string) string {
	for _, node := range nodes {
		input := node.Attributes.UiNodeInputAttributes
		if input == nil || input.Name != name {
			continue
		}
		if input.Value == nil {
			return ""
		}
		switch v := input.Value.(type) {
		case string:
			return v
		default:
			return fmt.Sprint(v)
		}
	}
	return ""
}
