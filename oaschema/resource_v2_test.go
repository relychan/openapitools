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
	"testing"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	v2high "github.com/pb33f/libopenapi/datamodel/high/v2"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/stretchr/testify/assert"
)

// TestConvertSwaggerToOpenAPIv3Document tests the main conversion function
func TestConvertSwaggerToOpenAPIv3Document(t *testing.T) {
	t.Run("minimal_swagger", func(t *testing.T) {
		swagger := &v2high.Swagger{
			Info: &base.Info{
				Title:   "Test API",
				Version: "1.0.0",
			},
		}

		result, err := convertSwaggerToOpenAPIv3Document(swagger)
		assert.NoError(t, err)
		assert.True(t, result != nil)
		assert.Equal(t, "3.0.0", result.Version)
		assert.Equal(t, "Test API", result.Info.Title)
		assert.Equal(t, "1.0.0", result.Info.Version)
		assert.True(t, result.Components != nil)
	})

	t.Run("with_host_and_basepath", func(t *testing.T) {
		swagger := &v2high.Swagger{
			Info: &base.Info{
				Title:   "Test API",
				Version: "1.0.0",
			},
			Host:     "api.example.com",
			BasePath: "/v1",
			Schemes:  []string{"https"},
		}

		result, err := convertSwaggerToOpenAPIv3Document(swagger)
		assert.NoError(t, err)
		assert.True(t, result != nil)
		assert.True(t, len(result.Servers) > 0)
		assert.Equal(t, "https://api.example.com/v1", result.Servers[0].URL)
	})

	t.Run("with_http_scheme", func(t *testing.T) {
		swagger := &v2high.Swagger{
			Info: &base.Info{
				Title:   "Test API",
				Version: "1.0.0",
			},
			Host:     "api.example.com",
			BasePath: "/v1",
			Schemes:  []string{"http"},
		}

		result, err := convertSwaggerToOpenAPIv3Document(swagger)
		assert.NoError(t, err)
		assert.True(t, result != nil)
		assert.True(t, len(result.Servers) > 0)
		assert.Equal(t, "http://api.example.com/v1", result.Servers[0].URL)
	})

	t.Run("with_root_basepath", func(t *testing.T) {
		swagger := &v2high.Swagger{
			Info: &base.Info{
				Title:   "Test API",
				Version: "1.0.0",
			},
			Host:     "api.example.com",
			BasePath: "/",
			Schemes:  []string{"https"},
		}

		result, err := convertSwaggerToOpenAPIv3Document(swagger)
		assert.NoError(t, err)
		assert.True(t, result != nil)
		assert.True(t, len(result.Servers) > 0)
		// Root basepath is included
		assert.Equal(t, "https://api.example.com", result.Servers[0].URL)
	})

	t.Run("with_definitions", func(t *testing.T) {
		schemas := orderedmap.New[string, *base.SchemaProxy]()
		schemas.Set("User", base.CreateSchemaProxy(&base.Schema{
			Type: []string{"object"},
		}))

		swagger := &v2high.Swagger{
			Info: &base.Info{
				Title:   "Test API",
				Version: "1.0.0",
			},
			Definitions: &v2high.Definitions{
				Definitions: schemas,
			},
		}

		result, err := convertSwaggerToOpenAPIv3Document(swagger)
		assert.NoError(t, err)
		assert.True(t, result != nil)
		assert.True(t, result.Components.Schemas != nil)
		assert.Equal(t, 1, result.Components.Schemas.Len())
	})
}

// TestGetStyleFromCollectionFormat tests the style conversion function
func TestGetStyleFromCollectionFormat(t *testing.T) {
	testCases := []struct {
		name             string
		location         ParameterLocation
		collectionFormat string
		expectedStyle    string
		expectedExplode  bool
	}{
		{
			name:             "path_simple",
			location:         InPath,
			collectionFormat: "",
			expectedStyle:    "simple",
			expectedExplode:  false,
		},
		{
			name:             "path_multi",
			location:         InPath,
			collectionFormat: "multi",
			expectedStyle:    "simple",
			expectedExplode:  true,
		},
		{
			name:             "header_simple",
			location:         InHeader,
			collectionFormat: "",
			expectedStyle:    "simple",
			expectedExplode:  false,
		},
		{
			name:             "query_default",
			location:         InQuery,
			collectionFormat: "",
			expectedStyle:    "form",
			expectedExplode:  false,
		},
		{
			name:             "query_ssv",
			location:         InQuery,
			collectionFormat: "ssv",
			expectedStyle:    "spaceDelimited",
			expectedExplode:  false,
		},
		{
			name:             "query_tsv",
			location:         InQuery,
			collectionFormat: "tsv",
			expectedStyle:    "pipeDelimited",
			expectedExplode:  false,
		},
		{
			name:             "query_pipes",
			location:         InQuery,
			collectionFormat: "pipes",
			expectedStyle:    "pipeDelimited",
			expectedExplode:  false,
		},
		{
			name:             "query_multi",
			location:         InQuery,
			collectionFormat: "multi",
			expectedStyle:    "form",
			expectedExplode:  true,
		},
		{
			name:             "cookie_default",
			location:         InCookie,
			collectionFormat: "",
			expectedStyle:    "form",
			expectedExplode:  false,
		},
		{
			name:             "cookie_multi",
			location:         InCookie,
			collectionFormat: "multi",
			expectedStyle:    "form",
			expectedExplode:  true,
		},
		{
			name:             "unknown_location",
			location:         10,
			collectionFormat: "",
			expectedStyle:    "",
			expectedExplode:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			style, explode := getStyleFromCollectionFormat(tc.location, tc.collectionFormat)
			assert.Equal(t, tc.expectedStyle, style)
			assert.Equal(t, tc.expectedExplode, explode)
		})
	}
}

