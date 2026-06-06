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
	"strings"

	"github.com/relychan/goutils"
	"github.com/relychan/goutils/httperror"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
)

var (
	ErrProxyActionRequired  = errors.New("proxy action of GraphQL type must exist")
	ErrInvalidRequestMethod = errors.New(
		"invalid GraphQL request method. Accept GET or POST",
	)
	ErrGraphQLQueryEmpty            = errors.New("query is required for graphql proxy")
	ErrGraphQLUnsupportedQueryBatch = errors.New("graphql query batch is not supported")
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

// convertVariableTypeFromParam coerces a parameter value to the Go type that matches the
// declared GraphQL scalar (bool, int*, uint*, float*). Returns the original string for
// unknown or nil types.
func convertVariableTypeFromParam(varDef *ast.VariableDefinition, value any) (any, error) {
	if varDef.Type == nil {
		// unknown type. Returns the original value.
		return value, nil
	}

	// Query and Header parameters can be an array of strings.
	if strValues, ok := value.([]string); ok {
		switch len(strValues) {
		case 0:
			return nil, nil
		case 1:
			return convertVariableTypeFromUnknownValue(varDef, strValues[0])
		default:
			return convertVariableTypeFromUnknownValue(varDef, value)
		}
	}

	if anyValues, ok := value.([]any); ok {
		switch len(anyValues) {
		case 0:
			return nil, nil
		case 1:
			if strValue, ok := anyValues[0].(string); ok {
				return convertVariableTypeFromUnknownValue(varDef, strValue)
			}
		default:
		}
	}

	return convertVariableTypeFromUnknownValue(varDef, value)
}

// convertVariableTypeFromUnknownValue coerces an arbitrary value to the declared GraphQL scalar type.
// String and *string values are handled via convertVariableTypeFromString; other types use
// nullable decoders. Returns the original value for unknown or unrecognized types.
func convertVariableTypeFromUnknownValue(varDef *ast.VariableDefinition, value any) (any, error) {
	if varDef.Type == nil || value == nil {
		// unknown type. Returns the original value.
		return value, nil
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

func newGraphQLResponseEncodeError(code string, err error) error {
	respErr := httperror.NewServerError(httperror.ValidationError{
		Detail: err.Error(),
		Code:   code,
	})
	respErr.Detail = "failed to process graphql response"

	return respErr
}

func isEvaluatedError(value any) bool {
	if value == nil {
		return false
	}

	switch v := value.(type) {
	case bool:
		return v
	case int:
		return v != 0
	case int64:
		return v != 0
	case float64:
		return v != 0
	case string:
		return v == "true"
	default:
		return false
	}
}
