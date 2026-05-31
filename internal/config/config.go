// Package config loads and validates configuration for the rmesh agent subcommand.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	MaxLabelKeys     = 32
	MaxLabelKeyLen   = 64
	MaxLabelValueLen = 256

	// DefaultBLEPin matches the Meshtastic Python CLI default for radios in
	// FIXED_PIN BLE mode. Operators can override via transport.ble_pin or the
	// --ble-pin CLI flag.
	DefaultBLEPin = "123456"
)

// Config is the on-disk agent configuration.
type Config struct {
	AgentID    string            `yaml:"agent_id"`
	Transport  TransportConfig   `yaml:"transport"`
	MQTT       MQTTConfig        `yaml:"mqtt"`
	Forward    ForwardConfig     `yaml:"forward"`
	Labels     map[string]string `yaml:"labels"`
	Synthesise SynthesiseConfig  `yaml:"synthesise"`
}

// ForwardConfig governs which observed packets are eligible for publish.
type ForwardConfig struct {
	// RespectOkToMqtt drops passthrough packets whose sender explicitly cleared
	// the Meshtastic ok_to_mqtt consent bit (decoded Data.bitfield bit 0).
	// Synthetic NodeDB traffic and packets this gateway cannot decode are never
	// affected. Nil means unset; the default is true.
	RespectOkToMqtt *bool `yaml:"respect_ok_to_mqtt,omitempty"`
}

// RespectOk reports the effective ok_to_mqtt policy (defaults to true).
func (f ForwardConfig) RespectOk() bool {
	return f.RespectOkToMqtt == nil || *f.RespectOkToMqtt
}

// TransportConfig selects the local radio connection.
type TransportConfig struct {
	// URL scheme: serial:/dev/ttyUSB0, http://192.168.1.10:4403, ble://AA:BB:CC:DD:EE:FF
	URL string `yaml:"url"`
	// BLEPin is a 6-digit passkey used when the radio is in FIXED_PIN BLE mode.
	// Defaults to 123456 (matches Meshtastic Python CLI). Currently informational
	// only — pairing is brokered by the host OS on macOS / BlueZ on Linux; rmesh
	// cannot inject the PIN programmatically yet.
	BLEPin string `yaml:"ble_pin,omitempty"`
}

// MQTTConfig holds RelayMesh broker credentials.
type MQTTConfig struct {
	BrokerURL   string `yaml:"broker_url"`
	Username    string `yaml:"username"`
	Password    string `yaml:"password"`
	TopicPrefix string `yaml:"topic_prefix"`
	ClientID    string `yaml:"client_id"`
}

// SynthesiseConfig controls NodeDB-derived synthetic envelopes.
type SynthesiseConfig struct {
	NodeInfo   CadenceConfig `yaml:"nodeinfo"`
	Position   CadenceConfig `yaml:"position"`
	MapReport  CadenceConfig `yaml:"mapreport"`
	NodeDBPoll time.Duration `yaml:"nodedb_poll"`
}

// CadenceConfig is per-kind emission policy.
type CadenceConfig struct {
	Enabled                  bool          `yaml:"enabled"`
	Interval                 time.Duration `yaml:"interval"`
	OnFirstSeen              bool          `yaml:"on_first_seen"`
	Jitter                   time.Duration `yaml:"jitter"`
	RespectPositionPrecision bool          `yaml:"respect_position_precision"`
}

// Load reads and validates configuration from path.
func Load(path string) (Config, error) {
	cfg, err := LoadRaw(path)
	if err != nil {
		return Config{}, err
	}
	if err := cfg.Finalize(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// LoadRaw reads and parses the config file but does not Finalize. Callers that
// apply CLI overrides between parse and validate use this and call Finalize
// themselves.
func LoadRaw(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}

// Finalize validates the config and applies defaults. Call after in-memory
// edits (for example CLI flag overrides) before use.
func (c *Config) Finalize() error {
	if err := c.Validate(); err != nil {
		return err
	}
	c.applyDefaults()
	return nil
}

// Validate checks required fields and label bounds.
func (c *Config) Validate() error {
	if strings.TrimSpace(c.Transport.URL) == "" {
		return errors.New("transport.url is required")
	}
	if pin := strings.TrimSpace(c.Transport.BLEPin); pin != "" {
		if len(pin) < 6 || len(pin) > 8 {
			return fmt.Errorf("transport.ble_pin must be 6-8 digits")
		}
		for _, r := range pin {
			if r < '0' || r > '9' {
				return fmt.Errorf("transport.ble_pin must be digits only")
			}
		}
	}
	if strings.TrimSpace(c.MQTT.BrokerURL) == "" {
		return errors.New("mqtt.broker_url is required")
	}
	if strings.TrimSpace(c.MQTT.TopicPrefix) == "" {
		return errors.New("mqtt.topic_prefix is required")
	}
	if strings.TrimSpace(c.MQTT.Username) == "" {
		return errors.New("mqtt.username is required")
	}
	if strings.TrimSpace(c.MQTT.Password) == "" {
		return errors.New("mqtt.password is required")
	}
	if len(c.Labels) > MaxLabelKeys {
		return fmt.Errorf("labels: at most %d keys allowed", MaxLabelKeys)
	}
	for k, v := range c.Labels {
		if strings.HasPrefix(k, "relaymesh.") {
			return fmt.Errorf("labels: key %q uses reserved prefix relaymesh.", k)
		}
		if len(k) == 0 || len(k) > MaxLabelKeyLen {
			return fmt.Errorf("labels: key %q length must be 1..%d", k, MaxLabelKeyLen)
		}
		if len(v) > MaxLabelValueLen {
			return fmt.Errorf("labels: value for %q exceeds %d chars", k, MaxLabelValueLen)
		}
	}
	return nil
}

func (c *Config) applyDefaults() {
	if c.AgentID == "" {
		c.AgentID = "local"
	}
	if c.Transport.BLEPin == "" {
		c.Transport.BLEPin = DefaultBLEPin
	}
	if c.MQTT.ClientID == "" {
		c.MQTT.ClientID = "rmesh-" + c.AgentID
	}
	if c.Forward.RespectOkToMqtt == nil {
		on := true
		c.Forward.RespectOkToMqtt = &on
	}
	if c.Synthesise.NodeDBPoll == 0 {
		c.Synthesise.NodeDBPoll = 5 * time.Minute
	}
	applyCadenceDefaults(&c.Synthesise.NodeInfo, 6*time.Hour, true)
	applyCadenceDefaults(&c.Synthesise.Position, 30*time.Minute, false)
	applyCadenceDefaults(&c.Synthesise.MapReport, 6*time.Hour, false)
}

func applyCadenceDefaults(c *CadenceConfig, interval time.Duration, onFirst bool) {
	wasZero := c.Interval == 0
	if c.Interval == 0 {
		c.Interval = interval
	}
	if wasZero {
		c.Enabled = true
	}
	if onFirst && !c.OnFirstSeen {
		c.OnFirstSeen = onFirst
	}
}
