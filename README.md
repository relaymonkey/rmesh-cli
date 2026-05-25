# rmesh — RelayMesh edge agent

Public, Apache-2.0 edge daemon that connects to a local Meshtastic node via the **Phone API** and forwards traffic to RelayMesh over **MQTT only** — the same ingest path firmware gateways use.

```
Phone API (USB / BLE / TCP)  →  rmesh  →  EMQX  →  relaymesh-backend/cmd/ingest
```

## Quick start

```bash
make build
cp config.example.yaml /etc/rmesh/config.yaml
# Fill mqtt.* from RelayMesh → Credentials

make doctor
make observe   # dry-run JSONL, no broker publish
make run       # production
```

Or install globally: `make install`

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
