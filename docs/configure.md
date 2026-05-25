# rmesh configuration reference

See [`config.example.yaml`](../config.example.yaml) for a annotated starting point.

## Environment

| Variable | Default | Purpose |
|---|---|---|
| `RMESH_CONFIG` | platform default (see below) | Agent config file path (also `--config`) |
| `RMESH_API_URL` | `https://mesh.relaymonkey.com` | RelayMesh REST API origin (`/api/v1/...`) |
| `RMESH_AUTH_URL` | `https://auth.relaymonkey.com` | Ory Kratos public URL for `rmesh auth login` |
| `RMESH_SESSION_FILE` | `~/.rmesh/session.json` | Saved CLI session (mode `0600`; override path) |
| `RMESH_DEFAULT_NETWORK_FILE` | `~/.rmesh/default-network.json` | Default network for cloud commands |
| `RMESH_STREAM_URL` | same as `RMESH_API_URL` (`:8090` → `:8091`) | streamd WebSocket origin for `traffic live`. streamd must accept `X-Session-Token` (rebuild relaymesh-backend if live returns 401). |

Default paths when env vars are unset:

| File | macOS | Linux / other |
|---|---|---|
| Agent config | `~/.rmesh/config.yaml` | `/etc/rmesh/config.yaml` |
| CLI session | `~/.rmesh/session.json` | `~/.rmesh/session.json` |
| Default network | `~/.rmesh/default-network.json` | `~/.rmesh/default-network.json` |

| `EDITOR` / `VISUAL` | `nano` | Used by `rmesh config edit` / `rmesh config -e` |

Local dev — set in your shell before using `rmesh`:

```bash
export RMESH_API_URL=http://localhost:8090
export RMESH_AUTH_URL=http://localhost:4433
```

## Shell tab completion (zsh)

Add to the end of `~/.zshrc`:

```bash
eval "$(rmesh completion zsh)"
```

Then `source ~/.zshrc`. Re-run after upgrading rmesh. Dynamic completion (`network use`, `--network`) requires `rmesh auth login`.

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
rmesh config edit       # open config in $EDITOR
rmesh config -e         # same (git-style shorthand)
rmesh config edit --config ../config.example.yaml

rmesh auth login        # sign in (prompts for email/password)
rmesh auth status       # verify session (like gh auth status)
rmesh auth logout

Native login stores a Kratos **session token** (`ory_st_*`) at `~/.rmesh/session.json`.
The CLI sends it as `X-Session-Token` on API calls (not a browser cookie).

rmesh network list      # networks you can access (alias: rmesh networks list)
rmesh network use ID    # set default network (UUID, slug, short_id, or name)
rmesh network current   # show default network
rmesh network list -o json
rmesh network list -o id  # one UUID per line (scripting)

rmesh traffic list              # historical traffic (default network, limit 100)
rmesh traffic list --filter packet_type:eq:1
rmesh traffic list --fields ingest_ts,source_node_id,packet_type,id
rmesh traffic text              # adds filter packet_type:eq:1
rmesh traffic text --fields ingest_ts,source_node_id,dest_node_id,decoded.value.text
rmesh traffic list --filter source_node_id:eq:!abc123
rmesh traffic list --node-filter node.long_name:contains:EU
rmesh traffic list --from 2026-05-25T00:00:00Z --to 2026-05-25T23:59:59Z
rmesh traffic list --q "hello mesh"
rmesh traffic live              # WebSocket stream (Ctrl+C to stop)
rmesh traffic text live         # live text messages only
rmesh traffic live --text       # same as traffic text live

# Override default network on any command:
rmesh traffic text --network my-network-slug

# --fields and --filter use the same ids as the Traffic UI (no CLI aliases).
# --fields accepts any column id; values are read from the envelope JSON by path.
# Only "summary" is computed client-side (same as the UI Summary column).

# Legacy alias (still works):
rmesh messages text

rmesh agent doctor      # validate config + USB/node connectivity
rmesh agent observe     # JSONL dry-run, no MQTT
rmesh agent run         # production publish
rmesh agent pair        # requires auth; cloud pairing API stub
```

Spec: [RelayMesh-Edge README](https://github.com/relaymonkey/agent-specifications/tree/main/projects/RelayMesh-Edge)
