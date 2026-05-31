# rmesh — RelayMesh CLI

A command-line tool for [RelayMesh](https://mesh.relaymonkey.com) — authenticate, manage networks, inspect traffic and operate local nodes from the terminal.

## Install

**macOS** — [Homebrew](https://brew.sh):

```bash
brew install --cask relaymonkey/tap/rmesh
```

**macOS or Linux** — install script:

```bash
curl -fsSL https://raw.githubusercontent.com/relaymonkey/rmesh-cli/main/scripts/install.sh | bash
```

**Windows** (PowerShell):

```powershell
irm https://raw.githubusercontent.com/relaymonkey/rmesh-cli/main/scripts/install.ps1 | iex
```

Manual downloads: [GitHub Releases](https://github.com/relaymonkey/rmesh-cli/releases).

## Quick start

```bash
rmesh auth login
rmesh network list
rmesh network use <id>
rmesh traffic text
```

## What it does

**Cloud (RelayMesh API)**

- Sign in and check session state (`auth login`, `auth status`, `auth logout`)
- List networks and set which one commands target by default (`network list`, `network use`)
- Query historical traffic with the same filters and column ids as the web UI (`traffic list`, `traffic text`)
- Stream live traffic over WebSocket (`traffic live`)
- List, read, copy, edit and promote saved device configs — personal library and per-network templates (`device config`)

**Local node (agent)**

- Connect a radio over serial, HTTP, or BLE and forward packets to RelayMesh, honouring each sender's `ok_to_mqtt` consent by default (`agent run`)
- Dry-run ingest as JSONL without publishing (`agent observe`)
- Validate config, transport, and node database connectivity (`agent doctor`)

**Local device**

- Read and write the full radio configuration surface (settings, modules, and channels)
- Copy configs between a live device, local files and cloud-saved configs (`device config show`, `copy`, `edit`)

**Shell**

- Tab completion for zsh, bash, fish and PowerShell (`completion`)

## Documentation

- [Configuration and commands](docs/configure.md)
- [Build, local dev and contributing](docs/developing.md)

## License

Apache-2.0 — see [LICENSE](LICENSE).
