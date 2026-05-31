package config_test

import (
	"testing"
	"time"

	"github.com/relaymonkey/relaymesh-edge/internal/config"
)

func TestOverridesApply(t *testing.T) {
	cfg := config.Config{
		AgentID: "file-agent",
		Transport: config.TransportConfig{
			URL: "serial:/dev/ttyUSB0",
		},
		MQTT: config.MQTTConfig{
			BrokerURL:   "mqtt://localhost:1883",
			Username:    "user",
			Password:    "secret",
			TopicPrefix: "rm/n/test",
		},
		Labels: map[string]string{"site": "east"},
	}

	transport := "ble://Meshtastic_ab12"
	interval := 15 * time.Minute
	enabled := false
	respect := false
	overrides := config.Overrides{
		TransportURL:     &transport,
		Labels:           map[string]string{"site": "west", "role": "observer"},
		PositionEnabled:  &enabled,
		PositionInterval: &interval,
		RespectOkToMqtt:  &respect,
	}
	overrides.Apply(&cfg)

	if cfg.Forward.RespectOk() {
		t.Fatal("expected forward.respect_ok_to_mqtt override to false")
	}

	if cfg.Transport.URL != transport {
		t.Fatalf("transport.url = %q", cfg.Transport.URL)
	}
	if cfg.Labels["site"] != "west" || cfg.Labels["role"] != "observer" {
		t.Fatalf("labels = %#v", cfg.Labels)
	}
	if cfg.Synthesise.Position.Enabled {
		t.Fatal("expected position synthesis disabled")
	}
	if cfg.Synthesise.Position.Interval != interval {
		t.Fatalf("position interval = %v", cfg.Synthesise.Position.Interval)
	}
	if cfg.AgentID != "file-agent" {
		t.Fatalf("agent_id changed unexpectedly: %q", cfg.AgentID)
	}
}
