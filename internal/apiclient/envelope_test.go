package apiclient

import (
	"encoding/json"
	"testing"
)

func TestMessageEnvelopeAt(t *testing.T) {
	env := MessageEnvelope([]byte(`{"id":"m1","decoded":{"value":{"text":"hi"}}}`))

	v, ok := env.At("id")
	if !ok || v != "m1" {
		t.Fatalf("id = %v ok=%v", v, ok)
	}
	v, ok = env.At("decoded.value.text")
	if !ok || v != "hi" {
		t.Fatalf("text = %v ok=%v", v, ok)
	}
	if _, ok := env.At("missing.path"); ok {
		t.Fatal("expected missing path")
	}
}

func TestMessageEnvelopeIntField(t *testing.T) {
	env := MessageEnvelope([]byte(`{"packet_type":67,"nested":{"count":3}}`))
	n, ok := env.IntField("packet_type")
	if !ok || n != 67 {
		t.Fatalf("packet_type = %d ok=%v", n, ok)
	}
	n, ok = env.IntField("nested.count")
	if !ok || n != 3 {
		t.Fatalf("count = %d ok=%v", n, ok)
	}
}

func TestMessageEnvelopeStringField(t *testing.T) {
	env := MessageEnvelope([]byte(`{"source_node_id":"!abc123"}`))
	if got := env.StringField("source_node_id"); got != "!abc123" {
		t.Fatalf("got %q", got)
	}
	if got := env.StringField("missing"); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestMessageEnvelopeMarshalJSON(t *testing.T) {
	raw := []byte(`{"id":"m1"}`)
	env := MessageEnvelope(raw)
	out, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(raw) {
		t.Fatalf("marshal = %s", out)
	}

	var empty MessageEnvelope
	out, err = json.Marshal(empty)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "null" {
		t.Fatalf("empty marshal = %s", out)
	}
}

func TestFormatLeaf(t *testing.T) {
	tests := []struct {
		in   any
		want string
	}{
		{nil, "—"},
		{"", "—"},
		{"  ", "—"},
		{"ok", "ok"},
		{float64(67), "67"},
		{float64(1.5), "1.5"},
		{true, "true"},
		{false, "false"},
		{map[string]any{"a": 1}, `{"a":1}`},
	}
	for _, tc := range tests {
		if got := FormatLeaf(tc.in); got != tc.want {
			t.Fatalf("FormatLeaf(%#v) = %q want %q", tc.in, got, tc.want)
		}
	}
}
