// Copyright 2026 RelyChan Pte. Ltd
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package openapiclient

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hasura/gotel/otelutils"
	"github.com/relychan/goutils"
	"github.com/relychan/goutils/httpheader"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/openapiclient/handler/proxyhandler"
	"github.com/relychan/openapitools/openapiclient/handler/resthandler/contenttype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProxyClient_RESTful(t *testing.T) {
	configPath := "./testdata/jsonplaceholder.yaml"

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	config, err := goutils.ReadJSONOrYAMLFile[oaschema.OpenAPIResourceDefinition](context.TODO(), configPath)
	assert.NoError(t, err)

	client, err := NewProxyClient(
		context.TODO(),
		config,
		nil,
		WithTimeout(time.Minute),
	)
	assert.NoError(t, err)

	ctx := otelutils.NewContextWithLogger(context.TODO(), logger)

	testCases := []struct {
		Name         string
		Request      *http.Request
		StatusCode   int
		ResponseBody any
		ErrorMessage string
	}{
		{
			Name: "getAlbums",
			Request: &http.Request{
				URL: &url.URL{
					Path: "/api/v1/albums",
				},
				Method: "GET",
			},
			StatusCode: 200,
		},
		{
			Name: "countAlbums",
			Request: &http.Request{
				URL: &url.URL{
					Path: "/api/v1/albums-count",
				},
				Method: http.MethodPost,
			},
			StatusCode: 200,
			ResponseBody: map[string]any{
				"count": float64(100),
			},
		},
		{
			Name: "getPostByID",
			Request: &http.Request{
				URL: &url.URL{
					Path: "/api/v1/posts/1",
				},
				Method: "GET",
			},
			StatusCode: 200,
			ResponseBody: map[string]any{
				"userId": float64(1),
				"id":     float64(1),
				"title":  "sunt aut facere repellat provident occaecati excepturi optio reprehenderit",
				"body":   "quia et suscipit\nsuscipit recusandae consequuntur expedita et cum\nreprehenderit molestiae ut ut quas totam\nnostrum rerum est autem sunt rem eveniet architecto",
			},
		},
		{
			Name: "notFound",
			Request: &http.Request{
				URL: &url.URL{
					Path: "/not-found",
				},
				Method: "GET",
			},
			StatusCode:   404,
			ErrorMessage: "not found",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name+"_execute", func(t *testing.T) {
			response, respBody, err := client.Execute(
				context.TODO(),
				tc.Request.Method,
				tc.Request.URL.String(),
				tc.Request.Header,
				nil,
			)

			if tc.ErrorMessage != "" {
				assert.ErrorContains(t, err, tc.ErrorMessage)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.StatusCode, response.StatusCode)
			}

			if tc.ResponseBody != nil {
				assert.Equal(t, tc.ResponseBody, respBody)
			}
		})

		t.Run(tc.Name+"_stream", func(t *testing.T) {
			writer := httptest.NewRecorder()
			request := tc.Request.WithContext(ctx)
			_, err := client.Stream(writer, request)

			if tc.ErrorMessage != "" {
				assert.ErrorContains(t, err, tc.ErrorMessage)
				assert.Equal(t, httpheader.ContentTypeJSON, writer.Header().Get(httpheader.ContentType))
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tc.StatusCode, writer.Code)

			if tc.ResponseBody != nil {
				var respBody any
				err := json.Unmarshal(writer.Body.Bytes(), &respBody)
				assert.NoError(t, err)
				assert.Equal(t, tc.ResponseBody, respBody)
			}
		})
	}
}

