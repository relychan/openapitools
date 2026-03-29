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
	"github.com/pb33f/libopenapi/datamodel/high/base"
	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/relychan/gohttpc/authc/authscheme"
	"github.com/relychan/openapitools/oaschema"
	"github.com/stretchr/testify/assert"
	"go.yaml.in/yaml/v4"
)

func TestNewOpenAPIv3Authenticator_EmptyDocument(t *testing.T) {
	doc := &highv3.Document{}

	auth, err := NewOpenAPIv3Authenticator(doc, goenvconf.GetOSEnv)
	assert.NoError(t, err)
	assert.True(t, auth != nil)
	assert.Equal(t, 0, len(auth.securitySchemes))
}

func TestNewOpenAPIv3Authenticator_NoSecuritySchemes(t *testing.T) {
	doc := &highv3.Document{
		Components: &highv3.Components{},
	}

	auth, err := NewOpenAPIv3Authenticator(doc, goenvconf.GetOSEnv)
	assert.NoError(t, err)
	assert.True(t, auth != nil)
	assert.Equal(t, 0, len(auth.securitySchemes))
}

func TestNewOpenAPIv3Authenticator_WithAPIKey(t *testing.T) {
	apiKeyValue := goenvconf.NewEnvStringValue("test-api-key")
	creds := APIKeyCredentials{
		APIKey: &apiKeyValue,
	}

	credsData, err := yaml.Marshal(creds)
	assert.NoError(t, err)

	var credsNode yaml.Node
	err = yaml.Unmarshal(credsData, &credsNode)
	assert.NoError(t, err)

	extensions := orderedmap.New[string, *yaml.Node]()
	extensions.Set(oaschema.XRelySecurityCredentials, &credsNode)

	securitySchemes := orderedmap.New[string, *highv3.SecurityScheme]()
	securitySchemes.Set("apiKey", &highv3.SecurityScheme{
		Type:       oaschema.APIKeyScheme.String(),
		Name:       "X-API-Key",
		In:         "header",
		Extensions: extensions,
	})

	doc := &highv3.Document{
		Components: &highv3.Components{
			SecuritySchemes: securitySchemes,
		},
	}

	auth, err := NewOpenAPIv3Authenticator(doc, goenvconf.GetOSEnv)
	assert.NoError(t, err)
	assert.True(t, auth != nil)
	assert.Equal(t, 1, len(auth.securitySchemes))
}

func TestNewOpenAPIv3Authenticator_WithBasicAuth(t *testing.T) {
	username := goenvconf.NewEnvStringValue("test-user")
	password := goenvconf.NewEnvStringValue("test-pass")
	creds := BasicCredentials{
		Username: &username,
		Password: &password,
	}

	credsData, err := yaml.Marshal(creds)
	assert.NoError(t, err)

	var credsNode yaml.Node
	err = yaml.Unmarshal(credsData, &credsNode)
	assert.NoError(t, err)

	extensions := orderedmap.New[string, *yaml.Node]()
	extensions.Set(oaschema.XRelySecurityCredentials, &credsNode)

	securitySchemes := orderedmap.New[string, *highv3.SecurityScheme]()
	securitySchemes.Set("basicAuth", &highv3.SecurityScheme{
		Type:       oaschema.HTTPAuthScheme.String(),
		Scheme:     oaschema.BasicAuthScheme.String(),
		Name:       "Authorization",
		Extensions: extensions,
	})

	doc := &highv3.Document{
		Components: &highv3.Components{
			SecuritySchemes: securitySchemes,
		},
	}

	auth, err := NewOpenAPIv3Authenticator(doc, goenvconf.GetOSEnv)
	assert.NoError(t, err)
	assert.True(t, auth != nil)
	assert.Equal(t, 1, len(auth.securitySchemes))
}

func TestOpenAPIAuthenticator_GetAuthenticator_Empty(t *testing.T) {
	auth := &OpenAPIAuthenticator{
		securitySchemes: make(map[string]authscheme.HTTPClientAuthenticator),
	}

	result := auth.GetAuthenticator(nil)
	assert.True(t, result == nil)
}

