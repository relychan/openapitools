package benchmarks

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/relychan/gohttpc"
	"github.com/relychan/goutils"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/openapiclient"
)

// goos: darwin
// goarch: arm64
// pkg: github.com/relychan/openapitools/tests/benchmarks
// cpu: Apple M3 Pro
// BenchmarkProxyClient/http_client_get-11         	   28222	     39528 ns/op	    8316 B/op	     108 allocs/op
// BenchmarkProxyClient/proxy_rest_get-11          	   30286	     39540 ns/op	    9984 B/op	     136 allocs/op
// BenchmarkProxyClient/http_client_graphql-11     	    4489	    266435 ns/op	   24366 B/op	     219 allocs/op
// BenchmarkProxyClient/proxy_client_graphql_get-11    26367	     45222 ns/op	   14881 B/op	     194 allocs/op
// BenchmarkProxyClient/proxy_client_graphql_post-11   25732	     46617 ns/op	   15218 B/op	     198 allocs/op
func BenchmarkProxyClient(b *testing.B) {
	// Start server in a different process
	// cd ./tests/benchmarks/server && go run .

	oasDef, err := goutils.ReadJSONOrYAMLFile[oaschema.OpenAPIResourceDefinition](context.Background(), "./openapi.yaml")
	if err != nil {
		panic(err)
	}

	client, err := openapiclient.NewProxyClient(context.TODO(), oasDef)
	if err != nil {
		panic(err)
	}

	b.Run("http_client_get", func(b *testing.B) {
		c := gohttpc.NewClient()

		for b.Loop() {
			res, err := c.R(http.MethodGet, "http://localhost:8080/mock").Execute(context.TODO())
			if err != nil {
				panic(err)
			}
			_ = res.Body.Close()
		}
	})

	b.Run("proxy_rest_get", func(b *testing.B) {
		for b.Loop() {
			_, _, err := client.Execute(context.Background(), http.MethodGet, "/mock", nil, nil)
			if err != nil {
				panic(err)
			}
		}
	})

	b.Run("http_client_graphql", func(b *testing.B) {
		c := gohttpc.NewClient()
		bodyBytes, err := json.Marshal(map[string]any{
			"query": "query GetUsers { users { id }}",
		})
		if err != nil {
			panic(err)
		}

		for b.Loop() {
			request := c.R(http.MethodPost, "http://localhost:8080/graphql")

			request.SetBody(bytes.NewBuffer(bodyBytes))

			res, err := request.Execute(context.TODO())
			if err != nil {
				panic(err)
			}
			_ = res.Body.Close()
		}
	})

	b.Run("proxy_client_graphql_get", func(b *testing.B) {
		for b.Loop() {
			_, _, err := client.Execute(context.Background(), http.MethodGet, "/users", nil, nil)
			if err != nil {
				panic(err)
			}
		}
	})

	b.Run("proxy_client_graphql_post", func(b *testing.B) {
		for b.Loop() {
			_, _, err := client.Execute(context.Background(), http.MethodPost, "/users", nil, nil)
			if err != nil {
				panic(err)
			}
		}
	})
}
