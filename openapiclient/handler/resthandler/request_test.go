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
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/jmespath-community/go-jmespath"
	"github.com/relychan/gohttpc"
	"github.com/relychan/gotransform/jmes"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/openapiclient/handler/proxyhandler"
	"github.com/relychan/openapitools/openapiclient/handler/resthandler/parameter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestRequestFunc creates a NewRequestFunc backed by a real gohttpc client.
func newTestRequestFunc(baseURL string) proxyhandler.NewRequestFunc {
	client := gohttpc.NewClient()

	return func(method string, uri string) *gohttpc.RequestWithClient {
		target := uri
		if target == "" {
			target = baseURL
		}

		return client.R(method, target)
	}
}

// newRESTRequest builds a minimal proxyhandler.Request for tests.
func newRESTRequest(method, path string, body any) *proxyhandler.Request {
	u := &url.URL{
		Path: path,
	}

	return proxyhandler.NewRequest(method, u, http.Header{}, body)
}

// newRESTRequestWithQuery builds a proxyhandler.Request with query parameters.
func newRESTRequestWithQuery(method, rawURL string, body any) *proxyhandler.Request {
	u, _ := url.Parse(rawURL)

	return proxyhandler.NewRequest(method, u, http.Header{}, body)
}

// newTestHandleOptions returns minimal ProxyHandleOptions for tests.
func newTestHandleOptions(baseURL string) *proxyhandler.ProxyHandleOptions {
	return &proxyhandler.ProxyHandleOptions{
		NewRequest: newTestRequestFunc(baseURL),
	}
}

// ---- extractQueryValuesFromPath ----

func TestEvaluateRequestPath(t *testing.T) {
	input := RESTfulHandler{
		customRequest: &customRESTRequest{},
	}

	req := &proxyhandler.Request{}
	req.SetURLParams(map[string]any{
		"id":     "1",
		"postId": "2",
	})

	uri, _, err := input.evaluateRequestPath(
		"https://localhost:8080/users/{id}/posts/{postId}",
		req,
		map[string]any{},
	)
	assert.NoError(t, err)
	assert.Equal(t, "https://localhost:8080/users/1/posts/2", uri)
}

func TestExtractQueryValuesFromPath(t *testing.T) {
	t.Run("no_query", func(t *testing.T) {
		path, values, err := extractQueryValuesFromPath("/users/1")
		require.NoError(t, err)
		assert.Equal(t, "/users/1", path)
		assert.Empty(t, values)
	})

	t.Run("with_query", func(t *testing.T) {
		path, values, err := extractQueryValuesFromPath("/users?limit=10&offset=0")
		require.NoError(t, err)
		assert.Equal(t, "/users", path)
		assert.Equal(t, "10", values.Get("limit"))
		assert.Equal(t, "0", values.Get("offset"))
	})

	t.Run("with_fragment", func(t *testing.T) {
		path, values, err := extractQueryValuesFromPath("/users?limit=5#section")
		require.NoError(t, err)
		assert.Equal(t, "/users#section", path)
		assert.Equal(t, "5", values.Get("limit"))
	})

	t.Run("empty_path", func(t *testing.T) {
		path, values, err := extractQueryValuesFromPath("")
		require.NoError(t, err)
		assert.Equal(t, "", path)
		assert.Empty(t, values)
	})
}

// ---- getDestinedContentType ----

func TestGetDestinedContentType(t *testing.T) {
	t.Run("uses_handler_content_type", func(t *testing.T) {
		handler := &RESTfulHandler{requestContentType: "application/xml"}
		req := newRESTRequest(http.MethodPost, "/", nil)
		assert.Equal(t, "application/xml", handler.getDestinedContentType(req))
	})

	t.Run("falls_back_to_request_header", func(t *testing.T) {
		handler := &RESTfulHandler{}
		u, _ := url.Parse("/")
		header := http.Header{}
		header.Set("Content-Type", "text/plain")
		req := proxyhandler.NewRequest(http.MethodPost, u, header, nil)
		assert.Equal(t, "text/plain", handler.getDestinedContentType(req))
	})

	t.Run("defaults_to_json", func(t *testing.T) {
		handler := &RESTfulHandler{}
		req := newRESTRequest(http.MethodPost, "/", nil)
		assert.Equal(t, "application/json", handler.getDestinedContentType(req))
	})
}