func TestOpenAPIAuthenticator_GetAuthenticator_WithOptionalSecurity(t *testing.T) {
	auth := &OpenAPIAuthenticator{
		securitySchemes: make(map[string]authscheme.HTTPClientAuthenticator),
		security: []*base.SecurityRequirement{
			{
				ContainsEmptyRequirement: true,
			},
		},
	}

	result := auth.GetAuthenticator(nil)
	assert.True(t, result == nil)
}

func TestParseStringFromExtensions(t *testing.T) {
	testCases := []struct {
		name          string
		key           string
		value         string
		expectedValue string
		expectError   bool
	}{
		{
			name:          "valid string",
			key:           "test-key",
			value:         "test-value",
			expectedValue: "test-value",
			expectError:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			extensions := orderedmap.New[string, *yaml.Node]()

			var valueNode yaml.Node
			err := yaml.Load([]byte(tc.value), &valueNode)
			assert.NoError(t, err)

			extensions.Set(tc.key, &valueNode)

			result, err := parseStringFromExtensions(extensions, tc.key)

			if tc.expectError {
				assert.True(t, err != nil)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedValue, result)
			}
		})
	}
}

// TestNewOpenAPIv3Authenticator tests creating an authenticator
func TestNewOpenAPIv3Authenticator(t *testing.T) {
	t.Run("with_nil_components", func(t *testing.T) {
		doc := &highv3.Document{}
		auth, err := NewOpenAPIv3Authenticator(doc, goenvconf.GetOSEnv)
		assert.NoError(t, err)
		assert.True(t, auth != nil)
		assert.Equal(t, 0, len(auth.securitySchemes))
	})

	t.Run("with_nil_security_schemes", func(t *testing.T) {
		doc := &highv3.Document{
			Components: &highv3.Components{},
		}
		auth, err := NewOpenAPIv3Authenticator(doc, goenvconf.GetOSEnv)
		assert.NoError(t, err)
		assert.True(t, auth != nil)
		assert.Equal(t, 0, len(auth.securitySchemes))
	})

	t.Run("with_api_key_security_scheme", func(t *testing.T) {
		securitySchemes := orderedmap.New[string, *highv3.SecurityScheme]()
		securitySchemes.Set("api_key", &highv3.SecurityScheme{
			Type: "apiKey",
			Name: "X-API-Key",
			In:   "header",
		})

		doc := &highv3.Document{
			Components: &highv3.Components{
				SecuritySchemes: securitySchemes,
			},
		}

		auth, err := NewOpenAPIv3Authenticator(doc, goenvconf.GetOSEnv)
		assert.NoError(t, err)
		assert.True(t, auth != nil)
		// API key without credentials won't create an authenticator
		assert.Equal(t, 0, len(auth.securitySchemes))
	})

	t.Run("with_http_basic_security_scheme", func(t *testing.T) {
		securitySchemes := orderedmap.New[string, *highv3.SecurityScheme]()
		securitySchemes.Set("basic_auth", &highv3.SecurityScheme{
			Type:   "http",
			Scheme: "basic",
		})

		doc := &highv3.Document{
			Components: &highv3.Components{
				SecuritySchemes: securitySchemes,
			},
		}

		auth, err := NewOpenAPIv3Authenticator(doc, goenvconf.GetOSEnv)
		assert.NoError(t, err)
		assert.True(t, auth != nil)
		// Basic auth without credentials won't create an authenticator
		assert.Equal(t, 0, len(auth.securitySchemes))
	})

	t.Run("with_http_bearer_security_scheme", func(t *testing.T) {
		securitySchemes := orderedmap.New[string, *highv3.SecurityScheme]()
		securitySchemes.Set("bearer_auth", &highv3.SecurityScheme{
			Type:   "http",
			Scheme: "bearer",
		})

		doc := &highv3.Document{
			Components: &highv3.Components{
				SecuritySchemes: securitySchemes,
			},
		}

		auth, err := NewOpenAPIv3Authenticator(doc, goenvconf.GetOSEnv)
		assert.NoError(t, err)
		assert.True(t, auth != nil)
		// Bearer auth without credentials won't create an authenticator
		assert.Equal(t, 0, len(auth.securitySchemes))
	})
}

