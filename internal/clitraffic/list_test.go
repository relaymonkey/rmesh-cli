package clitraffic

import (
	"context"
	"testing"

	"github.com/relaymonkey/relaymesh-edge/internal/apiclient"
	"github.com/relaymonkey/relaymesh-edge/internal/clioutput"
)

func TestListDefaultFields(t *testing.T) {
	client := &fakeCloudClient{
		list: apiclient.MessageList{
			Items: []apiclient.MessageEnvelope{
				[]byte(`{"id":"m1","ingest_ts":"2026-05-01T12:00:00Z","source_node_id":"!abc","packet_type":67,"channel_index":0,"payload_size":12,"gateway_id":"!gw","encrypted":false}`),
			},
		},
	}

	out, err := List(context.Background(), client, "net-1", ListInput{}, clioutput.FormatTable)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Table.Headers) != 8 {
		t.Fatalf("headers = %v", out.Table.Headers)
	}
	if len(out.IDs) != 1 || out.IDs[0] != "m1" {
		t.Fatalf("ids = %v", out.IDs)
	}
}

func TestListTextModeJSON(t *testing.T) {
	client := &fakeCloudClient{
		list: apiclient.MessageList{
			Items: []apiclient.MessageEnvelope{
				[]byte(`{"id":"m1","packet_type":1,"decoded":{"value":{"text":"hi"}}}`),
			},
		},
	}
	out, err := List(context.Background(), client, "net-1", ListInput{TextOnly: true}, clioutput.FormatJSON)
	if err != nil {
		t.Fatal(err)
	}
	rows, ok := out.Raw.([]map[string]any)
	if !ok || len(rows) != 1 {
		t.Fatalf("raw = %#v", out.Raw)
	}
}
