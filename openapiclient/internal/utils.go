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

package internal

import (
	"slices"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/relychan/openapitools/oaschema"
)

// extractParametersFromOperationV3 returns the path-level parameters of the path item,
// plus synthetic string path parameters for keys in the route pattern that are not declared at the path level.
// Operation-level parameters are not included. They override path-level parameters
// per operation, see [oaschema.InheritedPathItemParameters].
func extractParametersFromOperationV3(
	operations *highv3.PathItem,
	paramKeys []string,
) []*highv3.Parameter {
	params := slices.Clone(operations.Parameters)

	// validates and add unknown parameters from the request pattern
	for _, key := range paramKeys {
		if slices.ContainsFunc(params, func(param *highv3.Parameter) bool {
			return param != nil && param.In == oaschema.InPath.String() && param.Name == key
		}) {
			continue
		}

		params = append(params, &highv3.Parameter{
			Name:     key,
			In:       oaschema.InPath.String(),
			Required: new(true),
			Schema: base.CreateSchemaProxy(&base.Schema{
				Type: []string{"string"},
			}),
		})
	}

	return params
}

// cut the first path of the url and parse the query param if exists.
func cutURLPath(search string) (string, string) { //nolint:revive
	if search == "" {
		return search, ""
	}

	var endPathIndex int

	maxLength := len(search)

L:
	for ; endPathIndex < maxLength; endPathIndex++ {
		c := search[endPathIndex]

		switch c {
		case '/':
			break L
		case '?', '#':
			return search[:endPathIndex], ""
		default:
		}
	}

	if endPathIndex == maxLength {
		return search, ""
	}

	return search[0:endPathIndex], search[endPathIndex+1:]
}
