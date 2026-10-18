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
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/relychan/gotransform/transformtypes"
	"github.com/relychan/goutils/httperror"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/oasvalidator"
	"github.com/relychan/openapitools/openapiclient/handler/proxyhandler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace/noop"
)

type capturedRequest struct {
	Path        string
	RawPath     string
	RawQuery    string
	ContentType string
	Body        string
}

func newCaptureServer(t *testing.T, captured *capturedRequest) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		captured.Path = r.URL.Path
		captured.RawPath = r.URL.EscapedPath()
		captured.RawQuery = r.URL.RawQuery
		captured.ContentType = r.Header.Get("Content-Type")
		captured.Body = string(body)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(server.Close)

	return server
}

func newObjectSchema(required ...string) *base.Schema {
	return &base.Schema{
		Type:     []string{"object"},
		Required: required,
	}
}

// Bug 1: decoded (non-reader) bodies must be encoded and forwarded when there is no custom request config.
func TestPrepareRequest_NoCustomRequest_EncodesDecodedBody(t *testing.T) {
	var captured capturedRequest

	server := newCaptureServer(t, &captured)

	handler := &RESTfulHandler{
		request: &oaschema.MediaType{ContentType: "application/json"},
	}

	req := newRESTRequest(http.MethodPost, "/api/users", map[string]any{"name": "Alice"})

	_, body, err := handler.Handle(context.Background(), req, newTestHandleOptions(server.URL))
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"ok": true}, body)

	assert.Equal(t, "application/json", captured.ContentType)
	assert.JSONEq(t, `{"name":"Alice"}`, captured.Body)
}

func TestPrepareRequest_NoCustomRequest_EncodesDecodedBodyXML(t *testing.T) {
	var captured capturedRequest

	server := newCaptureServer(t, &captured)

	handler := &RESTfulHandler{
		request: &oaschema.MediaType{ContentType: "application/xml"},
	}

	req := newRESTRequest(http.MethodPost, "/api/users", map[string]any{"name": "Alice"})

	_, _, err := handler.Handle(context.Background(), req, newTestHandleOptions(server.URL))
	require.NoError(t, err)

	assert.Equal(t, "application/xml", captured.ContentType)
	assert.Contains(t, captured.Body, "<name>Alice</name>")
}

// Bug 2: path template values must not inject query strings or fragments.
func TestEvaluateRequestPath_RejectsQueryAndFragment(t *testing.T) {
	handler := &RESTfulHandler{customRequest: &customRESTRequest{}}

	for _, value := range []string{"1?role=admin", "1#frag"} {
		req := &proxyhandler.Request{}
		req.SetURLParams(map[string]any{"id": value})

		_, err := handler.evaluateRequestPath("/users/{id}", req, map[string]any{})
		require.Error(t, err, value)
	}

	req := &proxyhandler.Request{}
	req.SetURLParams(map[string]any{"id": "john doe"})

	path, err := handler.evaluateRequestPath("/users/{id}", req, map[string]any{})
	require.NoError(t, err)
	assert.Equal(t, "/users/john doe", path)
}

func TestTransformRequest_PathParamEncodedOnce(t *testing.T) {
	var captured capturedRequest

	server := newCaptureServer(t, &captured)

	handler := &RESTfulHandler{
		request: &oaschema.MediaType{},
		customRequest: &customRESTRequest{
			Path: "/users/{id}",
		},
	}

	req := newRESTRequest(http.MethodGet, "/users/x", nil)
	req.SetURLParams(map[string]any{"id": "john doe"})

	_, _, err := handler.Handle(context.Background(), req, newTestHandleOptions(server.URL))
	require.NoError(t, err)

	assert.Equal(t, "/users/john doe", captured.Path)
	assert.Equal(t, "/users/john%20doe", captured.RawPath)
}

func TestTransformRequest_PathParamQueryInjection(t *testing.T) {
	var captured capturedRequest

	server := newCaptureServer(t, &captured)

	handler := &RESTfulHandler{
		request: &oaschema.MediaType{},
		customRequest: &customRESTRequest{
			Path: "/users/{id}",
		},
	}

	req := newRESTRequest(http.MethodGet, "/users/x", nil)
	req.SetURLParams(map[string]any{"id": "1?role=admin"})

	_, _, err := handler.Handle(context.Background(), req, newTestHandleOptions(server.URL))
	require.Error(t, err)

	// The request must be rejected before reaching the upstream.
	assert.Empty(t, captured.Path)
	assert.Empty(t, captured.RawQuery)
}

