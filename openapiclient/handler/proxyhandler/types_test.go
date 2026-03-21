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

package proxyhandler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hasura/goenvconf"
	"github.com/stretchr/testify/assert"
)

func TestNewProxyHandlerOptions_GetEnvFunc(t *testing.T) {
	testCases := []struct {
		name     string
		options  NewProxyHandlerOptions
		expected bool // true if should return custom func, false if should return default
	}{
		{
			name: "with custom GetEnv function",
			options: NewProxyHandlerOptions{
				GetEnv: func(key string) (string, error) {
					return "custom-value", nil
				},
			},
			expected: true,
		},
		{
			name: "without GetEnv function",
			options: NewProxyHandlerOptions{
				Method: "GET",
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			getEnvFunc := tc.options.GetEnvFunc()
			assert.True(t, getEnvFunc != nil)

			if tc.expected {
				// Test custom function
				val, err := getEnvFunc("test")
				assert.NoError(t, err)
				assert.Equal(t, "custom-value", val)
			} else {
				// Test default function (should be goenvconf.GetOSEnv)
				assert.True(t, getEnvFunc != nil)
			}
		})
	}
}

func TestAPIKeyCredentials(t *testing.T) {
	apiKey := goenvconf.NewEnvStringValue("test-api-key")
	creds := APIKeyCredentials{
		APIKey: &apiKey,
	}

	assert.True(t, creds.APIKey != nil)
	val, err := creds.APIKey.GetCustom(goenvconf.GetOSEnv)
	assert.NoError(t, err)
	assert.Equal(t, "test-api-key", val)
}

func TestBasicCredentials(t *testing.T) {
	username := goenvconf.NewEnvStringValue("test-user")
	password := goenvconf.NewEnvStringValue("test-pass")

	creds := BasicCredentials{
		Username: &username,
		Password: &password,
	}

	assert.True(t, creds.Username != nil)
	assert.True(t, creds.Password != nil)

	user, err := creds.Username.GetCustom(goenvconf.GetOSEnv)
	assert.NoError(t, err)
	assert.Equal(t, "test-user", user)

	pass, err := creds.Password.GetCustom(goenvconf.GetOSEnv)
	assert.NoError(t, err)
	assert.Equal(t, "test-pass", pass)
}

func TestOAuth2Credentials(t *testing.T) {
	clientID := goenvconf.NewEnvStringValue("client-id")
	clientSecret := goenvconf.NewEnvStringValue("client-secret")

	creds := OAuth2Credentials{
		ClientID:     &clientID,
		ClientSecret: &clientSecret,
		EndpointParams: map[string]goenvconf.EnvString{
			"scope": goenvconf.NewEnvStringValue("read write"),
		},
	}

	assert.True(t, creds.ClientID != nil)
	assert.True(t, creds.ClientSecret != nil)
	assert.Equal(t, 1, len(creds.EndpointParams))

	id, err := creds.ClientID.GetCustom(goenvconf.GetOSEnv)
	assert.NoError(t, err)
	assert.Equal(t, "client-id", id)

	secret, err := creds.ClientSecret.GetCustom(goenvconf.GetOSEnv)
	assert.NoError(t, err)
	assert.Equal(t, "client-secret", secret)

	scope, err := creds.EndpointParams["scope"].GetCustom(goenvconf.GetOSEnv)
	assert.NoError(t, err)
	assert.Equal(t, "read write", scope)
}

func TestInsertRouteOptions(t *testing.T) {
	customGetEnv := func(key string) (string, error) {
		return "custom-value", nil
	}

	options := InsertRouteOptions{
		GetEnv: customGetEnv,
	}

	assert.True(t, options.GetEnv != nil)
	val, err := options.GetEnv("test")
	assert.NoError(t, err)
	assert.Equal(t, "custom-value", val)
}

func TestOAuth2CredentialsErrors(t *testing.T) {
	t.Run("errOAuth2ClientCredentialsRequired", func(t *testing.T) {
		err := errOAuth2ClientCredentialsRequired
		assert.ErrorContains(t, err, "clientId and clientSecret")
	})

	t.Run("errOAuth2TokenURLRequired", func(t *testing.T) {
		err := errOAuth2TokenURLRequired
		assert.ErrorContains(t, err, "tokenUrl")
	})
}

