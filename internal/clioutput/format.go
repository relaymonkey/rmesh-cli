package clioutput

import (
	"fmt"
	"strings"
)

// Format selects how list/get commands render to stdout.
type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
	FormatYAML  Format = "yaml"
	FormatID    Format = "id"
)

// ParseFormat normalizes -o / --output values.
func ParseFormat(s string) (Format, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "table":
		return FormatTable, nil
	case "json":
		return FormatJSON, nil
	case "yaml", "yml":
		return FormatYAML, nil
	case "id", "ids":
		return FormatID, nil
	default:
		return "", fmt.Errorf("unknown output format %q (try table, json, yaml, id)", s)
	}
}
