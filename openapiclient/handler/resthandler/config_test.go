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

	"github.com/hasura/goenvconf"
	"github.com/relychan/gotransform"
	"github.com/relychan/gotransform/jmes"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/oasvalidator/parameter"
	"github.com/stretchr/testify/assert"
)

// TestProxyRESTRequestConfigIsZero tests the IsZero method
func TestProxyRESTRequestConfigIsZero(t *testing.T) {
	testCases := []struct {
		name     string
		config   ProxyRESTfulRequestConfig
		expected bool
	}{
		{
			name:     "empty_config",
			config:   ProxyRESTfulRequestConfig{},
			expected: true,
		},
		{
			name: "with_path",
			config: ProxyRESTfulRequestConfig{
				URL: "/test",
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

// TestProxyCustomRESTResponseConfigIsZero tests the IsZero method
func TestProxyCustomRESTResponseConfigIsZero(t *testing.T) {
	testCases := []struct {
		name     string
		config   ProxyCustomRESTfulResponseConfig
		expected bool
	}{
		{
			name:     "empty_config",
			config:   ProxyCustomRESTfulResponseConfig{},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.config.IsZero()
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestCustomRESTResponseIsZero tests the IsZero method for customRESTResponse
func TestCustomRESTResponseIsZero(t *testing.T) {
	testCases := []struct {
		name     string
		response customRESTResponse
		expected bool
	}{
		{
			name:     "empty_response",
			response: customRESTResponse{},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.response.IsZero()
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestCustomRESTRequestIsZero tests the IsZero method for customRESTRequest
func TestCustomRESTRequestIsZero(t *testing.T) {
	testCases := []struct {
		name     string
		request  customRESTRequest
		expected bool
	}{
		{
			name:     "empty_request",
			request:  customRESTRequest{},
			expected: true,
		},
		{
			name: "with_path",
			request: customRESTRequest{
				URL: "/test",
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.request.IsZero()
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestNewCustomRESTResponse tests creating custom REST response
func TestNewCustomRESTResponse(t *testing.T) {
	t.Run("nil_config", func(t *testing.T) {
		result, err := newCustomRESTResponse(nil, goenvconf.GetOSEnv)
		assert.NoError(t, err)
		assert.True(t, result == nil)
	})

	t.Run("empty_config", func(t *testing.T) {
		config := &ProxyCustomRESTfulResponseConfig{}
		result, err := newCustomRESTResponse(config, goenvconf.GetOSEnv)
		assert.NoError(t, err)
		assert.True(t, result == nil)
	})
}

// TestNewCustomRESTRequestFromConfig tests creating custom REST request
func TestNewCustomRESTRequestFromConfig(t *testing.T) {
	t.Run("nil_config", func(t *testing.T) {
		result, err := newCustomRESTRequestFromConfig(nil, goenvconf.GetOSEnv)
		assert.NoError(t, err)
		assert.True(t, result == nil)
	})

	t.Run("empty_config", func(t *testing.T) {
		config := &ProxyRESTfulRequestConfig{}
		result, err := newCustomRESTRequestFromConfig(config, goenvconf.GetOSEnv)
		assert.NoError(t, err)
		assert.True(t, result == nil)
	})

	t.Run("with_path", func(t *testing.T) {
		config := &ProxyRESTfulRequestConfig{
			URL: "/custom/path",
		}
		result, err := newCustomRESTRequestFromConfig(config, goenvconf.GetOSEnv)
		assert.NoError(t, err)
		assert.True(t, result != nil)
		assert.Equal(t, "/custom/path", result.URL)
	})

	t.Run("with_headers", func(t *testing.T) {
		baseParam := parameter.BaseParameter{
			Name: "X-Custom-Header",
			In:   oaschema.InHeader,
		}
		config := &ProxyRESTfulRequestConfig{
			Parameters: []ProxyRESTfulParameterConfig{
				{
					BaseParameter: parameter.BaseParameter{
						Name: "X-Custom-Header",
						In:   oaschema.InHeader,
					},
					FieldMappingEntryConfig: jmes.FieldMappingEntryConfig{
						Path: new("headers.authorization"),
					},
				},
			},
		}
		result, err := newCustomRESTRequestFromConfig(config, goenvconf.GetOSEnv)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, len(result.Parameters), 1)
		assert.Equal(t, result.Parameters[0].BaseParameter, baseParam)
	})

	t.Run("with_path_and_headers", func(t *testing.T) {
		config := &ProxyRESTfulRequestConfig{
			Parameters: []ProxyRESTfulParameterConfig{
				{
					BaseParameter: parameter.BaseParameter{
						Name: "Authorization",
						In:   oaschema.InHeader,
					},
					FieldMappingEntryConfig: jmes.FieldMappingEntryConfig{
						Path: new("headers.auth"),
					},
				},
			},
			URL: "/api/endpoint",
		}
		result, err := newCustomRESTRequestFromConfig(config, goenvconf.GetOSEnv)
		assert.NoError(t, err)
		assert.True(t, result != nil)
		assert.Equal(t, "/api/endpoint", result.URL)
		assert.True(t, len(result.Parameters) > 0)
	})

	t.Run("invalid_headers_config", func(t *testing.T) {
		config := &ProxyRESTfulRequestConfig{
			Parameters: []ProxyRESTfulParameterConfig{
				{
					BaseParameter: parameter.BaseParameter{
						Name: "X-Invalid",
						In:   10,
					},
				},
			},
		}
		_, err := newCustomRESTRequestFromConfig(config, goenvconf.GetOSEnv)
		assert.True(t, err != nil)
		assert.ErrorContains(t, err, "failed to evaluate the parameter")
	})

	t.Run("with_body_only", func(t *testing.T) {
		bodyConfig := gotransform.TemplateTransformerConfig{}
		config := &ProxyRESTfulRequestConfig{
			Body: &bodyConfig,
		}
		_, err := newCustomRESTRequestFromConfig(config, goenvconf.GetOSEnv)
		// May fail due to invalid body config, which is expected
		if err != nil {
			assert.ErrorContains(t, err, "failed to initialize custom request body")
		}
	})
}
