package cmd

import "testing"

func TestParseLabelOverrides(t *testing.T) {
	labels, err := parseLabelOverrides([]string{"site=east", "role=observer"})
	if err != nil {
		t.Fatal(err)
	}
	if labels["site"] != "east" || labels["role"] != "observer" {
		t.Fatalf("labels = %#v", labels)
	}

	if _, err := parseLabelOverrides([]string{"bad"}); err == nil {
		t.Fatal("expected error for malformed label")
	}
}
