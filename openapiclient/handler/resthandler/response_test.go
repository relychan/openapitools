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

package resthandler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/relychan/openapitools/openapiclient/handler/proxyhandler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// noopSpan returns a noop trace span for use in unit tests.
func noopSpan() trace.Span {
	return noop.Span{}
}

// marshalRESTBody encodes v as JSON and returns an io.ReadCloser.
func marshalRESTBody(v any) io.ReadCloser {
	b, _ := json.Marshal(v)

	return io.NopCloser(bytes.NewReader(b))
}

// ---- writeRawResponse ----

func TestWriteRawResponse_NilBody(t *testing.T) {
	handler := &RESTfulHandler{}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       nil,
		Header:     http.Header{},
	}

	w := httptest.NewRecorder()

	err := handler.writeRawResponse(context.Background(), resp, w, &proxyhandler.ProxyHandleOptions{})
	require.NoError(t, err)
	assert.Equal(t, resp.StatusCode, w.Code)
}

func TestWriteRawResponse_BufferedJSONResponse(t *testing.T) {
	handler := &RESTfulHandler{}
	payload := map[string]any{"id": "1", "name": "Alice"}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       marshalRESTBody(payload),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}

	w := httptest.NewRecorder()

	err := handler.writeRawResponse(context.Background(), resp, w, &proxyhandler.ProxyHandleOptions{})
	require.NoError(t, err)
	assert.Equal(t, resp.StatusCode, w.Code)

	var result map[string]any

	err = json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, payload, result)
}

func TestWriteRawResponse_StreamingResponse(t *testing.T) {
	handler := &RESTfulHandler{}
	payload := map[string]any{"streamed": true}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       marshalRESTBody(payload),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}

	recorder := httptest.NewRecorder()
	err := handler.writeRawResponse(context.Background(), resp, recorder, &proxyhandler.ProxyHandleOptions{})
	require.NoError(t, err)
	assert.Equal(t, resp.StatusCode, recorder.Code)

	var written map[string]any
	err = json.Unmarshal(recorder.Body.Bytes(), &written)
	require.NoError(t, err)
	assert.Equal(t, payload, written)
}

func TestWriteRawResponse_StreamingWritesStatusCode(t *testing.T) {
	handler := &RESTfulHandler{}
	resp := &http.Response{
		StatusCode: http.StatusCreated,
		Body:       marshalRESTBody(map[string]any{"ok": true}),
		Header:     http.Header{},
	}

	recorder := httptest.NewRecorder()
	err := handler.writeRawResponse(context.Background(), resp, recorder, &proxyhandler.ProxyHandleOptions{})
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, recorder.Code)
}

// ---- postTransformedResponse ----

func TestPostTransformedResponse_NoError_NoDebug(t *testing.T) {
	handler := &RESTfulHandler{}
	logger := slog.Default()
	_ = logger

	// non-debug logger — should return nil without logging.
	err := handler.postTransformedResponse(
		context.Background(),
		noopSpan(),
		slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo})),
		"application/json",
		map[string]any{"id": "1"},
		map[string]any{"result": "ok"},
		nil,
	)
	assert.NoError(t, err)
}

func TestPostTransformedResponse_WithError(t *testing.T) {
	handler := &RESTfulHandler{}

	inputErr := assert.AnError
	err := handler.postTransformedResponse(
		context.Background(),
		noopSpan(),
		slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo})),
		"application/json",
		nil,
		nil,
		inputErr,
	)
	assert.Error(t, err)
}

func TestPostTransformedResponse_NilBodies(t *testing.T) {
	handler := &RESTfulHandler{}

	err := handler.postTransformedResponse(
		context.Background(),
		noopSpan(),
		slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo})),
		"text/plain",
		nil,
		nil,
		nil,
	)
	assert.NoError(t, err)
}
