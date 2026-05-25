package forwarder

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
	"github.com/exepirit/meshtastic-go/pkg/meshtastic/proto"
	protobuf "google.golang.org/protobuf/proto"

	"github.com/relaymonkey/relaymesh-edge/internal/config"
	"github.com/relaymonkey/relaymesh-edge/internal/envelope"
	"github.com/relaymonkey/relaymesh-edge/internal/labels"
)

// Publisher sends ServiceEnvelopes to RelayMesh MQTT.
type Publisher struct {
	conn        *autopaho.ConnectionManager
	topicPrefix string
	labelsJSON  string
	agentID     string
}

// Connect establishes an MQTT v5 session.
func Connect(ctx context.Context, cfg config.MQTTConfig, agentID string, operatorLabels map[string]string) (*Publisher, error) {
	labelsJSON, err := labels.MarshalJSON(operatorLabels)
	if err != nil {
		return nil, err
	}

	brokerURL, err := normalizeBrokerURL(cfg.BrokerURL)
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(brokerURL)
	if err != nil {
		return nil, fmt.Errorf("parse broker url: %w", err)
	}

	clientConfig := autopaho.ClientConfig{
		BrokerUrls:        []*url.URL{u},
		KeepAlive:         30,
		ConnectRetryDelay: 3 * time.Second,
		OnConnectionUp: func(_ *autopaho.ConnectionManager, _ *paho.Connack) {},
		ClientConfig: paho.ClientConfig{
			ClientID: cfg.ClientID,
		},
	}
	if cfg.Username != "" {
		clientConfig.SetUsernamePassword(cfg.Username, []byte(cfg.Password))
	}

	conn, err := autopaho.NewConnection(ctx, clientConfig)
	if err != nil {
		return nil, fmt.Errorf("mqtt connect: %w", err)
	}
	if err := conn.AwaitConnection(ctx); err != nil {
		return nil, fmt.Errorf("mqtt await connection: %w", err)
	}

	if agentID == "" {
		agentID = extractAgentID(cfg.ClientID)
	}
	return &Publisher{
		conn:        conn,
		topicPrefix: strings.TrimRight(cfg.TopicPrefix, "/"),
		labelsJSON:  labelsJSON,
		agentID:     agentID,
	}, nil
}

// Close shuts down the MQTT session.
func (p *Publisher) Close(ctx context.Context) {
	if p == nil || p.conn == nil {
		return
	}
	_ = p.conn.Disconnect(ctx)
}

// PublishPassthrough forwards a packet heard over Phone API.
func (p *Publisher) PublishPassthrough(ctx context.Context, env *proto.ServiceEnvelope) error {
	return p.publish(ctx, env, labels.IngestSource(p.agentID))
}

// PublishSynthetic forwards a NodeDB-synthesised envelope.
func (p *Publisher) PublishSynthetic(ctx context.Context, env *proto.ServiceEnvelope) error {
	return p.publish(ctx, env, labels.IngestSourceNodeDB(p.agentID))
}

func (p *Publisher) publish(ctx context.Context, env *proto.ServiceEnvelope, ingestSource string) error {
	if env == nil || env.Packet == nil {
		return fmt.Errorf("publish: nil envelope")
	}
	payload, err := protobuf.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}
	topic := envelope.PublishTopic(p.topicPrefix, env.GetChannelId(), env.GetGatewayId())
	var props paho.UserProperties
	props.Add(labels.PropIngestSource, ingestSource)
	props.Add(labels.PropLabels, p.labelsJSON)

	_, err = p.conn.Publish(ctx, &paho.Publish{
		Topic:   topic,
		QoS:     1,
		Payload: payload,
		Properties: &paho.PublishProperties{
			User: props,
		},
	})
	if err != nil {
		return fmt.Errorf("mqtt publish %s: %w", topic, err)
	}
	return nil
}

func normalizeBrokerURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("empty broker url")
	}
	if strings.HasPrefix(raw, "mqtt://") || strings.HasPrefix(raw, "mqtts://") ||
		strings.HasPrefix(raw, "tcp://") || strings.HasPrefix(raw, "ssl://") ||
		strings.HasPrefix(raw, "ws://") || strings.HasPrefix(raw, "wss://") {
		return raw, nil
	}
	host := raw
	if !strings.Contains(host, ":") {
		host = net.JoinHostPort(host, "1883")
	}
	return "mqtt://" + host, nil
}

func extractAgentID(clientID string) string {
	const prefix = "rmesh-"
	if strings.HasPrefix(clientID, prefix) {
		return strings.TrimPrefix(clientID, prefix)
	}
	return clientID
}
