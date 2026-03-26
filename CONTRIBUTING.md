# Development

## Packages

### `oaschema`

Parses and validates OpenAPI resource definitions. Supports:

- **OpenAPI 3.0** documents (direct or by reference via `ref`)
- **Swagger 2.0** documents (auto-converted to OpenAPI 3.0)
- Merging an inline `spec` overlay on top of a referenced document
- JSON and YAML unmarshaling

The core type is `OpenAPIResourceDefinition`:

```go
type OpenAPIResourceDefinition struct {
    Settings *OpenAPIResourceSettings
    Ref      string           // path or URL to an OpenAPI document
    Spec     *highv3.Document // inline spec, merged over Ref if both are set
}
```

Call `Build(ctx)` to load, convert (if Swagger 2.0), and merge the spec.

#### Settings

`OpenAPIResourceSettings` controls runtime proxy behaviour:

| Field | Description |
|---|---|
| `expose` | Whether to expose this API (default `true`) |
| `basePath` | Strip this prefix from incoming request paths |
| `http` | HTTP client configuration (timeouts, TLS, etc.) |
| `headers` | Static headers injected into every upstream request; values support env-var substitution |
| `forwardHeaders` | Lists of request/response header names to forward |
| `healthCheck` | HTTP health check policy for load-balancer recovery |

### `openapiclient`

Builds and runs an HTTP proxy client from an `OpenAPIResourceDefinition`.

```go
client, err := openapiclient.NewProxyClient(ctx, metadata, clientOptions)
// ...
resp, body, err := client.Execute(ctx, httpRequest)
```

Internally it:
1. Builds the OpenAPI spec via `oaschema`.
2. Creates an HTTP client from `Settings.HTTP` (or uses the one in `clientOptions`).
3. Registers all `spec.servers` as weighted round-robin hosts with optional health checks.
4. Builds a trie-based route tree from the OpenAPI path/method definitions.
5. On each `Execute` call: strips `basePath`, matches the route, delegates to the appropriate handler, and wraps errors as [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457) problem details.

#### Server extensions

Custom x- fields on OpenAPI server objects:

| Extension | Type | Description |
|---|---|---|
| `x-rely-server-weight` | `int` | Weighted round-robin weight (>1 to take effect) |
| `x-rely-server-headers` | `map[string]string` | Per-server headers with env-var substitution |

### Handlers

Each OpenAPI operation resolves to one of:

#### REST handler (`resthandler`)

Translates path, query, header, and cookie parameters plus request body into an outgoing HTTP request. Supports full OpenAPI content-type encoding:

- JSON, YAML, XML
- `multipart/form-data`, `application/x-www-form-urlencoded`
- Plain text, binary, data URI

Response bodies are decoded symmetrically.

#### GraphQL handler (`graphqlhandler`)

Maps an incoming REST-style request to a GraphQL operation. Configured via an `x-rely-proxy` extension on the operation:

```yaml
x-rely-proxy:
  request:
    query: |
      query GetUser($id: ID!) { user(id: $id) { name email } }
    variables:
      id: "$.params.userId"
    headers:
      X-Tenant: "$.headers.x-tenant"
  response:
    httpErrorCode: 400   # map GraphQL errors to this HTTP status
    body: "$.data.user"  # JMESPATH expression to reshape the response
```

Variables are resolved in priority order: explicit mapping → path/query params → GraphQL default.


### JSON Schema generation

`jsonschema/generator/` is a standalone Go module that generates `jsonschema/openapitools.schema.json` from the `oaschema` Go types. Run it via:

```bash
make build-jsonschema
# or directly:
./jsonschema/generator/build.sh
```

The build script creates a temporary Go workspace to link the generator module against the root module, runs the generator, then removes the workspace files.