// TestGetAuthenticatorWithNoSchemes tests getting an authenticator with no schemes
func TestGetAuthenticatorWithNoSchemes(t *testing.T) {
	auth := &OpenAPIAuthenticator{
		securitySchemes: map[string]authscheme.HTTPClientAuthenticator{},
	}

	result := auth.GetAuthenticator(nil)
	assert.True(t, result == nil)
}

// TestGetAuthenticator_WithSecurityRequirements tests GetAuthenticator with various security requirements
func TestGetAuthenticator_WithSecurityRequirements(t *testing.T) {
	t.Run("with_matching_security_requirement", func(t *testing.T) {
		// Create an authenticator with a scheme
		apiKeyValue := goenvconf.NewEnvStringValue("test-key")
		creds := APIKeyCredentials{
			APIKey: &apiKeyValue,
		}
		credsData, _ := yaml.Marshal(creds)
		var credsNode yaml.Node
		_ = yaml.Unmarshal(credsData, &credsNode)

		extensions := orderedmap.New[string, *yaml.Node]()
		extensions.Set(oaschema.XRelySecurityCredentials, &credsNode)

		securitySchemes := orderedmap.New[string, *highv3.SecurityScheme]()
		securitySchemes.Set("api_key", &highv3.SecurityScheme{
			Type:       "apiKey",
			Name:       "X-API-Key",
			In:         "header",
			Extensions: extensions,
		})

		doc := &highv3.Document{
			Components: &highv3.Components{
				SecuritySchemes: securitySchemes,
			},
		}

		auth, err := NewOpenAPIv3Authenticator(doc, goenvconf.GetOSEnv)
		assert.NoError(t, err)
		assert.True(t, auth != nil)

		// Create security requirement
		requirements := orderedmap.New[string, []string]()
		requirements.Set("api_key", []string{})

		security := []*base.SecurityRequirement{
			{
				Requirements: requirements,
			},
		}

		result := auth.GetAuthenticator(security)
		assert.True(t, result != nil)
	})

	t.Run("with_empty_requirement_optional", func(t *testing.T) {
		apiKeyValue := goenvconf.NewEnvStringValue("test-key")
		creds := APIKeyCredentials{
			APIKey: &apiKeyValue,
		}
		credsData, _ := yaml.Marshal(creds)
		var credsNode yaml.Node
		_ = yaml.Unmarshal(credsData, &credsNode)

		extensions := orderedmap.New[string, *yaml.Node]()
		extensions.Set(oaschema.XRelySecurityCredentials, &credsNode)

		securitySchemes := orderedmap.New[string, *highv3.SecurityScheme]()
		securitySchemes.Set("api_key", &highv3.SecurityScheme{
			Type:       "apiKey",
			Name:       "X-API-Key",
			In:         "header",
			Extensions: extensions,
		})

		doc := &highv3.Document{
			Components: &highv3.Components{
				SecuritySchemes: securitySchemes,
			},
		}

		auth, err := NewOpenAPIv3Authenticator(doc, goenvconf.GetOSEnv)
		assert.NoError(t, err)

		security := []*base.SecurityRequirement{
			{
				ContainsEmptyRequirement: true,
			},
		}

		result := auth.GetAuthenticator(security)
		assert.True(t, result == nil)
	})

	t.Run("with_default_authenticator", func(t *testing.T) {
		apiKeyValue := goenvconf.NewEnvStringValue("test-key")
		creds := APIKeyCredentials{
			APIKey: &apiKeyValue,
		}
		credsData, _ := yaml.Marshal(creds)
		var credsNode yaml.Node
		_ = yaml.Unmarshal(credsData, &credsNode)

		extensions := orderedmap.New[string, *yaml.Node]()
		extensions.Set(oaschema.XRelySecurityCredentials, &credsNode)

		securitySchemes := orderedmap.New[string, *highv3.SecurityScheme]()
		securitySchemes.Set("api_key", &highv3.SecurityScheme{
			Type:       "apiKey",
			Name:       "X-API-Key",
			In:         "header",
			Extensions: extensions,
		})

		doc := &highv3.Document{
			Components: &highv3.Components{
				SecuritySchemes: securitySchemes,
			},
		}

		auth, err := NewOpenAPIv3Authenticator(doc, goenvconf.GetOSEnv)
		assert.NoError(t, err)

		// Create security requirement with non-matching name
		requirements := orderedmap.New[string, []string]()
		requirements.Set("other_key", []string{})

		security := []*base.SecurityRequirement{
			{
				Requirements: requirements,
			},
		}

		result := auth.GetAuthenticator(security)
		assert.True(t, result != nil) // Should return default authenticator
	})
}

