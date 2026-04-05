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
	"testing"

	"github.com/jmespath-community/go-jmespath"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProxyCustomGraphQLResponseConfigIsZero tests the IsZero method
func TestProxyCustomGraphQLResponseConfigIsZero(t *testing.T) {
	testCases := []struct {
		name     string
		config   ProxyCustomGraphQLResponseConfig
		expected bool
	}{
		{
			name:     "empty_config",
			config:   ProxyCustomGraphQLResponseConfig{},
			expected: true,
		},
		{
			name:     "with_http_error_code",
			config:   ProxyCustomGraphQLResponseConfig{HTTPErrorCode: new(400)},
			expected: false,
		},
		{
			name:     "with_http_errors",
			config:   ProxyCustomGraphQLResponseConfig{HTTPErrors: map[string][]string{"400": {"errors[0].message == 'bad'"}}},
			expected: false,
		},
		{
			name:     "with_nil_body",
			config:   ProxyCustomGraphQLResponseConfig{Body: nil},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.config.IsZero())
		})
	}
}

// TestNewProxyCustomGraphQLResponse tests creating custom GraphQL response
func TestNewProxyCustomGraphQLResponse(t *testing.T) {
	t.Run("nil_config", func(t *testing.T) {
		result, err := newProxyCustomGraphQLResponse(nil, nil)
		assert.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("empty_config", func(t *testing.T) {
		result, err := newProxyCustomGraphQLResponse(&ProxyCustomGraphQLResponseConfig{}, nil)
		assert.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("with_http_error_code", func(t *testing.T) {
		config := &ProxyCustomGraphQLResponseConfig{HTTPErrorCode: new(400)}
		result, err := newProxyCustomGraphQLResponse(config, nil)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 400, *result.HTTPErrorCode)
		assert.Empty(t, result.HTTPErrors)
		assert.Nil(t, result.Body)
	})

	t.Run("with_http_error_code_500", func(t *testing.T) {
		config := &ProxyCustomGraphQLResponseConfig{HTTPErrorCode: new(500)}
		result, err := newProxyCustomGraphQLResponse(config, nil)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 500, *result.HTTPErrorCode)
	})

	t.Run("with_valid_http_errors", func(t *testing.T) {
		config := &ProxyCustomGraphQLResponseConfig{
			HTTPErrorCode: new(400),
			HTTPErrors: map[string][]string{
				"404": {"errors[0].extensions.code == 'NOT_FOUND'"},
				"500": {"errors[0].extensions.code == 'INTERNAL'", "errors[0].message == 'server error'"},
			},
		}
		result, err := newProxyCustomGraphQLResponse(config, nil)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Len(t, result.HTTPErrors, 2)
		// keys are sorted, so 404 comes before 500
		assert.Equal(t, 404, result.HTTPErrors[0].Status)
		assert.Len(t, result.HTTPErrors[0].Expressions, 1)
		assert.Equal(t, 500, result.HTTPErrors[1].Status)
		assert.Len(t, result.HTTPErrors[1].Expressions, 2)
	})

	t.Run("http_errors_sorted_by_key", func(t *testing.T) {
		config := &ProxyCustomGraphQLResponseConfig{
			HTTPErrorCode: new(400),
			HTTPErrors: map[string][]string{
				"503": {"errors[0].message == 'unavailable'"},
				"401": {"errors[0].extensions.code == 'UNAUTHORIZED'"},
				"422": {"errors[0].extensions.code == 'UNPROCESSABLE'"},
			},
		}
		result, err := newProxyCustomGraphQLResponse(config, nil)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Len(t, result.HTTPErrors, 3)
		assert.Equal(t, 401, result.HTTPErrors[0].Status)
		assert.Equal(t, 422, result.HTTPErrors[1].Status)
		assert.Equal(t, 503, result.HTTPErrors[2].Status)
	})

	t.Run("http_errors_empty_expressions_skipped", func(t *testing.T) {
		config := &ProxyCustomGraphQLResponseConfig{
			HTTPErrorCode: new(400),
			HTTPErrors: map[string][]string{
				"404": {},
				"500": {"errors[0].message == 'error'"},
			},
		}
		_, err := newProxyCustomGraphQLResponse(config, nil)
		require.ErrorContains(t, err, "http error mapping must contain at least one expression")
	})

	t.Run("invalid_http_error_key_not_a_number", func(t *testing.T) {
		config := &ProxyCustomGraphQLResponseConfig{
			HTTPErrorCode: new(400),
			HTTPErrors: map[string][]string{
				"not-a-number": {"errors[0].message == 'x'"},
			},
		}
		result, err := newProxyCustomGraphQLResponse(config, nil)
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("invalid_jmespath_expression", func(t *testing.T) {
		config := &ProxyCustomGraphQLResponseConfig{
			HTTPErrorCode: new(400),
			HTTPErrors: map[string][]string{
				"400": {"[[[invalid"},
			},
		}
		result, err := newProxyCustomGraphQLResponse(config, nil)
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("error_detail_pointer_for_invalid_key", func(t *testing.T) {
		config := &ProxyCustomGraphQLResponseConfig{
			HTTPErrorCode: new(400),
			HTTPErrors: map[string][]string{
				"bad-key": {"errors[0].message == 'x'"},
			},
		}
		_, err := newProxyCustomGraphQLResponse(config, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "/response/httpErrors/bad-key")
	})

	t.Run("error_detail_pointer_for_invalid_expression", func(t *testing.T) {
		config := &ProxyCustomGraphQLResponseConfig{
			HTTPErrorCode: new(400),
			HTTPErrors: map[string][]string{
				"400": {"[[[invalid"},
			},
		}
		_, err := newProxyCustomGraphQLResponse(config, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "/response/httpErrors/400/0")
	})
}

// TestProxyCustomGraphQLResponseIsZero tests the IsZero method
func TestProxyCustomGraphQLResponseIsZero(t *testing.T) {
	compiledExpr, err := jmespath.Compile("errors[0].message")
	require.NoError(t, err)

	testCases := []struct {
		name     string
		response proxyCustomGraphQLResponse
		expected bool
	}{
		{
			name:     "empty_response",
			response: proxyCustomGraphQLResponse{},
			expected: true,
		},
		{
			name:     "with_http_error_code",
			response: proxyCustomGraphQLResponse{HTTPErrorCode: new(400)},
			expected: false,
		},
		{
			name: "with_http_errors",
			response: proxyCustomGraphQLResponse{
				HTTPErrors: []proxyHTTPErrorMapping{
					{Status: 404, Expressions: []jmespath.JMESPath{compiledExpr}},
				},
			},
			expected: false,
		},
		{
			name:     "with_nil_body",
			response: proxyCustomGraphQLResponse{Body: nil},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.response.IsZero())
		})
	}
}
