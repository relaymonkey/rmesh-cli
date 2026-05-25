package climessages

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/relaymonkey/relaymesh-edge/internal/apiclient"
)

// SummarizeDecoded mirrors the Traffic table Summary column (traffic-decoded-summary.ts).
func SummarizeDecoded(raw apiclient.MessageEnvelope) string {
	if len(raw) == 0 {
		return ""
	}
	var root struct {
		Decoded json.RawMessage `json:"decoded"`
	}
	if err := json.Unmarshal(raw, &root); err != nil || len(root.Decoded) == 0 {
		return ""
	}
	return summarizeDecodedJSON(root.Decoded)
}

func summarizeDecodedJSON(raw json.RawMessage) string {
	var d decodedView
	if err := json.Unmarshal(raw, &d); err != nil {
		return ""
	}
	if d.Error != "" && len(d.Value) == 0 {
		return ""
	}
	if d.DecoderKind == "raw_payload" || d.DecoderKind == "ruleset_utf8" {
		return stringField(d.Value, "text")
	}
	switch d.PortnumName {
	case "TEXT_MESSAGE_APP", "ALERT_APP", "RANGE_TEST_APP":
		return stringField(d.Value, "text")
	case "POSITION_APP":
		return formatLatLon(d.Value)
	case "NODEINFO_APP", "MAP_REPORT_APP":
		parts := []string{
			firstNonEmpty(stringField(d.Value, "long_name"), stringField(d.Value, "short_name")),
			stringField(d.Value, "hw_model_name"),
			stringField(d.Value, "firmware_version"),
		}
		return joinNonEmpty(parts, " · ")
	case "TELEMETRY_APP":
		return summarizeTelemetry(d.Value)
	case "ROUTING_APP":
		return summarizeRouting(d.Value)
	case "TRACEROUTE_APP":
		return summarizeTraceroute(d.Value)
	case "NEIGHBORINFO_APP":
		return summarizeNeighborInfo(d.Value)
	default:
		return ""
	}
}

type decodedView struct {
	DecoderKind string         `json:"decoder_kind"`
	PortnumName string         `json:"portnum_name"`
	Value       map[string]any `json:"value"`
	Error       string         `json:"error"`
}

func summarizeTelemetry(v map[string]any) string {
	if dm := pickObject(v, "device_metrics"); dm != nil {
		return walkMetrics(dm, []metricField{
			{"battery_level", func(n float64) string { return fmt.Sprintf("%.0f%%", n) }},
			{"voltage", func(n float64) string { return fmt.Sprintf("%.2fV", n) }},
			{"channel_utilization", func(n float64) string { return fmt.Sprintf("ch:%.1f%%", n) }},
			{"air_util_tx", func(n float64) string { return fmt.Sprintf("tx:%.1f%%", n) }},
		})
	}
	if em := pickObject(v, "environment_metrics"); em != nil {
		return walkMetrics(em, []metricField{
			{"temperature", func(n float64) string { return fmt.Sprintf("%.1f°C", n) }},
			{"relative_humidity", func(n float64) string { return fmt.Sprintf("%.0f%%RH", n) }},
			{"barometric_pressure", func(n float64) string { return fmt.Sprintf("%.0f hPa", n) }},
		})
	}
	return ""
}

type metricField struct {
	key    string
	format func(float64) string
}

func walkMetrics(obj map[string]any, fields []metricField) string {
	const cap = 5
	parts := make([]string, 0, cap)
	for _, f := range fields {
		if len(parts) >= cap {
			break
		}
		if n, ok := asFloat(obj[f.key]); ok {
			parts = append(parts, f.format(n))
		}
	}
	return strings.Join(parts, " · ")
}

func summarizeRouting(v map[string]any) string {
	switch err := v["error_reason"].(type) {
	case string:
		if err != "" {
			return err
		}
	case float64:
		if err != 0 {
			return fmt.Sprintf("error %.0f", err)
		}
	}
	return ""
}

func summarizeTraceroute(v map[string]any) string {
	fwd := len(asSlice(v["route"]))
	back := len(asSlice(v["route_back"]))
	if fwd == 0 && back == 0 {
		return ""
	}
	if back > 0 {
		return fmt.Sprintf("%d→ · ←%d hops", fwd, back)
	}
	return fmt.Sprintf("%d hops", fwd)
}

func summarizeNeighborInfo(v map[string]any) string {
	n := len(asSlice(v["neighbors"]))
	if n == 0 {
		return ""
	}
	if n == 1 {
		return "1 neighbor"
	}
	return fmt.Sprintf("%d neighbors", n)
}

func formatLatLon(v map[string]any) string {
	lat, latOK := asFloat(v["latitude_i"])
	lon, lonOK := asFloat(v["longitude_i"])
	if !latOK || !lonOK || (lat == 0 && lon == 0) {
		return ""
	}
	head := fmt.Sprintf("%.4f, %.4f", lat/1e7, lon/1e7)
	if alt, ok := asFloat(v["altitude"]); ok && alt != 0 {
		return fmt.Sprintf("%s · %.0f m", head, alt)
	}
	return head
}

func stringField(v map[string]any, key string) string {
	if x, ok := v[key].(string); ok {
		return strings.TrimSpace(x)
	}
	return ""
}

func pickObject(v map[string]any, key string) map[string]any {
	x, ok := v[key].(map[string]any)
	if !ok {
		return nil
	}
	return x
}

func asFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return 0, false
		}
		return x, true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func asSlice(v any) []any {
	s, ok := v.([]any)
	if !ok {
		return nil
	}
	return s
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func joinNonEmpty(parts []string, sep string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, sep)
}
