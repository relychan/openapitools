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
	"strconv"
	"strings"

	"github.com/relychan/goutils"
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

func convertVariableTypeFromString(varDef *ast.VariableDefinition, value string) (any, error) {
	if varDef.Type == nil {
		// unknown type. Returns the original value.
		return value, nil
	}

	switch strings.ToLower(varDef.Type.NamedType) {
	case "bool", "boolean":
		return strconv.ParseBool(value)
	case "int", "int8", "int16", "int32", "int64":
		return strconv.ParseInt(value, 10, 64)
	case "uint", "uint8", "uint16", "uint32", "uint64":
		return strconv.ParseUint(value, 10, 64)
	case "number", "decimal", "float", "float32", "float64", "double":
		return strconv.ParseFloat(value, 64)
	default:
		// unknown type. Returns the original value.
		return value, nil
	}
}

func convertVariableTypeFromUnknownValue(varDef *ast.VariableDefinition, value any) (any, error) {
	if varDef.Type == nil || value == nil {
		// unknown type. Returns the original value.
		return value, nil
	}

	if str, ok := value.(string); ok {
		return convertVariableTypeFromString(varDef, str)
	}

	if strPtr, ok := value.(*string); ok {
		if strPtr == nil {
			return nil, nil
		}

		return convertVariableTypeFromString(varDef, *strPtr)
	}

	switch strings.ToLower(varDef.Type.NamedType) {
	case "bool", "boolean":
		return goutils.DecodeNullableBoolean(value)
	case "int", "int8", "int16", "int32", "int64":
		return goutils.DecodeNullableNumber[int64](value)
	case "uint", "uint8", "uint16", "uint32", "uint64":
		return goutils.DecodeNullableNumber[uint64](value)
	case "number", "decimal", "float", "float32", "float64", "double":
		return goutils.DecodeNullableNumber[float64](value)
	default:
		// unknown type. Returns the original value.
		return value, nil
	}
}
