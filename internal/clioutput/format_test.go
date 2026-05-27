package clioutput

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseFormat(t *testing.T) {
	tests := []struct {
		in   string
		want Format
	}{
		{"", FormatTable},
		{"table", FormatTable},
		{"json", FormatJSON},
		{"yaml", FormatYAML},
		{"yml", FormatYAML},
		{"id", FormatID},
		{"ids", FormatID},
	}
	for _, tt := range tests {
		got, err := ParseFormat(tt.in)
		if err != nil {
			t.Fatalf("ParseFormat(%q) err = %v", tt.in, err)
		}
		if got != tt.want {
			t.Fatalf("ParseFormat(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
	if _, err := ParseFormat("xml"); err == nil {
		t.Fatal("expected error for unknown format")
	}
}

func TestRenderTableAndID(t *testing.T) {
	table := Table{
		Headers: []string{"NAME", "ID"},
		Rows:    [][]string{{"alpha", "uuid-1"}},
	}
	var buf bytes.Buffer
	if err := Render(&buf, FormatTable, table, nil, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "alpha") || !strings.Contains(buf.String(), "NAME") {
		t.Fatalf("table output = %q", buf.String())
	}

	buf.Reset()
	if err := Render(&buf, FormatID, Table{}, nil, []string{"uuid-1", "uuid-2"}); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "uuid-1\nuuid-2\n" {
		t.Fatalf("id output = %q", buf.String())
	}
}

func TestRenderJSON(t *testing.T) {
	var buf bytes.Buffer
	raw := map[string]any{"items": []string{"a"}}
	if err := Render(&buf, FormatJSON, Table{}, raw, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"items"`) {
		t.Fatalf("json output = %q", buf.String())
	}
}
