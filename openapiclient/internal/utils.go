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
	"net/url"
	"slices"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/relychan/openapitools/oaschema"
)

func extractParametersFromOperationV3(
	operations *highv3.PathItem,
	paramKeys []string,
) []*highv3.Parameter {
	params := operations.Parameters
	params = oaschema.ExtractCommonParametersOfOperation(params, operations.Get)
	params = oaschema.ExtractCommonParametersOfOperation(params, operations.Post)
	params = oaschema.ExtractCommonParametersOfOperation(params, operations.Put)
	params = oaschema.ExtractCommonParametersOfOperation(params, operations.Patch)
	params = oaschema.ExtractCommonParametersOfOperation(params, operations.Delete)
	params = oaschema.ExtractCommonParametersOfOperation(params, operations.Head)
	params = oaschema.ExtractCommonParametersOfOperation(params, operations.Options)
	params = oaschema.ExtractCommonParametersOfOperation(params, operations.Query)
	params = oaschema.ExtractCommonParametersOfOperation(params, operations.Trace)

	if operations.AdditionalOperations != nil {
		for iter := operations.AdditionalOperations.Oldest(); iter != nil; iter = iter.Next() {
			if iter.Value == nil {
				continue
			}

			params = oaschema.ExtractCommonParametersOfOperation(params, iter.Value)
		}
	}

	// validates and add unknown parameters from the request pattern
	for _, key := range paramKeys {
		if slices.ContainsFunc(params, func(param *highv3.Parameter) bool {
			return param.In == oaschema.InPath.String() && param.Name == key
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

// cut the first path of the url and parse the query param if exists. Ignore fragments.
func cutURLPath(search string) (string, string, url.Values, error) { //nolint:revive
	if search == "" {
		return search, "", nil, nil
	}

	var endPathIndex int

	maxLength := len(search)

L:
	for ; endPathIndex < maxLength; endPathIndex++ {
		c := search[endPathIndex]

		switch c {
		case '/', '#':
			break L
		case '?':
			if endPathIndex == maxLength-1 {
				return search[:endPathIndex], "", nil, nil
			}

			queryParams, err := url.ParseQuery(search[endPathIndex+1:])
			if err != nil {
				return "", "", nil, err
			}

			return search[:endPathIndex], "", queryParams, nil
		default:
		}
	}

	if endPathIndex == maxLength {
		return search, "", nil, nil
	}

	return search[0:endPathIndex], search[endPathIndex+1:], nil, nil
}
