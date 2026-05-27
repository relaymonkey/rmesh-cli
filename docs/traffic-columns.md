# Traffic column defaults (rmesh ↔ UI)

The `rmesh traffic` commands and the RelayMesh Traffic page share the same **column id** vocabulary: dotted JSON paths on the message envelope (e.g. `ingest_ts`, `decoded.value.text`).

When `--fields` is omitted, rmesh uses fixed default column lists. The web UI uses the same defaults on first visit (before the operator saves a column preference).

## Canonical defaults

### Historical list (`rmesh traffic list`)

| Column id | Notes |
|-----------|--------|
| `ingest_ts` | |
| `source_node_id` | |
| `packet_type` | Packet type numeric id |
| `channel_index` | |
| `payload_size` | |
| `gateway_id` | |
| `summary` | Computed client-side (not in `/messages/fields`) |
| `encrypted` | Computed client-side |

**rmesh:** `internal/climessages.DefaultListFields`  
**Web UI:** Traffic table default columns

### Text list (`rmesh traffic text`)

| Column id |
|-----------|
| `ingest_ts` |
| `source_node_id` |
| `dest_node_id` |
| `decoded.value.text` |

**rmesh:** `internal/climessages.DefaultTextFields`

## When you change defaults

Update **all** of:

1. This file
2. `internal/climessages/fields.go`
3. RelayMesh web UI traffic default columns

The API field catalog (`GET /messages/fields`) drives dynamic decoded columns in the UI; rmesh tab-completion reads it at runtime. Static defaults above are the fallback when no saved preference / no `--fields` flag.

## Related

- Summary column logic mirrors the Traffic UI `summarizeDecoded()` helper.
  **rmesh:** `internal/climessages/summary.go`
- REST shapes rmesh uses: `internal/apiclient/types.go` (aligned with RelayMesh API schema)
- Packet type display rule: Displays packet types with both raw numeric value and readable label.