// TestConvertSecurityScheme tests security scheme conversion
func TestConvertSecurityScheme(t *testing.T) {
	converter := &swaggerToOpenAPIv3Converter{}

	t.Run("basic_auth", func(t *testing.T) {
		v2Scheme := &v2high.SecurityScheme{
			Type:        BasicAuthScheme.String(),
			Description: "Basic Authentication",
		}

		result := converter.convertSecurityScheme(v2Scheme)
		assert.True(t, result != nil)
		assert.Equal(t, HTTPAuthScheme.String(), result.Type)
		assert.Equal(t, BasicAuthScheme.String(), result.Scheme)
		assert.Equal(t, "Basic Authentication", result.Description)
	})

	t.Run("oauth2_implicit", func(t *testing.T) {
		scopes := orderedmap.New[string, string]()
		scopes.Set("read", "Read access")
		scopes.Set("write", "Write access")

		v2Scheme := &v2high.SecurityScheme{
			Type:             OAuth2Scheme.String(),
			Flow:             "implicit",
			AuthorizationUrl: "https://example.com/oauth/authorize",
			Scopes: &v2high.Scopes{
				Values: scopes,
			},
		}

		result := converter.convertSecurityScheme(v2Scheme)
		assert.True(t, result != nil)
		assert.Equal(t, OAuth2Scheme.String(), result.Type)
		assert.True(t, result.Flows != nil)
		assert.True(t, result.Flows.Implicit != nil)
		assert.Equal(t, "https://example.com/oauth/authorize", result.Flows.Implicit.AuthorizationUrl)
		assert.Equal(t, 2, result.Flows.Implicit.Scopes.Len())
	})

	t.Run("oauth2_password", func(t *testing.T) {
		scopes := orderedmap.New[string, string]()
		v2Scheme := &v2high.SecurityScheme{
			Type:     OAuth2Scheme.String(),
			Flow:     "password",
			TokenUrl: "https://example.com/oauth/token",
			Scopes: &v2high.Scopes{
				Values: scopes,
			},
		}

		result := converter.convertSecurityScheme(v2Scheme)
		assert.True(t, result != nil)
		assert.True(t, result.Flows != nil)
		assert.True(t, result.Flows.Password != nil)
		assert.Equal(t, "https://example.com/oauth/token", result.Flows.Password.TokenUrl)
	})

	t.Run("oauth2_application", func(t *testing.T) {
		scopes := orderedmap.New[string, string]()
		v2Scheme := &v2high.SecurityScheme{
			Type:     OAuth2Scheme.String(),
			Flow:     "application",
			TokenUrl: "https://example.com/oauth/token",
			Scopes: &v2high.Scopes{
				Values: scopes,
			},
		}

		result := converter.convertSecurityScheme(v2Scheme)
		assert.True(t, result != nil)
		assert.True(t, result.Flows != nil)
		assert.True(t, result.Flows.ClientCredentials != nil)
	})

	t.Run("oauth2_accessCode", func(t *testing.T) {
		scopes := orderedmap.New[string, string]()
		v2Scheme := &v2high.SecurityScheme{
			Type:             OAuth2Scheme.String(),
			Flow:             "accessCode",
			AuthorizationUrl: "https://example.com/oauth/authorize",
			TokenUrl:         "https://example.com/oauth/token",
			Scopes: &v2high.Scopes{
				Values: scopes,
			},
		}

		result := converter.convertSecurityScheme(v2Scheme)
		assert.True(t, result != nil)
		assert.True(t, result.Flows != nil)
		assert.True(t, result.Flows.AuthorizationCode != nil)
	})

	t.Run("apiKey", func(t *testing.T) {
		v2Scheme := &v2high.SecurityScheme{
			Type: "apiKey",
			Name: "X-API-Key",
			In:   "header",
		}

		result := converter.convertSecurityScheme(v2Scheme)
		assert.True(t, result != nil)
		assert.Equal(t, "apiKey", result.Type)
		assert.Equal(t, "X-API-Key", result.Name)
		assert.Equal(t, "header", result.In)
	})
}