// TestCreateOAuthAuthenticator tests OAuth2 authenticator creation
func TestCreateOAuthAuthenticator(t *testing.T) {
	t.Run("with_valid_oauth2_client_credentials", func(t *testing.T) {
		clientID := goenvconf.NewEnvStringValue("test-client-id")
		clientSecret := goenvconf.NewEnvStringValue("test-client-secret")
		creds := OAuth2Credentials{
			ClientID:     &clientID,
			ClientSecret: &clientSecret,
		}
		credsData, _ := yaml.Marshal(creds)
		var credsNode yaml.Node
		_ = yaml.Unmarshal(credsData, &credsNode)

		extensions := orderedmap.New[string, *yaml.Node]()
		extensions.Set(oaschema.XRelySecurityCredentials, &credsNode)

		securitySchemes := orderedmap.New[string, *highv3.SecurityScheme]()
		securitySchemes.Set("oauth2", &highv3.SecurityScheme{
			Type: "oauth2",
			Flows: &highv3.OAuthFlows{
				ClientCredentials: &highv3.OAuthFlow{
					TokenUrl:   "https://example.com/token",
					Extensions: extensions,
				},
			},
		})

		doc := &highv3.Document{
			Components: &highv3.Components{
				SecuritySchemes: securitySchemes,
			},
		}

		auth, err := NewOpenAPIv3Authenticator(doc, goenvconf.GetOSEnv)
		assert.NoError(t, err)
		assert.True(t, auth != nil)
		assert.Equal(t, 1, len(auth.securitySchemes))
	})

	t.Run("with_oauth2_missing_client_id", func(t *testing.T) {
		clientSecret := goenvconf.NewEnvStringValue("test-client-secret")
		creds := OAuth2Credentials{
			ClientSecret: &clientSecret,
		}
		credsData, _ := yaml.Marshal(creds)
		var credsNode yaml.Node
		_ = yaml.Unmarshal(credsData, &credsNode)

		extensions := orderedmap.New[string, *yaml.Node]()
		extensions.Set(oaschema.XRelySecurityCredentials, &credsNode)

		securitySchemes := orderedmap.New[string, *highv3.SecurityScheme]()
		securitySchemes.Set("oauth2", &highv3.SecurityScheme{
			Type: "oauth2",
			Flows: &highv3.OAuthFlows{
				ClientCredentials: &highv3.OAuthFlow{
					TokenUrl:   "https://example.com/token",
					Extensions: extensions,
				},
			},
		})

		doc := &highv3.Document{
			Components: &highv3.Components{
				SecuritySchemes: securitySchemes,
			},
		}

		_, err := NewOpenAPIv3Authenticator(doc, goenvconf.GetOSEnv)
		assert.ErrorContains(t, err, "clientId and clientSecret must not be empty")
	})

	t.Run("with_oauth2_no_token_url", func(t *testing.T) {
		clientID := goenvconf.NewEnvStringValue("test-client-id")
		clientSecret := goenvconf.NewEnvStringValue("test-client-secret")
		creds := OAuth2Credentials{
			ClientID:     &clientID,
			ClientSecret: &clientSecret,
		}
		credsData, _ := yaml.Marshal(creds)
		var credsNode yaml.Node
		_ = yaml.Unmarshal(credsData, &credsNode)

		extensions := orderedmap.New[string, *yaml.Node]()
		extensions.Set(oaschema.XRelySecurityCredentials, &credsNode)

		securitySchemes := orderedmap.New[string, *highv3.SecurityScheme]()
		securitySchemes.Set("oauth2", &highv3.SecurityScheme{
			Type: "oauth2",
			Flows: &highv3.OAuthFlows{
				ClientCredentials: &highv3.OAuthFlow{
					Extensions: extensions,
				},
			},
		})

		doc := &highv3.Document{
			Components: &highv3.Components{
				SecuritySchemes: securitySchemes,
			},
		}

		_, err := NewOpenAPIv3Authenticator(doc, goenvconf.GetOSEnv)
		assert.ErrorContains(t, err, "tokenUrl is required")
	})

	t.Run("with_oauth2_no_flows", func(t *testing.T) {
		securitySchemes := orderedmap.New[string, *highv3.SecurityScheme]()
		securitySchemes.Set("oauth2", &highv3.SecurityScheme{
			Type: "oauth2",
		})

		doc := &highv3.Document{
			Components: &highv3.Components{
				SecuritySchemes: securitySchemes,
			},
		}

		auth, err := NewOpenAPIv3Authenticator(doc, goenvconf.GetOSEnv)
		assert.NoError(t, err)
		assert.True(t, auth != nil)
		// Should not create authenticator without flows
		assert.Equal(t, 0, len(auth.securitySchemes))
	})

	t.Run("with_oauth2_refresh_url", func(t *testing.T) {
		clientID := goenvconf.NewEnvStringValue("test-client-id")
		clientSecret := goenvconf.NewEnvStringValue("test-client-secret")
		creds := OAuth2Credentials{
			ClientID:     &clientID,
			ClientSecret: &clientSecret,
		}
		credsData, _ := yaml.Marshal(creds)
		var credsNode yaml.Node
		_ = yaml.Unmarshal(credsData, &credsNode)

		extensions := orderedmap.New[string, *yaml.Node]()
		extensions.Set(oaschema.XRelySecurityCredentials, &credsNode)

		securitySchemes := orderedmap.New[string, *highv3.SecurityScheme]()
		securitySchemes.Set("oauth2", &highv3.SecurityScheme{
			Type: "oauth2",
			Flows: &highv3.OAuthFlows{
				ClientCredentials: &highv3.OAuthFlow{
					TokenUrl:   "https://example.com/token",
					RefreshUrl: "https://example.com/refresh",
					Extensions: extensions,
				},
			},
		})

		doc := &highv3.Document{
			Components: &highv3.Components{
				SecuritySchemes: securitySchemes,
			},
		}

		auth, err := NewOpenAPIv3Authenticator(doc, goenvconf.GetOSEnv)
		assert.NoError(t, err)
		assert.True(t, auth != nil)
		assert.Equal(t, 1, len(auth.securitySchemes))
	})

	t.Run("with_oauth2_custom_token_location", func(t *testing.T) {
		clientID := goenvconf.NewEnvStringValue("test-client-id")
		clientSecret := goenvconf.NewEnvStringValue("test-client-secret")
		creds := OAuth2Credentials{
			ClientID:     &clientID,
			ClientSecret: &clientSecret,
		}
		credsData, _ := yaml.Marshal(creds)
		var credsNode yaml.Node
		_ = yaml.Unmarshal(credsData, &credsNode)

		extensions := orderedmap.New[string, *yaml.Node]()
		extensions.Set(oaschema.XRelySecurityCredentials, &credsNode)

		securitySchemes := orderedmap.New[string, *highv3.SecurityScheme]()
		securitySchemes.Set("oauth2", &highv3.SecurityScheme{
			Type:   "oauth2",
			Name:   "X-Custom-Token",
			In:     "header",
			Scheme: "custom",
			Flows: &highv3.OAuthFlows{
				ClientCredentials: &highv3.OAuthFlow{
					TokenUrl:   "https://example.com/token",
					Extensions: extensions,
				},
			},
		})

		doc := &highv3.Document{
			Components: &highv3.Components{
				SecuritySchemes: securitySchemes,
			},
		}

		auth, err := NewOpenAPIv3Authenticator(doc, goenvconf.GetOSEnv)
		assert.NoError(t, err)
		assert.True(t, auth != nil)
		assert.Equal(t, 1, len(auth.securitySchemes))
	})
}

