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

	"github.com/pb33f/libopenapi/datamodel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"
)

func TestExtractOpenAPIv3SpecInfoFromYAML(t *testing.T) {
	testCases := []struct {
		name            string
		yaml            string
		expectError     bool
		expectedVersion string
		expectedFormat  string
	}{
		{
			name: "openapi_3.0",
			yaml: `openapi: "3.0.0"
info:
  title: Test
  version: "1.0"
paths: {}`,
			expectError:     false,
			expectedVersion: "3.0.0",
			expectedFormat:  datamodel.OAS3,
		},
		{
			name: "openapi_3.1",
			yaml: `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}`,
			expectError:     false,
			expectedVersion: "3.1.0",
			expectedFormat:  datamodel.OAS31,
		},
		{
			name: "openapi_3.2",
			yaml: `openapi: "3.2.0"
info:
  title: Test
  version: "1.0"
paths: {}`,
			expectError:     false,
			expectedVersion: "3.2.0",
			expectedFormat:  datamodel.OAS32,
		},
		{
			name: "swagger_2.0_rejected",
			yaml: `swagger: "2.0"
info:
  title: Test
  version: "1.0"
paths: {}`,
			expectError: true,
		},
		{
			name:        "missing_openapi_field",
			yaml:        `info:\n  title: Test`,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var node yaml.Node
			require.NoError(t, yaml.Unmarshal([]byte(tc.yaml), &node))

			info, err := extractOpenAPIv3SpecInfoFromYAML(&node)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedVersion, info.Version)
				assert.Equal(t, tc.expectedFormat, info.SpecFormat)
			}
		})
	}
}

func TestParseVersionTypeData(t *testing.T) {
	testCases := []struct {
		name          string
		input         string
		expectError   bool
		expectedVer   string
		expectedMajor int
	}{
		{
			name:          "version_3.0.0",
			input:         "3.0.0",
			expectError:   false,
			expectedVer:   "3.0.0",
			expectedMajor: 3,
		},
		{
			name:          "version_3.1.0",
			input:         "3.1.0",
			expectError:   false,
			expectedVer:   "3.1.0",
			expectedMajor: 3,
		},
		{
			name:          "version_with_whitespace",
			input:         "  3.0.0  ",
			expectError:   false,
			expectedVer:   "3.0.0",
			expectedMajor: 3,
		},
		{
			name:        "empty_string",
			input:       "",
			expectError: true,
		},
		{
			name:        "whitespace_only",
			input:       "   ",
			expectError: true,
		},
		{
			name:          "version_2.0",
			input:         "2.0",
			expectError:   false,
			expectedVer:   "2.0",
			expectedMajor: 2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ver, major, err := parseVersionTypeData(tc.input)

			if tc.expectError {
				assert.Error(t, err)
				assert.ErrorIs(t, err, ErrInvalidOpenAPIVersion)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedVer, ver)
				assert.Equal(t, tc.expectedMajor, major)
			}
		})
	}
}
