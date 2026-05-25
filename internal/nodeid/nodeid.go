package nodeid

import "fmt"

// FromNum formats a Meshtastic node number as !xxxxxxxx.
func FromNum(num uint32) string {
	return fmt.Sprintf("!%08x", num)
}
