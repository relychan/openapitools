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

func mergeOrderedMaps[K comparable, V any](dest, src *orderedmap.Map[K, V]) *orderedmap.Map[K, V] {
	if src == nil || src.Len() == 0 {
		return dest
	}

	if dest == nil {
		return src
	}

	for iter := src.Oldest(); iter != nil; iter = iter.Next() {
		dest.Set(iter.Key, iter.Value)
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
			if IsContentTypeJSON(item) {
				return item
			}
		}
	}

	return contentType
}

// IsContentTypeXML checks if the content type is XML.
func IsContentTypeXML(contentType string) bool {
	return strings.HasPrefix(contentType, httpheader.ContentTypeXML) ||
		strings.HasPrefix(contentType, httpheader.ContentTypeTextXML) ||
		strings.HasSuffix(contentType, "+xml")
}

// IsContentTypeJSON checks if the content type is JSON.
func IsContentTypeJSON(contentType string) bool {
	return strings.HasPrefix(contentType, httpheader.ContentTypeJSON) ||
		strings.HasSuffix(contentType, "+json")
}

// IsContentTypeText checks if the content type relates to text.
func IsContentTypeText(contentType string) bool {
	return strings.HasPrefix(contentType, "text/")
}

// IsContentTypeMultipartForm checks the content type relates to multipart form.
func IsContentTypeMultipartForm(contentType string) bool {
	return strings.HasPrefix(contentType, "multipart/")
}
