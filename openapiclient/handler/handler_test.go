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

package handler

import (
	"testing"

	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/openapiclient/handler/graphqlhandler"
	"github.com/relychan/openapitools/openapiclient/handler/proxyhandler"
	"github.com/relychan/openapitools/openapiclient/handler/resthandler"
	"github.com/stretchr/testify/assert"
	"go.yaml.in/yaml/v4"
)

func TestNewProxyHandler(t *testing.T) {
	testCases := []struct {
		name          string
		operation     *highv3.Operation
		options       *proxyhandler.NewProxyHandlerOptions
		expectedType  proxyhandler.ProxyActionType
		expectError   bool
		errorContains string
	}{
		{
			name: "REST handler without proxy action",
			operation: &highv3.Operation{
				OperationId: "testOperation",
			},
			options: &proxyhandler.NewProxyHandlerOptions{
				Method: "GET",
			},
			expectedType: resthandler.ProxyActionTypeREST,
			expectError:  false,
		},
		{
			name: "REST handler with explicit proxy action",
			operation: createOperationWithProxyAction(t, resthandler.ProxyRESTfulActionConfig{
				Type: resthandler.ProxyActionTypeREST,
				Request: &resthandler.ProxyRESTfulRequestConfig{
					URL: "/test",
				},
			}),
			options: &proxyhandler.NewProxyHandlerOptions{
				Method: "POST",
			},
			expectedType: resthandler.ProxyActionTypeREST,
			expectError:  false,
		},
		{
			name: "GraphQL handler with valid query",
			operation: createOperationWithProxyAction(t, graphqlhandler.ProxyGraphQLActionConfig{
				Type: graphqlhandler.ProxyTypeGraphQL,
				Request: &graphqlhandler.ProxyGraphQLRequestConfig{
					Query: "query { users { id name } }",
				},
			}),
			options: &proxyhandler.NewProxyHandlerOptions{
				Method: "POST",
			},
			expectedType: graphqlhandler.ProxyTypeGraphQL,
			expectError:  false,
		},
		{
			name: "GraphQL handler with invalid query",
			operation: createOperationWithProxyAction(t, graphqlhandler.ProxyGraphQLActionConfig{
				Type: graphqlhandler.ProxyTypeGraphQL,
				Request: &graphqlhandler.ProxyGraphQLRequestConfig{
					Query: "invalid query {",
				},
			}),
			options: &proxyhandler.NewProxyHandlerOptions{
				Method: "POST",
			},
			expectError:   true,
			errorContains: "Unexpected Name",
		},
		{
			name: "unsupported proxy type",
			operation: createOperationWithProxyAction(t, map[string]any{
				"type": "unsupported",
			}),
			options: &proxyhandler.NewProxyHandlerOptions{
				Method: "GET",
			},
			expectError:   true,
			errorContains: "unsupported proxy type",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handler, err := NewProxyHandler(tc.operation, tc.options)

			if tc.expectError {
				assert.True(t, err != nil, "expected error but got nil")
				if tc.errorContains != "" {
					assert.ErrorContains(t, err, tc.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.True(t, handler != nil)
				assert.Equal(t, tc.expectedType, handler.Type())
			}
		})
	}
}

func TestRegisterProxyHandler(t *testing.T) {
	// Save original constructors
	originalConstructors := make(map[proxyhandler.ProxyActionType]proxyhandler.NewProxyHandlerFunc)
	for k, v := range proxyHandlerConstructors {
		originalConstructors[k] = v
	}

	// Restore original constructors after test
	defer func() {
		proxyHandlerConstructors = originalConstructors
	}()

	customType := proxyhandler.ProxyActionType("custom")
	customConstructor := func(
		operation *highv3.Operation,
		proxyAction *yaml.Node,
		options *proxyhandler.NewProxyHandlerOptions,
	) (proxyhandler.ProxyHandler, error) {
		return nil, nil
	}

	RegisterProxyHandler(customType, customConstructor)

	_, exists := proxyHandlerConstructors[customType]
	assert.True(t, exists, "custom handler should be registered")
}

// Helper function to create an operation with a proxy action extension
func createOperationWithProxyAction(t *testing.T, action any) *highv3.Operation {
	t.Helper()

	extensions := orderedmap.New[string, *yaml.Node]()

	actionData, err := yaml.Marshal(action)
	assert.NoError(t, err)

	var actionNode yaml.Node
	err = yaml.Unmarshal(actionData, &actionNode)
	assert.NoError(t, err)

	extensions.Set(oaschema.XRelyProxyAction, &actionNode)

	return &highv3.Operation{
		OperationId: "testOperation",
		Extensions:  extensions,
	}
}
