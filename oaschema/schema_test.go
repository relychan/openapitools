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
	"github.com/stretchr/testify/assert"
)

func TestGetSchemaTypes_NullType(t *testing.T) {
	testCases := []struct {
		name             string
		schema           *base.Schema
		expectedTypes    []string
		expectedNullable bool
	}{
		{
			name:             "string_and_null",
			schema:           &base.Schema{Type: []string{"string", "null"}},
			expectedTypes:    []string{String},
			expectedNullable: true,
		},
		{
			name:             "null_first",
			schema:           &base.Schema{Type: []string{"null", "integer"}},
			expectedTypes:    []string{Integer},
			expectedNullable: true,
		},
		{
			name:             "string_only",
			schema:           &base.Schema{Type: []string{"string"}},
			expectedTypes:    []string{String},
			expectedNullable: false,
		},
		{
			name: "null_in_union",
			schema: &base.Schema{
				OneOf: []*base.SchemaProxy{
					base.CreateSchemaProxy(&base.Schema{Type: []string{"null"}}),
					base.CreateSchemaProxy(&base.Schema{Type: []string{"number"}}),
				},
			},
			expectedTypes:    []string{Number},
			expectedNullable: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			types, nullable := GetSchemaTypes(tc.schema)
			assert.Equal(t, tc.expectedTypes, types)
			assert.Equal(t, tc.expectedNullable, nullable)
		})
	}
}
