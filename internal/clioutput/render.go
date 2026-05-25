package clioutput

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"gopkg.in/yaml.v3"
)

// Table is a tab-separated human view (default for list commands).
type Table struct {
	Headers []string
	Rows    [][]string
}

// Render writes structured command output. Pass table for table mode, raw for
// json/yaml, and lines for id mode (one value per line, no header).
func Render(w io.Writer, format Format, table Table, raw any, lines []string) error {
	switch format {
	case FormatTable:
		return writeTable(w, table)
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(raw)
	case FormatYAML:
		enc := yaml.NewEncoder(w)
		enc.SetIndent(2)
		defer enc.Close()
		return enc.Encode(raw)
	case FormatID:
		for _, line := range lines {
			if _, err := fmt.Fprintln(w, line); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}

func writeTable(w io.Writer, t Table) error {
	if len(t.Headers) == 0 {
		return nil
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, strings.Join(t.Headers, "\t")); err != nil {
		return err
	}
	for _, row := range t.Rows {
		if _, err := fmt.Fprintln(tw, strings.Join(row, "\t")); err != nil {
			return err
		}
	}
	return tw.Flush()
}
