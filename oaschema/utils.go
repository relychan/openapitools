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

// IsSchemaTypeEmpty checks if the schema type is empty.
func IsSchemaTypeEmpty(schema *base.Schema) bool {
	return schema == nil || (len(schema.Type) == 0 &&
		len(schema.AllOf) == 0 &&
		len(schema.AnyOf) == 0 &&
		len(schema.OneOf) == 0 &&
		schema.Properties == nil &&
		schema.AdditionalProperties == nil &&
		schema.Items == nil)
}

// ExtractSchemaTypes returns available types of the schema, and check if it is nullable.
func ExtractSchemaTypes(schema *base.Schema) ( //nolint:revive,nonamedreturns
	types []string,
	allOf []*base.Schema,
	oneOf []*base.Schema,
	anyOf []*base.Schema,
	isNullable bool,
) {
	if schema == nil {
		return nil, nil, nil, nil, true
	}

	allOf = ExtractSchemaProxies(schema.AllOf)
	oneOf = ExtractSchemaProxies(schema.OneOf)
	anyOf = ExtractSchemaProxies(schema.AnyOf)

	types = make(
		[]string, 0,
		max(1, len(schema.Type)+len(allOf)+len(oneOf)+len(anyOf)),
	)

	evalSchema := func(item *base.Schema) {
		for _, schemaType := range item.Type {
			normalizedType, _ := NormalizeType(schemaType)

			if !slices.Contains(types, normalizedType) {
				types = append(types, normalizedType)
			}
		}

		isNullable = isNullable || (item.Nullable != nil && *item.Nullable)

		if len(item.Type) > 0 {
			return
		}

		if ((item.Properties != nil && item.Properties.Len() > 0) || item.AdditionalProperties != nil ||
			(item.PatternProperties != nil && item.PatternProperties.Len() > 0)) &&
			!slices.Contains(types, Object) {
			types = append(types, Object)

			return
		}

		if item.Items != nil && !slices.Contains(types, Array) {
			types = append(types, Array)
		}
	}

	evalUnionType := func(schemas []*base.Schema) {
		for _, item := range schemas {
			evalSchema(item)
		}
	}

	evalSchema(schema)
	evalUnionType(allOf)
	evalUnionType(oneOf)
	evalUnionType(anyOf)

	if len(types) > 0 {
		types = slices.Clip(types)
	}

	return types, allOf, oneOf, anyOf, isNullable
}

// ExtractSchemaProxies returns schema references of schema proxies.
func ExtractSchemaProxies(proxies []*base.SchemaProxy) []*base.Schema {
	results := make([]*base.Schema, 0, len(proxies))

	for _, item := range proxies {
		if item == nil {
			continue
		}

		itemSchema := item.Schema()
		if itemSchema == nil {
			continue
		}

		results = append(results, itemSchema)
	}

	return slices.Clip(results)
}

// GetUnionSchemaTypes returns unique types of union schemas.
func GetUnionSchemaTypes(schemas []*base.Schema) ([]string, bool) {
	if len(schemas) == 0 {
		return nil, false
	}

	var (
		results  = make([]string, 0, 2)
		nullable bool
	)

	for _, item := range schemas {
		if item == nil {
			continue
		}

		nullable = nullable || (item.Nullable != nil && *item.Nullable)

		if len(item.Type) == 0 {
			continue
		}

		for _, t := range item.Type {
			if t == "" {
				continue
			}

			if t == "null" {
				nullable = true

				continue
			}

			nt, _ := NormalizeType(t)

			if !slices.Contains(results, nt) {
				results = append(results, nt)
			}
		}
	}

	return slices.Clip(results), nullable
}
