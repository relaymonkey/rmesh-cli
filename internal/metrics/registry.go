package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// Registry holds node RF gauges and the Prometheus registry backing /metrics.
type Registry struct {
	prom *prometheus.Registry

	channelUtil *prometheus.GaugeVec
	airUtilTx   *prometheus.GaugeVec
	updatedAt   *prometheus.GaugeVec

	agentID   string
	gatewayID string
}

// NewRegistry builds a per-agent metrics registry with process collectors.
func NewRegistry(agentID, gatewayID string) *Registry {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	channelUtil := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "rmesh_node_channel_utilization_ratio",
		Help: "Observed channel utilization for a mesh node (0-1; Meshtastic reports percent).",
	}, []string{"agent_id", "gateway_id", "node_id", "source"})

	airUtilTx := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "rmesh_node_air_util_tx_ratio",
		Help: "Observed TX airtime utilization for a mesh node (0-1; Meshtastic reports percent).",
	}, []string{"agent_id", "gateway_id", "node_id", "source"})

	updatedAt := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "rmesh_node_metrics_updated_timestamp_seconds",
		Help: "Unix time when node RF metrics were last updated.",
	}, []string{"agent_id", "gateway_id", "node_id"})

	reg.MustRegister(channelUtil, airUtilTx, updatedAt)

	return &Registry{
		prom:        reg,
		channelUtil: channelUtil,
		airUtilTx:   airUtilTx,
		updatedAt:   updatedAt,
		agentID:     agentID,
		gatewayID:   gatewayID,
	}
}

// Prometheus returns the underlying registry for the HTTP handler.
func (r *Registry) Prometheus() *prometheus.Registry {
	return r.prom
}
