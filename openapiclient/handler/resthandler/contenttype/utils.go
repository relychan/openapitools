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

package contenttype

import (
	"net/http"
	"strings"

	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/relychan/goutils/httpheader"
)

// IsContentTypeXML checks if the content type is XML.
func IsContentTypeXML(contentType string) bool {
	return strings.HasPrefix(contentType, httpheader.ContentTypeXML) ||
		strings.HasPrefix(contentType, httpheader.ContentTypeTextXML)
}

func getHeadersFromSchema(
	headers http.Header,
	schema *orderedmap.Map[string, *highv3.Header],
) http.Header {
	result := http.Header{}

	for iter := schema.First(); iter != nil; iter = iter.Next() {
		key := iter.Key()

		value := headers.Get(key)
		if value != "" {
			result.Set(key, value)
		}
	}

	return result
}
