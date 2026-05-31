# rmesh configuration reference

See [`config.example.yaml`](../config.example.yaml) for a annotated starting point.

## Install

Prebuilt binaries ship for:

| OS | Architectures | Install |
|---|---|---|
| **macOS** | Intel (amd64) and Apple Silicon (arm64) | [Homebrew](#homebrew-macos) or [install script](#install-script-macos-linux) |
| **Linux** | amd64, arm64, armv7 (Pi Zero W2, etc.) | [Install script](#install-script-macos-linux) |
| **Windows** | amd64 | [PowerShell script](#windows) |

[GitHub Releases](https://github.com/relaymonkey/rmesh-cli/releases) lists every published archive if you prefer a manual download.

### Install script (macOS, Linux)

Downloads the matching release asset and installs `rmesh` to `/usr/local/bin` (uses `sudo` when needed) or `~/.local/bin`:

```bash
curl -fsSL https://raw.githubusercontent.com/relaymonkey/rmesh-cli/main/scripts/install.sh | bash
```

Pin a version or install directory:

```bash
RMESH_VERSION=v1.0.1 bash -c "$(curl -fsSL https://raw.githubusercontent.com/relaymonkey/rmesh-cli/main/scripts/install.sh)"
RMESH_INSTALL_DIR="$HOME/.local/bin" bash -c "$(curl -fsSL https://raw.githubusercontent.com/relaymonkey/rmesh-cli/main/scripts/install.sh)"
```

Script source: [`scripts/install.sh`](../scripts/install.sh).

### Homebrew (macOS)

[Homebrew](https://brew.sh) on macOS — installs the Intel or Apple Silicon build automatically:

```bash
brew install --cask relaymonkey/tap/rmesh
```

Tap repo: [`relaymonkey/homebrew-tap`](https://github.com/relaymonkey/homebrew-tap).

### Windows

PowerShell — installs to `%LOCALAPPDATA%\Programs\rmesh` and adds it to your user `Path`:

```powershell
irm https://raw.githubusercontent.com/relaymonkey/rmesh-cli/main/scripts/install.ps1 | iex
```

Pin a version:

```powershell
$env:RMESH_VERSION = "v1.0.1"; irm https://raw.githubusercontent.com/relaymonkey/rmesh-cli/main/scripts/install.ps1 | iex
```

Script source: [`scripts/install.ps1`](../scripts/install.ps1).

## Environment

| Variable | Default | Purpose |
|---|---|---|
| `RMESH_CONFIG` | platform default (see below) | `rmesh agent` config file path (also `--config`) |
| `RMESH_API_URL` | `https://mesh.relaymonkey.com` | RelayMesh REST API origin (`/api/v1/...`) |
| `RMESH_AUTH_URL` | `https://auth.relaymonkey.com` | RelayMesh auth URL for `rmesh auth login` |
| `RMESH_SESSION_FILE` | `~/.rmesh/session.json` | Saved CLI session (mode `0600`; override path) |
| `RMESH_DEFAULT_NETWORK_FILE` | `~/.rmesh/default-network.json` | Default network for cloud commands |
| `RMESH_STREAM_URL` | same as `RMESH_API_URL` (`:8090` → `:8091`) | WebSocket origin for `traffic live` (local dev often uses a separate port) |

Default paths when env vars are unset:

| File | macOS | Linux / other |
|---|---|---|
| `rmesh agent` config | `~/.rmesh/config.yaml` | `/etc/rmesh/config.yaml` |
| CLI session | `~/.rmesh/session.json` | `~/.rmesh/session.json` |
| Default network | `~/.rmesh/default-network.json` | `~/.rmesh/default-network.json` |

| `EDITOR` / `VISUAL` | `nano` | Used by `rmesh config edit` / `rmesh config -e` |

Local dev — see [developing.md](developing.md).

## Required fields

| Field | Description |
|---|---|
| `transport.url` | Local radio connection URL: `serial:/dev/ttyUSB0`, `http://192.168.1.10:4403`, `ble://AA:BB:CC:DD:EE:FF` |
| `mqtt.broker_url` | RelayMesh MQTT broker (`mqtt://host:1883`) |
| `mqtt.username` / `mqtt.password` | Scoped credential from RelayMesh UI |
| `mqtt.topic_prefix` | From credential issuance (`rm/n/<short_id>`) |

## Labels

Free-form string map stamped on every publish via MQTT 5 user property `relaymesh_labels`. Keys prefixed `relaymesh.` are reserved for cloud metadata.

## Forwarding (ok_to_mqtt consent)

Meshtastic packets can carry a per-packet consent bit (`ok_to_mqtt`, bit 0 of the decoded `Data.bitfield`) that signals whether the sender is willing to have the packet uploaded to MQTT. Modern firmware stamps this bit on every packet it originates, driven by the device's `config.lora.config_ok_to_mqtt` setting.

By default `rmesh agent` **honours that consent**: a passthrough packet whose sender explicitly cleared the bit is dropped (not published). Packets with no decoded bitfield — older firmware, relayed traffic, or packets this gateway cannot decrypt — make no statement and are forwarded. Synthetic NodeDB traffic (`nodeinfo` / `position` / `mapreport`) is the operator's own derived data and is never filtered.

| Field | Default | Description |
|---|---|---|
| `forward.respect_ok_to_mqtt` | `true` | Drop passthrough packets whose sender cleared the `ok_to_mqtt` bit |

Forward everything regardless of the sender's preference (the pre-`D-232` behaviour):

```bash
rmesh agent run --ignore-ok-to-mqtt
# or persist it in config.yaml:
rmesh agent run --set forward.respect_ok_to_mqtt=false
```

`--ignore-ok-to-mqtt` is a presence flag; it overrides `forward.respect_ok_to_mqtt` from the config file for that run.

> **Note:** Meshtastic's `config.lora.config_ok_to_mqtt` defaults to **off**, so a device left on factory defaults clears the bit on its own packets — those packets are filtered while the default policy is on. Either enable `config_ok_to_mqtt` on the originating devices, or run with `--ignore-ok-to-mqtt`, to forward that traffic. Use `rmesh agent observe` to preview what would be dropped: filtered packets appear with a `"dropped":"ok_to_mqtt"` field in the JSONL.

## Synthesis cadence

`rmesh agent` synthesises **nodeinfo**, **position**, and **map report** traffic from the local node database for kinds the cloud cannot infer from RF-only ghosts.

Each kind supports `interval`, `on_first_seen` and `jitter`. Content-hash diffing avoids re-emitting unchanged rows; `--reset-cadence` clears timers.

Synthetic rows carry ingest source `edge:{agent_id}:nodedb` (passthrough uses `edge:{agent_id}`).

## Overriding config from the CLI

Every `rmesh agent` verb that reads `config.yaml` exposes only the override flags it actually consumes — so `rmesh agent observe --help` doesn't list `--mqtt-*` (observe never publishes), and `rmesh agent pair --help` doesn't list radio flags (it talks to the cloud API).

For one-off tweaks or for keys without a dedicated flag, every config-reading verb accepts `--set path=value` (repeatable). The path is the dotted yaml key:

```bash
rmesh agent observe --set synthesise.position.interval=12m
rmesh agent run     --set mqtt.broker_url=ssl://broker.example:8883 --set labels.site=oslo
rmesh agent doctor  --set transport.url=ble://AA:BB:CC:DD:EE:FF
```

Typed flags (`--mqtt-broker-url …`) win over `--set` when both target the same key. Unknown paths are hard errors, so typos don't silently no-op.

## Commands

```bash
# Agent config file (~/.rmesh/config.yaml or /etc/rmesh/config.yaml)
rmesh config edit       # open config in $EDITOR
rmesh config -e         # same (git-style shorthand)
rmesh config edit --config ../config.example.yaml

# Cloud session
rmesh auth login        # sign in (prompts for email/password)
rmesh auth status       # verify session (like gh auth status)
rmesh auth logout
# Session is saved at ~/.rmesh/session.json after login.

# Networks (requires auth)
rmesh network list      # networks you can access (alias: rmesh networks list)
rmesh network use <id>  # set default network (UUID from network list)
rmesh network current   # show default network
rmesh network list -o json
rmesh network list -o id  # one UUID per line (scripting)

# Traffic (requires auth; filters/columns match the web UI — see traffic-columns.md)
rmesh traffic list              # historical traffic (default network, limit 100)
rmesh traffic list --filter packet_type:eq:1
rmesh traffic list --fields ingest_ts,source_node_id,packet_type,id
rmesh traffic text              # adds filter packet_type:eq:1
rmesh traffic text --fields ingest_ts,source_node_id,dest_node_id,decoded.value.text
rmesh traffic list --filter source_node_id:eq:!abc123
rmesh traffic list --node-filter node.long_name:contains:EU
rmesh traffic list --gateway-filter gateway.id:eq:bridge-1
rmesh traffic list --from 2026-05-25T00:00:00Z --to 2026-05-25T23:59:59Z
rmesh traffic list --q "hello mesh"
rmesh traffic list -o json      # table | json | yaml | id
rmesh traffic live              # WebSocket stream (Ctrl+C to stop)
rmesh traffic text live         # live text messages only
rmesh traffic live --text       # same as traffic text live
rmesh traffic text --network 742a055f-af02-4b99-a510-157ce0c34b9c  # override default network
# Legacy alias: rmesh messages text

# Device configs (device / file / cloud sources)
rmesh device config show --from device                    # read live radio → stdout
rmesh device config show --from cloud:<network>/<label>   # read saved cloud config
rmesh device config copy --from device --to ./backup.yaml # snapshot radio to file
rmesh device config copy --from ./eu-868.yaml --to device # apply file to radio
rmesh device config copy --from cloud:home/eu-868 --to device --dry-run
rmesh device config edit --from device                    # $EDITOR round-trip
rmesh device config list                                  # personal library (cross-network)
rmesh device config list --network <id>                   # templates on a network
rmesh device config promote --from cloud:mine/<label> --to <network>      # $EDITOR preview on TTY
rmesh device config promote --from cloud:mine/<label> --to <network> --no-edit
rmesh device config delete --from cloud:mine/<label>      # --yes to skip prompt
# Sources/destinations:
#   device[:url]              live radio
#   file:<path>               local file
#   cloud:mine/<label>        personal library (user-scoped, no network)
#   cloud:<network>/<label>   network template
#   -                         stdout
# Deprecated aliases: get → show; set → copy; cloud:<n>/mine/<label> → cloud:mine/<label>

# Local agent (local radio → RelayMesh cloud)
rmesh agent doctor      # validate config, transport, and node database connectivity
rmesh agent observe     # JSONL dry-run, no cloud publish
rmesh agent run         # production publish
rmesh agent pair        # requires auth; cloud pairing API stub

# Shell
eval "$(rmesh completion zsh)"   # bash | fish | powershell also supported
```
