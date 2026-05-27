package clitraffic

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/relaymonkey/relaymesh-edge/internal/apiclient"
)

func TestLiveTableOutput(t *testing.T) {
	textEnv := apiclient.MessageEnvelope([]byte(`{"packet_type":1,"source_node_id":"!abc","decoded":{"portnum_name":"TEXT_MESSAGE_APP","value":{"text":"hello"}}}`))
	telemetryEnv := apiclient.MessageEnvelope([]byte(`{"packet_type":67,"source_node_id":"!def"}`))

	client := &fakeCloudClient{
		stream: func(_ context.Context, networkID string, hello func(map[string]any), onMsg func(apiclient.MessageEnvelope)) error {
			if networkID != "net-1" {
				t.Fatalf("networkID = %q", networkID)
			}
			hello(map[string]any{"network_id": "net-1", "server_ts": "2026-05-01T12:00:00Z"})
			onMsg(textEnv)
			onMsg(telemetryEnv)
			return nil
		},
	}

	var out, errOut bytes.Buffer
	err := Live(context.Background(), client, "net-1", LiveInput{}, &out, &errOut)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errOut.String(), "Live stream connected") {
		t.Fatalf("stderr = %q", errOut.String())
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %q", out.String())
	}
	if !strings.Contains(lines[0], "hello") {
		t.Fatalf("text line = %q", lines[0])
	}
	if !strings.Contains(lines[1], "67") {
		t.Fatalf("telemetry line = %q", lines[1])
	}
}

func TestLiveTextOnlyFilter(t *testing.T) {
	textEnv := apiclient.MessageEnvelope([]byte(`{"packet_type":1,"decoded":{"portnum_name":"TEXT_MESSAGE_APP","value":{"text":"hi"}}}`))
	otherEnv := apiclient.MessageEnvelope([]byte(`{"packet_type":67}`))

	client := &fakeCloudClient{
		stream: func(_ context.Context, _ string, _ func(map[string]any), onMsg func(apiclient.MessageEnvelope)) error {
			onMsg(textEnv)
			onMsg(otherEnv)
			return nil
		},
	}

	var out bytes.Buffer
	err := Live(context.Background(), client, "net-1", LiveInput{TextOnly: true}, &out, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(out.String(), "\n") != 1 {
		t.Fatalf("out = %q", out.String())
	}
	if !strings.Contains(out.String(), "hi") {
		t.Fatalf("out = %q", out.String())
	}
}

func TestLiveJSONOutput(t *testing.T) {
	env := apiclient.MessageEnvelope([]byte(`{"packet_type":1,"source_node_id":"!abc","decoded":{"portnum_name":"TEXT_MESSAGE_APP","value":{"text":"ping"}}}`))
	client := &fakeCloudClient{
		stream: func(_ context.Context, _ string, _ func(map[string]any), onMsg func(apiclient.MessageEnvelope)) error {
			onMsg(env)
			return nil
		},
	}

	var out bytes.Buffer
	err := Live(context.Background(), client, "net-1", LiveInput{
		FieldsRaw: "source_node_id,decoded.value.text",
		Output:    "json",
	}, &out, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	body := out.String()
	if !strings.Contains(body, `"source_node_id": "!abc"`) {
		t.Fatalf("out = %q", body)
	}
	if !strings.Contains(body, `"decoded.value.text": "ping"`) {
		t.Fatalf("out = %q", body)
	}
}
