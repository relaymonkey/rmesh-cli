package config

import "time"

// Overrides holds optional agent config fields set from CLI flags.
// A nil pointer means "leave the loaded config value unchanged".
type Overrides struct {
	AgentID      *string
	TransportURL *string
	BLEPin       *string

	MQTTBrokerURL   *string
	MQTTUsername    *string
	MQTTPassword    *string
	MQTTTopicPrefix *string
	MQTTClientID    *string

	Labels map[string]string

	SynthesiseNodeDBPoll *time.Duration

	NodeInfoEnabled   *bool
	NodeInfoInterval  *time.Duration
	NodeInfoOnFirstSeen *bool
	NodeInfoJitter    *time.Duration

	PositionEnabled                  *bool
	PositionInterval                 *time.Duration
	PositionOnFirstSeen              *bool
	PositionJitter                   *time.Duration
	PositionRespectPositionPrecision *bool

	MapReportEnabled      *bool
	MapReportInterval     *time.Duration
	MapReportOnFirstSeen  *bool
	MapReportJitter       *time.Duration
}

// Apply merges non-nil override fields into cfg.
func (o Overrides) Apply(cfg *Config) {
	if o.AgentID != nil {
		cfg.AgentID = *o.AgentID
	}
	if o.TransportURL != nil {
		cfg.Transport.URL = *o.TransportURL
	}
	if o.BLEPin != nil {
		cfg.Transport.BLEPin = *o.BLEPin
	}
	if o.MQTTBrokerURL != nil {
		cfg.MQTT.BrokerURL = *o.MQTTBrokerURL
	}
	if o.MQTTUsername != nil {
		cfg.MQTT.Username = *o.MQTTUsername
	}
	if o.MQTTPassword != nil {
		cfg.MQTT.Password = *o.MQTTPassword
	}
	if o.MQTTTopicPrefix != nil {
		cfg.MQTT.TopicPrefix = *o.MQTTTopicPrefix
	}
	if o.MQTTClientID != nil {
		cfg.MQTT.ClientID = *o.MQTTClientID
	}
	if len(o.Labels) > 0 {
		if cfg.Labels == nil {
			cfg.Labels = make(map[string]string, len(o.Labels))
		}
		for k, v := range o.Labels {
			cfg.Labels[k] = v
		}
	}

	if o.SynthesiseNodeDBPoll != nil {
		cfg.Synthesise.NodeDBPoll = *o.SynthesiseNodeDBPoll
	}
	applyCadenceOverride(&cfg.Synthesise.NodeInfo, cadenceOverride{
		Enabled:   o.NodeInfoEnabled,
		Interval:  o.NodeInfoInterval,
		OnFirst:   o.NodeInfoOnFirstSeen,
		Jitter:    o.NodeInfoJitter,
		RespectPP: nil,
	})
	applyCadenceOverride(&cfg.Synthesise.Position, cadenceOverride{
		Enabled:   o.PositionEnabled,
		Interval:  o.PositionInterval,
		OnFirst:   o.PositionOnFirstSeen,
		Jitter:    o.PositionJitter,
		RespectPP: o.PositionRespectPositionPrecision,
	})
	applyCadenceOverride(&cfg.Synthesise.MapReport, cadenceOverride{
		Enabled:   o.MapReportEnabled,
		Interval:  o.MapReportInterval,
		OnFirst:   o.MapReportOnFirstSeen,
		Jitter:    o.MapReportJitter,
		RespectPP: nil,
	})
}

type cadenceOverride struct {
	Enabled   *bool
	Interval  *time.Duration
	OnFirst   *bool
	Jitter    *time.Duration
	RespectPP *bool
}

func applyCadenceOverride(c *CadenceConfig, o cadenceOverride) {
	if o.Enabled != nil {
		c.Enabled = *o.Enabled
	}
	if o.Interval != nil {
		c.Interval = *o.Interval
	}
	if o.OnFirst != nil {
		c.OnFirstSeen = *o.OnFirst
	}
	if o.Jitter != nil {
		c.Jitter = *o.Jitter
	}
	if o.RespectPP != nil {
		c.RespectPositionPrecision = *o.RespectPP
	}
}
