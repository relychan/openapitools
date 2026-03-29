# CLAUDE.md

## Project Overview

`github.com/relychan/openapitools` — a Go library providing OpenAPI-aware HTTP proxy client capabilities. Two main packages:

- **`oaschema`** — parses OpenAPI 3.0 / Swagger 2.0 resource definitions, handles JSON/YAML unmarshaling, converts Swagger 2.0 → OpenAPI 3.0, and merges spec overlays.
- **`openapiclient`** — builds a proxy client from an `oaschema.OpenAPIResourceDefinition`: HTTP client, weighted round-robin load balancer, trie-based route matching, and per-operation handlers (REST, GraphQL).

The `jsonschema/generator/` subdirectory is a **separate Go module** (`go.mod`) that generates `jsonschema/openapitools.schema.json` from the `oaschema` Go types.

## Development Commands

```bash
make test          # go test -race -timeout 3m -coverpkg=./... ./...
make lint          # golangci-lint run
make lint-fix      # golangci-lint run --fix
make format        # gofmt -w -s .
make build-jsonschema  # regenerate jsonschema/openapitools.schema.json
```

## Code Style & Linting

Configured in `.golangci.yml` — all linters enabled by default with these key settings:

- **Line length**: 200 characters max (lll)
- **Cyclomatic complexity**: 20 max (cyclop)
- **Cognitive complexity**: 40 min (gocognit)
- **Function length**: 140 lines / 50 statements (funlen)
- **Nesting complexity**: 10 min (nestif)
- **Max public structs per package**: 10 (revive)

Disabled linters: `varnamelen`, `exhaustruct`, `wrapcheck`, `wsl`, `mnd`, `gochecknoglobals`, `nilnil`, `contextcheck`, `tagalign`, `tagliatelle`, `ireturn`, `recvcheck`.

Formatters in use: `gci`, `gofmt`, `gofumpt`, `goimports`, `golines`.

Banned packages: `math/rand` (use `math/rand/v2`), `github.com/pkg/errors` (use stdlib `errors`).

Test files (`*_test.go`) are excluded from linter checks.

## Architecture Notes

### Route matching

`openapiclient/internal/tree.go` implements a trie with four node types: static, regexp, parameter (`:name`), and catch-all (`*`). `metadata.go` populates the tree from OpenAPI path/method definitions at client init time.

### Handler dispatch

Each OpenAPI operation gets exactly one handler, selected by the `x-rely-proxy` extension type field:
- No extension → REST handler (`resthandler`)
- `type: graphql` → GraphQL handler (`graphqlhandler`)

### OpenAPI extensions

| Extension | Scope | Purpose |
|---|---|---|
| `x-rely-server-weight` | server object | Weighted round-robin weight |
| `x-rely-server-headers` | server object | Per-server injected headers |
| `x-rely-proxy` | operation object | Handler config (REST params, GraphQL query/variables) |

### Error handling

All errors returned from `Execute` are [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457) `goutils.RFC9457Error` values. Internal errors use `fmt.Errorf("context: %w", err)`.

### Observability

OpenTelemetry tracing (`go.opentelemetry.io/otel`) is integrated at the `Execute` level and within handlers. Span names: `"Proxy"` (execute), handler spans inherit from context. Structured logging via `log/slog` with `gotel.GetLogger(ctx)`.