func TestRESTHandler_GraphQLServer(t *testing.T) {
	configPath := "./testdata/rickandmortyapi.yaml"

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	config, err := goutils.ReadJSONOrYAMLFile[oaschema.OpenAPIResourceDefinition](context.TODO(), configPath)
	assert.NoError(t, err)

	client, err := NewProxyClient(context.TODO(), config)
	assert.NoError(t, err)

	ctx := otelutils.NewContextWithLogger(context.TODO(), logger)

	testCases := []struct {
		Name         string
		Request      *proxyhandler.Request
		StatusCode   int
		ResponseBody any
		ErrorMessage string
	}{
		{
			Name: "getCharacters",
			Request: proxyhandler.NewRequest("GET", &url.URL{
				Path: "/characters",
			}, nil, nil),
			StatusCode: 200,
		},
		{
			Name: "getCharacterByID",
			Request: proxyhandler.NewRequest("GET", &url.URL{
				Path: "/characters/1",
			}, nil, nil),
			StatusCode: 200,
			ResponseBody: map[string]any{
				"data": map[string]any{
					"character": map[string]any{
						"id":   "1",
						"name": "Rick Sanchez",
					},
				},
			},
		},
		{
			Name: "notFound",
			Request: proxyhandler.NewRequest("GET", &url.URL{
				Path: "/not-found",
			}, nil, nil),
			StatusCode:   404,
			ErrorMessage: "not found",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name+"_execute", func(t *testing.T) {
			response, result, err := client.Execute(
				ctx,
				tc.Request.Method(),
				tc.Request.URL(),
				tc.Request.Header(),
				tc.Request.Body(),
			)

			if tc.ErrorMessage != "" {
				assert.ErrorContains(t, err, tc.ErrorMessage)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.StatusCode, response.StatusCode)
			}

			if tc.ResponseBody != nil {
				assert.Equal(t, tc.ResponseBody, result)
			}
		})

		t.Run(tc.Name+"_stream", func(t *testing.T) {
			writer := httptest.NewRecorder()
			_, err := client.Stream(writer, &http.Request{
				URL:    tc.Request.GetURL(),
				Method: tc.Request.Method(),
				Header: tc.Request.Header(),
			})

			if tc.ErrorMessage != "" {
				assert.ErrorContains(t, err, tc.ErrorMessage)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tc.StatusCode, writer.Code)
			assert.Equal(t, httpheader.ContentTypeJSON, writer.Header().Get(httpheader.ContentType))

			if tc.ResponseBody != nil {
				var respBody any
				err := json.Unmarshal(writer.Body.Bytes(), &respBody)
				assert.NoError(t, err)
				assert.Equal(t, tc.ResponseBody, respBody)
			}
		})
	}
}

// NOTE: Run the script at testdata/tls/create-certs.sh before running TLS tests.

func TestProxyClient_Auth(t *testing.T) {
	serverContext := createMockServer(t)
	defer serverContext.Server.Close()

	t.Setenv("SERVER_URL", serverContext.Server.URL)
	t.Setenv("API_KEY", serverContext.APIKey)
	t.Setenv("USERNAME", serverContext.Username)
	t.Setenv("PASSWORD", serverContext.Password)
	t.Setenv("QUERY_FOO", "bar")

	keyPem, err := os.ReadFile(filepath.Join("testdata/tls/certs", "client.key"))
	if err != nil {
		t.Fatalf("failed to load client key: %s", err)
	}

	keyData := base64.StdEncoding.EncodeToString(keyPem)
	t.Setenv("TLS_KEY_PEM", string(keyData))

	configPath := "./testdata/test.yaml"

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	config, err := goutils.ReadJSONOrYAMLFile[oaschema.OpenAPIResourceDefinition](context.TODO(), configPath)
	assert.NoError(t, err)

	client, err := NewProxyClient(
		context.TODO(),
		config,
		nil,
		WithTimeout(time.Minute),
	)
	assert.NoError(t, err)

	ctx := otelutils.NewContextWithLogger(context.TODO(), logger)

	testCases := []struct {
		Name         string
		Request      *http.Request
		StatusCode   int
		ResponseBody any
		ErrorMessage string
	}{
		{
			Name: "apiKey",
			Request: &http.Request{
				URL: &url.URL{
					Path: "/auth/api-key",
				},
				Method: "GET",
			},
			StatusCode:   200,
			ResponseBody: "OK",
		},
		{
			Name: "basic",
			Request: &http.Request{
				URL: &url.URL{
					Path: "/auth/basic",
				},
				Method: "GET",
			},
			StatusCode:   200,
			ResponseBody: "OK",
		},
		{
			Name: "forward-header",
			Request: &http.Request{
				URL: &url.URL{
					Path:     "/auth/forward",
					RawQuery: "test=true",
				},
				Method: "POST",
				Header: http.Header{
					"X-Auth-Token": []string{serverContext.APIKey},
				},
			},
			StatusCode:   200,
			ResponseBody: "OK",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name+"_execute", func(t *testing.T) {
			response, respBody, err := client.Execute(
				context.TODO(),
				tc.Request.Method,
				tc.Request.URL.String(),
				tc.Request.Header,
				nil,
			)

			if tc.ErrorMessage != "" {
				require.ErrorContains(t, err, tc.ErrorMessage)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.StatusCode, response.StatusCode)
			}

			if tc.ResponseBody != nil {
				require.Equal(t, tc.ResponseBody, respBody)
			}
		})

		t.Run(tc.Name+"_stream", func(t *testing.T) {
			writer := httptest.NewRecorder()
			request := tc.Request.WithContext(ctx)
			resp, err := client.Stream(writer, request)

			if tc.ErrorMessage != "" {
				require.ErrorContains(t, err, tc.ErrorMessage)
				require.Equal(t, httpheader.ContentTypeJSON, writer.Header().Get(httpheader.ContentType))
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, tc.StatusCode, writer.Code)

			if tc.ResponseBody != nil {
				respBody, err := contenttype.Decode(resp.Header.Get(httpheader.ContentType), writer.Body)
				require.NoError(t, err)
				require.Equal(t, tc.ResponseBody, respBody)
			}
		})
	}
}

