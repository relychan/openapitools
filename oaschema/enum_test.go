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

	"github.com/pb33f/go-yaml"
	"github.com/stretchr/testify/assert"
)

func TestParseSecuritySchemeType(t *testing.T) {
	testCases := []struct {
		name        string
		input       string
		expected    SecuritySchemeType
		expectError bool
	}{
		{
			name:        "valid apiKey",
			input:       "apiKey",
			expected:    APIKeyScheme,
			expectError: false,
		},
		{
			name:        "valid http",
			input:       "http",
			expected:    HTTPAuthScheme,
			expectError: false,
		},
		{
			name:        "valid basic",
			input:       "basic",
			expected:    BasicAuthScheme,
			expectError: false,
		},
		{
			name:        "valid cookie",
			input:       "cookie",
			expected:    CookieAuthScheme,
			expectError: false,
		},
		{
			name:        "valid oauth2",
			input:       "oauth2",
			expected:    OAuth2Scheme,
			expectError: false,
		},
		{
			name:        "valid openIdConnect",
			input:       "openIdConnect",
			expected:    OpenIDConnectScheme,
			expectError: false,
		},
		{
			name:        "valid mutualTLS",
			input:       "mutualTLS",
			expected:    MutualTLSScheme,
			expectError: false,
		},
		{
			name:        "invalid scheme",
			input:       "invalid",
			expected:    255,
			expectError: true,
		},
		{
			name:        "empty string",
			input:       "",
			expected:    255,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ParseSecuritySchemeType(tc.input)
			if tc.expectError {
				assert.ErrorIs(t, err, ErrInvalidSecuritySchemeType)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestSupportedSecuritySchemeTypes(t *testing.T) {
	schemes := SupportedSecuritySchemeTypes()
	assert.True(t, len(schemes) == 7)
}

func TestSecuritySchemeType(t *testing.T) {
	t.Run("encode_json", func(t *testing.T) {
		rawJson := `"apiKey"`
		var result SecuritySchemeType

		assert.NoError(t, json.Unmarshal([]byte(rawJson), &result))
		assert.Equal(t, APIKeyScheme, result)

		rawResult, err := json.Marshal(result)
		assert.NoError(t, err)
		assert.Equal(t, rawJson, string(rawResult))
	})

	t.Run("encode_yaml", func(t *testing.T) {
		rawYaml := `"apiKey"`
		var result SecuritySchemeType

		assert.NoError(t, yaml.Unmarshal([]byte(rawYaml), &result))
		assert.Equal(t, APIKeyScheme, result)

		rawResult, err := yaml.Marshal(result)
		assert.NoError(t, err)
		assert.Equal(t, "apiKey\n", string(rawResult))
	})
}

func TestEnumUnmarshalJSON_InvalidInput(t *testing.T) {
	inputs := []string{`0`, `1`, `""`, `"x`, `null`, `{}`, `"apiKey`}

	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			assert.NotPanics(t, func() {
				var scheme SecuritySchemeType
				assert.Error(t, scheme.UnmarshalJSON([]byte(input)))

				var location ParameterLocation
				assert.Error(t, location.UnmarshalJSON([]byte(input)))

				var style ParameterEncodingStyle
				assert.Error(t, style.UnmarshalJSON([]byte(input)))
			})
		})
	}

	t.Run("struct_with_non_string_location", func(t *testing.T) {
		var param Parameter

		assert.NotPanics(t, func() {
			assert.Error(t, json.Unmarshal([]byte(`{"name":"id","in":0}`), &param))
		})
	})

	t.Run("escaped_string", func(t *testing.T) {
		var location ParameterLocation

		assert.NoError(t, location.UnmarshalJSON([]byte(`"path"`)))
		assert.Equal(t, InPath, location)
	})
}

func TestEnumIsValid_Bounds(t *testing.T) {
	assert.True(t, MutualTLSScheme.IsValid())
	assert.Equal(t, "mutualTLS", MutualTLSScheme.String())
	assert.False(t, SecuritySchemeType(7).IsValid())
	assert.NotPanics(t, func() {
		assert.Equal(t, "", SecuritySchemeType(7).String())
		assert.Equal(t, "", ParameterLocation(6).String())
		assert.Equal(t, "", ParameterEncodingStyle(8).String())
	})
	assert.Equal(t, "formData", InFormData.String())
	assert.Equal(t, "cookie", EncodingStyleCookie.String())
}
