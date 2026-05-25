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

package graphqlhandler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/jmespath-community/go-jmespath"
	"github.com/relychan/gotransform/jmes"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/openapiclient/handler/proxyhandler"
	"github.com/stretchr/testify/assert"
	"github.com/vektah/gqlparser/v2/ast"
	"go.yaml.in/yaml/v4"
)

func TestTransformRequest(t *testing.T) {
	testCases := []struct {
		Name         string
		Handler      GraphQLHandler
		TemplateData *proxyhandler.Request
		Expected     map[string]any
	}{
		{
			Name:         "empty",
			Handler:      GraphQLHandler{},
			TemplateData: &proxyhandler.Request{},
			Expected:     map[string]any{},
		},
		{
			Name: "param_simple",
			Handler: GraphQLHandler{
				variableDefinitions: ast.VariableDefinitionList{
					{
						Variable: "name",
					},
				},
				variables: map[string]jmes.FieldMappingEntry{
					"name": {
						Path: jmespath.MustCompile("param.name"),
					},
				},
			},
			TemplateData: func() *proxyhandler.Request {
				request := &proxyhandler.Request{}

				request.SetURLParams(map[string]any{
					"name": "Queen",
				})

				return request
			}(),
			Expected: map[string]any{
				"name": "Queen",
			},
		},
		{
			Name: "query_simple",
			Handler: GraphQLHandler{
				variableDefinitions: ast.VariableDefinitionList{
					{
						Variable: "limit",
					},
					{
						Variable: "offset",
					},
				},
				variables: map[string]jmes.FieldMappingEntry{
					"limit": {
						Path: jmespath.MustCompile("query.limit[0]"),
					},
					"offset": {
						Path: jmespath.MustCompile("query.offset[0]"),
					},
				},
			},
			TemplateData: func() *proxyhandler.Request {
				request := &proxyhandler.Request{}

				request.SetQueryParams(map[string]any{
					"limit":  []string{"10"},
					"offset": []string{"1"},
				})

				return request
			}(),
			Expected: map[string]any{
				"limit":  "10",
				"offset": "1",
			},
		},
		{
			Name: "with_default_value",
			Handler: GraphQLHandler{
				variableDefinitions: ast.VariableDefinitionList{
					{
						Variable: "status",
					},
				},
				variables: map[string]jmes.FieldMappingEntry{
					"status": {
						Path:    jmespath.MustCompile("param.status"),
						Default: "active",
					},
				},
			},
			TemplateData: &proxyhandler.Request{},
			Expected: map[string]any{
				"status": "active",
			},
		},
		{
			Name: "body_variable",
			Handler: GraphQLHandler{
				variableDefinitions: ast.VariableDefinitionList{
					{
						Variable: "body",
					},
				},
				variables: map[string]jmes.FieldMappingEntry{},
			},
			TemplateData: proxyhandler.NewRequest(http.MethodGet, nil, nil, map[string]any{
				"name": "test",
			}),
			Expected: map[string]any{
				"body": map[string]any{
					"name": "test",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			result, err := tc.Handler.resolveRequestVariables(tc.TemplateData, tc.TemplateData.ToMap())
			assert.NoError(t, err)
			assert.Equal(t, tc.Expected, result)
		})
	}
}

func TestGraphQLHandler_Type(t *testing.T) {
	handler := &GraphQLHandler{}
	assert.Equal(t, "graphql", string(handler.Type()))
}

func TestResolveRequestExtensions(t *testing.T) {
	testCases := []struct {
		name         string
		handler      GraphQLHandler
		templateData *proxyhandler.Request
		expected     map[string]any
	}{
		{
			name: "empty extensions",
			handler: GraphQLHandler{
				extensions: map[string]jmes.FieldMappingEntry{},
			},
			templateData: &proxyhandler.Request{},
			expected:     map[string]any{},
		},
		{
			name: "extension with path",
			handler: GraphQLHandler{
				extensions: map[string]jmes.FieldMappingEntry{
					"tracing": {
						Path: jmespath.MustCompile("headers.x_trace_id"),
					},
				},
			},
			templateData: proxyhandler.NewRequest(http.MethodGet, nil, map[string][]string{
				"x_trace_id": {"trace-123"},
			}, nil),
			expected: map[string]any{
				"tracing": "trace-123",
			},
		},
		{
			name: "extension with default value",
			handler: GraphQLHandler{
				extensions: map[string]jmes.FieldMappingEntry{
					"version": {
						Default: "1.0",
					},
				},
			},
			templateData: &proxyhandler.Request{},
			expected: map[string]any{
				"version": "1.0",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tc.handler.resolveRequestExtensions(tc.templateData.ToMap())
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestNewGraphQLHandler tests the NewGraphQLHandler function
func TestNewGraphQLHandler(t *testing.T) {
	t.Run("nil_proxy_action", func(t *testing.T) {
		operation := &oaschema.Operation{}
		options := &proxyhandler.NewProxyHandlerOptions{}

		handler, err := NewGraphQLHandler(operation, nil, options)
		assert.ErrorIs(t, err, ErrProxyActionRequired)
		assert.True(t, handler == nil)
	})

	t.Run("invalid_yaml", func(t *testing.T) {
		operation := &oaschema.Operation{}
		options := &proxyhandler.NewProxyHandlerOptions{}

		// Create invalid YAML node
		var rawAction yaml.Node
		rawAction.SetString("invalid")

		handler, err := NewGraphQLHandler(operation, &rawAction, options)
		assert.True(t, err != nil)
		assert.True(t, handler == nil)
	})

	t.Run("missing_request_config", func(t *testing.T) {
		operation := &oaschema.Operation{}
		options := &proxyhandler.NewProxyHandlerOptions{}

		config := ProxyGraphQLActionConfig{
			Type: ProxyTypeGraphQL,
		}
		configData, _ := yaml.Marshal(config)
		var rawAction yaml.Node
		_ = yaml.Unmarshal(configData, &rawAction)

		handler, err := NewGraphQLHandler(operation, &rawAction, options)
		assert.ErrorContains(t, err, ErrGraphQLQueryEmpty.Error())
		assert.True(t, handler == nil)
	})

	t.Run("empty_query", func(t *testing.T) {
		operation := &oaschema.Operation{}
		options := &proxyhandler.NewProxyHandlerOptions{}

		config := ProxyGraphQLActionConfig{
			Type: ProxyTypeGraphQL,
			Request: &ProxyGraphQLRequestConfig{
				Query: "",
			},
		}
		configData, _ := yaml.Marshal(config)
		var rawAction yaml.Node
		_ = yaml.Unmarshal(configData, &rawAction)

		handler, err := NewGraphQLHandler(operation, &rawAction, options)
		assert.ErrorIs(t, err, ErrGraphQLQueryEmpty)
		assert.True(t, handler == nil)
	})

	t.Run("invalid_graphql_query", func(t *testing.T) {
		operation := &oaschema.Operation{}
		options := &proxyhandler.NewProxyHandlerOptions{}

		config := ProxyGraphQLActionConfig{
			Type: ProxyTypeGraphQL,
			Request: &ProxyGraphQLRequestConfig{
				Query: "query {",
			},
		}
		configData, _ := yaml.Marshal(config)
		var rawAction yaml.Node
		_ = yaml.Unmarshal(configData, &rawAction)

		handler, err := NewGraphQLHandler(operation, &rawAction, options)
		assert.True(t, err != nil)
		assert.True(t, handler == nil)
	})

	t.Run("valid_simple_query", func(t *testing.T) {
		operation := &oaschema.Operation{}
		options := &proxyhandler.NewProxyHandlerOptions{}

		config := ProxyGraphQLActionConfig{
			Type: ProxyTypeGraphQL,
			Request: &ProxyGraphQLRequestConfig{
				Query: "query { users { id name } }",
			},
		}
		configData, _ := yaml.Marshal(config)
		var rawAction yaml.Node
		_ = yaml.Unmarshal(configData, &rawAction)

		handler, err := NewGraphQLHandler(operation, &rawAction, options)
		assert.NoError(t, err)
		assert.True(t, handler != nil)
	})

	t.Run("with_variables", func(t *testing.T) {
		operation := &oaschema.Operation{}
		options := &proxyhandler.NewProxyHandlerOptions{}

		// Use YAML string to avoid type issues
		yamlConfig := `
type: graphql
request:
  query: "query GetUser($id: ID!) { user(id: $id) { id name } }"
  variables:
    id:
      path: "param.id"
`
		var rawAction yaml.Node
		err := yaml.Unmarshal([]byte(yamlConfig), &rawAction)
		assert.NoError(t, err)

		handler, err := NewGraphQLHandler(operation, &rawAction, options)
		assert.NoError(t, err)
		assert.True(t, handler != nil)
	})

	t.Run("with_extensions", func(t *testing.T) {
		operation := &oaschema.Operation{}
		options := &proxyhandler.NewProxyHandlerOptions{}

		yamlConfig := `
type: graphql
request:
  query: "query { users { id } }"
  extensions:
    tracing:
      path: "headers.x_trace_id"
`
		var rawAction yaml.Node
		err := yaml.Unmarshal([]byte(yamlConfig), &rawAction)
		assert.NoError(t, err)

		handler, err := NewGraphQLHandler(operation, &rawAction, options)
		assert.NoError(t, err)
		assert.True(t, handler != nil)
	})

	t.Run("with_response_config", func(t *testing.T) {
		operation := &oaschema.Operation{}
		options := &proxyhandler.NewProxyHandlerOptions{}

		yamlConfig := `
type: graphql
request:
  query: "query { users { id } }"
response:
  httpErrorCode: 400
`
		var rawAction yaml.Node
		err := yaml.Unmarshal([]byte(yamlConfig), &rawAction)
		assert.NoError(t, err)

		handler, err := NewGraphQLHandler(operation, &rawAction, options)
		assert.NoError(t, err)
		assert.True(t, handler != nil)
	})
}

// TestConvertVariableTypeFromString tests type conversion from string
func TestConvertVariableTypeFromString(t *testing.T) {
	testCases := []struct {
		name        string
		varDef      *ast.VariableDefinition
		value       string
		expected    any
		expectError bool
	}{
		{
			name:     "nil_type",
			varDef:   &ast.VariableDefinition{},
			value:    "test",
			expected: "test",
		},
		{
			name: "bool_true",
			varDef: &ast.VariableDefinition{
				Type: &ast.Type{NamedType: "Boolean"},
			},
			value:    "true",
			expected: new(true),
		},
		{
			name: "bool_false",
			varDef: &ast.VariableDefinition{
				Type: &ast.Type{NamedType: "Bool"},
			},
			value:    "false",
			expected: new(false),
		},
		{
			name: "bool_invalid",
			varDef: &ast.VariableDefinition{
				Type: &ast.Type{NamedType: "Boolean"},
			},
			value:       "invalid",
			expectError: true,
		},
		{
			name: "int",
			varDef: &ast.VariableDefinition{
				Type: &ast.Type{NamedType: "Int"},
			},
			value:    "42",
			expected: new(int64(42)),
		},
		{
			name: "int64",
			varDef: &ast.VariableDefinition{
				Type: &ast.Type{NamedType: "Int64"},
			},
			value:    "9223372036854775807",
			expected: new(int64(9223372036854775807)),
		},
		{
			name: "int_invalid",
			varDef: &ast.VariableDefinition{
				Type: &ast.Type{NamedType: "Int"},
			},
			value:       "not_a_number",
			expectError: true,
		},
		{
			name: "uint",
			varDef: &ast.VariableDefinition{
				Type: &ast.Type{NamedType: "UInt"},
			},
			value:    "42",
			expected: new(uint64(42)),
		},
		{
			name: "uint64",
			varDef: &ast.VariableDefinition{
				Type: &ast.Type{NamedType: "UInt64"},
			},
			value:    "18446744073709551615",
			expected: new(uint64(18446744073709551615)),
		},
		{
			name: "float",
			varDef: &ast.VariableDefinition{
				Type: &ast.Type{NamedType: "Float"},
			},
			value:    "3.14",
			expected: new(float64(3.14)),
		},
		{
			name: "double",
			varDef: &ast.VariableDefinition{
				Type: &ast.Type{NamedType: "Double"},
			},
			value:    "2.718",
			expected: new(float64(2.718)),
		},
		{
			name: "decimal",
			varDef: &ast.VariableDefinition{
				Type: &ast.Type{NamedType: "Decimal"},
			},
			value:    "1.5",
			expected: new(float64(1.5)),
		},
		{
			name: "float_invalid",
			varDef: &ast.VariableDefinition{
				Type: &ast.Type{NamedType: "Float"},
			},
			value:       "not_a_float",
			expectError: true,
		},
		{
			name: "string_unknown_type",
			varDef: &ast.VariableDefinition{
				Type: &ast.Type{NamedType: "CustomType"},
			},
			value:    "custom_value",
			expected: "custom_value",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := convertVariableTypeFromParam(tc.varDef, tc.value)

			if tc.expectError {
				assert.True(t, err != nil)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

// TestConvertVariableTypeFromUnknownValue tests type conversion from unknown value
func TestConvertVariableTypeFromUnknownValue(t *testing.T) {
	testCases := []struct {
		name        string
		varDef      *ast.VariableDefinition
		value       any
		expected    any
		expectError bool
	}{
		{
			name:     "nil_type",
			varDef:   &ast.VariableDefinition{},
			value:    "test",
			expected: "test",
		},
		{
			name:     "nil_value",
			varDef:   &ast.VariableDefinition{Type: &ast.Type{NamedType: "String"}},
			value:    nil,
			expected: nil,
		},
		{
			name: "string_to_bool",
			varDef: &ast.VariableDefinition{
				Type: &ast.Type{NamedType: "Boolean"},
			},
			value:    "true",
			expected: new(true),
		},
		{
			name: "string_ptr_to_int",
			varDef: &ast.VariableDefinition{
				Type: &ast.Type{NamedType: "Int"},
			},
			value:    new("42"),
			expected: new(int64(42)),
		},
		{
			name: "bool_value",
			varDef: &ast.VariableDefinition{
				Type: &ast.Type{NamedType: "Boolean"},
			},
			value:    true,
			expected: new(true),
		},
		{
			name: "int_value",
			varDef: &ast.VariableDefinition{
				Type: &ast.Type{NamedType: "Int"},
			},
			value:    int64(42),
			expected: new(int64(42)),
		},
		{
			name: "uint_value",
			varDef: &ast.VariableDefinition{
				Type: &ast.Type{NamedType: "UInt"},
			},
			value:    uint64(42),
			expected: new(uint64(42)),
		},
		{
			name: "float_value",
			varDef: &ast.VariableDefinition{
				Type: &ast.Type{NamedType: "Float"},
			},
			value:    float64(3.14),
			expected: new(float64(3.14)),
		},
		{
			name: "unknown_type_passthrough",
			varDef: &ast.VariableDefinition{
				Type: &ast.Type{NamedType: "CustomType"},
			},
			value:    map[string]any{"key": "value"},
			expected: map[string]any{"key": "value"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := convertVariableTypeFromUnknownValue(tc.varDef, tc.value)

			if tc.expectError {
				assert.True(t, err != nil)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

// TestTransformResponse tests the transformResponse function
func TestTransformResponse(t *testing.T) {
	t.Run("valid_response_no_custom_config", func(t *testing.T) {
		handler := &GraphQLHandler{
			customResponse: &proxyCustomGraphQLResponse{},
		}

		responseBody := map[string]any{
			"data": map[string]any{
				"user": map[string]any{
					"id":   "1",
					"name": "John",
				},
			},
		}
		bodyBytes, _ := json.Marshal(responseBody)

		resp := &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
		}

		respBody, err := handler.handleTransformResponse(context.TODO(), resp)
		assert.NoError(t, err)
		assert.True(t, resp != nil)
		assert.Equal(t, responseBody, respBody)
	})

	t.Run("invalid_json_response", func(t *testing.T) {
		handler := &GraphQLHandler{
			customResponse: &proxyCustomGraphQLResponse{},
		}

		resp := &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewReader([]byte("invalid json"))),
		}

		_, err := handler.handleTransformResponse(context.TODO(), resp)
		assert.ErrorContains(t, err, "Server Error")
	})

	t.Run("response_with_errors_and_custom_error_code", func(t *testing.T) {
		errorCode := 400
		handler := &GraphQLHandler{
			customResponse: &proxyCustomGraphQLResponse{
				HTTPErrorCode: &errorCode,
			},
		}

		responseBody := map[string]any{
			"errors": []any{
				map[string]any{
					"message": "Field not found",
				},
			},
		}
		bodyBytes, _ := json.Marshal(responseBody)

		resp := &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
		}

		_, err := handler.handleTransformResponse(context.TODO(), resp)
		assert.ErrorContains(t, err, "Field not found")
		assert.Equal(t, 400, resp.StatusCode)
	})

	t.Run("response_without_errors_keeps_status", func(t *testing.T) {
		errorCode := 400
		handler := &GraphQLHandler{
			customResponse: &proxyCustomGraphQLResponse{
				HTTPErrorCode: &errorCode,
			},
		}

		responseBody := map[string]any{
			"data": map[string]any{
				"user": map[string]any{
					"id": "1",
				},
			},
		}
		bodyBytes, _ := json.Marshal(responseBody)

		resp := &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
		}

		respBody, err := handler.handleTransformResponse(context.TODO(), resp)
		assert.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, responseBody, respBody)
	})
}

// TestResolveRequestVariablesWithTypes tests variable resolution with type conversion
func TestResolveRequestVariablesWithTypes(t *testing.T) {
	t.Run("param_with_int_type", func(t *testing.T) {
		handler := GraphQLHandler{
			variableDefinitions: ast.VariableDefinitionList{
				{
					Variable: "limit",
					Type:     &ast.Type{NamedType: "Int"},
				},
			},
			variables: map[string]jmes.FieldMappingEntry{},
		}

		templateData := proxyhandler.Request{}
		templateData.SetURLParams(map[string]any{
			"limit": "10",
		})

		result, err := handler.resolveRequestVariables(&templateData, templateData.ToMap())
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"limit": new(int64(10))}, result)
	})

	t.Run("query_with_bool_type", func(t *testing.T) {
		handler := GraphQLHandler{
			variableDefinitions: ast.VariableDefinitionList{
				{
					Variable: "active",
					Type:     &ast.Type{NamedType: "Boolean"},
				},
			},
			variables: map[string]jmes.FieldMappingEntry{},
		}

		templateData := proxyhandler.Request{}
		templateData.SetQueryParams(map[string]any{
			"active": []string{"true"},
		})

		result, err := handler.resolveRequestVariables(&templateData, templateData.ToMap())
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"active": new(true)}, result)
	})

	t.Run("variable_with_custom_mapping_and_type", func(t *testing.T) {
		handler := GraphQLHandler{
			variableDefinitions: ast.VariableDefinitionList{
				{
					Variable: "price",
					Type:     &ast.Type{NamedType: "Float"},
				},
			},
			variables: map[string]jmes.FieldMappingEntry{
				"price": {
					Path: jmespath.MustCompile("body.price"),
				},
			},
		}

		templateData := proxyhandler.Request{}
		templateData.SetBody(map[string]any{
			"price": "19.99",
		})

		result, err := handler.resolveRequestVariables(&templateData, templateData.ToMap())
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"price": new(float64(19.99))}, result)
	})

	t.Run("variable_with_nil_value", func(t *testing.T) {
		handler := GraphQLHandler{
			variableDefinitions: ast.VariableDefinitionList{
				{
					Variable: "optional",
					Type:     &ast.Type{NamedType: "String"},
				},
			},
			variables: map[string]jmes.FieldMappingEntry{
				"optional": {
					Path: jmespath.MustCompile("body.optional"),
				},
			},
		}

		templateData := proxyhandler.Request{}
		templateData.SetBody(map[string]any{})

		result, err := handler.resolveRequestVariables(&templateData, templateData.ToMap())
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"optional": nil}, result)
	})

	t.Run("invalid_type_conversion_from_param", func(t *testing.T) {
		handler := GraphQLHandler{
			variableDefinitions: ast.VariableDefinitionList{
				{
					Variable: "count",
					Type:     &ast.Type{NamedType: "Int"},
				},
			},
			variables: map[string]jmes.FieldMappingEntry{},
		}

		templateData := proxyhandler.Request{}
		templateData.SetURLParams(map[string]any{
			"count": "not_a_number",
		})

		_, err := handler.resolveRequestVariables(&templateData, templateData.ToMap())
		assert.True(t, err != nil)
		assert.ErrorContains(t, err, "failed to evaluate the type of variable")
	})

	t.Run("invalid_type_conversion_from_query", func(t *testing.T) {
		handler := GraphQLHandler{
			variableDefinitions: ast.VariableDefinitionList{
				{
					Variable: "active",
					Type:     &ast.Type{NamedType: "Boolean"},
				},
			},
			variables: map[string]jmes.FieldMappingEntry{},
		}

		templateData := proxyhandler.Request{}
		templateData.SetQueryParams(map[string]any{
			"active": []string{"not_a_bool"},
		})

		_, err := handler.resolveRequestVariables(&templateData, templateData.ToMap())
		assert.True(t, err != nil)
		assert.ErrorContains(t, err, "failed to evaluate the type of variable")
	})

	t.Run("invalid_type_conversion_from_custom_variable", func(t *testing.T) {
		handler := GraphQLHandler{
			variableDefinitions: ast.VariableDefinitionList{
				{
					Variable: "price",
					Type:     &ast.Type{NamedType: "Float"},
				},
			},
			variables: map[string]jmes.FieldMappingEntry{
				"price": {
					Path: jmespath.MustCompile("body.price"),
				},
			},
		}

		templateData := proxyhandler.Request{}
		templateData.SetBody(map[string]any{
			"price": "not_a_float",
		})

		_, err := handler.resolveRequestVariables(&templateData, templateData.ToMap())
		assert.True(t, err != nil)
		assert.ErrorContains(t, err, "failed to evaluate value of variable")
	})
}
