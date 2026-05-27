package clicomplete

import "strings"

// transportCompletionStage decides whether tab completion lists transport
// schemes or enumerates devices for a committed scheme.
//
// Stage 1 (scheme): empty or partial prefix — offer serial:/ble:///http(s):// only.
// Stage 2 (enumerate): after the user commits to a scheme, list ports or BLE radios.
func transportCompletionStage(toComplete string) string {
	lower := strings.ToLower(strings.TrimSpace(toComplete))
	switch {
	case strings.HasPrefix(lower, "serial:"):
		return "serial"
	case strings.HasPrefix(lower, "ble://"):
		return "ble"
	case strings.HasPrefix(lower, "http://"), strings.HasPrefix(lower, "https://"):
		return "http"
	default:
		return "scheme"
	}
}

func shouldListSerialPorts(toComplete string) bool {
	return transportCompletionStage(toComplete) == "serial"
}

func shouldDiscoverBLE(toComplete string) bool {
	return transportCompletionStage(toComplete) == "ble"
}
