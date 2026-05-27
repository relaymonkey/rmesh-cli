package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/relaymonkey/relaymesh-edge/internal/config"
)

func initAgentConfigFlags(cmd *cobra.Command) {
	f := cmd.PersistentFlags()

	f.String("agent-id", "", "override agent_id")
	f.String("transport-url", "", "override transport.url (serial, http, ble)")
	f.String("ble-pin", "", "BLE fixed-PIN passkey for paired radios (default 123456 — currently OS-handled, see docs)")

	f.String("mqtt-broker-url", "", "override mqtt.broker_url")
	f.String("mqtt-username", "", "override mqtt.username")
	f.String("mqtt-password", "", "override mqtt.password")
	f.String("mqtt-topic-prefix", "", "override mqtt.topic_prefix")
	f.String("mqtt-client-id", "", "override mqtt.client_id")

	f.StringArray("label", nil, "override/add label (key=value, repeatable)")

	f.Duration("synthesise-nodedb-poll", 0, "override synthesise.nodedb_poll")

	registerCadenceFlags(f, "synthesise-nodeinfo", "synthesise.nodeinfo", false)
	registerCadenceFlags(f, "synthesise-position", "synthesise.position", true)
	registerCadenceFlags(f, "synthesise-mapreport", "synthesise.mapreport", false)
}

func registerCadenceFlags(f *pflag.FlagSet, prefix, docPrefix string, withRespectPrecision bool) {
	f.Bool(prefix+"-enabled", false, "override "+docPrefix+".enabled")
	f.Duration(prefix+"-interval", 0, "override "+docPrefix+".interval")
	f.Bool(prefix+"-on-first-seen", false, "override "+docPrefix+".on_first_seen")
	f.Duration(prefix+"-jitter", 0, "override "+docPrefix+".jitter")
	if withRespectPrecision {
		f.Bool(prefix+"-respect-position-precision", false, "override "+docPrefix+".respect_position_precision")
	}
}

func loadAgentConfig(cmd *cobra.Command) (string, config.Config, error) {
	path, err := loadConfig()
	if err != nil {
		return "", config.Config{}, err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return "", config.Config{}, err
	}
	overrides, err := agentOverridesFromFlags(cmd.Flags())
	if err != nil {
		return "", config.Config{}, err
	}
	overrides.Apply(&cfg)
	if err := cfg.Finalize(); err != nil {
		return "", config.Config{}, err
	}
	return path, cfg, nil
}

func agentOverridesFromFlags(flags *pflag.FlagSet) (config.Overrides, error) {
	var o config.Overrides

	if flags.Changed("agent-id") {
		v, _ := flags.GetString("agent-id")
		o.AgentID = &v
	}
	if flags.Changed("transport-url") {
		v, _ := flags.GetString("transport-url")
		o.TransportURL = &v
	}
	if flags.Changed("ble-pin") {
		v, _ := flags.GetString("ble-pin")
		o.BLEPin = &v
	}
	if flags.Changed("mqtt-broker-url") {
		v, _ := flags.GetString("mqtt-broker-url")
		o.MQTTBrokerURL = &v
	}
	if flags.Changed("mqtt-username") {
		v, _ := flags.GetString("mqtt-username")
		o.MQTTUsername = &v
	}
	if flags.Changed("mqtt-password") {
		v, _ := flags.GetString("mqtt-password")
		o.MQTTPassword = &v
	}
	if flags.Changed("mqtt-topic-prefix") {
		v, _ := flags.GetString("mqtt-topic-prefix")
		o.MQTTTopicPrefix = &v
	}
	if flags.Changed("mqtt-client-id") {
		v, _ := flags.GetString("mqtt-client-id")
		o.MQTTClientID = &v
	}
	if flags.Changed("label") {
		pairs, _ := flags.GetStringArray("label")
		labels, err := parseLabelOverrides(pairs)
		if err != nil {
			return config.Overrides{}, err
		}
		o.Labels = labels
	}
	if flags.Changed("synthesise-nodedb-poll") {
		v, _ := flags.GetDuration("synthesise-nodedb-poll")
		o.SynthesiseNodeDBPoll = &v
	}

	if err := readCadenceOverrides(flags, "synthesise-nodeinfo", &o.NodeInfoEnabled, &o.NodeInfoInterval, &o.NodeInfoOnFirstSeen, &o.NodeInfoJitter, nil); err != nil {
		return config.Overrides{}, err
	}
	if err := readCadenceOverrides(flags, "synthesise-position", &o.PositionEnabled, &o.PositionInterval, &o.PositionOnFirstSeen, &o.PositionJitter, &o.PositionRespectPositionPrecision); err != nil {
		return config.Overrides{}, err
	}
	if err := readCadenceOverrides(flags, "synthesise-mapreport", &o.MapReportEnabled, &o.MapReportInterval, &o.MapReportOnFirstSeen, &o.MapReportJitter, nil); err != nil {
		return config.Overrides{}, err
	}

	return o, nil
}

func readCadenceOverrides(
	flags *pflag.FlagSet,
	prefix string,
	enabled **bool,
	interval **time.Duration,
	onFirst **bool,
	jitter **time.Duration,
	respectPP **bool,
) error {
	if flags.Changed(prefix + "-enabled") {
		v, _ := flags.GetBool(prefix + "-enabled")
		*enabled = &v
	}
	if flags.Changed(prefix + "-interval") {
		v, _ := flags.GetDuration(prefix + "-interval")
		*interval = &v
	}
	if flags.Changed(prefix + "-on-first-seen") {
		v, _ := flags.GetBool(prefix + "-on-first-seen")
		*onFirst = &v
	}
	if flags.Changed(prefix + "-jitter") {
		v, _ := flags.GetDuration(prefix + "-jitter")
		*jitter = &v
	}
	if respectPP != nil && flags.Changed(prefix+"-respect-position-precision") {
		v, _ := flags.GetBool(prefix + "-respect-position-precision")
		*respectPP = &v
	}
	return nil
}

func parseLabelOverrides(pairs []string) (map[string]string, error) {
	if len(pairs) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(pairs))
	for _, pair := range pairs {
		key, value, ok := strings.Cut(pair, "=")
		if !ok || key == "" {
			return nil, fmt.Errorf("label %q: expected key=value", pair)
		}
		out[key] = value
	}
	return out, nil
}
