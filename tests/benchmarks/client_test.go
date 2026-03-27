package benchmarks

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/relychan/gohttpc"
	"github.com/relychan/goutils"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/openapiclient"
	"github.com/relychan/openapitools/openapiclient/handler/proxyhandler"
)

// goos: darwin
// goarch: arm64
// pkg: github.com/relychan/openapitools/tests/benchmarks
// cpu: Apple M3 Pro
// BenchmarkProxyClient/http_client_get-11         	   20493	     54281 ns/op	    9905 B/op	     119 allocs/op
// BenchmarkProxyClient/rest_get-11             	   22260	     53863 ns/op	   12265 B/op	     153 allocs/op
// BenchmarkProxyClient/raw_http_graphql-11            15169	     66341 ns/op	   12068 B/op	     146 allocs/op
// BenchmarkProxyClient/graphql-11              	   19612	     61281 ns/op	   17273 B/op	     218 allocs/op
func BenchmarkProxyClient(b *testing.B) {
	// Start server in a different process
	// go run ./tests/benchmarks/server

	oasDef, err := goutils.ReadJSONOrYAMLFile[oaschema.OpenAPIResourceDefinition](context.Background(), "./openapi.yaml")
	if err != nil {
		panic(err)
	}

	client, err := openapiclient.NewProxyClient(context.TODO(), oasDef, nil)
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
			_, _, err := client.Execute(context.Background(), &proxyhandler.Request{
				Method: http.MethodGet,
				URL: &url.URL{
					Path: "/mock",
				},
			})
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

	b.Run("proxy_client_graphql", func(b *testing.B) {
		for b.Loop() {
			_, _, err := client.Execute(context.Background(), &proxyhandler.Request{
				Method: http.MethodGet,
				URL: &url.URL{
					Path: "/users",
				},
			})
			if err != nil {
				panic(err)
			}
		}
	})
}