// TestStaticAuthenticator tests static authenticator creation with various schemes
func TestStaticAuthenticator(t *testing.T) {
	t.Run("with_api_key_in_query", func(t *testing.T) {
		tokenValue := goenvconf.NewEnvStringValue("test-token")
		creds := APIKeyCredentials{
			APIKey: &tokenValue,
		}
		credsData, _ := yaml.Marshal(creds)
		var credsNode yaml.Node
		_ = yaml.Unmarshal(credsData, &credsNode)

		extensions := orderedmap.New[string, *yaml.Node]()
		extensions.Set(oaschema.XRelySecurityCredentials, &credsNode)

		securitySchemes := orderedmap.New[string, *highv3.SecurityScheme]()
		securitySchemes.Set("api_key", &highv3.SecurityScheme{
			Type:       "apiKey",
			In:         "query",
			Name:       "access_token",
			Extensions: extensions,
		})

		doc := &highv3.Document{
			Components: &highv3.Components{
				SecuritySchemes: securitySchemes,
			},
		}

		auth, err := NewOpenAPIv3Authenticator(doc, goenvconf.GetOSEnv)
		assert.NoError(t, err)
		assert.True(t, auth != nil)
		assert.Equal(t, 1, len(auth.securitySchemes))
	})

	t.Run("with_api_key_in_cookie", func(t *testing.T) {
		tokenValue := goenvconf.NewEnvStringValue("test-token")
		creds := APIKeyCredentials{
			APIKey: &tokenValue,
		}
		credsData, _ := yaml.Marshal(creds)
		var credsNode yaml.Node
		_ = yaml.Unmarshal(credsData, &credsNode)

		extensions := orderedmap.New[string, *yaml.Node]()
		extensions.Set(oaschema.XRelySecurityCredentials, &credsNode)

		securitySchemes := orderedmap.New[string, *highv3.SecurityScheme]()
		securitySchemes.Set("api_key", &highv3.SecurityScheme{
			Type:       "apiKey",
			In:         "cookie",
			Name:       "auth_token",
			Extensions: extensions,
		})

		doc := &highv3.Document{
			Components: &highv3.Components{
				SecuritySchemes: securitySchemes,
			},
		}

		auth, err := NewOpenAPIv3Authenticator(doc, goenvconf.GetOSEnv)
		assert.NoError(t, err)
		assert.True(t, auth != nil)
		assert.Equal(t, 1, len(auth.securitySchemes))
	})

	t.Run("with_invalid_location", func(t *testing.T) {
		tokenValue := goenvconf.NewEnvStringValue("test-token")
		creds := APIKeyCredentials{
			APIKey: &tokenValue,
		}
		credsData, _ := yaml.Marshal(creds)
		var credsNode yaml.Node
		_ = yaml.Unmarshal(credsData, &credsNode)

		extensions := orderedmap.New[string, *yaml.Node]()
		extensions.Set(oaschema.XRelySecurityCredentials, &credsNode)

		securitySchemes := orderedmap.New[string, *highv3.SecurityScheme]()
		securitySchemes.Set("api_key", &highv3.SecurityScheme{
			Type:       "apiKey",
			In:         "invalid_location",
			Name:       "token",
			Extensions: extensions,
		})

		doc := &highv3.Document{
			Components: &highv3.Components{
				SecuritySchemes: securitySchemes,
			},
		}

		_, err := NewOpenAPIv3Authenticator(doc, goenvconf.GetOSEnv)
		assert.ErrorContains(t, err, "invalid AuthLocation")
	})
}

