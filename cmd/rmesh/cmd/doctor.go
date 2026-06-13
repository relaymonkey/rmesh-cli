package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/exepirit/meshtastic-go/pkg/meshtastic"
	"github.com/spf13/cobra"

	"github.com/relaymonkey/relaymesh-edge/internal/cliui"
	rmdevice "github.com/relaymonkey/relaymesh-edge/internal/device"
	"github.com/relaymonkey/relaymesh-edge/internal/envelope"
	rmmetrics "github.com/relaymonkey/relaymesh-edge/internal/metrics"
	"github.com/relaymonkey/relaymesh-edge/internal/nodeid"
	rmtransport "github.com/relaymonkey/relaymesh-edge/internal/transport"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose config, transport, metrics port, and node database connectivity",
	RunE: func(cmd *cobra.Command, args []string) error {
		ui := cliui.New(cmd.OutOrStdout())
		errUI := cliui.New(cmd.ErrOrStderr())

		path, cfg, err := loadAgentConfig(cmd)
		if err != nil {
			_ = errUI.Fail("Config load failed", cliui.Field{Key: "error", Value: err.Error()})
			return err
		}
		if err := ui.Success("Config",
			cliui.Field{Key: "path", Value: path},
			cliui.Field{Key: "agent", Value: cfg.AgentID},
			cliui.Field{Key: "transport", Value: cfg.Transport.URL},
			cliui.Field{Key: "mqtt", Value: cfg.MQTT.BrokerURL},
			cliui.Field{Key: "topic", Value: cfg.MQTT.TopicPrefix},
			cliui.Field{Key: "labels", Value: fmt.Sprintf("%d keys", len(cfg.Labels))},
		); err != nil {
			return err
		}
		if strings.Contains(cfg.Transport.URL, "Bluetooth-Incoming-Port") {
			if err := ui.Warn("Bluetooth-Incoming-Port is not a mesh radio — use cu.usbmodem* or cu.usbserial*"); err != nil {
				return err
			}
		}

		if cfg.Metrics.Enabled {
			addr := cfg.Metrics.ListenAddr
			if err := rmmetrics.CheckListenAddr(addr); err != nil {
				_ = errUI.Fail("Metrics port unavailable",
					cliui.Field{Key: "listen_addr", Value: addr},
					cliui.Field{Key: "error", Value: err.Error()},
				)
				return err
			}
			if err := ui.Success("Metrics",
				cliui.Field{Key: "listen_addr", Value: addr},
				cliui.Field{Key: "bind", Value: "available"},
			); err != nil {
				return err
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		transport, err := rmtransport.Open(cfg.Transport.URL, rmtransport.Options{BLEPin: cfg.Transport.BLEPin})
		if err != nil {
			_ = errUI.Fail("Transport failed", cliui.Field{Key: "error", Value: err.Error()})
			return err
		}
		defer rmtransport.Close(transport)

		if _, err := meshtastic.NewConfiguredDevice(ctx, transport); err != nil {
			_ = errUI.Fail("Device connect failed", cliui.Field{Key: "error", Value: err.Error()})
			return err
		}
		state, err := rmdevice.GetState(ctx, transport)
		if err != nil {
			_ = errUI.Fail("Device state failed", cliui.Field{Key: "error", Value: err.Error()})
			return err
		}

		gateway := gatewayFromState(state.DeviceState)
		channel := envelope.PrimaryChannel(state.Channels, state.LoRa)
		deviceFields := []cliui.Field{
			{Key: "gateway", Value: gateway},
			{Key: "channel", Value: fmt.Sprintf("%s (index %d)", channel.ChannelID, channel.Index)},
			{Key: "nodes", Value: fmt.Sprintf("%d in node database", len(state.Nodes))},
		}
		if err := ui.Success("Device", deviceFields...); err != nil {
			return err
		}
		if channel.Name == "" && state.LoRa != nil && !state.LoRa.GetUsePreset() {
			if err := ui.Note("empty channel name + custom LoRa → Custom (matches firmware MQTT)"); err != nil {
				return err
			}
		}

		if len(state.Nodes) > 0 {
			if err := ui.Line("  sample nodes:"); err != nil {
				return err
			}
			limit := 5
			for i, n := range state.Nodes {
				if i >= limit {
					if err := ui.Note(fmt.Sprintf("… and %d more", len(state.Nodes)-limit)); err != nil {
						return err
					}
					break
				}
				name := "?"
				if n.User != nil {
					name = strings.TrimSpace(n.User.ShortName + " / " + n.User.LongName)
				}
				if err := ui.Line(fmt.Sprintf("    %s  %s", nodeid.FromNum(n.Num), name)); err != nil {
					return err
				}
			}
		}
		return ui.Success("All checks passed")
	},
}

func gatewayFromState(state meshtastic.DeviceState) string {
	if info, ok := state.CurrentNodeInfo(); ok && info.GetUser() != nil && info.User.GetId() != "" {
		return info.User.GetId()
	}
	if state.MyInfo != nil {
		return nodeid.FromNum(state.MyInfo.MyNodeNum)
	}
	return ""
}

func init() {
	doctorCmd.SilenceUsage = true
}
