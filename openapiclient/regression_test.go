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
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/relychan/goutils"
	"github.com/relychan/goutils/httperror"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/openapiclient/handler/proxyhandler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace/noop"
)

type recordedRequest struct {
	Method      string
	EscapedPath string
	Header      http.Header
}

type recordingServer struct {
	*httptest.Server

	mu       sync.Mutex
	requests []recordedRequest
}

func newRecordingServer(t *testing.T) *recordingServer {
	t.Helper()

	rs := &recordingServer{}
	rs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rs.mu.Lock()
		rs.requests = append(rs.requests, recordedRequest{
			Method:      r.Method,
			EscapedPath: r.URL.EscapedPath(),
			Header:      r.Header.Clone(),
		})
		rs.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))

	t.Cleanup(rs.Close)

	return rs
}

func (rs *recordingServer) Last(t *testing.T) recordedRequest {
	t.Helper()

	rs.mu.Lock()
	defer rs.mu.Unlock()

	require.NotEmpty(t, rs.requests)

	return rs.requests[len(rs.requests)-1]
}

func (rs *recordingServer) Count() int {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	return len(rs.requests)
}

func newTestProxyClient(t *testing.T, config string, options ...ClientOption) *ProxyClient {
	t.Helper()

	specPath := filepath.Join(t.TempDir(), "spec.yaml")
	require.NoError(t, os.WriteFile(specPath, []byte(config), 0o600))

	metadata, err := goutils.ReadJSONOrYAMLFile[oaschema.OpenAPIResourceDefinition](context.TODO(), specPath)
	require.NoError(t, err)

	client, err := NewProxyClient(context.TODO(), metadata, options...)
	require.NoError(t, err)

	t.Cleanup(func() { _ = client.Close() })

	return client
}

const regressionSpec = `
settings:
  basePath: %BASE_PATH%
spec:
  openapi: "3.1.0"
  info:
    title: test
    version: "1.0"
  servers:
    - url: %SERVER_URL%
  paths:
    /:
      get: {}
    /users/{name}:
      get:
        parameters:
          - name: name
            in: path
            required: true
            schema:
              type: string
    /pets:
      post:
        requestBody:
          required: true
          content:
            application/json:
              schema:
                type: object
    /pets/{id}:
      get: {}
    /optional-body:
      post:
        requestBody:
          content:
            application/json:
              schema:
                type: object
`

func newRegressionClient(t *testing.T, basePath string) (*ProxyClient, *recordingServer) {
	t.Helper()

	server := newRecordingServer(t)

	if basePath == "" {
		basePath = `""`
	}

	config := strings.NewReplacer("%SERVER_URL%", server.URL, "%BASE_PATH%", basePath).
		Replace(regressionSpec)

	return newTestProxyClient(t, config), server
}

// Bug: Execute routed on the non-decoded path and the upstream builder escaped it again.
func TestExecute_PathIsEscapedOnce(t *testing.T) {
	client, server := newRegressionClient(t, "")

	resp, _, err := client.Execute(context.TODO(), http.MethodGet, "/users/john%20doe", http.Header{}, nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "/users/john%20doe", server.Last(t).EscapedPath)

	// ServeHTTP must behave the same.
	req := httptest.NewRequest(http.MethodGet, "/users/john%20doe", nil)
	writer := httptest.NewRecorder()
	client.ServeHTTP(writer, req)

	assert.Equal(t, http.StatusOK, writer.Code)
	assert.Equal(t, "/users/john%20doe", server.Last(t).EscapedPath)

	_, _, err = client.Execute(context.TODO(), http.MethodGet, "/users/john%zz", http.Header{}, nil)
	httpErr, ok := errors.AsType[*httperror.HTTPError](err)
	require.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, httpErr.Status)
}