// TestConvertParameter tests parameter conversion
func TestConvertParameter(t *testing.T) {
	converter := &swaggerToOpenAPIv3Converter{}

	t.Run("simple_parameter", func(t *testing.T) {
		v2Param := &v2high.Parameter{
			Name:        "id",
			In:          InPath.String(),
			Description: "User ID",
			Required:    new(true),
			Type:        "string",
		}

		result := converter.convertParameter(v2Param)
		assert.True(t, result != nil)
		assert.Equal(t, "id", result.Name)
		assert.Equal(t, InPath.String(), result.In)
		assert.Equal(t, "User ID", result.Description)
		assert.True(t, result.Required != nil)
		assert.Equal(t, true, *result.Required)
		assert.True(t, result.Schema != nil)
	})

	t.Run("parameter_with_schema", func(t *testing.T) {
		schema := base.CreateSchemaProxy(&base.Schema{
			Type: []string{"object"},
		})

		v2Param := &v2high.Parameter{
			Name:   "body",
			In:     InBody.String(),
			Schema: schema,
		}

		result := converter.convertParameter(v2Param)
		assert.True(t, result != nil)
		assert.True(t, result.Schema != nil)
	})

	t.Run("parameter_with_allow_empty_value", func(t *testing.T) {
		v2Param := &v2high.Parameter{
			Name:            "filter",
			In:              InQuery.String(),
			AllowEmptyValue: new(true),
		}

		result := converter.convertParameter(v2Param)
		assert.True(t, result != nil)
		assert.Equal(t, true, result.AllowEmptyValue)
	})

	t.Run("parameter_with_collection_format", func(t *testing.T) {
		v2Param := &v2high.Parameter{
			Name:             "tags",
			In:               InQuery.String(),
			CollectionFormat: "multi",
		}

		result := converter.convertParameter(v2Param)
		assert.True(t, result != nil)
		assert.Equal(t, "form", result.Style)
		assert.True(t, result.Explode != nil)
		assert.Equal(t, true, *result.Explode)
	})
}

// TestSchemaFromParameter tests schema creation from parameter
func TestSchemaFromParameter(t *testing.T) {
	converter := &swaggerToOpenAPIv3Converter{}

	t.Run("string_parameter", func(t *testing.T) {
		v2Param := &v2high.Parameter{
			Type:   "string",
			Format: "email",
		}

		result := converter.schemaFromParameter(v2Param)
		assert.True(t, result != nil)
		assert.True(t, result.Schema() != nil)
		assert.Equal(t, "email", result.Schema().Format)
	})

	t.Run("array_with_items", func(t *testing.T) {
		v2Param := &v2high.Parameter{
			Type: "array",
			Items: &v2high.Items{
				Type: "string",
			},
		}

		result := converter.schemaFromParameter(v2Param)
		assert.True(t, result != nil)
		assert.True(t, result.Schema() != nil)
		assert.True(t, result.Schema().Items != nil)
	})

	t.Run("with_pattern", func(t *testing.T) {
		v2Param := &v2high.Parameter{
			Type:    "string",
			Pattern: "^[a-z]+$",
		}

		result := converter.schemaFromParameter(v2Param)
		assert.True(t, result != nil)
		assert.True(t, result.Schema() != nil)
		assert.Equal(t, "^[a-z]+$", result.Schema().Pattern)
	})
}

// TestConvertPaths tests path conversion
func TestConvertPaths(t *testing.T) {
	converter := &swaggerToOpenAPIv3Converter{
		swagger: &v2high.Swagger{},
	}

	t.Run("nil_paths", func(t *testing.T) {
		result := converter.convertPaths()
		assert.True(t, result == nil)
	})

	t.Run("empty_path_items", func(t *testing.T) {
		converter.swagger.Paths = &v2high.Paths{}
		result := converter.convertPaths()
		assert.True(t, result != nil)
	})
}

// TestConvertOperation tests operation conversion
func TestConvertOperation(t *testing.T) {
	converter := &swaggerToOpenAPIv3Converter{
		swagger: &v2high.Swagger{},
	}

	t.Run("nil_operation", func(t *testing.T) {
		result := converter.convertOperation(nil)
		assert.True(t, result == nil)
	})

	t.Run("simple_operation", func(t *testing.T) {
		operation := &v2high.Operation{
			Summary:     "Test operation",
			Description: "Test description",
			OperationId: "testOp",
		}

		result := converter.convertOperation(operation)
		assert.True(t, result != nil)
		assert.Equal(t, "Test operation", result.Summary)
		assert.Equal(t, "Test description", result.Description)
		assert.Equal(t, "testOp", result.OperationId)
	})
}
