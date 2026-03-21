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
	"mime"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/relychan/goutils/httpheader"
	"github.com/stretchr/testify/assert"
)

func TestEncodeMultipartForm(t *testing.T) {
	testCases := []struct {
		Name      string
		Value     any
		Headers   http.Header
		MediaType *v3.MediaType
		Expected  string
	}{
		{
			Name: "simple",
			Value: map[string]any{
				"name": "bar",
				"age":  10,
				"address": map[string]any{
					"street": "3, Garden St",
					"city":   "Hillsbery, UT",
				},
				"profileImage": "aGVsbG8gd29ybGQ=",
				"xmlData": []any{
					map[string]any{
						"street": "3, Garden St",
					},
					map[string]any{
						"city": "Hillsbery, UT",
					},
				},
			},
			Headers: http.Header{
				"X-Custom-Header": []string{"x-header"},
			},
			MediaType: &v3.MediaType{
				Encoding: func() *orderedmap.Map[string, *v3.Encoding] {
					headers := orderedmap.New[string, *v3.Header]()
					headers.Set("X-Custom-Header", &v3.Header{
						Description: "This is a custom header",
						Schema: base.CreateSchemaProxy(&base.Schema{
							Type: []string{"string"},
						}),
					})

					result := orderedmap.New[string, *v3.Encoding]()
					result.Set("profileImage", &v3.Encoding{
						ContentType: "image/png",
						Headers:     headers,
					})
					result.Set("xmlData", &v3.Encoding{
						ContentType: httpheader.ContentTypeTextXML,
					})
					return result
				}(),
			},
			Expected: `--
Content-Disposition: form-data; name="xmlData"
Content-Type: text/xml

<?xml version="1.0" encoding="UTF-8"?>
<xml><xml><street>3, Garden St</street></xml><xml><city>Hillsbery, UT</city></xml></xml>
--
Content-Disposition: form-data; name="profileImage"; filename="profileImage"
Content-Type: image/png
X-Custom-Header: x-header

hello world
--
Content-Disposition: form-data; name="name"
Content-Type: text/plain

bar
--
Content-Disposition: form-data; name="age"
Content-Type: text/plain

10
--
Content-Disposition: form-data; name="address"
Content-Type: application/json

{"city":"Hillsbery, UT","street":"3, Garden St"}
----`,
		},
	}

	splitFormDataString := func(input string) []string {
		blocks := strings.Split(input, "--\n")

		blocks[len(blocks)-1] = strings.TrimRight(blocks[len(blocks)-1], "-")
		slices.Sort(blocks)

		return blocks
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			encodedValue, contentType, err := EncodeMultipartForm(tc.Value, tc.Headers, tc.MediaType)
			assert.Nil(t, err)
			mediaType, headers, parseErr := mime.ParseMediaType(contentType)
			assert.NoError(t, parseErr)
			assert.Equal(t, "multipart/form-data", mediaType)
			boundary := headers["boundary"]
			result := strings.TrimSpace(
				strings.ReplaceAll(
					strings.ReplaceAll(string(encodedValue), boundary, ""),
					"\r", ""),
			)
			assert.Equal(t, splitFormDataString(tc.Expected), splitFormDataString(result))
		})
	}
}

// cpu: Apple M3 Pro
// BenchmarkEncodeMultipartForm-11    	  129406	      8532 ns/op	   13202 B/op	     142 allocs/op
func BenchmarkEncodeMultipartForm(b *testing.B) {
	value := map[string]any{
		"name": "bar",
		"age":  10,
		"address": map[string]any{
			"street": "3, Garden St",
			"city":   "Hillsbery, UT",
		},
		"profileImage": "aGVsbG8gd29ybGQ=",
		"xmlData": map[string]any{
			"street": "3, Garden St",
			"city":   "Hillsbery, UT",
		},
	}
	headers := http.Header{
		"X-Custom-Header": []string{"x-header"},
	}

	mediaType := &v3.MediaType{
		Encoding: func() *orderedmap.Map[string, *v3.Encoding] {
			headers := orderedmap.New[string, *v3.Header]()
			headers.Set("X-Custom-Header", &v3.Header{
				Description: "This is a custom header",
				Schema: base.CreateSchemaProxy(&base.Schema{
					Type: []string{"string"},
				}),
			})

			result := orderedmap.New[string, *v3.Encoding]()
			result.Set("profileImage", &v3.Encoding{
				ContentType: "image/png",
				Headers:     headers,
			})
			result.Set("xmlData", &v3.Encoding{
				ContentType: httpheader.ContentTypeTextXML,
			})
			return result
		}(),
	}

	for b.Loop() {
		EncodeMultipartForm(value, headers, mediaType)
	}
}