// Bug: the required request body was enforced in ServeHTTP only.
func TestExecute_RequiredRequestBody(t *testing.T) {
	client, server := newRegressionClient(t, "")

	_, _, err := client.Execute(context.TODO(), http.MethodPost, "/pets", http.Header{}, nil)
	httpErr, ok := errors.AsType[*httperror.HTTPError](err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, http.StatusBadRequest, httpErr.Status)
	assert.Equal(t, "request body is required", httpErr.Detail)

	writer := httptest.NewRecorder()
	_, err = client.Stream( //nolint:bodyclose
		context.TODO(),
		writer,
		proxyhandler.NewRequest(http.MethodPost, &url.URL{Path: "/pets"}, http.Header{}, nil),
	)
	httpErr, ok = errors.AsType[*httperror.HTTPError](err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, http.StatusBadRequest, httpErr.Status)
	assert.Equal(t, http.StatusBadRequest, writer.Code)
	assert.Equal(t, 0, server.Count())

	// An absent optional body is not validated against the schema.
	resp, _, err := client.Execute(context.TODO(), http.MethodPost, "/optional-body", http.Header{}, nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// A provided required body passes.
	resp, _, err = client.Execute(context.TODO(), http.MethodPost, "/pets", http.Header{
		"Content-Type": []string{"application/json"},
	}, map[string]any{"name": "pet"})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// Bug: the base path was trimmed without a segment boundary.
func TestFindRoute_BasePathSegmentBoundary(t *testing.T) {
	client, server := newRegressionClient(t, "/api")

	resp, _, err := client.Execute(context.TODO(), http.MethodGet, "/api/pets/1", http.Header{}, nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "/pets/1", server.Last(t).EscapedPath)

	resp, _, err = client.Execute(context.TODO(), http.MethodGet, "/api", http.Header{}, nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	for _, path := range []string{"/apiv2/pets/1", "/pets/1", "/ap"} {
		_, _, err = client.Execute(context.TODO(), http.MethodGet, path, http.Header{}, nil)
		httpErr, ok := errors.AsType[*httperror.HTTPError](err)
		require.True(t, ok, path)
		assert.Equal(t, http.StatusNotFound, httpErr.Status, path)
	}

	assert.Equal(t, 2, server.Count())

	path, ok := trimBasePath("/api/", "api/")
	assert.True(t, ok)
	assert.Equal(t, "/", path)
}

// Bug: traversal paths and validation errors became 500 errors with internal details.
func TestExecute_TraversalPathIsBadRequest(t *testing.T) {
	client, server := newRegressionClient(t, "")

	for _, path := range []string{"/pets/%2E%2E", "/pets/.."} {
		_, _, err := client.Execute(context.TODO(), http.MethodGet, path, http.Header{}, nil)
		httpErr, ok := errors.AsType[*httperror.HTTPError](err)
		require.True(t, ok, path)
		assert.Equal(t, http.StatusBadRequest, httpErr.Status, path)
	}

	req := httptest.NewRequest(http.MethodGet, "/pets/%2E%2E", nil)
	writer := httptest.NewRecorder()
	client.ServeHTTP(writer, req)
	assert.Equal(t, http.StatusBadRequest, writer.Code)

	assert.Equal(t, 0, server.Count())
}

func TestHandleError_StatusMapping(t *testing.T) {
	client := &ProxyClient{}
	span := noop.Span{}

	status, err := client.handleError(context.TODO(), span, errors.New("dial tcp 10.0.0.1:5432: secret internal detail"), "/x")
	assert.Equal(t, http.StatusInternalServerError, status)
	assert.NotContains(t, err.Error(), "secret internal detail")

	status, err = client.handleError(context.TODO(), span, &httperror.ValidationError{Detail: "invalid path"}, "/x")
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Contains(t, err.Error(), "invalid path")

	status, _ = client.handleError(context.TODO(), span, httperror.ValidationError{Detail: "invalid"}, "/x")
	assert.Equal(t, http.StatusBadRequest, status)

	status, _ = client.handleError(context.TODO(), span, httperror.HTTPError{Status: http.StatusConflict}, "/x")
	assert.Equal(t, http.StatusConflict, status)
}

// Bug: WithGetEnvFunc was not applied to settings.http because it was loaded before client options.
func TestWithGetEnvFunc_AppliedToHTTPSettings(t *testing.T) {
	server := newRecordingServer(t)

	config := `
settings:
  http:
    authentication:
      type: http
      tokenLocation:
        in: header
        name: Authorization
        scheme: bearer
      value:
        env: TEST_CUSTOM_GETENV_TOKEN
spec:
  openapi: "3.1.0"
  info:
    title: test
    version: "1.0"
  servers:
    - url: ` + server.URL + `
  paths:
    /private:
      get: {}
    /public:
      get:
        security: []
`

	getEnv := func(key string) (string, error) {
		if key == "TEST_CUSTOM_GETENV_TOKEN" {
			return "custom-secret", nil
		}

		return "", nil
	}

	client := newTestProxyClient(t, config, WithGetEnvFunc(getEnv))

	_, _, err := client.Execute(context.TODO(), http.MethodGet, "/private", http.Header{}, nil)
	require.NoError(t, err)
	assert.Equal(t, "Bearer custom-secret", server.Last(t).Header.Get("Authorization"))

	// explicit `security: []` disables the default authenticator too.
	_, _, err = client.Execute(context.TODO(), http.MethodGet, "/public", http.Header{}, nil)
	require.NoError(t, err)
	assert.Empty(t, server.Last(t).Header.Get("Authorization"))
}

// Bug: the global security fallback picked a random scheme for operations
// with explicit `security: []` or unmatched requirements.
func TestProxyClient_SecurityDoesNotLeakCredentials(t *testing.T) {
	server := newRecordingServer(t)

	config := `
spec:
  openapi: "3.1.0"
  info:
    title: test
    version: "1.0"
  servers:
    - url: ` + server.URL + `
  security:
    - keyA: []
  components:
    securitySchemes:
      keyA:
        type: apiKey
        in: header
        name: X-Key-A
        x-rely-security-credentials:
          apiKey:
            value: a
      keyB:
        type: apiKey
        in: header
        name: X-Key-B
        x-rely-security-credentials:
          apiKey:
            value: b
  paths:
    /private:
      get: {}
    /public:
      get:
        security: []
    /b:
      get:
        security:
          - keyB: []
`

	client := newTestProxyClient(t, config)

	for range 20 {
		_, _, err := client.Execute(context.TODO(), http.MethodGet, "/private", http.Header{}, nil)
		require.NoError(t, err)
		assert.Equal(t, "a", server.Last(t).Header.Get("X-Key-A"))
		assert.Empty(t, server.Last(t).Header.Get("X-Key-B"))

		_, _, err = client.Execute(context.TODO(), http.MethodGet, "/public", http.Header{}, nil)
		require.NoError(t, err)
		assert.Empty(t, server.Last(t).Header.Get("X-Key-A"))
		assert.Empty(t, server.Last(t).Header.Get("X-Key-B"))

		_, _, err = client.Execute(context.TODO(), http.MethodGet, "/b", http.Header{}, nil)
		require.NoError(t, err)
		assert.Equal(t, "b", server.Last(t).Header.Get("X-Key-B"))
		assert.Empty(t, server.Last(t).Header.Get("X-Key-A"))
	}
}

type flushRecorder struct {
	*httptest.ResponseRecorder

	flushed bool
}

func (fr *flushRecorder) Flush() {
	fr.flushed = true
	fr.ResponseRecorder.Flush()
}

type unencodableError struct{}

func (unencodableError) Error() string { return `bad "error" <value>` }

func (unencodableError) MarshalJSON() ([]byte, error) {
	return nil, errors.New(`cannot "encode" <this>`)
}

// Bug: error bodies were appended after a partially written response.
func TestWriteErrorResponse_AfterPartialWrite(t *testing.T) {
	t.Run("skip_after_header_written", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		writer := newTrackingResponseWriter(recorder)

		writer.WriteHeader(http.StatusOK)
		writeErrorResponse(context.TODO(), writer, http.StatusBadGateway, httperror.NewServerError())

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Empty(t, recorder.Body.String())
	})

	t.Run("skip_after_partial_body", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		writer := newTrackingResponseWriter(recorder)

		_, _ = writer.Write([]byte(`{"partial":`))
		writeErrorResponse(context.TODO(), writer, http.StatusBadGateway, httperror.NewServerError())

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, `{"partial":`, recorder.Body.String())
		assert.Equal(t, len(`{"partial":`), writer.BytesWritten())
	})

	t.Run("write_when_untouched", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		writer := newTrackingResponseWriter(recorder)

		writeErrorResponse(context.TODO(), writer, http.StatusBadGateway, httperror.NewHTTPError(http.StatusBadGateway, "x"))

		assert.Equal(t, http.StatusBadGateway, recorder.Code)
		assert.True(t, json.Valid(recorder.Body.Bytes()))
	})

	t.Run("json_safe_fallback", func(t *testing.T) {
		recorder := httptest.NewRecorder()

		writeErrorResponse(context.TODO(), recorder, http.StatusBadRequest, unencodableError{})

		assert.Equal(t, http.StatusInternalServerError, recorder.Code)
		assert.True(t, json.Valid(recorder.Body.Bytes()), recorder.Body.String())
	})

	t.Run("preserves_flusher_and_unwrap", func(t *testing.T) {
		recorder := &flushRecorder{ResponseRecorder: httptest.NewRecorder()}
		writer := newTrackingResponseWriter(recorder)

		var w http.ResponseWriter = writer

		flusher, ok := w.(http.Flusher)
		require.True(t, ok)

		flusher.Flush()
		assert.True(t, recorder.flushed)
		assert.True(t, writer.HeaderWritten())

		recorder.flushed = false
		require.NoError(t, http.NewResponseController(w).Flush())
		assert.True(t, recorder.flushed)
		assert.Equal(t, http.ResponseWriter(recorder), writer.Unwrap())

		// wrapping twice returns the same writer.
		assert.Same(t, writer, newTrackingResponseWriter(writer))
	})
}
