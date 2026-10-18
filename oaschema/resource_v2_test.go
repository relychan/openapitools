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
	"os"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	v2high "github.com/pb33f/libopenapi/datamodel/high/v2"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		assert.Equal(
			t,
			"https://example.com/oauth/authorize",
			result.Flows.Implicit.AuthorizationUrl,
		)
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

func buildSwagger2RegressionDocument(t *testing.T) *v3high.Document {
	t.Helper()

	rawBytes, err := os.ReadFile("testdata/swagger2/swagger.yaml")
	require.NoError(t, err)

	doc, err := libopenapi.NewDocument(rawBytes)
	require.NoError(t, err)

	v2Model, err := doc.BuildV2Model()
	require.NoError(t, err)

	var result *v3high.Document

	require.NotPanics(t, func() {
		result, err = convertSwaggerToOpenAPIv3Document(&v2Model.Model)
	})
	require.NoError(t, err)

	return result
}

func TestConvertSwaggerToOpenAPIv3Document_Regressions(t *testing.T) {
	result := buildSwagger2RegressionDocument(t)

	listItems := result.Paths.PathItems.GetOrZero("/items").Get
	require.NotNil(t, listItems)

	findParam := func(name string) *base.Schema {
		for _, param := range listItems.Parameters {
			if param.Name == name {
				return param.Schema.Schema()
			}
		}

		t.Fatalf("parameter %s not found", name)

		return nil
	}

	t.Run("param_min_items_without_max_items", func(t *testing.T) {
		schema := findParam("tags")
		require.NotNil(t, schema.MinItems)
		assert.Equal(t, int64(1), *schema.MinItems)
		assert.Nil(t, schema.MaxItems)
	})

	t.Run("param_min_length_is_not_max_length", func(t *testing.T) {
		schema := findParam("q")
		require.NotNil(t, schema.MinLength)
		assert.Equal(t, int64(2), *schema.MinLength)
		assert.Nil(t, schema.MaxLength)
	})

	t.Run("items_constraints", func(t *testing.T) {
		items := findParam("tags").Items.A.Schema()
		require.NotNil(t, items.MinLength)
		require.NotNil(t, items.MaxLength)
		assert.Equal(t, int64(3), *items.MinLength)
		assert.Equal(t, int64(10), *items.MaxLength)
		assert.Nil(t, items.Maximum)
		assert.Nil(t, items.Minimum)
		assert.Nil(t, items.MultipleOf)
		assert.Nil(t, items.ExclusiveMaximum)
		assert.Nil(t, items.ExclusiveMinimum)
		assert.Nil(t, items.UniqueItems)
	})

	t.Run("header_constraints", func(t *testing.T) {
		resp := listItems.Responses.Codes.GetOrZero("200")
		require.NotNil(t, resp)

		token := resp.Headers.GetOrZero("X-Token").Schema.Schema()
		require.NotNil(t, token.MinLength)
		assert.Equal(t, int64(4), *token.MinLength)
		assert.Nil(t, token.MaxLength)
		assert.Nil(t, token.Maximum)
		assert.Nil(t, token.Minimum)

		rate := resp.Headers.GetOrZero("X-Rate-Limit").Schema.Schema()
		require.NotNil(t, rate.Minimum)
		assert.Equal(t, float64(1), *rate.Minimum)
		assert.Nil(t, rate.Maximum)
		assert.Nil(t, rate.MultipleOf)
	})

	t.Run("operation_consumes_without_produces", func(t *testing.T) {
		upload := result.Paths.PathItems.GetOrZero("/upload").Post
		require.NotNil(t, upload)
		require.NotNil(t, upload.RequestBody)

		_, ok := upload.RequestBody.Content.Get("multipart/form-data")
		assert.True(t, ok)
		_, ok = upload.RequestBody.Content.Get("application/json")
		assert.False(t, ok)
	})

	t.Run("oauth2_without_scopes", func(t *testing.T) {
		scheme := result.Components.SecuritySchemes.GetOrZero("oauth")
		require.NotNil(t, scheme)
		require.NotNil(t, scheme.Flows)
		require.NotNil(t, scheme.Flows.ClientCredentials)
		assert.Equal(t, "https://example.com/token", scheme.Flows.ClientCredentials.TokenUrl)
	})

	t.Run("base_path_without_host", func(t *testing.T) {
		require.Len(t, result.Servers, 1)
		assert.Equal(t, "/api/v1", result.Servers[0].URL)
	})
}

func TestSchemaFromParameter_MinItemsOnly(t *testing.T) {
	converter := &swaggerToOpenAPIv3Converter{swagger: &v2high.Swagger{}}

	var proxy *base.SchemaProxy

	require.NotPanics(t, func() {
		proxy = converter.schemaFromParameter(&v2high.Parameter{
			Type:      "array",
			MinItems:  new(2),
			MinLength: new(3),
		})
	})

	schema := proxy.Schema()
	require.NotNil(t, schema.MinItems)
	assert.Equal(t, int64(2), *schema.MinItems)
	assert.Nil(t, schema.MaxItems)
	require.NotNil(t, schema.MinLength)
	assert.Equal(t, int64(3), *schema.MinLength)
	assert.Nil(t, schema.MaxLength)
}

func TestConvertSecurityScheme_OAuth2NilScopes(t *testing.T) {
	converter := &swaggerToOpenAPIv3Converter{swagger: &v2high.Swagger{}}

	require.NotPanics(t, func() {
		result := converter.convertSecurityScheme(&v2high.SecurityScheme{
			Type:     "oauth2",
			Flow:     "implicit",
			TokenUrl: "https://example.com/token",
		})
		require.NotNil(t, result.Flows.Implicit)
		assert.Nil(t, result.Flows.Implicit.Scopes)
	})
}

func TestConvertServers(t *testing.T) {
	testCases := []struct {
		name     string
		swagger  *v2high.Swagger
		expected []string
	}{
		{name: "nothing", swagger: &v2high.Swagger{}, expected: nil},
		{name: "root_base_path_only", swagger: &v2high.Swagger{BasePath: "/"}, expected: nil},
		{
			name:     "base_path_only",
			swagger:  &v2high.Swagger{BasePath: "/v2"},
			expected: []string{"/v2"},
		},
		{
			name:     "base_path_without_slash",
			swagger:  &v2high.Swagger{BasePath: "v2"},
			expected: []string{"/v2"},
		},
		{
			name:     "host_only",
			swagger:  &v2high.Swagger{Host: "example.com"},
			expected: []string{"https://example.com"},
		},
		{
			name: "host_and_base_path",
			swagger: &v2high.Swagger{
				Host:     "example.com",
				BasePath: "/v2",
				Schemes:  []string{"http"},
			},
			expected: []string{"http://example.com/v2"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			converter := &swaggerToOpenAPIv3Converter{swagger: tc.swagger}

			var urls []string

			for _, server := range converter.convertServers() {
				urls = append(urls, server.URL)
			}

			assert.Equal(t, tc.expected, urls)
		})
	}
}
