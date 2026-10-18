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

package proxyhandler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/relychan/openapitools/oaschema"
	"github.com/stretchr/testify/assert"
)

func TestForwardResponseHeaders_MultiValueAndFraming(t *testing.T) {
	options := &ProxyHandleOptions{
		Settings: &oaschema.OpenAPIResourceSettings{
			ForwardHeaders: &oaschema.OpenAPIForwardHeadersConfig{
				Response: []string{
					"set-cookie",
					"X-Request-Id",
					"Content-Length",
					"Content-Encoding",
					"Transfer-Encoding",
					"X-Missing",
				},
			},
		},
	}

	response := &http.Response{
		Header: http.Header{
			"Set-Cookie":        []string{"a=1", "b=2"},
			"X-Request-Id":      []string{"req-1"},
			"Content-Length":    []string{"100"},
			"Content-Encoding":  []string{"gzip"},
			"Transfer-Encoding": []string{"chunked"},
		},
	}

	writer := httptest.NewRecorder()
	options.ForwardResponseHeaders(writer, response)

	assert.Equal(t, []string{"a=1", "b=2"}, writer.Header().Values("Set-Cookie"))
	assert.Equal(t, "req-1", writer.Header().Get("X-Request-Id"))
	assert.Empty(t, writer.Header().Values("Content-Length"))
	assert.Empty(t, writer.Header().Values("Content-Encoding"))
	assert.Empty(t, writer.Header().Values("Transfer-Encoding"))
	assert.Empty(t, writer.Header().Values("X-Missing"))

	// the response header slice must not be shared with the writer.
	writer.Header().Add("Set-Cookie", "c=3")
	assert.Equal(t, []string{"a=1", "b=2"}, response.Header.Values("Set-Cookie"))
}
