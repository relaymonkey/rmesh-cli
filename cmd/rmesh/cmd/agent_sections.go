package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/relaymonkey/relaymesh-edge/internal/config"
)

// configSection is a contextual flag group: a verb opts into the sections it
// actually reads, and `--help` only advertises those flags. See `D-220`.
type configSection struct {
	name     string
	register func(*pflag.FlagSet)
	read     func(*pflag.FlagSet, *config.Overrides) error
}

var (
	sectionIdentity = configSection{
		name:     "identity",
		register: registerIdentityFlags,
		read:     readIdentityOverrides,
	}
	sectionTransport = configSection{
		name:     "transport",
		register: registerTransportFlags,
		read:     readTransportOverrides,
	}
	sectionMQTT = configSection{
		name:     "mqtt",
		register: registerMQTTFlags,
		read:     readMQTTOverrides,
	}
	sectionForward = configSection{
		name:     "forward",
		register: registerForwardFlags,
		read:     readForwardOverrides,
	}
	sectionSynthesise = configSection{
		name:     "synthesise",
		register: registerSynthesiseFlags,
		read:     readSynthesiseOverrides,
	}
	sectionLabels = configSection{
		name:     "labels",
		register: registerLabelsFlags,
		read:     readLabelsOverrides,
	}
	sectionMetrics = configSection{
		name:     "metrics",
		register: registerMetricsFlags,
		read:     readMetricsOverrides,
	}
)

// bindConfigSections registers the section flags on cmd and stores the section
// list on the command so loadAgentConfig can replay it.
//
// Flags are scoped to cmd (Flags()) not PersistentFlags() — they belong to the
// verb, not the `agent` parent. This is what keeps `observe --help` from
// advertising flags it would silently ignore (e.g. --mqtt-* in observe mode).
func bindConfigSections(cmd *cobra.Command, sections ...configSection) {
	f := cmd.Flags()
	f.StringArray("set", nil, "override config value (dotted path, repeatable): --set mqtt.broker_url=ssl://… or --set synthesise.position.interval=15s")
	for _, s := range sections {
		s.register(f)
	}
	cmdSections[cmd] = sections
}

var cmdSections = map[*cobra.Command][]configSection{}

// --- identity ---

func registerIdentityFlags(f *pflag.FlagSet) {
	f.String("agent-id", "", "override agent_id")
}

func readIdentityOverrides(f *pflag.FlagSet, o *config.Overrides) error {
	if f.Changed("agent-id") {
		v, _ := f.GetString("agent-id")
		o.AgentID = &v
	}
	return nil
}

// --- transport ---

func registerTransportFlags(f *pflag.FlagSet) {
	f.String("transport-url", "", "override transport.url (serial, http, ble)")
	f.String("ble-pin", "", "BLE fixed-PIN passkey for paired radios (default 123456 — currently OS-handled, see docs)")
}

func readTransportOverrides(f *pflag.FlagSet, o *config.Overrides) error {
	if f.Changed("transport-url") {
		v, _ := f.GetString("transport-url")
		o.TransportURL = &v
	}
	if f.Changed("ble-pin") {
		v, _ := f.GetString("ble-pin")
		o.BLEPin = &v
	}
	return nil
}

// --- mqtt ---

func registerMQTTFlags(f *pflag.FlagSet) {
	f.String("mqtt-broker-url", "", "override mqtt.broker_url")
	f.String("mqtt-username", "", "override mqtt.username")
	f.String("mqtt-password", "", "override mqtt.password")
	f.String("mqtt-topic-prefix", "", "override mqtt.topic_prefix")
	f.String("mqtt-client-id", "", "override mqtt.client_id")
}

func readMQTTOverrides(f *pflag.FlagSet, o *config.Overrides) error {
	if f.Changed("mqtt-broker-url") {
		v, _ := f.GetString("mqtt-broker-url")
		o.MQTTBrokerURL = &v
	}
	if f.Changed("mqtt-username") {
		v, _ := f.GetString("mqtt-username")
		o.MQTTUsername = &v
	}
	if f.Changed("mqtt-password") {
		v, _ := f.GetString("mqtt-password")
		o.MQTTPassword = &v
	}
	if f.Changed("mqtt-topic-prefix") {
		v, _ := f.GetString("mqtt-topic-prefix")
		o.MQTTTopicPrefix = &v
	}
	if f.Changed("mqtt-client-id") {
		v, _ := f.GetString("mqtt-client-id")
		o.MQTTClientID = &v
	}
	return nil
}