// ---- evaluateRequestPath ----

func TestEvaluateRequestPath_EmptyPath(t *testing.T) {
	handler := &RESTfulHandler{customRequest: &customRESTRequest{}}
	path, values, err := handler.evaluateRequestPath("", &proxyhandler.Request{}, map[string]any{})
	require.NoError(t, err)
	assert.Equal(t, "", path)
	assert.Empty(t, values)
}

func TestEvaluateRequestPath_StaticPath(t *testing.T) {
	handler := &RESTfulHandler{customRequest: &customRESTRequest{}}
	path, values, err := handler.evaluateRequestPath(
		"/users",
		&proxyhandler.Request{},
		map[string]any{},
	)
	require.NoError(t, err)
	assert.Equal(t, "/users", path)
	assert.Empty(t, values)
}

func TestEvaluateRequestPath_WithParams(t *testing.T) {
	handler := &RESTfulHandler{customRequest: &customRESTRequest{}}

	req := &proxyhandler.Request{}
	req.SetURLParams(map[string]any{"id": "42", "postId": "7"})

	path, _, err := handler.evaluateRequestPath(
		"/users/{id}/posts/{postId}",
		req,
		map[string]any{},
	)
	require.NoError(t, err)
	assert.Equal(t, "/users/42/posts/7", path)
}

func TestEvaluateRequestPath_MissingParam(t *testing.T) {
	handler := &RESTfulHandler{customRequest: &customRESTRequest{}}
	_, _, err := handler.evaluateRequestPath(
		"/users/{id}",
		&proxyhandler.Request{},
		map[string]any{},
	)
	assert.Error(t, err)
}

