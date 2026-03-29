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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecodeDataURI(t *testing.T) {
	testCases := []struct {
		input    string
		expected DataURI
		errorMsg string
	}{
		{
			input: "data:image/png;a=b;base64,aGVsbG8gd29ybGQ=",
			expected: DataURI{
				MediaType: "image/png",
				Parameters: map[string]string{
					"a": "b",
				},
				Data: []byte("hello world"),
			},
		},
		{
			input: "data:text/plain,hello_world",
			expected: DataURI{
				MediaType:  "text/plain",
				Data:       []byte("hello_world"),
				Parameters: map[string]string{},
			},
		},
		{
			input: "data:text/plain;ascii,hello_world",
			expected: DataURI{
				MediaType:  "text/plain",
				Data:       []byte("hello_world"),
				Parameters: map[string]string{},
			},
		},
		{
			input: "aGVsbG8gd29ybGQ=",
			expected: DataURI{
				Data: []byte("hello world"),
			},
		},
		{
			input:    "aadawdda ada",
			errorMsg: "illegal base64 data at input byte",
		},
		{
			input:    "data:text/plain",
			errorMsg: "invalid data uri",
		},
		{
			input:    "data:image/png;a=b;base64, test =",
			errorMsg: "illegal base64 data at input",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			data, err := DecodeDataURI(tc.input)

			if tc.errorMsg != "" {
				assert.ErrorContains(t, err, tc.errorMsg)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, *data)
			}
		})
	}
}
