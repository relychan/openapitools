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

	"github.com/stretchr/testify/assert"
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
			name: "with_http_error_code",
			config: ProxyCustomGraphQLResponseConfig{
				HTTPErrorCode: func() *int { i := 400; return &i }(),
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.config.IsZero()
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestNewProxyCustomGraphQLResponse tests creating custom GraphQL response
func TestNewProxyCustomGraphQLResponse(t *testing.T) {
	t.Run("nil_config", func(t *testing.T) {
		result, err := newProxyCustomGraphQLResponse(nil, nil)
		assert.NoError(t, err)
		assert.True(t, result == nil)
	})

	t.Run("empty_config", func(t *testing.T) {
		config := &ProxyCustomGraphQLResponseConfig{}
		result, err := newProxyCustomGraphQLResponse(config, nil)
		assert.NoError(t, err)
		assert.True(t, result == nil)
	})

	t.Run("with_http_error_code", func(t *testing.T) {
		errorCode := 400
		config := &ProxyCustomGraphQLResponseConfig{
			HTTPErrorCode: &errorCode,
		}
		result, err := newProxyCustomGraphQLResponse(config, nil)
		assert.NoError(t, err)
		assert.True(t, result != nil)
		assert.Equal(t, 400, *result.HTTPErrorCode)
	})
}

// TestProxyCustomGraphQLResponseIsZero tests the IsZero method
func TestProxyCustomGraphQLResponseIsZero(t *testing.T) {
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
			name: "with_http_error_code",
			response: proxyCustomGraphQLResponse{
				HTTPErrorCode: func() *int { i := 400; return &i }(),
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.response.IsZero()
			assert.Equal(t, tc.expected, result)
		})
	}
}