// TestUnsupportedSecuritySchemes tests unsupported security scheme types
func TestUnsupportedSecuritySchemes(t *testing.T) {
	t.Run("with_openIdConnect_scheme", func(t *testing.T) {
		securitySchemes := orderedmap.New[string, *highv3.SecurityScheme]()
		securitySchemes.Set("oidc", &highv3.SecurityScheme{
			Type:             "openIdConnect",
			OpenIdConnectUrl: "https://example.com/.well-known/openid-configuration",
		})

		doc := &highv3.Document{
			Components: &highv3.Components{
				SecuritySchemes: securitySchemes,
			},
		}

		auth, err := NewOpenAPIv3Authenticator(doc, goenvconf.GetOSEnv)
		assert.NoError(t, err)
		assert.True(t, auth != nil)
		// OpenID Connect is not supported, so no authenticator should be created
		assert.Equal(t, 0, len(auth.securitySchemes))
	})

	t.Run("with_mutualTLS_scheme", func(t *testing.T) {
		securitySchemes := orderedmap.New[string, *highv3.SecurityScheme]()
		securitySchemes.Set("mtls", &highv3.SecurityScheme{
			Type: "mutualTLS",
		})

		doc := &highv3.Document{
			Components: &highv3.Components{
				SecuritySchemes: securitySchemes,
			},
		}

		auth, err := NewOpenAPIv3Authenticator(doc, goenvconf.GetOSEnv)
		assert.NoError(t, err)
		assert.True(t, auth != nil)
		// Mutual TLS is not supported, so no authenticator should be created
		assert.Equal(t, 0, len(auth.securitySchemes))
	})
}