// --- forward ---

func registerForwardFlags(f *pflag.FlagSet) {
	f.Bool("ignore-ok-to-mqtt", false, "forward passthrough packets even when the sender cleared the Meshtastic ok_to_mqtt consent bit (overrides forward.respect_ok_to_mqtt)")
}

func readForwardOverrides(f *pflag.FlagSet, o *config.Overrides) error {
	if f.Changed("ignore-ok-to-mqtt") {
		ignore, _ := f.GetBool("ignore-ok-to-mqtt")
		respect := !ignore
		o.RespectOkToMqtt = &respect
	}
	return nil
}

// --- synthesise ---

func registerSynthesiseFlags(f *pflag.FlagSet) {
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

func readSynthesiseOverrides(f *pflag.FlagSet, o *config.Overrides) error {
	if f.Changed("synthesise-nodedb-poll") {
		v, _ := f.GetDuration("synthesise-nodedb-poll")
		o.SynthesiseNodeDBPoll = &v
	}
	if err := readCadenceOverrides(f, "synthesise-nodeinfo", &o.NodeInfoEnabled, &o.NodeInfoInterval, &o.NodeInfoOnFirstSeen, &o.NodeInfoJitter, nil); err != nil {
		return err
	}
	if err := readCadenceOverrides(f, "synthesise-position", &o.PositionEnabled, &o.PositionInterval, &o.PositionOnFirstSeen, &o.PositionJitter, &o.PositionRespectPositionPrecision); err != nil {
		return err
	}
	if err := readCadenceOverrides(f, "synthesise-mapreport", &o.MapReportEnabled, &o.MapReportInterval, &o.MapReportOnFirstSeen, &o.MapReportJitter, nil); err != nil {
		return err
	}
	return nil
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

// --- labels ---

func registerLabelsFlags(f *pflag.FlagSet) {
	f.StringArray("label", nil, "override/add label (key=value, repeatable)")
}

func readLabelsOverrides(f *pflag.FlagSet, o *config.Overrides) error {
	if f.Changed("label") {
		pairs, _ := f.GetStringArray("label")
		labels, err := parseLabelOverrides(pairs)
		if err != nil {
			return err
		}
		o.Labels = labels
	}
	return nil
}

func parseLabelOverrides(pairs []string) (map[string]string, error) {
	if len(pairs) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(pairs))
	for _, pair := range pairs {
		key, value, ok := splitKV(pair)
		if !ok || key == "" {
			return nil, fmt.Errorf("label %q: expected key=value", pair)
		}
		out[key] = value
	}
	return out, nil
}

// --- metrics ---

func registerMetricsFlags(f *pflag.FlagSet) {
	f.Bool("metrics-enabled", false, "expose Prometheus /metrics for local node RF gauges")
	f.String("metrics-listen-addr", "", "override metrics.listen_addr (default 127.0.0.1:19092)")
	f.Duration("metrics-nodedb-refresh-interval", 0, "override metrics.nodedb_refresh_interval (0 inherits synthesise.nodedb_poll)")
}

func readMetricsOverrides(f *pflag.FlagSet, o *config.Overrides) error {
	if f.Changed("metrics-enabled") {
		v, _ := f.GetBool("metrics-enabled")
		o.MetricsEnabled = &v
	}
	if f.Changed("metrics-listen-addr") {
		v, _ := f.GetString("metrics-listen-addr")
		o.MetricsListenAddr = &v
	}
	if f.Changed("metrics-nodedb-refresh-interval") {
		v, _ := f.GetDuration("metrics-nodedb-refresh-interval")
		o.MetricsNodeDBRefreshInterval = &v
	}
	return nil
}

func splitKV(s string) (string, string, bool) {
	for i := 0; i < len(s); i++ {
		if s[i] == '=' {
			return s[:i], s[i+1:], true
		}
	}
	return s, "", false
}
