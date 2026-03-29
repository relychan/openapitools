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
	"testing"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/stretchr/testify/assert"
)

func newTestHeaderSchema(keys ...string) *orderedmap.Map[string, *highv3.Header] {
	schema := orderedmap.New[string, *highv3.Header]()
	for _, key := range keys {
		schema.Set(key, &highv3.Header{
			Schema: base.CreateSchemaProxy(&base.Schema{
				Type: []string{"string"},
			}),
		})
	}

	return schema
}

func TestGetHeadersFromSchema(t *testing.T) {
	testCases := []struct {
		name     string
		headers  http.Header
		schema   []string
		expected http.Header
	}{
		{
			name: "matching headers are extracted",
			headers: http.Header{
				"X-Request-Id": []string{"abc-123"},
				"Content-Type": []string{"application/json"},
			},
			schema:   []string{"X-Request-Id"},
			expected: http.Header{"X-Request-Id": []string{"abc-123"}},
		},
		{
			name: "non-matching headers are excluded",
			headers: http.Header{
				"Authorization": []string{"Bearer token"},
			},
			schema:   []string{"X-Custom-Header"},
			expected: http.Header{},
		},
		{
			name:     "empty schema returns empty result",
			headers:  http.Header{"X-Header": []string{"value"}},
			schema:   []string{},
			expected: http.Header{},
		},
		{
			name:     "empty headers returns empty result",
			headers:  http.Header{},
			schema:   []string{"X-Custom"},
			expected: http.Header{},
		},
		{
			name: "multiple schema keys with partial matches",
			headers: http.Header{
				"X-Trace-Id": []string{"trace-001"},
				"X-Span-Id":  []string{"span-001"},
			},
			schema: []string{"X-Trace-Id", "X-Span-Id", "X-Missing"},
			expected: http.Header{
				"X-Trace-Id": []string{"trace-001"},
				"X-Span-Id":  []string{"span-001"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			schema := newTestHeaderSchema(tc.schema...)
			result := getHeadersFromSchema(tc.headers, schema)
			assert.Equal(t, tc.expected, result)
		})
	}
}
