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
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncode(t *testing.T) {
	testCases := []struct {
		name        string
		contentType string
		body        any
		expected    []byte
		errMsg      string
	}{
		{
			name:        "JSON content type",
			contentType: "application/json",
			body:        map[string]any{"key": "value"},
			expected:    []byte(`{"key":"value"}`),
		},
		{
			name:        "JSON content type with charset",
			contentType: "application/json; charset=utf-8",
			body:        42,
			expected:    []byte(`42`),
		},
		{
			name:        "XML content type",
			contentType: "application/xml",
			body:        map[string]any{"name": "test"},
			expected:    []byte("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<xml><name>test</name></xml>"),
		},
		{
			name:        "text/plain content type",
			contentType: "text/plain",
			body:        "hello text",
			expected:    []byte("hello text"),
		},
		{
			name:        "text/html content type",
			contentType: "text/html",
			body:        "<b>bold</b>",
			expected:    []byte("<b>bold</b>"),
		},
		{
			name:        "binary (octet-stream) content type",
			contentType: "application/octet-stream",
			body:        []byte("binary content"),
			expected:    []byte("binary content"),
		},
		{
			name:        "unknown content type defaults to binary",
			contentType: "application/custom",
			body:        []byte("custom data"),
			expected:    []byte("custom data"),
		},
		{
			name:        "unknown content type with struct falls through to JSON",
			contentType: "application/custom",
			body:        map[string]any{"x": 1},
			expected:    []byte(`{"x":1}`),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Encode(tc.contentType, tc.body)
			if tc.errMsg != "" {
				assert.ErrorContains(t, err, tc.errMsg)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestWrite(t *testing.T) {
	testCases := []struct {
		name        string
		contentType string
		body        any
		expected    string
	}{
		{
			name:        "JSON content type",
			contentType: "application/json",
			body:        map[string]any{"a": "b"},
			expected:    "{\"a\":\"b\"}\n",
		},
		{
			name:        "XML content type",
			contentType: "text/xml",
			body:        map[string]any{"tag": "val"},
			expected:    "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<xml><tag>val</tag></xml>",
		},
		{
			name:        "text content type",
			contentType: "text/plain",
			body:        "plain text",
			expected:    "plain text",
		},
		{
			name:        "binary content type",
			contentType: "application/octet-stream",
			body:        []byte("bytes"),
			expected:    "bytes",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			_, err := Write(buf, tc.contentType, tc.body)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, buf.String())
		})
	}
}

func TestDecode(t *testing.T) {
	testCases := []struct {
		name        string
		contentType string
		body        string
		expected    any
		errMsg      string
	}{
		{
			name:        "nil reader returns nil",
			contentType: "application/json",
			body:        "",
			expected:    nil,
		},
		{
			name:        "JSON content type",
			contentType: "application/json",
			body:        `{"key":"value"}`,
			expected:    map[string]any{"key": "value"},
		},
		{
			name:        "JSON array",
			contentType: "application/json",
			body:        `[1,2,3]`,
			expected:    []any{float64(1), float64(2), float64(3)},
		},
		{
			name:        "XML content type",
			contentType: "application/xml",
			body:        `<?xml version="1.0" encoding="UTF-8"?><xml><name>test</name></xml>`,
			expected:    map[string]any{"name": "test"},
		},
		{
			name:        "text/plain content type",
			contentType: "text/plain",
			body:        "hello world",
			expected:    "hello world",
		},
		{
			name:        "binary content type",
			contentType: "application/octet-stream",
			body:        "raw bytes",
			expected:    []byte("raw bytes"),
		},
		{
			name:        "unknown content type defaults to binary",
			contentType: "application/custom",
			body:        "custom",
			expected:    []byte("custom"),
		},
		{
			name:        "invalid JSON returns error",
			contentType: "application/json",
			body:        `{invalid json}`,
			errMsg:      "invalid character",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var reader *strings.Reader
			if tc.expected == nil && tc.errMsg == "" {
				result, err := Decode(tc.contentType, nil)
				require.NoError(t, err)
				assert.Nil(t, result)
				return
			}

			reader = strings.NewReader(tc.body)
			result, err := Decode(tc.contentType, reader)
			if tc.errMsg != "" {
				assert.ErrorContains(t, err, tc.errMsg)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}
