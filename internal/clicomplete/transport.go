package clicomplete

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	goserial "go.bug.st/serial"
	"github.com/spf13/cobra"

	"github.com/relaymonkey/relaymesh-edge/internal/cliconfig"
	"github.com/relaymonkey/relaymesh-edge/internal/config"
	rmtransport "github.com/relaymonkey/relaymesh-edge/internal/transport"
)

// TransportURLProvider completes --transport-url for rmesh agent commands.
func TransportURLProvider(ctx context.Context, toComplete string) ([]Item, cobra.ShellCompDirective, error) {
	var items []Item
	seen := make(map[string]struct{})

	add := func(value, desc string) {
		if !prefixMatch(toComplete, value) {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		items = append(items, Item{Value: value, Description: desc})
	}

	var bleCh <-chan []rmtransport.BLEDevice
	if shouldDiscoverBLE(toComplete) {
		ch := make(chan []rmtransport.BLEDevice, 1)
		bleCh = ch
		go func() {
			ch <- cachedBLEDevices(ctx)
		}()
	}

	if url, err := AgentConfigTransportURL(); err == nil && url != "" {
		add(url, "from agent config")
	}

	for _, scheme := range []struct {
		value string
		desc  string
	}{
		{"serial:", "USB/serial device"},
		{"http://", "meshtasticd HTTP"},
		{"https://", "meshtasticd HTTPS"},
		{"ble://", "Bluetooth LE"},
	} {
		if transportCompletionStage(toComplete) != "scheme" {
			continue
		}
		if prefixMatch(toComplete, scheme.value) {
			add(scheme.value, scheme.desc)
		}
	}

	if shouldListSerialPorts(toComplete) {
		ports, err := goserial.GetPortsList()
		if err == nil {
			for _, port := range ports {
				add("serial:"+port, serialPortDescription(port))
			}
		}
	}

	if shouldDiscoverBLE(toComplete) {
		for _, dev := range <-bleCh {
			desc := dev.Name
			if desc == "" {
				desc = "Bluetooth LE"
			}
			if dev.RSSI != 0 {
				desc = fmt.Sprintf("%s (%ddBm)", desc, dev.RSSI)
			}
			add(dev.URL, desc)
			if dev.AltURL != "" {
				add(dev.AltURL, desc+" (UUID)")
			}
		}
	}

	return items, cobra.ShellCompDirectiveNoFileComp, nil
}

// AgentConfigTransportURL returns transport.url from the default agent config file.
func AgentConfigTransportURL() (string, error) {
	cfg, err := config.Load(cliconfig.AgentConfigPath())
	if err != nil {
		return "", err
	}
	return cfg.Transport.URL, nil
}

func serialPortDescription(port string) string {
	switch runtime.GOOS {
	case "darwin":
		if strings.Contains(port, "Bluetooth-Incoming") {
			return "not a mesh radio"
		}
		if strings.Contains(port, "usbmodem") || strings.Contains(port, "usbserial") {
			return "likely mesh radio"
		}
	case "linux":
		if strings.HasPrefix(port, "/dev/ttyUSB") || strings.HasPrefix(port, "/dev/ttyACM") {
			return "likely mesh radio"
		}
	}
	return "serial device"
}