// TestSecuritySchemesWithoutCredentials tests security schemes without credentials
func TestSecuritySchemesWithoutCredentials(t *testing.T) {
	t.Run("with_api_key_no_credentials", func(t *testing.T) {
		securitySchemes := orderedmap.New[string, *highv3.SecurityScheme]()
		securitySchemes.Set("api_key", &highv3.SecurityScheme{
			Type: "apiKey",
			Name: "X-API-Key",
			In:   "header",
		})

		doc := &highv3.Document{
			Components: &highv3.Components{
				SecuritySchemes: securitySchemes,
			},
		}

		auth, err := NewOpenAPIv3Authenticator(doc, goenvconf.GetOSEnv)
		assert.NoError(t, err)
		assert.True(t, auth != nil)
		// No credentials, so no authenticator should be created
		assert.Equal(t, 0, len(auth.securitySchemes))
	})

	t.Run("with_basic_auth_no_credentials", func(t *testing.T) {
		securitySchemes := orderedmap.New[string, *highv3.SecurityScheme]()
		securitySchemes.Set("basic", &highv3.SecurityScheme{
			Type:   "http",
			Scheme: "basic",
		})

		doc := &highv3.Document{
			Components: &highv3.Components{
				SecuritySchemes: securitySchemes,
			},
		}

		auth, err := NewOpenAPIv3Authenticator(doc, goenvconf.GetOSEnv)
		assert.NoError(t, err)
		assert.True(t, auth != nil)
		// No credentials, so no authenticator should be created
		assert.Equal(t, 0, len(auth.securitySchemes))
	})

	t.Run("with_basic_auth_empty_credentials", func(t *testing.T) {
		creds := BasicCredentials{}
		credsData, _ := yaml.Marshal(creds)
		var credsNode yaml.Node
		_ = yaml.Unmarshal(credsData, &credsNode)

		extensions := orderedmap.New[string, *yaml.Node]()
		extensions.Set(oaschema.XRelySecurityCredentials, &credsNode)

		securitySchemes := orderedmap.New[string, *highv3.SecurityScheme]()
		securitySchemes.Set("basic", &highv3.SecurityScheme{
			Type:       "http",
			Scheme:     "basic",
			Extensions: extensions,
		})

		doc := &highv3.Document{
			Components: &highv3.Components{
				SecuritySchemes: securitySchemes,
			},
		}

		auth, err := NewOpenAPIv3Authenticator(doc, goenvconf.GetOSEnv)
		assert.NoError(t, err)
		assert.True(t, auth != nil)
		// Empty credentials, so no authenticator should be created
		assert.Equal(t, 0, len(auth.securitySchemes))
	})

	t.Run("with_api_key_empty_credentials", func(t *testing.T) {
		creds := APIKeyCredentials{}
		credsData, _ := yaml.Marshal(creds)
		var credsNode yaml.Node
		_ = yaml.Unmarshal(credsData, &credsNode)

		extensions := orderedmap.New[string, *yaml.Node]()
		extensions.Set(oaschema.XRelySecurityCredentials, &credsNode)

		securitySchemes := orderedmap.New[string, *highv3.SecurityScheme]()
		securitySchemes.Set("api_key", &highv3.SecurityScheme{
			Type:       "apiKey",
			Name:       "X-API-Key",
			In:         "header",
			Extensions: extensions,
		})

		doc := &highv3.Document{
			Components: &highv3.Components{
				SecuritySchemes: securitySchemes,
			},
		}

		auth, err := NewOpenAPIv3Authenticator(doc, goenvconf.GetOSEnv)
		assert.NoError(t, err)
		assert.True(t, auth != nil)
		// Empty credentials, so no authenticator should be created
		assert.Equal(t, 0, len(auth.securitySchemes))
	})
}

