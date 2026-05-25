package cmd

import (
	"errors"
	"fmt"

	"github.com/relaymonkey/relaymesh-edge/internal/apiclient"
	"github.com/relaymonkey/relaymesh-edge/internal/session"
)

func apiclientFromSession(saved session.Saved) *apiclient.Client {
	return apiclient.New(saved)
}

func requireSession() (session.Saved, *apiclient.Client, error) {
	saved, err := session.Load()
	if err != nil {
		if errors.Is(err, session.ErrNotLoggedIn) {
			return session.Saved{}, nil, fmt.Errorf("not logged in — run: rmesh auth login")
		}
		return session.Saved{}, nil, err
	}
	return saved, apiclient.New(saved), nil
}
