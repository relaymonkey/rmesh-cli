# Prometheus metrics

`rmesh agent` can expose a local Prometheus `/metrics` endpoint with per-node RF gauges — channel utilization and transmit airtime — scraped into your own Prometheus or Grafana stack.

The endpoint is **optional and off by default**. It complements RelayMesh cloud analytics; it does not replace them.

## Quick start

```bash
rmesh agent run --metrics-enabled
# or dry-run without MQTT publish:
rmesh agent observe --metrics-enabled
```

Verify the endpoint (default bind is loopback only):

```bash
curl -s http://127.0.0.1:19092/metrics | grep rmesh_node
```

Enable persistently in `config.yaml`:

```yaml
metrics:
  enabled: true
  # listen_addr: "127.0.0.1:19092"   # default
  # nodedb_refresh_interval: 0       # inherit synthesise.nodedb_poll (5m)
```

CLI overrides:

```bash
rmesh agent run --metrics-enabled
rmesh agent run --metrics-enabled --metrics-listen-addr 0.0.0.0:19092
rmesh agent run --metrics-enabled --metrics-nodedb-refresh-interval 30s
```

## Configuration

| Key | Default | Description |
|-----|---------|-------------|
| `metrics.enabled` | `false` | Start the HTTP `/metrics` server |
| `metrics.listen_addr` | `127.0.0.1:19092` | TCP bind address |
| `metrics.nodedb_refresh_interval` | `0` | NodeDB gauge refresh; `0` inherits `synthesise.nodedb_poll` |

**Listen address**

- **Default `127.0.0.1:19092`** — reachable only on the host running `rmesh`. Use this when Prometheus scrapes locally.
- **Port `19092`** — chosen to avoid common conflicts (for example Docker Desktop on `:9092` and other services on nearby ports).
- **Remote scraper** — set `metrics.listen_addr` to `0.0.0.0:19092` or the host's LAN address, and restrict access with a host firewall. The endpoint has **no authentication** (see [Security](#security)).

See [`configure.md`](configure.md) for transport, MQTT, synthesis, and other agent settings.

## How gauges are updated

Two inputs feed the same series:

| Source | Trigger | `source` label |
|--------|---------|----------------|
| **Telemetry** (Meshtastic port 67) | Each heard telemetry packet | `telemetry` |
| **NodeDB `device_metrics`** | Live node-info frames and scheduled NodeDB refresh | `nodedb` |

Telemetry is **event-driven**. NodeDB refresh uses `metrics.nodedb_refresh_interval`:

- **`0` (default)** — inherit `synthesise.nodedb_poll` (default `5m`).
- **Explicit value** — minimum `10s`. Shorter intervals (for example `30s`) refresh Grafana panels without changing MQTT synthesis cadence.

When the metrics interval is **shorter** than `synthesise.nodedb_poll`, the agent performs extra NodeDB reads on the metrics tick. When it is **equal or longer**, the tick republishes from the in-memory NodeDB without an extra radio read.

Telemetry **overwrites** NodeDB values for the same `node_id` (prefer `source="telemetry"` in queries when both exist).

Gauges update even when passthrough is dropped for `ok_to_mqtt` — local observability is independent of MQTT publish policy.

## Metric reference

| Series | Labels | Value |
|--------|--------|-------|
| `rmesh_node_channel_utilization_ratio` | `agent_id`, `gateway_id`, `node_id`, `source` | 0–1 (Meshtastic reports percent ÷ 100) |
| `rmesh_node_air_util_tx_ratio` | same | 0–1 |
| `rmesh_node_metrics_updated_timestamp_seconds` | `agent_id`, `gateway_id`, `node_id` | Unix time of last update |

Standard Go process collectors (`go_*`, `process_*`) are included on the same endpoint.

### Channel utilization vs transmit airtime

Meshtastic distinguishes these; do not treat them as interchangeable:

- **`channel_utilization`** — RF activity on the **channel** (includes traffic the node cannot decode). Use this for congestion monitoring on busy meshes.
- **`air_util_tx`** — this node's **transmit** duty cycle (regulatory limits apply, for example ~10% in EU868).

In Grafana, show percent as `rmesh_node_channel_utilization_ratio * 100`.

## Prometheus scrape

Example job when Prometheus runs on another host (agent bound to all interfaces):

```yaml
scrape_configs:
  - job_name: rmesh
    scrape_interval: 30s
    static_configs:
      - targets: ["192.168.1.50:19092"]
```

When Prometheus runs on the same machine, scrape `127.0.0.1:19092`.

## Grafana

Channel utilization (percent) from live telemetry:

```promql
rmesh_node_channel_utilization_ratio{source="telemetry"} * 100
```

Stale series (no update in 5 minutes):

```promql
time() - rmesh_node_metrics_updated_timestamp_seconds > 300
```

## Security

`/metrics` is unauthenticated. Bind `127.0.0.1` (default) when the scraper is co-located. For LAN scraping, use a firewall rule that allows only your Prometheus host.

## Cardinality and staleness

One label set per heard `node_id`. Series are not removed automatically when a node goes quiet — use `rmesh_node_metrics_updated_timestamp_seconds` for staleness alerts. Practical ceiling is on the order of hundreds of nodes per agent.

## Coexistence with MQTT

- `/metrics` is read-only; MQTT forwarding is unchanged.
- Works with **`agent run`** (cloud publish) and **`agent observe`** (JSONL only, no MQTT).

## Troubleshooting

| Symptom | Likely cause |
|---------|----------------|
| `connection refused` | Metrics not enabled, or agent not running |
| `curl: (52) Empty reply from server` | Another process owns the port; check `lsof -i :19092` and set a different `metrics.listen_addr` |
| HTTP 200 but no `rmesh_node_*` lines | No node has reported `device_metrics` yet — wait for telemetry (port 67) or NodeDB refresh |

On bind failure the agent logs `metrics server stopped` at error level with the underlying `address already in use` message.
