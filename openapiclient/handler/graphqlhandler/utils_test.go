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

package graphqlhandler

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateGraphQLString(t *testing.T) {
	testCases := []struct {
		name          string
		query         string
		expectError   bool
		errorContains string
		checkHandler  func(t *testing.T, handler *GraphQLHandler)
	}{
		{
			name:          "empty query",
			query:         "",
			expectError:   true,
			errorContains: "query is required",
		},
		{
			name:          "invalid GraphQL syntax",
			query:         "query {",
			expectError:   true,
			errorContains: "Expected Name",
		},
		{
			name:        "valid simple query",
			query:       "query { users { id name } }",
			expectError: false,
			checkHandler: func(t *testing.T, handler *GraphQLHandler) {
				assert.True(t, handler != nil)
				assert.Equal(t, "query { users { id name } }", handler.query)
				assert.Equal(t, "query", string(handler.operation))
			},
		},
		{
			name:        "valid mutation",
			query:       "mutation CreateUser($name: String!) { createUser(name: $name) { id } }",
			expectError: false,
			checkHandler: func(t *testing.T, handler *GraphQLHandler) {
				assert.True(t, handler != nil)
				assert.Equal(t, "mutation", string(handler.operation))
				assert.Equal(t, 1, len(handler.variableDefinitions))
				assert.Equal(t, "name", handler.variableDefinitions[0].Variable)
			},
		},
		{
			name:        "query with operation name",
			query:       "query GetUsers { users { id } }",
			expectError: false,
			checkHandler: func(t *testing.T, handler *GraphQLHandler) {
				assert.True(t, handler != nil)
				assert.Equal(t, "GetUsers", handler.operationName)
			},
		},
		{
			name: "multiple operations (batch)",
			query: `
				query GetUsers { users { id } }
				query GetPosts { posts { id } }
			`,
			expectError:   true,
			errorContains: "batch is not supported",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handler, err := ValidateGraphQLString(tc.query)

			if tc.expectError {
				assert.True(t, err != nil, "expected error but got nil")
				if tc.errorContains != "" {
					assert.ErrorContains(t, err, tc.errorContains)
				}
			} else {
				assert.NoError(t, err)
				if tc.checkHandler != nil {
					tc.checkHandler(t, handler)
				}
			}
		})
	}
}