// TestNewRequestTemplateData tests creating request template data
func TestNewRequestTemplateData(t *testing.T) {
	t.Run("with_json_body", func(t *testing.T) {
		body := map[string]any{
			"name":  "test",
			"value": 123,
		}
		bodyBytes, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")

		paramValues := map[string]string{
			"id": "123",
		}

		data, alreadyRead, err := NewRequestTemplateData(req, "application/json", paramValues)
		assert.NoError(t, err)
		assert.True(t, alreadyRead)
		assert.True(t, data != nil)
		assert.Equal(t, "123", data.Params["id"])

		// Body is parsed as map[string]any
		bodyMap, ok := data.Body.(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, "test", bodyMap["name"])
		assert.Equal(t, float64(123), bodyMap["value"])
	})

	t.Run("with_empty_body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		paramValues := map[string]string{
			"userId": "456",
		}

		data, alreadyRead, err := NewRequestTemplateData(req, "", paramValues)
		assert.NoError(t, err)
		assert.True(t, alreadyRead)
		assert.True(t, data != nil)
		assert.Equal(t, "456", data.Params["userId"])
		assert.True(t, data.Body == nil)
	})

	t.Run("with_query_parameters", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test?search=query&limit=10", nil)
		paramValues := map[string]string{}

		data, alreadyRead, err := NewRequestTemplateData(req, "", paramValues)
		assert.NoError(t, err)
		assert.True(t, alreadyRead)
		assert.True(t, data != nil)
		assert.Equal(t, "query", data.QueryParams["search"][0])
		assert.Equal(t, "10", data.QueryParams["limit"][0])
	})

	t.Run("with_headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Custom-Header", "custom-value")
		req.Header.Set("Authorization", "Bearer token")
		paramValues := map[string]string{}

		data, alreadyRead, err := NewRequestTemplateData(req, "", paramValues)
		assert.NoError(t, err)
		assert.True(t, alreadyRead)
		assert.True(t, data != nil)
		assert.Equal(t, "custom-value", data.Headers["x-custom-header"])
		assert.Equal(t, "Bearer token", data.Headers["authorization"])
	})

	t.Run("with_invalid_json_body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader([]byte("invalid json")))
		req.Header.Set("Content-Type", "application/json")
		paramValues := map[string]string{}

		_, _, err := NewRequestTemplateData(req, "application/json", paramValues)
		assert.True(t, err != nil)
	})

	t.Run("with_unsupported_content_type", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader([]byte("some data")))
		req.Header.Set("Content-Type", "text/plain")
		paramValues := map[string]string{}

		data, alreadyRead, err := NewRequestTemplateData(req, "text/plain", paramValues)
		assert.NoError(t, err)
		assert.True(t, !alreadyRead)
		assert.True(t, data != nil)
		assert.True(t, data.Body == nil)
	})
}

// TestRequestTemplateDataToMap tests converting request template data to map
func TestRequestTemplateDataToMap(t *testing.T) {
	t.Run("with_all_fields", func(t *testing.T) {
		data := &RequestTemplateData{
			Params: map[string]string{
				"id": "123",
			},
			QueryParams: map[string][]string{
				"search": {"query"},
			},
			Headers: map[string]string{
				"x-test": "value",
			},
			Body: map[string]any{
				"name": "test",
			},
		}

		result := data.ToMap()
		assert.True(t, result != nil)
		assert.Equal(t, data.Params, result["param"])
		assert.Equal(t, data.QueryParams, result["query"])
		assert.Equal(t, data.Headers, result["headers"])
		assert.Equal(t, data.Body, result["body"])
	})

	t.Run("with_empty_fields", func(t *testing.T) {
		data := &RequestTemplateData{}
		result := data.ToMap()
		assert.True(t, result != nil)
		assert.True(t, result["param"] != nil)
		assert.True(t, result["query"] != nil)
		assert.True(t, result["headers"] != nil)
		assert.True(t, result["body"] == nil)
	})
}
