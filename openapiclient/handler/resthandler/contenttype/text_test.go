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
	"encoding"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type textMarshalerStub struct {
	data []byte
	err  error
}

func (t *textMarshalerStub) MarshalText() ([]byte, error) {
	return t.data, t.err
}

var _ encoding.TextMarshaler = (*textMarshalerStub)(nil)

func TestEncodeText(t *testing.T) {
	testCases := []struct {
		name     string
		input    any
		expected []byte
		errMsg   string
	}{
		{
			name:     "string scalar",
			input:    "hello",
			expected: []byte("hello"),
		},
		{
			name:     "int scalar",
			input:    42,
			expected: []byte("42"),
		},
		{
			name:     "float scalar",
			input:    3.14,
			expected: []byte("3.14"),
		},
		{
			name:     "bool scalar",
			input:    true,
			expected: []byte("true"),
		},
		{
			name:     "bytes passthrough",
			input:    []byte("raw bytes"),
			expected: []byte("raw bytes"),
		},
		{
			name:     "nil any falls through to JSON",
			input:    nil,
			expected: []byte("null"),
		},
		{
			name:     "TextMarshaler success",
			input:    &textMarshalerStub{data: []byte("text data")},
			expected: []byte("text data"),
		},
		{
			name:   "TextMarshaler error",
			input:  &textMarshalerStub{err: errors.New("text marshal failed")},
			errMsg: "text marshal failed",
		},
		{
			name:     "map falls through to JSON",
			input:    map[string]any{"key": "value"},
			expected: []byte("{\"key\":\"value\"}\n"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := EncodeText(tc.input)
			if tc.errMsg != "" {
				assert.ErrorContains(t, err, tc.errMsg)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestWriteText(t *testing.T) {
	testCases := []struct {
		name     string
		input    any
		expected []byte
		errMsg   string
	}{
		{
			name:     "string scalar",
			input:    "hello world",
			expected: []byte("hello world"),
		},
		{
			name:     "int scalar",
			input:    100,
			expected: []byte("100"),
		},
		{
			name:     "bytes passthrough",
			input:    []byte("raw bytes"),
			expected: []byte("raw bytes"),
		},
		{
			name:   "TextMarshaler error",
			input:  &textMarshalerStub{err: errors.New("marshal error")},
			errMsg: "marshal error",
		},
		{
			name:     "TextMarshaler success",
			input:    &textMarshalerStub{data: []byte("marshaled")},
			expected: []byte("marshaled"),
		},
		{
			name:     "map falls through to JSON",
			input:    map[string]any{"a": "b"},
			expected: []byte("{\"a\":\"b\"}\n"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			_, err := WriteText(buf, tc.input)
			if tc.errMsg != "" {
				assert.ErrorContains(t, err, tc.errMsg)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, buf.Bytes())
			}
		})
	}
}
