# CLAUDE.md

## Project Overview

`github.com/relychan/openapitools` — a Go library providing OpenAPI-aware HTTP proxy client capabilities. Two main packages:

- **`oaschema`** — parses OpenAPI 3.0 / Swagger 2.0 resource definitions, handles JSON/YAML unmarshaling, converts Swagger 2.0 → OpenAPI 3.0, and merges spec overlays.
- **`openapiclient`** — builds a proxy client from an `oaschema.OpenAPIResourceDefinition`: HTTP client, weighted round-robin load balancer, trie-based route matching, and per-operation handlers (REST, GraphQL).

The `jsonschema/generator/` subdirectory is a **separate Go module** (`go.mod`) that generates `jsonschema/openapitools.schema.json` from the `oaschema` Go types.

## Development Commands

```bash
make test          # go test -v -race -timeout 3m -coverpkg=./... -coverprofile=coverage.out ./...
make lint          # golangci-lint run
make lint-fix      # golangci-lint run --fix
make format        # gofmt -w -s .
make build-jsonschema  # regenerate jsonschema/openapitools.schema.json (uses Go workspaces)
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

### Package layout

```
oaschema/                        # OpenAPI schema parsing
  resource.go                    # OpenAPIResourceDefinition (Ref + Spec fields)
  config.go                      # OpenAPIResourceSettings (HTTP config, headers, health check)
  enum.go                        # Extension name constants, security/encoding enums
  error.go                       # Error constants for encoding/decoding stages
  resource_v2.go / resource_v3.go  # Swagger 2.0 / OpenAPI 3.0 specific handling
  utils.go                       # Spec building/merging utilities

openapiclient/
  client.go                      # ProxyClient struct — Execute(), Stream(), Metadata(), Close()
  execute.go                     # Execute() and Stream() with OpenTelemetry tracing
  metadata.go                    # BuildMetadataTree() — populates trie at init time
  option.go                      # ClientOption functional options
  utils.go                       # newRequest(), writeErrorResponse(), parseServerURL()
  types.go                       # Error vars, tracingResponseWriter, tracer
  handler/
    handler.go                   # NewProxyHandler() factory — selects handler by extension type
    proxyhandler/
      handler.go                 # ProxyHandler interface: Type(), Handle(), Stream()
      types.go                   # Request, ProxyHandleOptions, RequestTemplateData, credential types
      security.go                # OpenAPIAuthenticator — manages security schemes
    resthandler/
      handler.go                 # RESTfulHandler implementation
      config.go                  # ProxyRESTfulRequestConfig; ProxyActionTypeREST constant
      request.go / response.go   # Request/response building and transformation
      contenttype/               # Per-content-type encoders/decoders (XML, binary, multipart, URL-encoded, text, data URI)
      parameter/                 # Path, query, header parameter serialization
    graphqlhandler/
      handler.go                 # GraphQLHandler implementation
      config.go                  # GraphQL-specific configuration (ProxyGraphQLActionConfig, ProxyGraphQLRequestConfig, ProxyCustomGraphQLResponseConfig)
      request.go                 # GraphQL request construction
      response.go                # GraphQL response evaluation — JMESPath-based error mapping, HTTP status resolution
      utils.go                   # ValidateGraphQLString, type conversions, error utilities
  internal/
    tree.go                      # Trie-based route matching
    types.go                     # Route, MethodHandler, Node structs; routing error types

jsonschema/generator/            # Separate Go module — generates openapitools.schema.json
```

### Route matching

`openapiclient/internal/tree.go` implements a trie with four node types: static, regexp, parameter (`:name`), and catch-all (`*`). `metadata.go` populates the tree from OpenAPI path/method definitions at client init time.

### Handler dispatch

Each OpenAPI operation gets exactly one handler, selected by the `x-rely-proxy-action` extension type field:
- No extension → REST handler (`resthandler`)
- `type: graphql` → GraphQL handler (`graphqlhandler`)

### Client options

`openapiclient/option.go` provides functional options for `ProxyClient`:
- `WithHTTPClient()`, `WithTimeout()`, `WithRetry()`, `WithAuthenticator()`
- `WithUserAgent()`, `WithGetEnvFunc()`
- `WithTraceHighCardinalityPath()`, `WithMetricHighCardinalityPath()`
- `WithCustomAttributesFunc()`, `EnableClientTrace()`
- `AllowTraceRequestHeaders()`, `AllowTraceResponseHeaders()`

### OpenAPI extensions

| Extension | Scope | Purpose |
|---|---|---|
| `x-rely-server-weight` | server object | Weighted round-robin weight |
| `x-rely-server-headers` | server object | Per-server injected headers |
| `x-rely-proxy-action` | operation object | Handler config (REST params, GraphQL query/variables) |
| `x-rely-server-security-schemes` | server object | Server-level security scheme definitions |
| `x-rely-server-security` | server object | Server-level security requirements |
| `x-rely-server-tls` | server object | TLS configuration |
| `x-rely-security-credentials` | operation object | Per-operation credential overrides |
| `x-rely-oauth2-token-url-env` | security scheme | Env var for OAuth2 token URL |
| `x-rely-oauth2-refresh-url-env` | security scheme | Env var for OAuth2 refresh URL |

### GraphQL handler configuration

`ProxyGraphQLActionConfig` (set via `x-rely-proxy-action` with `type: graphql`) has two sub-configs:

**`ProxyGraphQLRequestConfig`** — upstream request shape:
- `URL` — override request URL
- `Method` — `GET` or `POST` (default: `POST`)
- `Headers` — per-header JMESPath expressions for request transformation
- `Query` — GraphQL query string (required)
- `Variables` — JMESPath-mapped GraphQL variables
- `Extensions` — JMESPath-mapped GraphQL extensions

**`ProxyCustomGraphQLResponseConfig`** — response error mapping:
- `HTTPErrorCode` — default HTTP status to return when GraphQL `errors` field is present (400–599)
- `HTTPErrors` — `map[string][]string`: maps HTTP status code (string key) → one or more JMESPath expressions evaluated against GraphQL error objects; first matching expression wins, rules evaluated in ascending status-code order
- `Body` — optional `TemplateTransformerConfig` for response body transformation

### Error handling

All errors returned from `Execute` are [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457) `goutils.RFC9457Error` values. Internal errors use `fmt.Errorf("context: %w", err)`. `writeErrorResponse()` checks `tracingResponseWriter.BytesWritten()` to avoid writing a response twice.

### Observability

OpenTelemetry tracing (`go.opentelemetry.io/otel`) is integrated at the `Execute` level and within handlers. Span names: `"Proxy"` (execute), handler spans inherit from context. Structured logging via `log/slog` with `gotel.GetLogger(ctx)`.
