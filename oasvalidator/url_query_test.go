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

package oasvalidator

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func makeEnvGetter(vars map[string]string) func(string) (string, error) {
	return func(key string) (string, error) {
		if v, ok := vars[key]; ok {
			return v, nil
		}

		return "", errors.New("env var not found: " + key)
	}
}

func TestReplaceURLTemplate(t *testing.T) {
	testCases := []struct {
		name        string
		input       string
		env         map[string]string
		expected    string
		expectedErr error
	}{
		{
			name:        "empty input returns empty string",
			input:       "",
			env:         nil,
			expected:    "",
			expectedErr: nil,
		},
		{
			name:        "no template variables, returned as-is",
			input:       "https://example.com/api/v1",
			env:         nil,
			expected:    "https://example.com/api/v1",
			expectedErr: nil,
		},
		{
			name:        "single variable replaced",
			input:       "https://{HOST}",
			env:         map[string]string{"HOST": "example.com"},
			expected:    "https://example.com",
			expectedErr: nil,
		},
		{
			name:        "variable in the middle of URL",
			input:       "https://{HOST}/api",
			env:         map[string]string{"HOST": "example.com"},
			expected:    "https://example.com/api",
			expectedErr: nil,
		},
		{
			name:        "variable at the end of URL",
			input:       "https://example.com/{PATH}",
			env:         map[string]string{"PATH": "v1/users"},
			expected:    "https://example.com/v1/users",
			expectedErr: nil,
		},
		{
			name:        "multiple variables replaced",
			input:       "https://{HOST}:{PORT}/api",
			env:         map[string]string{"HOST": "example.com", "PORT": "8080"},
			expected:    "https://example.com:8080/api",
			expectedErr: nil,
		},
		{
			name:        "two adjacent variables",
			input:       "{SCHEME}{HOST}",
			env:         map[string]string{"SCHEME": "https://", "HOST": "example.com"},
			expected:    "https://example.com",
			expectedErr: nil,
		},
		{
			name:        "variable replaced with empty string",
			input:       "https://example.com/{EMPTY}/end",
			env:         map[string]string{"EMPTY": ""},
			expected:    "https://example.com//end",
			expectedErr: nil,
		},
		{
			name:        "variable replaced with value containing special chars",
			input:       "{URL}",
			env:         map[string]string{"URL": "http://foo.bar:9000/path?q=1&r=2"},
			expected:    "http://foo.bar:9000/path?q=1&r=2",
			expectedErr: nil,
		},
		{
			name:        "unclosed bracket at end of string",
			input:       "https://example.com/{HOST",
			env:         nil,
			expected:    "",
			expectedErr: errUnclosedTemplateString,
		},
		{
			name:        "lone open bracket",
			input:       "{",
			env:         nil,
			expected:    "",
			expectedErr: errUnclosedTemplateString,
		},
		{
			name:        "unclosed bracket after static prefix",
			input:       "prefix{",
			env:         nil,
			expected:    "",
			expectedErr: errUnclosedTemplateString,
		},
		{
			name:        "unknown variable returns getter error",
			input:       "{UNKNOWN}",
			env:         map[string]string{},
			expected:    "",
			expectedErr: errors.New("env var not found: UNKNOWN"),
		},
		{
			name:        "error on first variable short-circuits remaining input",
			input:       "{MISSING}/rest/{OTHER}",
			env:         map[string]string{"OTHER": "ok"},
			expected:    "",
			expectedErr: errors.New("env var not found: MISSING"),
		},
		{
			name:        "plain text with no brackets",
			input:       "no-template-here",
			env:         nil,
			expected:    "no-template-here",
			expectedErr: nil,
		},
		// {
		// 	name:        "path traverse",
		// 	input:       "http://localhost:2000/../.env",
		// 	env:         nil,
		// 	expected:    "http://localhost:2000/../.env", // FIXME
		// 	expectedErr: nil,
		// },
		{
			name:        "empty variable name ({})",
			input:       "prefix{}suffix",
			env:         map[string]string{"": "replaced"},
			expected:    "",
			expectedErr: errInvalidURLTemplateSyntax,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			getter := makeEnvGetter(tc.env)
			result, err := ReplaceURLTemplate(tc.input, getter)

			if tc.expectedErr != nil {
				assert.EqualError(t, err, tc.expectedErr.Error())
				assert.Equal(t, tc.expected, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

// goos: darwin
// goarch: arm64
// pkg: github.com/relychan/openapitools/oasvalidator
// cpu: Apple M3 Pro
// BenchmarkReplaceURLTemplate/NoVars-11         	226891762	         5.302 ns/op	       0 B/op	       0 allocs/op
// BenchmarkReplaceURLTemplate/SingleVar-11      	 15668054	         76.35 ns/op	      72 B/op	       2 allocs/op
// BenchmarkReplaceURLTemplate/MultipleVars-11   	  8596990	         136.2 ns/op	      96 B/op	       2 allocs/op
// BenchmarkReplaceURLTemplate/LongInputNoVars-11   100000000	         10.95 ns/op	       0 B/op	       0 allocs/op
// BenchmarkReplaceURLTemplate/ManyVars-11            7961914	         158.1 ns/op	      72 B/op	       2 allocs/op
func BenchmarkReplaceURLTemplate(b *testing.B) {
	benchEnv := map[string]string{
		"HOST":   "example.com",
		"PORT":   "8080",
		"SCHEME": "https",
		"PATH":   "v1/users",
	}

	benchGetter := makeEnvGetter(benchEnv)

	b.Run("NoVars", func(b *testing.B) {
		input := "https://example.com/api/v1/users"

		for b.Loop() {
			_, _ = ReplaceURLTemplate(input, benchGetter)
		}
	})

	b.Run("SingleVar", func(b *testing.B) {
		input := "https://{HOST}/api/v1"

		for b.Loop() {
			_, _ = ReplaceURLTemplate(input, benchGetter)
		}
	})

	b.Run("MultipleVars", func(b *testing.B) {
		input := "{SCHEME}://{HOST}:{PORT}/{PATH}"

		for b.Loop() {
			_, _ = ReplaceURLTemplate(input, benchGetter)
		}
	})

	b.Run("LongInputNoVars", func(b *testing.B) {
		input := "https://example.com/" + strings.Repeat("segment/", 50) + "endpoint"

		for b.Loop() {
			_, _ = ReplaceURLTemplate(input, benchGetter)
		}
	})

	b.Run("ManyVars", func(b *testing.B) {
		getter := makeEnvGetter(map[string]string{
			"A": "alpha", "B": "bravo", "C": "charlie",
			"D": "delta", "E": "echo", "F": "foxtrot",
		})
		input := "{A}/{B}/{C}/{D}/{E}/{F}"

		for b.Loop() {
			_, _ = ReplaceURLTemplate(input, getter)
		}
	})
}
