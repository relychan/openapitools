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
