package clitraffic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/relaymonkey/relaymesh-edge/internal/apiclient"
	"github.com/relaymonkey/relaymesh-edge/internal/climessages"
)

// LiveInput holds live traffic stream options.
type LiveInput struct {
	TextOnly  bool
	FieldsRaw string
	Output    string
}

// Live streams traffic over WebSocket until ctx is cancelled.
func Live(ctx context.Context, client apiclient.CloudClient, networkID string, in LiveInput, out, errOut io.Writer) error {
	fields, err := climessages.ParseFields(in.FieldsRaw, in.TextOnly)
	if err != nil {
		return err
	}

	output := in.Output
	if output == "" {
		output = "table"
	}

	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")

	return client.StreamLive(ctx, networkID,
		func(hello map[string]any) {
			fmt.Fprintf(errOut, "connected: network=%v server_ts=%v\n", hello["network_id"], hello["server_ts"])
		},
		func(env apiclient.MessageEnvelope) {
			if in.TextOnly && !climessages.IsText(env) {
				return
			}
			if output == "json" {
				_ = enc.Encode(climessages.ProjectJSON([]apiclient.MessageEnvelope{env}, fields)[0])
				return
			}
			_, _ = fmt.Fprintln(out, climessages.FormatLiveLine(env, fields))
		},
	)
}
