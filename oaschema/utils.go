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
	"mime"
	"slices"
	"strings"

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

// ValidateContentType validates the content type and prefer the application/json content type
// if the content type string has many content types.
func ValidateContentType(contentType string) (string, error) {
	if contentType == "" {
		return contentType, nil
	}

	var result string

	for item := range strings.SplitSeq(contentType, ",") {
		trimmed := strings.TrimSpace(item)

		parsed, _, err := mime.ParseMediaType(trimmed)
		if err != nil {
			continue
		}

		if parsed == httpheader.ContentTypeJSON {
			return trimmed, nil
		}

		if result == "" {
			result = trimmed
		}
	}

	if result != "" {
		return result, nil
	}

	return "", ErrInvalidContentType
}

// EqualContentType checks if both content type are equal with parameters excluded.
func EqualContentType(left, right string) bool {
	leftMediaType, _, _ := strings.Cut(left, ";")
	rightMediaType, _, _ := strings.Cut(right, ";")

	return strings.EqualFold(
		strings.TrimSpace(leftMediaType),
		strings.TrimSpace(rightMediaType),
	)
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
