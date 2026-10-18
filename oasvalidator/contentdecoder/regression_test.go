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

package contentdecoder

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeDataURI_ASCIIPercentDecoding(t *testing.T) {
	result, err := DecodeDataURI("data:text/plain,hello%20world")
	require.NoError(t, err)
	assert.Equal(t, "hello world", string(result.Data))
	assert.Equal(t, "text/plain", result.MediaType)

	_, err = DecodeDataURI("data:text/plain,bad%zz")
	require.Error(t, err)
}

func TestDecode_EmptyBody(t *testing.T) {
	contentTypes := []string{"application/json", "application/problem+json", "application/xml", "text/xml"}

	readers := map[string]func() io.Reader{
		"http.NoBody":  func() io.Reader { return http.NoBody },
		"empty reader": func() io.Reader { return strings.NewReader("") },
		"empty buffer": func() io.Reader { return new(bytes.Buffer) },
	}

	for _, contentType := range contentTypes {
		for name, newReader := range readers {
			t.Run(contentType+"/"+name, func(t *testing.T) {
				result, err := Decode(contentType, newReader())
				require.NoError(t, err)
				assert.Nil(t, result)
			})
		}

		t.Run(contentType+"/unmarshal", func(t *testing.T) {
			result, err := Unmarshal(contentType, []byte{})
			require.NoError(t, err)
			assert.Nil(t, result)
		})
	}

	// non-empty bodies still decode after peeking.
	result, err := Decode("application/json", strings.NewReader(`{"a":1}`))
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"a": float64(1)}, result)

	result, err = Decode("application/xml", strings.NewReader(`<a>1</a>`))
	require.NoError(t, err)
	assert.Equal(t, "1", result)

	// malformed bodies are still errors.
	_, err = Decode("application/json", strings.NewReader(`{`))
	require.Error(t, err)
}
