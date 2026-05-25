package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/exepirit/meshtastic-go/pkg/meshtastic"
	"github.com/spf13/cobra"

	"github.com/relaymonkey/relaymesh-edge/internal/config"
	rmdevice "github.com/relaymonkey/relaymesh-edge/internal/device"
	"github.com/relaymonkey/relaymesh-edge/internal/envelope"
	"github.com/relaymonkey/relaymesh-edge/internal/nodeid"
	rmtransport "github.com/relaymonkey/relaymesh-edge/internal/transport"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose config, transport, and NodeDB connectivity",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := loadConfig()
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "config: %v\n", err)
			return err
		}
		cfg, err := config.Load(path)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "config validation: %v\n", err)
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "config: ok (%s)\n", path)
		fmt.Fprintf(cmd.OutOrStdout(), "  agent_id: %s\n", cfg.AgentID)
		fmt.Fprintf(cmd.OutOrStdout(), "  transport: %s\n", cfg.Transport.URL)
		if strings.Contains(cfg.Transport.URL, "Bluetooth-Incoming-Port") {
			fmt.Fprintln(cmd.OutOrStdout(), "  warning: Bluetooth-Incoming-Port is not a Meshtastic device — use cu.usbmodem* or cu.usbserial*")
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  mqtt broker: %s\n", cfg.MQTT.BrokerURL)
		fmt.Fprintf(cmd.OutOrStdout(), "  mqtt topic_prefix: %s\n", cfg.MQTT.TopicPrefix)
		fmt.Fprintf(cmd.OutOrStdout(), "  labels: %d keys\n", len(cfg.Labels))

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		transport, err := rmtransport.Open(cfg.Transport.URL)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "transport: %v\n", err)
			return err
		}
		defer rmtransport.Close(transport)

		if _, err := meshtastic.NewConfiguredDevice(ctx, transport); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "device connect: %v\n", err)
			return err
		}
		state, err := rmdevice.GetState(ctx, transport)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "device state: %v\n", err)
			return err
		}

		gateway := gatewayFromState(state.DeviceState)
		channel := envelope.PrimaryChannel(state.Channels, state.LoRa)
		fmt.Fprintf(cmd.OutOrStdout(), "device: ok\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  gateway_id: %s\n", gateway)
		fmt.Fprintf(cmd.OutOrStdout(), "  channel: %s (index %d)\n", channel.ChannelID, channel.Index)
		if channel.Name == "" && state.LoRa != nil && !state.LoRa.GetUsePreset() {
			fmt.Fprintln(cmd.OutOrStdout(), "  channel note: empty name + custom LoRa → Custom (matches firmware MQTT)")
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  nodedb nodes: %d\n", len(state.Nodes))

		if len(state.Nodes) > 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "  sample nodes:")
			limit := 5
			for i, n := range state.Nodes {
				if i >= limit {
					fmt.Fprintf(cmd.OutOrStdout(), "    ... and %d more\n", len(state.Nodes)-limit)
					break
				}
				name := "?"
				if n.User != nil {
					name = strings.TrimSpace(n.User.ShortName + " / " + n.User.LongName)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "    %s  %s\n", nodeid.FromNum(n.Num), name)
			}
		}
		fmt.Fprintln(cmd.OutOrStdout(), "doctor: all checks passed")
		return nil
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
