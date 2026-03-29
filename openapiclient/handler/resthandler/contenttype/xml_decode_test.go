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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeXML(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected any
		errMsg   string
	}{
		{
			name:     "simple element with text",
			input:    `<root>hello</root>`,
			expected: "hello",
		},
		{
			name:  "nested elements",
			input: `<root><name>test</name><value>42</value></root>`,
			expected: map[string]any{
				"name":  "test",
				"value": "42",
			},
		},
		{
			name:  "element with attributes",
			input: `<root id="1"><name>test</name></root>`,
			expected: map[string]any{
				"attributes": map[string]string{"id": "1"},
				"name":       "test",
			},
		},
		{
			name:  "element with attributes and text content only",
			input: `<root id="1">hello</root>`,
			expected: map[string]any{
				"attributes": map[string]string{"id": "1"},
				"content":    "hello",
			},
		},
		{
			name:  "repeated child elements become array",
			input: `<root><item>a</item><item>b</item><item>c</item></root>`,
			expected: map[string]any{
				"item": []any{"a", "b", "c"},
			},
		},
		{
			name:  "single child element",
			input: `<root><child>value</child></root>`,
			expected: map[string]any{
				"child": "value",
			},
		},
		{
			name:     "empty reader returns nil",
			input:    ``,
			expected: nil,
			errMsg:   "EOF",
		},
		{
			name:   "malformed XML returns error",
			input:  `<root><unclosed>`,
			errMsg: "EOF",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := DecodeXML(strings.NewReader(tc.input))
			if tc.errMsg != "" {
				assert.ErrorContains(t, err, tc.errMsg)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestDecodeXMLRoundtrip(t *testing.T) {
	testCases := []struct {
		name  string
		input map[string]any
	}{
		{
			name: "flat object",
			input: map[string]any{
				"id":     "10",
				"status": "active",
			},
		},
		{
			name: "nested object",
			input: map[string]any{
				"user": map[string]any{
					"name": "Alice",
					"age":  "30",
				},
			},
		},
		{
			name: "array of objects",
			input: map[string]any{
				"item": []any{
					map[string]any{"id": "1", "name": "first"},
					map[string]any{"id": "2", "name": "second"},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := EncodeXML(tc.input)
			require.NoError(t, err)

			decoded, err := DecodeXML(strings.NewReader(string(encoded)))
			require.NoError(t, err)

			assert.Equal(t, tc.input, decoded)
		})
	}
}
