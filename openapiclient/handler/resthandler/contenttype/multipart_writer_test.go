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
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func parseMultipartBody(t *testing.T, body []byte, contentType string) map[string][]byte {
	t.Helper()

	_, params, err := mime.ParseMediaType(contentType)
	require.NoError(t, err)

	reader := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	parts := map[string][]byte{}

	for {
		part, err := reader.NextPart()
		if err != nil {
			break
		}

		buf := new(bytes.Buffer)
		_, readErr := buf.ReadFrom(part)
		require.NoError(t, readErr)

		parts[part.FormName()] = buf.Bytes()
	}

	return parts
}

func TestMultipartWriterWriteField(t *testing.T) {
	buf := new(bytes.Buffer)
	w := NewMultipartWriter(buf)

	err := w.WriteField("username", "alice", http.Header{})
	require.NoError(t, err)

	require.NoError(t, w.Close())

	parts := parseMultipartBody(t, buf.Bytes(), w.FormDataContentType())
	assert.Equal(t, []byte("alice"), parts["username"])
}

func TestMultipartWriterWriteFieldWithHeaders(t *testing.T) {
	buf := new(bytes.Buffer)
	w := NewMultipartWriter(buf)

	headers := http.Header{"X-Custom": []string{"custom-value"}}
	err := w.WriteField("email", "alice@example.com", headers)
	require.NoError(t, err)

	require.NoError(t, w.Close())

	parts := parseMultipartBody(t, buf.Bytes(), w.FormDataContentType())
	assert.Equal(t, []byte("alice@example.com"), parts["email"])
}

func TestMultipartWriterWriteJSON(t *testing.T) {
	buf := new(bytes.Buffer)
	w := NewMultipartWriter(buf)

	err := w.WriteJSON("data", map[string]any{"id": 1, "name": "test"}, http.Header{})
	require.NoError(t, err)

	require.NoError(t, w.Close())

	parts := parseMultipartBody(t, buf.Bytes(), w.FormDataContentType())
	assert.Contains(t, string(parts["data"]), `"id"`)
	assert.Contains(t, string(parts["data"]), `"name"`)
}

func TestMultipartWriterWriteXML(t *testing.T) {
	buf := new(bytes.Buffer)
	w := NewMultipartWriter(buf)

	err := w.WriteXML("xmlField", map[string]any{"name": "test"}, http.Header{})
	require.NoError(t, err)

	require.NoError(t, w.Close())

	parts := parseMultipartBody(t, buf.Bytes(), w.FormDataContentType())
	assert.Contains(t, string(parts["xmlField"]), "<name>test</name>")
}

func TestMultipartWriterWriteBinary(t *testing.T) {
	buf := new(bytes.Buffer)
	w := NewMultipartWriter(buf)

	err := w.WriteBinary("file", []byte("file content"), "image/png", http.Header{})
	require.NoError(t, err)

	require.NoError(t, w.Close())

	// Binary parts use filename not FormName, so parse raw body
	rawBody := buf.String()
	assert.Contains(t, rawBody, "file content")
	assert.Contains(t, rawBody, "image/png")
}

func TestMultipartWriterWriteBinaryDefaultContentType(t *testing.T) {
	buf := new(bytes.Buffer)
	w := NewMultipartWriter(buf)

	err := w.WriteBinary("blob", []byte("data"), "", http.Header{})
	require.NoError(t, err)

	require.NoError(t, w.Close())

	assert.Contains(t, buf.String(), "application/octet-stream")
}

func TestMultipartWriterWriteDataURI(t *testing.T) {
	buf := new(bytes.Buffer)
	w := NewMultipartWriter(buf)

	// "hello world" base64 encoded
	err := w.WriteDataURI("upload", "data:text/plain;base64,aGVsbG8gd29ybGQ=", "text/plain", http.Header{})
	require.NoError(t, err)

	require.NoError(t, w.Close())

	assert.Contains(t, buf.String(), "hello world")
}

func TestMultipartWriterWriteDataURIInvalidBase64(t *testing.T) {
	buf := new(bytes.Buffer)
	w := NewMultipartWriter(buf)

	err := w.WriteDataURI("upload", "not valid base64 !!", "text/plain", http.Header{})
	assert.Error(t, err)
}

func TestMultipartWriterWriteDataURIPlainBase64(t *testing.T) {
	buf := new(bytes.Buffer)
	w := NewMultipartWriter(buf)

	// plain base64 without data: URI prefix
	err := w.WriteDataURI("file", "aGVsbG8gd29ybGQ=", "application/octet-stream", http.Header{})
	require.NoError(t, err)

	require.NoError(t, w.Close())

	assert.Contains(t, buf.String(), "hello world")
}

func TestMultipartWriterMultipleFields(t *testing.T) {
	buf := new(bytes.Buffer)
	w := NewMultipartWriter(buf)

	require.NoError(t, w.WriteField("name", "Alice", http.Header{}))
	require.NoError(t, w.WriteJSON("meta", map[string]any{"role": "admin"}, http.Header{}))
	require.NoError(t, w.WriteBinary("avatar", []byte("png bytes"), "image/png", http.Header{}))
	require.NoError(t, w.Close())

	rawBody := buf.String()
	assert.Contains(t, rawBody, "Alice")
	assert.Contains(t, rawBody, `"role"`)
	assert.Contains(t, rawBody, "png bytes")

	// Verify we have 3 boundary separators
	boundary := w.Boundary()
	assert.Equal(t, 4, strings.Count(rawBody, "--"+boundary))
}
