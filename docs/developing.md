# Developing rmesh

Build from source, run against a local RelayMesh stack, and use Makefile shortcuts for the `agent` subcommand.

## Build and install

```bash
make build          # bin/rmesh
make install        # $(go env GOPATH)/bin/rmesh
make test           # unit tests
make coverage       # coverage.out + summary
make ci             # tidy + vet + test + build
make update-deps    # go get -u ./...
```

Other targets: `make test-race`, `make coverage-web`, `make fmt`, `make vet`, `make lint`, `make clean`. Run `make help` for the full list.

## Local RelayMesh stack

With `relaymesh-backend` docker-compose running (API `:8090`, Kratos `:4433`):

```bash
export RMESH_API_URL=http://localhost:8090
export RMESH_AUTH_URL=http://localhost:4433
```

`RMESH_STREAM_URL` defaults from `RMESH_API_URL` (`:8090` → `:8091`) for `traffic live`. Rebuild relaymesh-backend if live returns 401.

## Agent subcommand (Makefile shortcuts)

These wrap `rmesh agent` using the platform default config path (override with `make doctor CONFIG=/path/to/config.yaml`):

```bash
make doctor    # rmesh agent doctor
make observe   # dry-run JSONL, no MQTT
```

First-time agent setup: copy [`config.example.yaml`](../config.example.yaml) to the default config path — see [configure.md](configure.md) for platform paths and every config key.

## CLI output conventions

Two layers:

| Layer | Package | When |
|-------|---------|------|
| **Data** | `internal/clioutput` | List/get commands with `-o table\|json\|yaml\|id` — machine-friendly rows |
| **Messages** | `internal/cliui` | Success confirmations, status panels, hints, stream notices |

Human messages use ✓ / ✗ / ● / → on TTY (plain `ok` / `error` / `->` when piped or `NO_COLOR` set). Example:

```
✓ Default network · EU
  id · 742a055f-af02-4b99-a510-157ce0c34b9c
```

## Shell tab completion (zsh)

Add to the end of `~/.zshrc`:

```bash
eval "$(rmesh completion zsh)"
```

Then `source ~/.zshrc`. Re-run after upgrading rmesh. Dynamic completion (`network use`, `--network`) requires `rmesh auth login`.

## Related docs

- [configure.md](configure.md) — config file, env vars, and command reference
- [traffic-columns.md](traffic-columns.md) — Traffic default columns shared with the web UI