// TestHTTPBearerScheme tests HTTP bearer authentication
func TestHTTPBearerScheme(t *testing.T) {
	t.Run("with_http_bearer_token", func(t *testing.T) {
		tokenValue := goenvconf.NewEnvStringValue("test-bearer-token")
		creds := APIKeyCredentials{
			APIKey: &tokenValue,
		}
		credsData, _ := yaml.Marshal(creds)
		var credsNode yaml.Node
		_ = yaml.Unmarshal(credsData, &credsNode)

		extensions := orderedmap.New[string, *yaml.Node]()
		extensions.Set(oaschema.XRelySecurityCredentials, &credsNode)

		securitySchemes := orderedmap.New[string, *highv3.SecurityScheme]()
		securitySchemes.Set("bearer", &highv3.SecurityScheme{
			Type:       "http",
			Scheme:     "bearer",
			Name:       "Authorization",
			Extensions: extensions,
		})

		doc := &highv3.Document{
			Components: &highv3.Components{
				SecuritySchemes: securitySchemes,
			},
		}

		auth, err := NewOpenAPIv3Authenticator(doc, goenvconf.GetOSEnv)
		assert.NoError(t, err)
		assert.True(t, auth != nil)
		assert.Equal(t, 1, len(auth.securitySchemes))
	})

	t.Run("with_http_custom_scheme", func(t *testing.T) {
		tokenValue := goenvconf.NewEnvStringValue("test-token")
		creds := APIKeyCredentials{
			APIKey: &tokenValue,
		}
		credsData, _ := yaml.Marshal(creds)
		var credsNode yaml.Node
		_ = yaml.Unmarshal(credsData, &credsNode)

		extensions := orderedmap.New[string, *yaml.Node]()
		extensions.Set(oaschema.XRelySecurityCredentials, &credsNode)

		securitySchemes := orderedmap.New[string, *highv3.SecurityScheme]()
		securitySchemes.Set("custom", &highv3.SecurityScheme{
			Type:       "http",
			Scheme:     "custom-scheme",
			Name:       "X-Custom-Auth",
			Extensions: extensions,
		})

		doc := &highv3.Document{
			Components: &highv3.Components{
				SecuritySchemes: securitySchemes,
			},
		}

		auth, err := NewOpenAPIv3Authenticator(doc, goenvconf.GetOSEnv)
		assert.NoError(t, err)
		assert.True(t, auth != nil)
		assert.Equal(t, 1, len(auth.securitySchemes))
	})
}