// Bug 4: empty response bodies must not fail decoding.
func TestDecodeRawResponse_EmptyBody(t *testing.T) {
	strictOptions := &proxyhandler.ProxyHandleOptions{
		Settings: &oaschema.OpenAPIResourceSettings{Strict: true},
	}

	handler := &RESTfulHandler{
		responses: map[string]*oaschema.MediaType{
			"application/json": {ContentType: "application/json", Schema: newObjectSchema("id")},
		},
	}

	testCases := []struct {
		name        string
		status      int
		contentType string
		body        io.ReadCloser
	}{
		{"no_content_no_body", http.StatusNoContent, "application/json", http.NoBody},
		{"created_empty_json", http.StatusCreated, "application/json", io.NopCloser(bytes.NewReader(nil))},
		{"ok_empty_xml", http.StatusOK, "application/xml", io.NopCloser(strings.NewReader(""))},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for _, options := range []*proxyhandler.ProxyHandleOptions{{}, strictOptions} {
				resp := &http.Response{
					StatusCode: tc.status,
					Header:     http.Header{"Content-Type": []string{tc.contentType}},
					Body:       tc.body,
				}

				result, err := handler.decodeRawResponse(context.Background(), resp, options)
				require.NoError(t, err)
				assert.Nil(t, result)
			}
		})
	}
}

func TestWriteRawResponse_EmptyBody(t *testing.T) {
	handler := &RESTfulHandler{}
	resp := &http.Response{
		StatusCode: http.StatusCreated,
		Header:     http.Header{"Content-Type": []string{"application/xml"}},
		Body:       io.NopCloser(strings.NewReader("")),
	}

	w := httptest.NewRecorder()

	err := handler.writeRawResponse(context.Background(), resp, w, &proxyhandler.ProxyHandleOptions{
		Settings: &oaschema.OpenAPIResourceSettings{Strict: true},
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Empty(t, w.Body.Bytes())
}

func TestTransformResponse_EmptyBody(t *testing.T) {
	transformer := &identityTransformer{}
	handler := &RESTfulHandler{customResponse: &customRESTResponse{Body: transformer}}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader("")),
	}

	result, err := handler.transformResponse(context.Background(), slog.Default(), resp, "application/json")
	require.NoError(t, err)
	assert.Nil(t, result)
	assert.True(t, transformer.called)
}

type identityTransformer struct {
	called bool
}

func (*identityTransformer) IsZero() bool { return false }

func (*identityTransformer) Type() transformtypes.TransformTemplateType { return "identity" }

func (it *identityTransformer) Transform(data any) (any, error) {
	it.called = true

	return data, nil
}

// Bug 5: strict mode must reject unexpected content types instead of passing through unvalidated.
func TestStrictResponse_UnexpectedContentType(t *testing.T) {
	handler := &RESTfulHandler{
		responses: map[string]*oaschema.MediaType{
			"application/json": {ContentType: "application/json", Schema: newObjectSchema("id")},
		},
	}

	options := &proxyhandler.ProxyHandleOptions{
		Settings: &oaschema.OpenAPIResourceSettings{Strict: true},
	}

	for _, contentType := range []string{"text/html", ""} {
		t.Run("decode_"+contentType, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{contentType}},
				Body:       io.NopCloser(strings.NewReader(`<html></html>`)),
			}

			_, err := handler.decodeRawResponse(context.Background(), resp, options)
			assertRemoteServerError(t, err)
		})

		t.Run("stream_"+contentType, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{contentType}},
				Body:       io.NopCloser(strings.NewReader(`<html></html>`)),
			}

			w := newHeaderTrackingWriter()

			err := handler.writeRawResponse(context.Background(), resp, w, options)
			assertRemoteServerError(t, err)
			assert.False(t, w.wroteHeader, "must not write the header before returning the error")
			assert.Empty(t, w.Body.Bytes())
		})
	}

	t.Run("non_strict_passes_through", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/html"}},
			Body:       io.NopCloser(strings.NewReader(`<html></html>`)),
		}

		w := httptest.NewRecorder()

		err := handler.writeRawResponse(context.Background(), resp, w, &proxyhandler.ProxyHandleOptions{})
		require.NoError(t, err)
		assert.Equal(t, `<html></html>`, w.Body.String())
	})

	t.Run("non_2xx_passes_through", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: http.StatusNotFound,
			Header:     http.Header{"Content-Type": []string{"text/html"}},
			Body:       io.NopCloser(strings.NewReader(`not found`)),
		}

		w := httptest.NewRecorder()

		err := handler.writeRawResponse(context.Background(), resp, w, options)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestStrictResponse_VendorJSONValidated(t *testing.T) {
	handler := &RESTfulHandler{
		responses: map[string]*oaschema.MediaType{
			"application/json": {ContentType: "application/json", Schema: newObjectSchema("id")},
		},
	}

	options := &proxyhandler.ProxyHandleOptions{
		Settings: &oaschema.OpenAPIResourceSettings{Strict: true},
	}

	newResp := func(body string) *http.Response {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/vnd.api+json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
		}
	}

	result, err := handler.decodeRawResponse(context.Background(), newResp(`{"id":1}`), options)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"id": float64(1)}, result)

	_, err = handler.decodeRawResponse(context.Background(), newResp(`{"name":"x"}`), options)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "response body is invalid")

	w := newHeaderTrackingWriter()
	err = handler.writeRawResponse(context.Background(), newResp(`{"name":"x"}`), w, options)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "response body is invalid")
	assert.False(t, w.wroteHeader)
}

