// Package config loads and validates rmesh agent configuration.
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
	DefaultConfigPath = "/etc/rmesh/config.yaml"
	MaxLabelKeys      = 32
	MaxLabelKeyLen    = 64
	MaxLabelValueLen  = 256
)

// Config is the on-disk agent configuration.
type Config struct {
	AgentID   string            `yaml:"agent_id"`
	Transport TransportConfig   `yaml:"transport"`
	MQTT      MQTTConfig        `yaml:"mqtt"`
	Labels    map[string]string `yaml:"labels"`
	Synthesise SynthesiseConfig `yaml:"synthesise"`
}

// TransportConfig selects the local Phone API connection.
type TransportConfig struct {
	// URL scheme: serial:/dev/ttyUSB0, http://192.168.1.10:4403, ble://MAC
	URL string `yaml:"url"`
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
	Enabled                 bool          `yaml:"enabled"`
	Interval                time.Duration `yaml:"interval"`
	OnFirstSeen             bool          `yaml:"on_first_seen"`
	Jitter                  time.Duration `yaml:"jitter"`
	RespectPositionPrecision bool         `yaml:"respect_position_precision"`
}

// Load reads and validates configuration from path.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	cfg.applyDefaults()
	return cfg, nil
}

// Validate checks required fields and label bounds.
func (c *Config) Validate() error {
	if strings.TrimSpace(c.Transport.URL) == "" {
		return errors.New("transport.url is required")
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
	if c.MQTT.ClientID == "" {
		c.MQTT.ClientID = "rmesh-" + c.AgentID
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