func TestEvaluateRequestPath_ParamFromCustomParameters(t *testing.T) {
	handler := &RESTfulHandler{
		customRequest: &customRESTRequest{
			Parameters: []ProxyRESTfulParameter{
				{
					FieldMappingEntry: jmes.FieldMappingEntry{
						Path: jmespath.MustCompile("param.userId"),
					},
					BaseParameter: parameter.BaseParameter{
						Name: "id",
						In:   oaschema.InPath,
					},
				},
			},
		},
	}

	req := &proxyhandler.Request{}
	req.SetURLParams(map[string]any{"userId": "99"})

	path, _, err := handler.evaluateRequestPath(
		"/users/{id}",
		req,
		map[string]any{
			"param": map[string]string{"userId": "99"},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, "/users/99", path)
}

// ---- prepareRequest ----

func TestPrepareRequest_NoCustomRequest_WithReaderBody(t *testing.T) {
	handler := &RESTfulHandler{}
	body := io.NopCloser(strings.NewReader(`{"key":"value"}`))
	req := newRESTRequest(http.MethodPost, "/api/data", body)
	opts := newTestHandleOptions("http://example.com")

	result, err := handler.prepareRequest(req, opts)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestPrepareRequest_NoCustomRequest_NilBody(t *testing.T) {
	handler := &RESTfulHandler{}
	req := newRESTRequest(http.MethodGet, "/api/data", nil)
	opts := newTestHandleOptions("http://example.com")

	result, err := handler.prepareRequest(req, opts)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestPrepareRequest_ZeroCustomRequest(t *testing.T) {
	handler := &RESTfulHandler{
		customRequest: &customRESTRequest{}, // zero value — treated as nil
	}
	req := newRESTRequest(http.MethodGet, "/api/data", nil)
	opts := newTestHandleOptions("http://example.com")

	result, err := handler.prepareRequest(req, opts)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

// ---- transformRequest ----

func TestTransformRequest_OverridesURLAndMethod(t *testing.T) {
	handler := &RESTfulHandler{
		customRequest: &customRESTRequest{
			URL:    "/new/path",
			Method: http.MethodPut,
		},
	}

	req := newRESTRequest(http.MethodGet, "/old/path", nil)
	opts := newTestHandleOptions("http://example.com")

	result, err := handler.transformRequest(req, opts)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, http.MethodPut, result.Method())
}

func TestTransformRequest_WithHeaderParam(t *testing.T) {
	handler := &RESTfulHandler{
		customRequest: &customRESTRequest{
			Parameters: []ProxyRESTfulParameter{
				{
					FieldMappingEntry: jmes.FieldMappingEntry{
						Path: jmespath.MustCompile("headers.authorization"),
					},
					BaseParameter: parameter.BaseParameter{
						Name: "Authorization",
						In:   oaschema.InHeader,
					},
				},
			},
		},
	}

	u, _ := url.Parse("/api/resource")
	header := http.Header{}
	header.Set("Authorization", "Bearer token123")
	req := proxyhandler.NewRequest(http.MethodGet, u, header, nil)
	opts := newTestHandleOptions("http://example.com")

	result, err := handler.transformRequest(req, opts)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "Bearer token123", result.Header().Get("Authorization"))
}

func TestTransformRequest_WithQueryParam(t *testing.T) {
	handler := &RESTfulHandler{
		customRequest: &customRESTRequest{
			Parameters: []ProxyRESTfulParameter{
				{
					FieldMappingEntry: jmes.FieldMappingEntry{
						Path: jmespath.MustCompile("query.limit"),
					},
					BaseParameter: parameter.BaseParameter{
						Name: "limit",
						In:   oaschema.InQuery,
					},
				},
			},
		},
	}

	req := newRESTRequestWithQuery(http.MethodGet, "/api/items?limit=20", nil)
	opts := newTestHandleOptions("http://example.com")

	result, err := handler.transformRequest(req, opts)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestTransformRequest_ForwardsAllQueryParams(t *testing.T) {
	forwardAll := true
	handler := &RESTfulHandler{
		customRequest: &customRESTRequest{
			URL:                   "/proxy",
			ForwardAllQueryParams: &forwardAll,
		},
	}

	req := newRESTRequestWithQuery(http.MethodGet, "/api/items?foo=bar&baz=qux", nil)
	opts := newTestHandleOptions("http://example.com")

	result, err := handler.transformRequest(req, opts)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Contains(t, result.URL(), "foo=bar")
	assert.Contains(t, result.URL(), "baz=qux")
}

func TestTransformRequest_DoesNotForwardQueryParamsWhenDisabled(t *testing.T) {
	forwardAll := false
	handler := &RESTfulHandler{
		customRequest: &customRESTRequest{
			URL:                   "/proxy",
			ForwardAllQueryParams: &forwardAll,
		},
	}

	req := newRESTRequestWithQuery(http.MethodGet, "/api/items?secret=abc", nil)
	opts := newTestHandleOptions("http://example.com")

	result, err := handler.transformRequest(req, opts)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotContains(t, result.URL(), "secret=abc")
}

func TestTransformRequest_WithJSONBody(t *testing.T) {
	handler := &RESTfulHandler{
		requestContentType: "application/json",
		customRequest: &customRESTRequest{
			URL: "/api/users",
		},
	}

	body := map[string]any{"name": "Alice"}
	req := newRESTRequest(http.MethodPost, "/api/users", body)
	opts := newTestHandleOptions("http://example.com")

	result, err := handler.transformRequest(req, opts)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestTransformRequest_WithReaderBody(t *testing.T) {
	handler := &RESTfulHandler{
		requestContentType: "application/json",
		customRequest: &customRESTRequest{
			URL: "/api/data",
		},
	}

	reader := bytes.NewReader([]byte(`{"key":"value"}`))
	req := newRESTRequest(http.MethodPost, "/api/data", reader)
	opts := newTestHandleOptions("http://example.com")

	result, err := handler.transformRequest(req, opts)
	require.NoError(t, err)
	assert.NotNil(t, result)
}
