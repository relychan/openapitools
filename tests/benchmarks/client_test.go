package benchmarks

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
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
// BenchmarkProxyClient/http_client_get-11         	   27229	     46687 ns/op	    9899 B/op	     124 allocs/op
// BenchmarkProxyClient/proxy_rest_get-11          	   25870	     46087 ns/op	   12178 B/op	     153 allocs/op
// BenchmarkProxyClient/http_client_graphql-11     	   23952	     50106 ns/op	   12244 B/op	     157 allocs/op
// BenchmarkProxyClient/proxy_client_graphql_get-11    22506	     53113 ns/op	   16910 B/op	     210 allocs/op
// BenchmarkProxyClient/proxy_client_graphql_post-11   21350	     55748 ns/op	   17721 B/op	     220 allocs/op
func BenchmarkProxyClient(b *testing.B) {
	server := startMockServer()
	defer server.Close()

	b.Setenv("SERVER_URL", server.URL)

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

		b.ResetTimer()

		for b.Loop() {
			res, err := c.R(http.MethodGet, server.URL+"/mock").Execute(context.TODO())
			if err != nil {
				panic(err)
			}
			goutils.CloseResponse(res)
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

		b.ResetTimer()

		for b.Loop() {
			request := c.R(http.MethodPost, server.URL+"/graphql")

			request.SetBody(bytes.NewBuffer(bodyBytes))

			res, err := request.Execute(context.TODO())
			if err != nil {
				panic(err)
			}
			goutils.CloseResponse(res)
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

func startMockServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/mock", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusOK)
		case http.MethodPost:
			w.WriteHeader(http.StatusOK)

			_, err := io.Copy(w, r.Body)
			if err != nil {
				slog.Error(err.Error())
			}
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/graphql", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost, http.MethodGet:
			body := io.NopCloser(bytes.NewBufferString(`{"data":{"users":[{"id":1}]}}`))

			w.WriteHeader(http.StatusOK)

			_, err := io.Copy(w, body)
			if err != nil {
				slog.Error(err.Error())
			}
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	return httptest.NewServer(mux)
}
