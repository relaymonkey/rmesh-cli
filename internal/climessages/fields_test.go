package climessages

import (
	"strings"
	"testing"
)

func TestParseFieldsPassesThroughRawIDs(t *testing.T) {
	fields, err := ParseFields("ingest_ts,source_node_id,decoded.value.text", true)
	if err != nil {
		t.Fatal(err)
	}
	want := "ingest_ts,source_node_id,decoded.value.text"
	if strings.Join(fields, ",") != want {
		t.Fatalf("got %v want %v", fields, want)
	}
}

func TestParseFieldsDefaultListMatchesUI(t *testing.T) {
	fields, err := ParseFields("", false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(fields, ",") != strings.Join(DefaultListFields, ",") {
		t.Fatalf("default list %v want %v", fields, DefaultListFields)
	}
}

func TestParseFieldsRejectsEmptyList(t *testing.T) {
	_, err := ParseFields(",", false)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFieldHeaderIsColumnID(t *testing.T) {
	if FieldHeader("source_node_id") != "source_node_id" {
		t.Fatal("header should match Traffic column id verbatim")
	}
}
