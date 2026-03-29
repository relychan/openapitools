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

type binaryMarshalerStub struct {
	data []byte
	err  error
}

func (b *binaryMarshalerStub) MarshalBinary() ([]byte, error) {
	return b.data, b.err
}

var _ encoding.BinaryMarshaler = (*binaryMarshalerStub)(nil)

func TestEncodeBinary(t *testing.T) {
	testCases := []struct {
		name     string
		input    any
		expected []byte
		errMsg   string
	}{
		{
			name:     "bytes passthrough",
			input:    []byte("hello world"),
			expected: []byte("hello world"),
		},
		{
			name:     "nil any returns empty JSON null",
			input:    nil,
			expected: []byte("null"),
		},
		{
			name:     "BinaryMarshaler success",
			input:    &binaryMarshalerStub{data: []byte("binary data")},
			expected: []byte("binary data"),
		},
		{
			name:   "BinaryMarshaler error",
			input:  &binaryMarshalerStub{err: errors.New("marshal failed")},
			errMsg: "marshal failed",
		},
		{
			name:     "string falls through to JSON",
			input:    "hello",
			expected: []byte(`"hello"`),
		},
		{
			name:     "int falls through to JSON",
			input:    42,
			expected: []byte(`42`),
		},
		{
			name:     "map falls through to JSON",
			input:    map[string]any{"key": "value"},
			expected: []byte(`{"key":"value"}`),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := EncodeBinary(tc.input)
			if tc.errMsg != "" {
				assert.ErrorContains(t, err, tc.errMsg)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestWriteBinary(t *testing.T) {
	testCases := []struct {
		name     string
		input    any
		expected []byte
		errMsg   string
	}{
		{
			name:     "bytes passthrough",
			input:    []byte("hello world"),
			expected: []byte("hello world"),
		},
		{
			name:     "nil any writes JSON null",
			input:    nil,
			expected: []byte("null\n"),
		},
		{
			name:     "BinaryMarshaler success",
			input:    &binaryMarshalerStub{data: []byte("binary data")},
			expected: []byte("binary data"),
		},
		{
			name:   "BinaryMarshaler error",
			input:  &binaryMarshalerStub{err: errors.New("marshal failed")},
			errMsg: "marshal failed",
		},
		{
			name:     "map falls through to JSON",
			input:    map[string]any{"key": "value"},
			expected: []byte("{\"key\":\"value\"}\n"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			_, err := WriteBinary(buf, tc.input)
			if tc.errMsg != "" {
				assert.ErrorContains(t, err, tc.errMsg)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, buf.Bytes())
			}
		})
	}
}
