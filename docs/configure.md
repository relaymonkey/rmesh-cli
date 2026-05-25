# rmesh configuration reference

See [`config.example.yaml`](../config.example.yaml) for a annotated starting point.

## Required fields

| Field | Description |
|---|---|
| `transport.url` | Phone API URL: `serial:/dev/ttyUSB0`, `http://192.168.1.10:4403`, `ble://MAC` |
| `mqtt.broker_url` | RelayMesh EMQX broker (`mqtt://host:1883`) |
| `mqtt.username` / `mqtt.password` | Scoped credential from RelayMesh UI |
| `mqtt.topic_prefix` | From credential issuance (`rm/n/<short_id>`) |

## Labels (`EDGE-D-02`)

Free-form string map stamped on every publish via MQTT 5 user property `relaymesh_labels`. Keys prefixed `relaymesh.` are reserved for cloud metadata.

## Synthesis cadence (`EDGE-D-08`)

The agent synthesises standard `ServiceEnvelope` packets from local NodeDB for kinds the cloud cannot infer from RF-only ghosts:

- `nodeinfo` — `NODEINFO_APP`
- `position` — `POSITION_APP`
- `mapreport` — `MAP_REPORT_APP`

Each kind supports `interval`, `on_first_seen`, and `jitter`. Content-hash diffing avoids re-emitting unchanged rows; `--reset-cadence` clears timers.

Synthetic rows carry ingest source `edge:{agent_id}:nodedb` (passthrough uses `edge:{agent_id}`).

## Commands

```bash
rmesh doctor    # validate config + USB/node connectivity
rmesh observe   # JSONL dry-run, no MQTT
rmesh run       # production publish
rmesh pair      # future UI pairing (stub)
```

Spec: [RelayMesh-Edge README](https://github.com/relaymonkey/agent-specifications/tree/main/projects/RelayMesh-Edge)
