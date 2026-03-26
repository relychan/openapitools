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
	"net/url"
	"testing"

	"github.com/relychan/gohttpc"
	"github.com/relychan/goutils"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/openapiclient/handler/proxyhandler"
	"github.com/stretchr/testify/assert"
)

func TestProxyClient_RESTful(t *testing.T) {
	configPath := "../tests/testdata/jsonplaceholder.yaml"

	config, err := goutils.ReadJSONOrYAMLFile[oaschema.OpenAPIResourceDefinition](context.TODO(), configPath)
	assert.NoError(t, err)

	client, err := NewProxyClient(context.TODO(), config, gohttpc.NewClientOptions())
	assert.NoError(t, err)

	testCases := []struct {
		Name         string
		Request      proxyhandler.Request
		StatusCode   int
		ResponseBody any
	}{
		// {
		// 	Name: "getAlbums",
		// 	Request: proxyhandler.Request{
		// 		URL: &url.URL{
		// 			Path: "/api/v1/albums",
		// 		},
		// 		Method: "GET",
		// 	},
		// 	StatusCode: 200,
		// },
		{
			Name: "getPostByID",
			Request: proxyhandler.Request{
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
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			response, result, err := client.Execute(context.TODO(), &tc.Request)
			assert.NoError(t, err)
			assert.Equal(t, tc.StatusCode, response.StatusCode)

			if tc.ResponseBody != nil {
				assert.Equal(t, tc.ResponseBody, result)
			}
		})
	}
}

// func TestRESTHandler_GraphQLServer(t *testing.T) {
// 	server, shutdown := initTestServer(t, "../testdata/rickandmortyapi/config.yaml")
// 	defer func() {
// 		server.Close()
// 		shutdown()
// 	}()

// 	testCases := []struct {
// 		Name         string
// 		Body         ddnrouter.PreRoutePluginRequestBody
// 		StatusCode   int
// 		ResponseBody any
// 	}{
// 		{
// 			Name: "getCharacters",
// 			Body: ddnrouter.PreRoutePluginRequestBody{
// 				Path:   server.URL + "/characters",
// 				Method: "GET",
// 			},
// 			StatusCode: 200,
// 		},
// 		{
// 			Name: "getCharacterByID",
// 			Body: ddnrouter.PreRoutePluginRequestBody{
// 				Path:   server.URL + "/characters/1",
// 				Method: "GET",
// 			},
// 			StatusCode: 200,
// 			ResponseBody: map[string]any{
// 				"data": map[string]any{
// 					"character": map[string]any{
// 						"id":   "1",
// 						"name": "Rick Sanchez",
// 					},
// 				},
// 			},
// 		},
// 	}

// 	for _, tc := range testCases {
// 		t.Run(tc.Name, func(t *testing.T) {
// 			runSuccessRequest(t, tc.Body, tc.StatusCode, tc.ResponseBody)
// 			runUnauthorizedRequest(t, tc.Body, tc.StatusCode, tc.ResponseBody)
// 		})
// 	}
// }

// func TestRESTHandler_NotFoundPath(t *testing.T) {
// 	server, shutdown := initTestServer(t, testPlaceholderConfig)
// 	defer func() {
// 		server.Close()
// 		shutdown()
// 	}()

// 	req, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/nonexistent", nil)
// 	assert.NilError(t, err)

// 	req.Header.Set("hasura-m-auth", "test-secret")

// 	resp, err := http.DefaultClient.Do(req)
// 	assert.NilError(t, err)
// 	defer resp.Body.Close()

// 	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
// }

// func TestRESTHandler_WithPathParams(t *testing.T) {
// 	server, shutdown := initTestServer(t, testPlaceholderConfig)
// 	defer func() {
// 		server.Close()
// 		shutdown()
// 	}()

// 	req, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/posts/1", nil)
// 	assert.NilError(t, err)

// 	req.Header.Set("hasura-m-auth", "test-secret")

// 	resp, err := http.DefaultClient.Do(req)
// 	assert.NilError(t, err)
// 	defer resp.Body.Close()

// 	assert.Equal(t, http.StatusOK, resp.StatusCode)

// 	var result map[string]any
// 	err = json.NewDecoder(resp.Body).Decode(&result)
// 	assert.NilError(t, err)
// 	assert.Equal(t, float64(1), result["id"])
// }

// func TestRESTHandler_GetAlbums(t *testing.T) {
// 	server, shutdown := initTestServer(t, testPlaceholderConfig)
// 	defer func() {
// 		server.Close()
// 		shutdown()
// 	}()

// 	req, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/albums", nil)
// 	assert.NilError(t, err)

// 	req.Header.Set("hasura-m-auth", "test-secret")

// 	resp, err := http.DefaultClient.Do(req)
// 	assert.NilError(t, err)
// 	defer resp.Body.Close()

// 	assert.Equal(t, http.StatusOK, resp.StatusCode)

// 	var result []map[string]any
// 	err = json.NewDecoder(resp.Body).Decode(&result)
// 	assert.NilError(t, err)
// 	assert.Assert(t, len(result) > 0)
// }

func mustParseURL(input string) *url.URL {
	result, err := url.Parse(input)
	if err != nil {
		panic(err)
	}

	return result
}
