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
	"testing"

	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/relychan/openapitools/openapiclient/handler/proxyhandler"
	"github.com/stretchr/testify/assert"
	"go.yaml.in/yaml/v4"
)

func TestRESTHandler_Type(t *testing.T) {
	handler := &RESTfulHandler{}
	assert.Equal(t, ProxyActionTypeREST, handler.Type())
}

func TestRESTHandler_Properties(t *testing.T) {
	testCases := []struct {
		name           string
		handler        *RESTfulHandler
		expectedMethod string
		expectedPath   string
		hasCustomPath  bool
	}{
		{
			name:          "handler with GET method",
			handler:       &RESTfulHandler{},
			expectedPath:  "",
			hasCustomPath: false,
		},
		{
			name: "handler with POST method and custom path",
			handler: &RESTfulHandler{
				customRequest: &customRESTRequest{
					Path:   "/custom/path",
					Method: "POST",
				},
			},
			expectedMethod: "POST",
			expectedPath:   "/custom/path",
			hasCustomPath:  true,
		},
		{
			name: "handler with PUT method",
			handler: &RESTfulHandler{
				customRequest: &customRESTRequest{
					Path:   "/api/resource",
					Method: "PUT",
				},
			},
			expectedMethod: "PUT",
			expectedPath:   "/api/resource",
			hasCustomPath:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.handler.customRequest != nil {
				assert.Equal(t, tc.expectedMethod, tc.handler.customRequest.Method)
			}

			assert.Equal(t, ProxyActionTypeREST, tc.handler.Type())
			if tc.expectedPath != "" {
				assert.Equal(t, tc.expectedPath, tc.handler.customRequest.Path)
			}
		})
	}
}

// TestNewRESTHandler tests the NewRESTHandler function
func TestNewRESTHandler(t *testing.T) {
	t.Run("nil_proxy_action", func(t *testing.T) {
		operation := &highv3.Operation{}
		options := &proxyhandler.NewProxyHandlerOptions{
			Method: "GET",
		}

		handler, err := NewRESTfulHandler(operation, nil, options)
		assert.NoError(t, err)
		assert.True(t, handler != nil)
		assert.Equal(t, ProxyActionTypeREST, handler.Type())
	})

	t.Run("with_custom_request_path", func(t *testing.T) {
		operation := &highv3.Operation{}
		options := &proxyhandler.NewProxyHandlerOptions{
			Method: "POST",
		}

		yamlConfig := `
type: rest
request:
  path: "/custom/path"
`
		var rawAction yaml.Node
		err := yaml.Unmarshal([]byte(yamlConfig), &rawAction)
		assert.NoError(t, err)

		handler, err := NewRESTfulHandler(operation, &rawAction, options)
		assert.NoError(t, err)
		assert.True(t, handler != nil)
	})

	t.Run("invalid_response_config", func(t *testing.T) {
		operation := &highv3.Operation{}
		options := &proxyhandler.NewProxyHandlerOptions{
			Method: "GET",
		}

		yamlConfig := `
type: rest
response:
  body:
    invalid: true
`
		var rawAction yaml.Node
		err := yaml.Unmarshal([]byte(yamlConfig), &rawAction)
		assert.NoError(t, err)

		_, err = NewRESTfulHandler(operation, &rawAction, options)
		assert.True(t, err != nil)
	})
}
