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

package oaschema

import (
	"encoding/json"
	"testing"

	"github.com/hasura/goenvconf"
	"github.com/stretchr/testify/assert"
	"go.yaml.in/yaml/v4"
)

func TestOpenAPIResourceSettings_JSONMarshal(t *testing.T) {
	testKey := "test-key"
	config := OpenAPIResourceSettings{
		BasePath: "/api/v1",
		Headers: map[string]goenvconf.EnvString{
			"X-API-Key": {Value: &testKey},
		},
	}

	data, err := json.Marshal(config)
	assert.NoError(t, err)

	var result OpenAPIResourceSettings
	err = json.Unmarshal(data, &result)
	assert.NoError(t, err)
	assert.Equal(t, "/api/v1", result.BasePath)
	assert.Equal(t, 1, len(result.Headers))
}

func TestOpenAPIResourceSettings_YAMLMarshal(t *testing.T) {
	config := OpenAPIResourceSettings{
		BasePath: "/api/v2",
	}

	data, err := yaml.Marshal(config)
	assert.NoError(t, err)

	var result OpenAPIResourceSettings
	err = yaml.Unmarshal(data, &result)
	assert.NoError(t, err)
	assert.Equal(t, "/api/v2", result.BasePath)
}

func TestOpenAPIResourceSettings_JSONUnmarshal(t *testing.T) {
	testCases := []struct {
		name        string
		jsonData    string
		expectError bool
		checkFunc   func(*testing.T, *OpenAPIResourceSettings)
	}{
		{
			name: "complete settings",
			jsonData: `{
				"basePath": "/api/v1",
				"forwardHeaders": {
					"request": ["Authorization", "X-Request-ID"],
					"response": ["X-Response-ID"]
				}
			}`,
			expectError: false,
			checkFunc: func(t *testing.T, settings *OpenAPIResourceSettings) {
				assert.Equal(t, "/api/v1", settings.BasePath)
				assert.True(t, settings.ForwardHeaders != nil)
				assert.Equal(t, 2, len(settings.ForwardHeaders.Request))
				assert.Equal(t, 1, len(settings.ForwardHeaders.Response))
			},
		},
		{
			name: "settings with health check",
			jsonData: `{
				"basePath": "/api",
				"healthCheck": {
					"http": {
						"path": "/health",
						"interval": 30,
						"timeout": 5
					}
				}
			}`,
			expectError: false,
			checkFunc: func(t *testing.T, settings *OpenAPIResourceSettings) {
				assert.Equal(t, "/api", settings.BasePath)
				assert.True(t, settings.HealthCheck != nil)
				assert.True(t, settings.HealthCheck.HTTP != nil)
				assert.Equal(t, "/health", settings.HealthCheck.HTTP.Path)
			},
		},
		{
			name:        "empty settings",
			jsonData:    `{}`,
			expectError: false,
			checkFunc: func(t *testing.T, settings *OpenAPIResourceSettings) {
				assert.Equal(t, "", settings.BasePath)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var settings OpenAPIResourceSettings
			err := json.Unmarshal([]byte(tc.jsonData), &settings)

			if tc.expectError {
				assert.True(t, err != nil)
			} else {
				assert.NoError(t, err)
				if tc.checkFunc != nil {
					tc.checkFunc(t, &settings)
				}
			}
		})
	}
}

func TestOpenAPIResourceSettings_YAMLUnmarshal(t *testing.T) {
	testCases := []struct {
		name        string
		yamlData    string
		expectError bool
		checkFunc   func(*testing.T, *OpenAPIResourceSettings)
	}{
		{
			name: "complete settings",
			yamlData: `basePath: /api/v1
forwardHeaders:
  request:
    - Authorization
    - X-Request-ID
  response:
    - X-Response-ID`,
			expectError: false,
			checkFunc: func(t *testing.T, settings *OpenAPIResourceSettings) {
				assert.Equal(t, "/api/v1", settings.BasePath)
				assert.True(t, settings.ForwardHeaders != nil)
				assert.Equal(t, 2, len(settings.ForwardHeaders.Request))
				assert.Equal(t, 1, len(settings.ForwardHeaders.Response))
			},
		},
		{
			name:        "empty settings",
			yamlData:    `{}`,
			expectError: false,
			checkFunc: func(t *testing.T, settings *OpenAPIResourceSettings) {
				assert.Equal(t, "", settings.BasePath)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var settings OpenAPIResourceSettings
			err := yaml.Unmarshal([]byte(tc.yamlData), &settings)

			if tc.expectError {
				assert.True(t, err != nil)
			} else {
				assert.NoError(t, err)
				if tc.checkFunc != nil {
					tc.checkFunc(t, &settings)
				}
			}
		})
	}
}

func TestOpenAPIForwardHeadersConfig_JSONMarshal(t *testing.T) {
	config := OpenAPIForwardHeadersConfig{
		Request:  []string{"Authorization", "X-Request-ID"},
		Response: []string{"X-Response-ID", "X-Trace-ID"},
	}

	data, err := json.Marshal(config)
	assert.NoError(t, err)

	var result OpenAPIForwardHeadersConfig
	err = json.Unmarshal(data, &result)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(result.Request))
	assert.Equal(t, 2, len(result.Response))
	assert.Equal(t, "Authorization", result.Request[0])
	assert.Equal(t, "X-Response-ID", result.Response[0])
}

func TestOpenAPIForwardHeadersConfig_YAMLMarshal(t *testing.T) {
	config := OpenAPIForwardHeadersConfig{
		Request:  []string{"Authorization"},
		Response: []string{"X-Response-ID"},
	}

	data, err := yaml.Marshal(config)
	assert.NoError(t, err)

	var result OpenAPIForwardHeadersConfig
	err = yaml.Unmarshal(data, &result)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(result.Request))
	assert.Equal(t, 1, len(result.Response))
}