type mockServerState struct {
	Server     *httptest.Server
	RetryCount int32
	APIKey     string
	Username   string
	Password   string

	counter atomic.Int32
}

func (mss *mockServerState) Increase() int32 {
	return mss.counter.Add(1)
}

func (mss *mockServerState) GetCounter() int32 {
	return mss.counter.Load()
}

func createMockServer(t *testing.T) *mockServerState {
	t.Helper()

	state := mockServerState{
		APIKey:   rand.Text(),
		Username: rand.Text(),
		Password: rand.Text(),
	}

	mux := http.NewServeMux()

	writeResponse := func(w http.ResponseWriter, body string) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}

	mux.HandleFunc("/auth/api-key", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodPost:
			counter := state.Increase()

			if counter < 2 {
				w.WriteHeader(http.StatusServiceUnavailable)

				return
			}

			apiKey := r.Header.Get("Authorization")
			expectedValue := "Bearer " + state.APIKey
			if apiKey != expectedValue {
				t.Errorf("invalid bearer auth, expected %s, got %s", expectedValue, apiKey)
				t.FailNow()
			}

			w.Header().Add("Content-Type", "text/plain")
			writeResponse(w, "OK")
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
	})

	mux.HandleFunc("/auth/basic", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodPost:
			expectedValue := "Basic " + base64.StdEncoding.EncodeToString([]byte(state.Username+":"+state.Password))
			headerValue := r.Header.Get("Authorization")

			if headerValue != expectedValue {
				t.Errorf("invalid bearer auth, expected %s, got %s", expectedValue, headerValue)
				t.FailNow()
			}

			writeResponse(w, "OK")
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
	})

	mux.HandleFunc("/auth/forward", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodPost:
			tokenValue := r.Header.Get("X-Auth-Token")
			testHeaderValue := r.Header.Get("X-Test-Header")

			require.Equal(t, state.APIKey, tokenValue, "invalid forwarded auth header")
			require.Equal(t, "true", testHeaderValue, "invalid X-Test-Header auth header")

			writeResponse(w, "OK")
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
	})

	var tlsConfig *tls.Config

	dir := "testdata/tls/certs"

	// load CA certificate file and add it to list of client CAs
	caCertFile, err := os.ReadFile(filepath.Join(dir, "ca.crt"))
	if err != nil {
		log.Fatalf("error reading CA certificate: %v", err)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCertFile)

	// Create the TLS Config with the CA pool and enable Client certificate validation
	cert, err := tls.LoadX509KeyPair(
		filepath.Join(dir, "server.pem"),
		filepath.Join(dir, "server.key"),
	)

	tlsConfig = &tls.Config{
		ClientCAs:    caCertPool,
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}

	server := httptest.NewUnstartedServer(mux)
	server.TLS = tlsConfig
	server.StartTLS()

	state.Server = server

	return &state
}
