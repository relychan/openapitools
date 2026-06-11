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
	"testing"

	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/relychan/openapitools/oaschema"
	"github.com/stretchr/testify/assert"
)

func TestEncodeFormURLEncoded(t *testing.T) {
	testCases := []struct {
		name      string
		value     any
		mediaType *oaschema.MediaType
		expected  string
	}{
		{
			name:  "empty",
			value: nil,
			mediaType: &oaschema.MediaType{
				Encoding: func() *orderedmap.Map[string, *highv3.Encoding] {
					result := orderedmap.New[string, *highv3.Encoding]()
					result.Set("id", &highv3.Encoding{
						Style:   "form",
						Explode: new(false),
					})

					return result
				}(),
			},
			expected: "",
		},
		{
			name: "form_single_explode",
			value: map[string]any{
				"id": "3",
			},
			mediaType: &oaschema.MediaType{
				Encoding: func() *orderedmap.Map[string, *highv3.Encoding] {
					result := orderedmap.New[string, *highv3.Encoding]()
					result.Set("id", &highv3.Encoding{
						Style:   "form",
						Explode: new(true),
					})

					return result
				}(),
			},
			expected: "id=3",
		},
		{
			name: "form_single",
			value: map[string]any{
				"id": "3",
			},
			mediaType: &oaschema.MediaType{
				Encoding: func() *orderedmap.Map[string, *highv3.Encoding] {
					result := orderedmap.New[string, *highv3.Encoding]()
					result.Set("id", &highv3.Encoding{
						Style:   "form",
						Explode: new(false),
					})

					return result
				}(),
			},
			expected: "id=3",
		},
		{
			name: "form_array",
			value: map[string]any{
				"id": []any{"3", "4", "5"},
			},
			mediaType: &oaschema.MediaType{
				Encoding: func() *orderedmap.Map[string, *highv3.Encoding] {
					result := orderedmap.New[string, *highv3.Encoding]()
					result.Set("id", &highv3.Encoding{
						Style:         "form",
						Explode:       new(false),
						AllowReserved: true,
					})

					return result
				}(),
			},
			expected: "id=3,4,5",
		},
		{
			name: "form_array_explode",
			value: map[string]any{
				"id": []any{"3", "4", "5"},
			},
			mediaType: &oaschema.MediaType{
				Encoding: func() *orderedmap.Map[string, *highv3.Encoding] {
					result := orderedmap.New[string, *highv3.Encoding]()
					result.Set("id", &highv3.Encoding{
						Style:         "form",
						Explode:       new(true),
						AllowReserved: true,
					})

					return result
				}(),
			},
			expected: "id=3&id=4&id=5",
		},
		{
			name: "form_object",
			value: map[string]any{
				"id": map[any]any{
					"role": "admin",
				},
			},
			mediaType: &oaschema.MediaType{
				Encoding: func() *orderedmap.Map[string, *highv3.Encoding] {
					result := orderedmap.New[string, *highv3.Encoding]()
					result.Set("id", &highv3.Encoding{
						Style:         "form",
						Explode:       new(false),
						AllowReserved: true,
					})

					return result
				}(),
			},
			expected: "id=role,admin",
		},
		{
			name: "form_object_explode",
			value: map[string]any{
				"id": map[any]any{
					"role": "admin",
				},
			},
			mediaType: &oaschema.MediaType{
				Encoding: func() *orderedmap.Map[string, *highv3.Encoding] {
					result := orderedmap.New[string, *highv3.Encoding]()
					result.Set("id", &highv3.Encoding{
						Style:         "form",
						Explode:       new(true),
						AllowReserved: true,
					})

					return result
				}(),
			},
			expected: "role=admin",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := EncodeFormURLEncoded(tc.value, tc.mediaType)
			assert.Nil(t, err)
			assert.Contains(t, tc.expected, string(result))
		})
	}
}

// BenchmarkEncodeURLEncode-11    	  1000000	      1115 ns/op	    1328 B/op	      27 allocs/op
func BenchmarkEncodeURLEncode(b *testing.B) {
	value := map[string]any{
		"role": []any{
			map[string]any{
				"user": []any{
					[]any{"admin", "anonymous"},
				},
			},
		},
	}

	mediaType := &highv3.MediaType{
		Encoding: func() *orderedmap.Map[string, *highv3.Encoding] {
			result := orderedmap.New[string, *highv3.Encoding]()
			result.Set("role", &highv3.Encoding{
				Style:         "form",
				Explode:       new(true),
				AllowReserved: true,
			})

			return result
		}(),
	}

	mediaType2 := &oaschema.MediaType{
		Encoding: mediaType.Encoding,
	}

	for b.Loop() {
		_, err := EncodeFormURLEncoded(value, mediaType2)
		if err != nil {
			panic(err)
		}
	}
}
