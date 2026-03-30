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
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/hasura/gotel/otelutils"
	"github.com/relychan/goutils"
	"github.com/relychan/goutils/httpheader"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/openapiclient/handler/proxyhandler"
	"github.com/stretchr/testify/assert"
)

func TestProxyClient_RESTful(t *testing.T) {
	configPath := "../tests/testdata/jsonplaceholder.yaml"

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
	configPath := "../tests/testdata/rickandmortyapi.yaml"

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
