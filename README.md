# rmesh — RelayMesh CLI

Public, Apache-2.0 CLI for RelayMesh edge services. The **agent** subcommand connects to a local Meshtastic node via the **Phone API** and forwards traffic to RelayMesh over **MQTT only** — the same ingest path firmware gateways use.

```
Phone API (USB / BLE / TCP)  →  rmesh agent  →  MQTT  →  RelayMesh backend
```

## Quick start

```bash
make build
# Default config: ~/.rmesh/config.yaml (macOS) or /etc/rmesh/config.yaml (Linux)
mkdir -p ~/.rmesh && cp config.example.yaml ~/.rmesh/config.yaml   # macOS
rmesh config edit
rmesh auth login
rmesh auth status   # verify session (gh-style)
rmesh network list  # list accessible networks (-o table|json|yaml|id)
rmesh network use <slug>   # set default for traffic commands
rmesh traffic text         # text messages on default network

make doctor    # rmesh agent doctor
make observe   # dry-run JSONL
make run       # publish to MQTT
```

### Local development

With `relaymesh-backend` docker-compose running (API `:8090`, Kratos `:4433`), set in your shell:

```bash
export RMESH_API_URL=http://localhost:8090
export RMESH_AUTH_URL=http://localhost:4433
```

Or install globally: `make install`

### Shell tab completion (zsh)

Add to the end of `~/.zshrc`:

```bash
eval "$(rmesh completion zsh)"
```

Then `source ~/.zshrc`. Re-run after upgrading rmesh so new subcommands and network autocomplete are registered.

## What it does

1. **Passthrough** — `FromRadio` mesh packets are wrapped in standard `ServiceEnvelope` protobufs and published unchanged (`from` / `to` preserved).
2. **NodeDB synthesis** — periodic `NODEINFO`, `POSITION`, and `MAP_REPORT` envelopes derived from local NodeDB so ghost nodes gain identity in ClickHouse.
3. **Provenance** — MQTT 5 user properties per `D-45`:
   - `relaymesh_ingest_source`: `edge:{agent_id}` or `edge:{agent_id}:nodedb`
   - `relaymesh_labels`: operator JSON label map

## Spec

Design decisions and cloud-side contract: [`agent-specifications/projects/RelayMesh-Edge`](https://github.com/relaymonkey/agent-specifications/tree/main/projects/RelayMesh-Edge).

Configuration reference: [`docs/configure.md`](docs/configure.md).

## License

Apache-2.0 — see [LICENSE](LICENSE).
