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

	"github.com/stretchr/testify/assert"
	"go.yaml.in/yaml/v4"
)

func TestOpenAPIResourceDefinition_UnmarshalJSON(t *testing.T) {
	testCases := []struct {
		name        string
		jsonData    string
		expectError bool
		checkFunc   func(*testing.T, *OpenAPIResourceDefinition)
	}{
		{
			name: "valid minimal spec",
			jsonData: `{
				"spec": {
					"openapi": "3.0.0",
					"info": {
						"title": "Test API",
						"version": "1.0.0"
					},
					"paths": {}
				}
			}`,
			expectError: false,
			checkFunc: func(t *testing.T, def *OpenAPIResourceDefinition) {
				assert.True(t, def.Spec != nil)
				assert.Equal(t, "Test API", def.Spec.Info.Title)
				assert.Equal(t, "1.0.0", def.Spec.Info.Version)
			},
		},
		{
			name: "valid spec with settings",
			jsonData: `{
				"settings": {
					"basePath": "/api/v1"
				},
				"spec": {
					"openapi": "3.0.0",
					"info": {
						"title": "Test API",
						"version": "1.0.0"
					},
					"paths": {}
				}
			}`,
			expectError: false,
			checkFunc: func(t *testing.T, def *OpenAPIResourceDefinition) {
				assert.True(t, def.Spec != nil)
				assert.Equal(t, "/api/v1", def.Settings.BasePath)
				assert.Equal(t, "Test API", def.Spec.Info.Title)
			},
		},
		{
			name: "valid spec with servers",
			jsonData: `{
				"spec": {
					"openapi": "3.0.0",
					"info": {
						"title": "Test API",
						"version": "1.0.0"
					},
					"servers": [
						{
							"url": "{SERVER_URL}",
							"variables": {
								"SERVER_URL": {
									"default": "https://api.example.com"
								}
							}
						}
					],
					"paths": {}
				}
			}`,
			expectError: false,
			checkFunc: func(t *testing.T, def *OpenAPIResourceDefinition) {
				assert.True(t, def.Spec != nil)
				assert.True(t, def.Spec.Servers != nil)
				assert.Equal(t, "{SERVER_URL}", def.Spec.Servers[0].URL)
				serverVariable, _ := def.Spec.Servers[0].Variables.Get("SERVER_URL")
				assert.Equal(t, "https://api.example.com", serverVariable.Default)
			},
		},
		{
			name: "missing spec",
			jsonData: `{
				"settings": {
					"basePath": "/api/v1"
				}
			}`,
			expectError: false,
			checkFunc:   nil,
		},
		{
			name:        "null spec",
			jsonData:    `{"spec": null}`,
			expectError: true,
			checkFunc:   nil,
		},
		{
			name:        "empty object",
			jsonData:    `{}`,
			expectError: false,
			checkFunc:   nil,
		},
		{
			name: "invalid spec format",
			jsonData: `{
				"spec": {
					"invalid": "data"
				}
			}`,
			expectError: true,
			checkFunc:   nil,
		},
		{
			name:        "invalid json",
			jsonData:    `{"spec": invalid}`,
			expectError: true,
			checkFunc:   nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var def OpenAPIResourceDefinition
			err := json.Unmarshal([]byte(tc.jsonData), &def)
			if tc.expectError {
				assert.True(t, err != nil, "expected error but got nil")
			} else {
				assert.NoError(t, err)
				if tc.checkFunc != nil {
					tc.checkFunc(t, &def)
				}
			}
		})
	}
}

func TestOpenAPIResourceDefinition_UnmarshalYAML(t *testing.T) {
	testCases := []struct {
		name        string
		yamlData    string
		expectError bool
		checkFunc   func(*testing.T, *OpenAPIResourceDefinition)
	}{
		{
			name: "valid minimal spec with servers and paths",
			yamlData: `spec:
  openapi: "3.0.0"
  info:
    title: Test API
    version: "1.0.0"
  servers:
    - url: "{SERVER_URL}"
      variables:
        SERVER_URL: 
          default: https://api.example.com
  paths:
    /users:
      get:
        operationId: getUsers`,
			expectError: false,
			checkFunc: func(t *testing.T, def *OpenAPIResourceDefinition) {
				assert.True(t, def.Spec != nil)
				assert.True(t, def.Spec.Servers != nil)
				assert.True(t, def.Spec.Paths != nil)
				assert.Equal(t, "{SERVER_URL}", def.Spec.Servers[0].URL)
				serverVariable, _ := def.Spec.Servers[0].Variables.Get("SERVER_URL")
				assert.Equal(t, "https://api.example.com", serverVariable.Default)
			},
		},
		{
			name: "valid spec with settings",
			yamlData: `settings:
  basePath: /api/v1
spec:
  openapi: "3.0.0"
  info:
    title: Test API
    version: "1.0.0"
  servers:
    - url: https://api.example.com
  paths: {}`,
			expectError: false,
			checkFunc: func(t *testing.T, def *OpenAPIResourceDefinition) {
				assert.True(t, def.Spec != nil)
				assert.Equal(t, "/api/v1", def.Settings.BasePath)
			},
		},
		{
			name: "missing spec",
			yamlData: `settings:
  basePath: /api/v1`,
			expectError: false,
			checkFunc:   nil,
		},
		{
			name:        "null spec",
			yamlData:    `spec: null`,
			expectError: true,
			checkFunc:   nil,
		},
		{
			name:        "empty object",
			yamlData:    `{}`,
			expectError: false,
			checkFunc:   nil,
		},
		{
			name: "invalid spec format",
			yamlData: `spec:
  invalid: data`,
			expectError: true,
			checkFunc:   nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var def OpenAPIResourceDefinition
			err := yaml.Unmarshal([]byte(tc.yamlData), &def)

			if tc.expectError {
				assert.True(t, err != nil, "expected error but got nil")
			} else {
				assert.NoError(t, err)
				if tc.checkFunc != nil {
					tc.checkFunc(t, &def)
				}
			}
		})
	}
}
