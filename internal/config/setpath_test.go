package config_test

import (
	"strings"
	"testing"
	"time"

	"github.com/relaymonkey/relaymesh-edge/internal/config"
)

func TestApplySetPaths(t *testing.T) {
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

	err := config.ApplySetPaths(&cfg, []string{
		"agent_id=cli-agent",
		"transport.url=ble://AA:BB:CC:DD:EE:FF",
		"mqtt.broker_url=ssl://broker.example:8883",
		"synthesise.position.enabled=false",
		"synthesise.position.interval=12m30s",
		"synthesise.nodedb_poll=2m",
		"forward.respect_ok_to_mqtt=false",
		"labels.role=observer",
	})
	if err != nil {
		t.Fatalf("ApplySetPaths: %v", err)
	}

	if cfg.Forward.RespectOkToMqtt == nil || *cfg.Forward.RespectOkToMqtt {
		t.Errorf("forward.respect_ok_to_mqtt should be set to false, got %v", cfg.Forward.RespectOkToMqtt)
	}

	if cfg.AgentID != "cli-agent" {
		t.Errorf("agent_id = %q", cfg.AgentID)
	}
	if cfg.Transport.URL != "ble://AA:BB:CC:DD:EE:FF" {
		t.Errorf("transport.url = %q", cfg.Transport.URL)
	}
	if cfg.MQTT.BrokerURL != "ssl://broker.example:8883" {
		t.Errorf("mqtt.broker_url = %q", cfg.MQTT.BrokerURL)
	}
	if cfg.Synthesise.Position.Enabled {
		t.Error("position.enabled should be false")
	}
	if want := 12*time.Minute + 30*time.Second; cfg.Synthesise.Position.Interval != want {
		t.Errorf("position.interval = %v, want %v", cfg.Synthesise.Position.Interval, want)
	}
	if cfg.Synthesise.NodeDBPoll != 2*time.Minute {
		t.Errorf("nodedb_poll = %v", cfg.Synthesise.NodeDBPoll)
	}
	if cfg.Labels["role"] != "observer" || cfg.Labels["site"] != "east" {
		t.Errorf("labels = %#v", cfg.Labels)
	}
}

func TestApplySetPaths_Errors(t *testing.T) {
	cases := []struct {
		name   string
		pairs  []string
		errSub string
	}{
		{"missing equals", []string{"transport.url"}, "expected path=value"},
		{"empty path", []string{"=foo"}, "empty path"},
		{"unknown top-level", []string{"bogus.field=x"}, `unknown field "bogus"`},
		{"unknown nested", []string{"mqtt.nope=x"}, `unknown field "nope"`},
		{"bad bool", []string{"synthesise.position.enabled=maybe"}, `expected bool`},
		{"bad duration", []string{"synthesise.position.interval=10"}, `expected duration`},
		{"descend into map", []string{"labels.site.extra=x"}, "descends into map"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.Config{}
			err := config.ApplySetPaths(&cfg, tc.pairs)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.errSub)
			}
			if !strings.Contains(err.Error(), tc.errSub) {
				t.Fatalf("err = %q, want substring %q", err.Error(), tc.errSub)
			}
		})
	}
}
