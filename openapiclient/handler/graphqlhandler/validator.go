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
	"errors"

	"github.com/vektah/gqlparser/ast"
	"github.com/vektah/gqlparser/parser"
)

var (
	ErrProxyActionInvalid           = errors.New("proxy action must exist with the graphql type")
	ErrGraphQLQueryEmpty            = errors.New("query is required for graphql proxy")
	ErrGraphQLUnsupportedQueryBatch = errors.New("graphql query batch is not supported")
	ErrGraphQLResponseRequired      = errors.New("graphql response must be a valid JSON object")
)

// ValidateGraphQLString parses and validates the GraphQL query string.
func ValidateGraphQLString(query string) (*GraphQLHandler, error) {
	if query == "" {
		return nil, ErrGraphQLQueryEmpty
	}

	doc, err := parser.ParseQuery(&ast.Source{
		Input: query,
	})
	if err != nil {
		return nil, err
	}

	switch len(doc.Operations) {
	case 0:
		return nil, ErrGraphQLQueryEmpty
	case 1:
		graphqlOperation := doc.Operations[0]

		handler := &GraphQLHandler{
			query:               query,
			variableDefinitions: graphqlOperation.VariableDefinitions,
			operationName:       graphqlOperation.Name,
			operation:           graphqlOperation.Operation,
		}

		return handler, nil
	default:
		return nil, ErrGraphQLUnsupportedQueryBatch
	}
}