func assertRemoteServerError(t *testing.T, err error) {
	t.Helper()

	require.Error(t, err)

	httpErr, ok := err.(*httperror.HTTPError)
	require.True(t, ok, "expected *httperror.HTTPError, got %T", err)
	assert.Equal(t, http.StatusInternalServerError, httpErr.Status)
	require.NotEmpty(t, httpErr.Errors)
	assert.Equal(t, oasvalidator.ErrCodeRemoteServerError, httpErr.Errors[0].Code)
}

// Bug 6: bodies must not be recorded into span attributes when debug is disabled and there is no error.
type attrRecordingSpan struct {
	noop.Span

	attrs []attribute.KeyValue
}

func (s *attrRecordingSpan) SetAttributes(kv ...attribute.KeyValue) {
	s.attrs = append(s.attrs, kv...)
}

func TestPostTransformedResponse_NoBodyAttributesWithoutDebug(t *testing.T) {
	handler := &RESTfulHandler{}

	span := &attrRecordingSpan{}
	err := handler.postTransformedResponse(
		context.Background(),
		span,
		slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo})),
		"application/json",
		map[string]any{"secret": "1"},
		map[string]any{"secret": "2"},
		nil,
	)
	require.NoError(t, err)
	assert.Empty(t, span.attrs)

	debugSpan := &attrRecordingSpan{}
	err = handler.postTransformedResponse(
		context.Background(),
		debugSpan,
		slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug})),
		"application/json",
		map[string]any{"secret": "1"},
		map[string]any{"secret": "2"},
		nil,
	)
	require.NoError(t, err)
	assert.NotEmpty(t, debugSpan.attrs)
}

// Bug 8: request logs must use the error level on errors.
func TestPrintRequestLog_ErrorLevel(t *testing.T) {
	handler := &RESTfulHandler{}

	var buf bytes.Buffer

	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	handler.printRequestLog(
		context.Background(),
		noopSpan(),
		logger,
		newRESTRequest(http.MethodGet, "/api", nil),
		nil,
		nil,
		assert.AnError,
	)

	assert.Contains(t, buf.String(), "level=ERROR")
}

// Bug 9: encoding errors must be returned before the response header is written.
type headerTrackingWriter struct {
	*httptest.ResponseRecorder

	wroteHeader bool
}

func newHeaderTrackingWriter() *headerTrackingWriter {
	return &headerTrackingWriter{ResponseRecorder: httptest.NewRecorder()}
}

func (w *headerTrackingWriter) WriteHeader(status int) {
	w.wroteHeader = true
	w.ResponseRecorder.WriteHeader(status)
}

func (w *headerTrackingWriter) Write(b []byte) (int, error) {
	w.wroteHeader = true

	return w.ResponseRecorder.Write(b)
}

func TestWriteEncodedResponse_ErrorBeforeHeader(t *testing.T) {
	w := newHeaderTrackingWriter()

	err := writeEncodedResponse(w, http.StatusOK, "application/json", map[string]any{"c": make(chan int)})
	require.Error(t, err)
	assert.False(t, w.wroteHeader)

	httpErr, ok := err.(*httperror.HTTPError)
	require.True(t, ok)
	assert.Equal(t, http.StatusInternalServerError, httpErr.Status)
}
