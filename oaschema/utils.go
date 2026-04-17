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
	"slices"
	"strings"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/relychan/goutils/httpheader"
)

// ExtractCommonParametersOfOperation extracts common parameters from operation's parameters.
func ExtractCommonParametersOfOperation(
	pathParams []*highv3.Parameter,
	operation *highv3.Operation,
) []*highv3.Parameter {
	if operation == nil || len(operation.Parameters) == 0 {
		return pathParams
	}

	remainParams := make([]*highv3.Parameter, 0, len(operation.Parameters))

	for _, param := range operation.Parameters {
		if slices.ContainsFunc(pathParams, func(originalParam *highv3.Parameter) bool {
			return param.Name == originalParam.Name && param.In == originalParam.In
		}) {
			continue
		}

		if param.In == InPath.String() {
			pathParams = append(pathParams, param)
		} else {
			remainParams = append(remainParams, param)
		}
	}

	operation.Parameters = slices.Clip(remainParams)

	return pathParams
}

// MergeParameters merge parameter slices by unique name and location.
func MergeParameters(dest []*highv3.Parameter, src []*highv3.Parameter) []*highv3.Parameter {
L:
	for _, srcParam := range src {
		for j, destParam := range dest {
			if destParam.Name == srcParam.Name && destParam.In == srcParam.In {
				dest[j] = srcParam

				continue L
			}
		}

		dest = append(dest, srcParam)
	}

	return dest
}

// GetDefaultContentType gets the default content type from the content map.
func GetDefaultContentType(contents *orderedmap.Map[string, *highv3.MediaType]) string {
	if contents == nil || contents.Len() == 0 {
		return ""
	}

	var contentType string

	iter := contents.First()

	contentType = iter.Key()

	for ; iter != nil; iter = iter.Next() {
		key := strings.ToLower(iter.Key())
		// always prefer JSON content type.
		for item := range strings.SplitSeq(key, ",") {
			item = strings.TrimSpace(item)
			if httpheader.IsContentTypeJSON(item) {
				return item
			}
		}
	}

	return contentType
}

// GetResponseContentTypeFromOperation gets the successful content type of the operation.
func GetResponseContentTypeFromOperation(operation *highv3.Operation) string {
	if operation.Responses == nil {
		return ""
	}

	var successResponse *highv3.Response

	for iter := operation.Responses.Codes.First(); iter != nil; iter = iter.Next() {
		status := iter.Key()

		if status == "200" || status == "201" || status == "204" {
			successResponse = iter.Value()

			break
		}
	}

	if successResponse != nil {
		return GetDefaultContentType(successResponse.Content)
	}

	if operation.Responses.Default != nil {
		return GetDefaultContentType(operation.Responses.Default.Content)
	}

	return ""
}

// MergeOrderedMap assigns properties of the source order map to another.
func MergeOrderedMap[K comparable, V any](dest, src *orderedmap.Map[K, V]) *orderedmap.Map[K, V] {
	if src == nil || src.Len() == 0 {
		return dest
	}

	if dest == nil {
		return dest
	}

	for iter := src.First(); iter != nil; iter = iter.Next() {
		key := iter.Key()

		_, present := dest.Get(key)
		if present {
			dest.Set(key, iter.Value())
		}
	}

	return dest
}

// NormalizeType normalize a schema type.
// Returns the type name and whether if it is a primitive type.
func NormalizeType(typeName string) (string, bool) {
	lowerTypeName := strings.ToLower(typeName)

	switch lowerTypeName {
	case "bool", "boolean":
		return Boolean, true
	case "string", "uuid", "varchar":
		return String, true
	case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64":
		return Integer, true
	case "number", "decimal", "float", "float32", "float64", "double":
		return Number, true
	default:
		// array, object and unknown type.
		return lowerTypeName, false
	}
}

// IsSchemaEmpty checks if the schema type is empty.
func IsSchemaEmpty(schema *base.Schema) bool {
	return schema == nil || (len(schema.Type) == 0 &&
		len(schema.AllOf) == 0 &&
		len(schema.AnyOf) == 0 &&
		len(schema.OneOf) == 0)
}
