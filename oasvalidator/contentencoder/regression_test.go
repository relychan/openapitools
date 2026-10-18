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

package contentencoder

import (
	"bytes"
	"mime"
	"mime/multipart"
	"net/http"
	"testing"

	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/relychan/openapitools/oaschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type multipartTestPart struct {
	ContentType string
	FileName    string
	Body        string
}

func readMultipartParts(t *testing.T, body []byte, contentType string) map[string]multipartTestPart {
	t.Helper()

	mediaType, params, err := mime.ParseMediaType(contentType)
	require.NoError(t, err)
	require.Equal(t, "multipart/form-data", mediaType)
	require.NotEmpty(t, params["boundary"])

	reader := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	parts := map[string]multipartTestPart{}

	for {
		part, err := reader.NextPart()
		if err != nil {
			break
		}

		buf := new(bytes.Buffer)
		_, err = buf.ReadFrom(part)
		require.NoError(t, err)

		parts[part.FormName()] = multipartTestPart{
			ContentType: part.Header.Get("Content-Type"),
			FileName:    part.FileName(),
			Body:        buf.String(),
		}
	}

	return parts
}

func newFileEncoding(key string, contentType string) *orderedmap.Map[string, *highv3.Encoding] {
	result := orderedmap.New[string, *highv3.Encoding]()
	result.Set(key, &highv3.Encoding{ContentType: contentType})

	return result
}

func TestEncode_MultipartFormData(t *testing.T) {
	media := &oaschema.MediaType{
		ContentType: "multipart/form-data",
		Encoding:    newFileEncoding("file", "image/png"),
	}

	body, contentType, err := Encode(
		"multipart/form-data",
		map[string]any{"name": "alice", "file": "aGVsbG8="},
		media,
		nil,
	)
	require.NoError(t, err)

	parts := readMultipartParts(t, body, contentType)
	assert.Equal(t, "alice", parts["name"].Body)
	assert.Equal(t, "hello", parts["file"].Body)
	assert.Equal(t, "image/png", parts["file"].ContentType)

	// nil media must not panic.
	body, contentType, err = Encode("multipart/form-data", map[string]any{"a": "b"}, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "b", readMultipartParts(t, body, contentType)["a"].Body)
}

func TestWrite_MultipartFormDataUnsupported(t *testing.T) {
	_, err := Write(new(bytes.Buffer), "multipart/form-data", map[string]any{"a": "b"}, nil)
	require.ErrorIs(t, err, ErrMultipartWriteUnsupported)
}

func TestEncodeMultipartForm_FileArrays(t *testing.T) {
	testCases := []struct {
		name  string
		value any
	}{
		{
			name:  "[]any",
			value: map[string]any{"files": []any{"aGVsbG8=", "d29ybGQ="}},
		},
		{
			name:  "[]string",
			value: map[string]any{"files": []string{"aGVsbG8=", "d29ybGQ="}},
		},
		{
			name:  "[][]byte",
			value: map[string]any{"files": [][]byte{[]byte("hello"), []byte("world")}},
		},
	}

	media := &highv3.MediaType{Encoding: newFileEncoding("files", "image/png")}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body, contentType, err := EncodeMultipartForm(tc.value, nil, media)
			require.NoError(t, err)

			parts := readMultipartParts(t, body, contentType)
			assert.Equal(t, "hello", parts["files[0]"].Body)
			assert.Equal(t, "world", parts["files[1]"].Body)
			assert.Equal(t, "image/png", parts["files[0]"].ContentType)
			assert.Equal(t, "files[0]", parts["files[0]"].FileName)
		})
	}
}

func TestEncodeMultipartForm_PointerRoot(t *testing.T) {
	stringMap := map[string]string{"name": "alice"}
	anyMap := map[string]any{"age": 10, "tags": []any{"a", "b"}}
	slice := []string{"x", "y"}

	testCases := []struct {
		name     string
		value    any
		expected map[string]string
	}{
		{
			name:     "*map[string]string",
			value:    &stringMap,
			expected: map[string]string{"name": "alice"},
		},
		{
			name:     "map[string]string",
			value:    stringMap,
			expected: map[string]string{"name": "alice"},
		},
		{
			name:     "*map[string]any",
			value:    &anyMap,
			expected: map[string]string{"age": "10", "tags[0]": "a", "tags[1]": "b"},
		},
		{
			name:     "*[]string",
			value:    &slice,
			expected: map[string]string{"[0]": "x", "[1]": "y"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body, contentType, err := EncodeMultipartForm(tc.value, nil, &highv3.MediaType{})
			require.NoError(t, err)

			parts := readMultipartParts(t, body, contentType)
			for key, expected := range tc.expected {
				assert.Equal(t, expected, parts[key].Body, key)
			}
		})
	}
}

func TestEncodeMultipartForm_NilMedia(t *testing.T) {
	body, contentType, err := EncodeMultipartForm(
		map[string]any{"name": "alice"},
		http.Header{},
		nil,
	)
	require.NoError(t, err)
	assert.Equal(t, "alice", readMultipartParts(t, body, contentType)["name"].Body)
}

func TestEncodeMultipartForm_ReflectMapAsJSON(t *testing.T) {
	type nested map[string]int

	body, contentType, err := EncodeMultipartForm(
		map[string]any{"obj": nested{"a": 1}},
		nil,
		&highv3.MediaType{Encoding: newFileEncoding("obj", "image/png")},
	)
	require.NoError(t, err)

	part := readMultipartParts(t, body, contentType)["obj"]
	assert.Equal(t, `{"a":1}`, part.Body)
	assert.Equal(t, "application/json", part.ContentType)
}

func TestEncodeXML_InvalidElementNames(t *testing.T) {
	invalidKey := `a><injected>x</injected><b`

	testCases := []struct {
		name  string
		value any
	}{
		{name: "map[string]any", value: map[string]any{invalidKey: "v"}},
		{name: "map[any]any", value: map[any]any{invalidKey: "v"}},
		{name: "map[string]string", value: map[string]string{invalidKey: "v"}},
		{name: "starts with digit", value: map[string]any{"1abc": "v"}},
		{name: "contains space", value: map[string]any{"a b": "v"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := EncodeXML(tc.value)
			require.ErrorContains(t, err, "invalid XML element name")
			assert.Nil(t, result)
		})
	}

	result, err := EncodeXML(map[string]any{"ns:valid-name_1.x": "v", "é": "u"})
	require.NoError(t, err)
	assert.Contains(t, string(result), "<ns:valid-name_1.x>v</ns:valid-name_1.x>")
	assert.Contains(t, string(result), "<é>u</é>")
}

type yamlMarshalerStub struct {
	Value string
}

func (y yamlMarshalerStub) MarshalYAML() (any, error) {
	return map[string]string{"value": y.Value}, nil
}

func TestWriteBinary_YAMLMarshaler(t *testing.T) {
	buf := new(bytes.Buffer)

	_, err := WriteBinary(buf, yamlMarshalerStub{Value: "hello"})
	require.NoError(t, err)
	assert.Equal(t, "value: hello\n", buf.String())
}
