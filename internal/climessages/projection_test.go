package climessages

import (
	"testing"

	"github.com/relaymonkey/relaymesh-edge/internal/apiclient"
)

func envelopeJSON(t *testing.T, body string) apiclient.MessageEnvelope {
	t.Helper()
	return apiclient.MessageEnvelope([]byte(body))
}

func TestSummarizeDecodedTextMessage(t *testing.T) {
	env := envelopeJSON(t, `{"decoded":{"portnum_name":"TEXT_MESSAGE_APP","value":{"text":"hello mesh"}}}`)
	if got := SummarizeDecoded(env); got != "hello mesh" {
		t.Fatalf("got %q", got)
	}
}

func TestSummarizeDecodedRawPayload(t *testing.T) {
	env := envelopeJSON(t, `{"decoded":{"decoder_kind":"raw_payload","value":{"text":"fallback utf8"}}}`)
	if got := SummarizeDecoded(env); got != "fallback utf8" {
		t.Fatalf("got %q", got)
	}
}

func TestSummarizeDecodedPosition(t *testing.T) {
	env := envelopeJSON(t, `{"decoded":{"portnum_name":"POSITION_APP","value":{"latitude_i":599123456,"longitude_i":108765432,"altitude":120}}}`)
	got := SummarizeDecoded(env)
	if got != "59.9123, 10.8765 · 120 m" {
		t.Fatalf("got %q", got)
	}
}

func TestSummarizeDecodedNodeInfo(t *testing.T) {
	env := envelopeJSON(t, `{"decoded":{"portnum_name":"NODEINFO_APP","value":{"long_name":"Base Camp","hw_model_name":"TBEAM","firmware_version":"2.5.0"}}}`)
	got := SummarizeDecoded(env)
	if got != "Base Camp · TBEAM · 2.5.0" {
		t.Fatalf("got %q", got)
	}
}

func TestSummarizeDecodedTelemetry(t *testing.T) {
	env := envelopeJSON(t, `{"decoded":{"portnum_name":"TELEMETRY_APP","value":{"device_metrics":{"battery_level":87,"voltage":3.74}}}}`)
	got := SummarizeDecoded(env)
	if got != "87% · 3.74V" {
		t.Fatalf("got %q", got)
	}
}

func TestSummarizeDecodedRoutingError(t *testing.T) {
	env := envelopeJSON(t, `{"decoded":{"portnum_name":"ROUTING_APP","value":{"error_reason":"NO_ROUTE"}}}`)
	if got := SummarizeDecoded(env); got != "NO_ROUTE" {
		t.Fatalf("got %q", got)
	}
}

func TestSummarizeDecodedTraceroute(t *testing.T) {
	env := envelopeJSON(t, `{"decoded":{"portnum_name":"TRACEROUTE_APP","value":{"route":[1,2,3],"route_back":[4]}}}`)
	if got := SummarizeDecoded(env); got != "3→ · ←1 hops" {
		t.Fatalf("got %q", got)
	}
}

func TestSummarizeDecodedNeighborInfo(t *testing.T) {
	env := envelopeJSON(t, `{"decoded":{"portnum_name":"NEIGHBORINFO_APP","value":{"neighbors":[{},{}]}}}`)
	if got := SummarizeDecoded(env); got != "2 neighbors" {
		t.Fatalf("got %q", got)
	}
}

func TestSummarizeDecodedEmptyWhenNoDecoded(t *testing.T) {
	env := envelopeJSON(t, `{"packet_type":1,"encrypted":true}`)
	if got := SummarizeDecoded(env); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestSummarizeDecodedEmptyOnDecodeError(t *testing.T) {
	env := envelopeJSON(t, `{"decoded":{"error":"ruleset miss","value":{}}}`)
	if got := SummarizeDecoded(env); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestFieldValueSummaryAndEncrypted(t *testing.T) {
	env := envelopeJSON(t, `{"encrypted":false,"decoded":{"portnum_name":"TEXT_MESSAGE_APP","value":{"text":"ping"}}}`)

	if got := FieldValue(env, "summary"); got != "ping" {
		t.Fatalf("summary = %q", got)
	}
	if got := FieldValue(env, "encrypted"); got != "cleartext" {
		t.Fatalf("encrypted = %q", got)
	}
	if got := FieldValue(env, "missing"); got != "—" {
		t.Fatalf("missing = %q", got)
	}
}

func TestProjectTableUsesFieldValues(t *testing.T) {
	env := envelopeJSON(t, `{"ingest_ts":"2026-05-01T12:00:00Z","packet_type":1,"encrypted":true,"decoded":{"portnum_name":"TEXT_MESSAGE_APP","value":{"text":"hi"}}}`)
	fields := []string{"ingest_ts", "packet_type", "summary", "encrypted"}
	headers, rows := ProjectTable([]apiclient.MessageEnvelope{env}, fields)
	if len(headers) != 4 || len(rows) != 1 {
		t.Fatalf("headers=%v rows=%v", headers, rows)
	}
	if rows[0][0] != "2026-05-01T12:00:00Z" {
		t.Fatalf("ingest_ts = %q", rows[0][0])
	}
	if rows[0][1] != "1" {
		t.Fatalf("packet_type = %q", rows[0][1])
	}
	if rows[0][2] != "hi" {
		t.Fatalf("summary = %q", rows[0][2])
	}
	if rows[0][3] != "encrypted" {
		t.Fatalf("encrypted = %q", rows[0][3])
	}
}

func TestProjectJSONSummaryNilWhenEmpty(t *testing.T) {
	env := envelopeJSON(t, `{"id":"m1","packet_type":67}`)
	rows := ProjectJSON([]apiclient.MessageEnvelope{env}, []string{"id", "summary"})
	if rows[0]["id"] != "m1" {
		t.Fatalf("id = %v", rows[0]["id"])
	}
	if rows[0]["summary"] != nil {
		t.Fatalf("summary = %v", rows[0]["summary"])
	}
}

func TestExtractTextAndIsText(t *testing.T) {
	text := envelopeJSON(t, `{"packet_type":1,"decoded":{"value":{"text":"hello"}}}`)
	if got := ExtractText(text); got != "hello" {
		t.Fatalf("text = %q", got)
	}
	if !IsText(text) {
		t.Fatal("expected text packet")
	}

	other := envelopeJSON(t, `{"packet_type":67}`)
	if IsText(other) {
		t.Fatal("expected non-text packet")
	}
}

func TestFormatLiveLineJoinsFields(t *testing.T) {
	env := envelopeJSON(t, `{"source_node_id":"!abc","decoded":{"portnum_name":"TEXT_MESSAGE_APP","value":{"text":"yo"}}}`)
	got := FormatLiveLine(env, []string{"source_node_id", "summary"})
	if got != "!abc yo" {
		t.Fatalf("got %q", got)
	}
}
