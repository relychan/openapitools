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

	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/stretchr/testify/assert"
)

func TestEncodeFormURLEncoded(t *testing.T) {
	testCases := []struct {
		name      string
		value     any
		mediaType *highv3.MediaType
		expected  []string
	}{
		{
			name:  "empty",
			value: nil,
			mediaType: &highv3.MediaType{
				Encoding: func() *orderedmap.Map[string, *highv3.Encoding] {
					result := orderedmap.New[string, *highv3.Encoding]()
					result.Set("id", &highv3.Encoding{
						Style:   "form",
						Explode: new(false),
					})

					return result
				}(),
			},
			expected: []string{""},
		},
		{
			name:  "form_explode_primitive",
			value: 3,
			mediaType: &highv3.MediaType{
				Encoding: func() *orderedmap.Map[string, *highv3.Encoding] {
					result := orderedmap.New[string, *highv3.Encoding]()
					result.Set("id", &highv3.Encoding{
						Style:   "form",
						Explode: new(true),
					})

					return result
				}(),
			},
			expected: []string{"3"},
		},
		{
			name: "form_single_explode",
			value: map[string]any{
				"id": "3",
			},
			mediaType: &highv3.MediaType{
				Encoding: func() *orderedmap.Map[string, *highv3.Encoding] {
					result := orderedmap.New[string, *highv3.Encoding]()
					result.Set("id", &highv3.Encoding{
						Style:   "form",
						Explode: new(true),
					})

					return result
				}(),
			},
			expected: []string{"id=3"},
		},
		{
			name: "form_single",
			value: map[string]any{
				"id": "3",
			},
			mediaType: &highv3.MediaType{
				Encoding: func() *orderedmap.Map[string, *highv3.Encoding] {
					result := orderedmap.New[string, *highv3.Encoding]()
					result.Set("id", &highv3.Encoding{
						Style:   "form",
						Explode: new(false),
					})

					return result
				}(),
			},
			expected: []string{"id=3"},
		},
		{
			name: "form_array",
			value: map[string]any{
				"id": []any{"3", "4", "5"},
			},
			mediaType: &highv3.MediaType{
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
			expected: []string{"id=3,4,5"},
		},
		{
			name: "form_array_explode",
			value: map[string]any{
				"id": []any{"3", "4", "5"},
			},
			mediaType: &highv3.MediaType{
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
			expected: []string{"id=3&id=4&id=5"},
		},
		{
			name: "form_object",
			value: map[string]any{
				"id": map[any]any{
					"role": "admin",
				},
			},
			mediaType: &highv3.MediaType{
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
			expected: []string{"id=role,admin"},
		},
		{
			name: "form_object_explode",
			value: map[string]any{
				"id": map[any]any{
					"role": "admin",
				},
			},
			mediaType: &highv3.MediaType{
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
			expected: []string{"role=admin"},
		},
		// {
		// 	name: "form_array_object",
		// 	value: map[any]any{
		// 		"role": []any{
		// 			map[string]any{
		// 				"user": "admin",
		// 			},
		// 		},
		// 	},
		// 	encoding: BaseParameter{
		// 		Name:          "id",
		// 		In:            oaschema.InQuery,
		// 		Explode:       new(false),
		// 		Style:         (oaschema.EncodingStyleForm),
		// 		AllowReserved: true,
		// 	},
		// 	expected: []string{"id=role[0][user],admin"},
		// },
		// {
		// 	name: "form_explode_array_object_multiple",
		// 	value: map[any]any{
		// 		"role": []any{
		// 			map[string]any{
		// 				"user": []any{
		// 					[]any{"admin", "anonymous"},
		// 				},
		// 			},
		// 		},
		// 	},
		// 	encoding: BaseParameter{
		// 		Name:          "id",
		// 		In:            oaschema.InQuery,
		// 		Explode:       new(true),
		// 		Style:         (oaschema.EncodingStyleForm),
		// 		AllowReserved: true,
		// 	},
		// 	expected: []string{
		// 		"role[0][user][0]=admin&role[0][user][0]=anonymous",
		// 		"role[0][user][0]=anonymous&role[0][user][0]=admin",
		// 	},
		// },
		// {
		// 	name:  "spaceDelimited_array",
		// 	value: []string{"3", "4", "5"},
		// 	encoding: BaseParameter{
		// 		Name:          "id",
		// 		In:            oaschema.InQuery,
		// 		Explode:       new(false),
		// 		Style:         (oaschema.EncodingStyleSpaceDelimited),
		// 		AllowReserved: true,
		// 	},
		// 	expected: []string{"id=3+4+5"},
		// },
		// {
		// 	name: "spaceDelimited_object",
		// 	value: map[string]any{
		// 		"R": "100",
		// 		"G": "200",
		// 	},
		// 	encoding: BaseParameter{
		// 		Name:          "color",
		// 		In:            oaschema.InQuery,
		// 		Style:         (oaschema.EncodingStyleSpaceDelimited),
		// 		Explode:       new(false),
		// 		AllowReserved: false,
		// 	},
		// 	expected: []string{
		// 		"color=G+200+R+100",
		// 		"color=R+100+G+200",
		// 	},
		// },
		// {
		// 	name:  "spaceDelimited_explode_array",
		// 	value: []any{"3", "4", "5"},
		// 	encoding: BaseParameter{
		// 		Name:          "id",
		// 		In:            oaschema.InQuery,
		// 		Style:         (oaschema.EncodingStyleSpaceDelimited),
		// 		Explode:       new(true),
		// 		AllowReserved: false,
		// 	},
		// 	expected: []string{"id=3&id=4&id=5"},
		// },
		// {
		// 	name:  "pipeDelimited_array",
		// 	value: []any{"3", "4", "5"},
		// 	encoding: BaseParameter{
		// 		Name:          "id",
		// 		In:            oaschema.InQuery,
		// 		Style:         (oaschema.EncodingStylePipeDelimited),
		// 		Explode:       new(false),
		// 		AllowReserved: true,
		// 	},
		// 	expected: []string{"id=3%7C4%7C5"},
		// },
		// {
		// 	name:  "pipeDelimited_explode_array",
		// 	value: []any{"3", "4", "5"},
		// 	encoding: BaseParameter{
		// 		Name:          "id",
		// 		In:            oaschema.InQuery,
		// 		Style:         (oaschema.EncodingStylePipeDelimited),
		// 		Explode:       new(true),
		// 		AllowReserved: true,
		// 	},
		// 	expected: []string{"id=3&id=4&id=5"},
		// },
		// {
		// 	name: "pipeDelimited_object",
		// 	value: map[string]any{
		// 		"R": "100",
		// 		"G": "200",
		// 	},
		// 	encoding: BaseParameter{
		// 		Name:          "color",
		// 		In:            oaschema.InQuery,
		// 		Style:         (oaschema.EncodingStylePipeDelimited),
		// 		Explode:       new(false),
		// 		AllowReserved: false,
		// 	},
		// 	expected: []string{
		// 		"color=G%7C200%7CR%7C100",
		// 		"color=R%7C100%7CG%7C200",
		// 	},
		// },
		// {
		// 	name:  "deepObject_array_explode",
		// 	value: []any{"3", "4", "5"},
		// 	encoding: BaseParameter{
		// 		Name:          "id",
		// 		In:            oaschema.InQuery,
		// 		Style:         (oaschema.EncodingStyleDeepObject),
		// 		Explode:       new(true),
		// 		AllowReserved: true,
		// 	},
		// 	expected: []string{"id[]=3&id[]=4&id[]=5"},
		// },
		// {
		// 	name: "deepObject_object_explode",
		// 	value: map[string]any{
		// 		"R": "100",
		// 		"G": "200",
		// 	},
		// 	encoding: BaseParameter{
		// 		Name:          "color",
		// 		In:            oaschema.InQuery,
		// 		Style:         (oaschema.EncodingStyleDeepObject),
		// 		Explode:       new(true),
		// 		AllowReserved: false,
		// 	},
		// 	expected: []string{
		// 		"color%5BR%5D=100&color%5BG%5D=200",
		// 		"color%5BG%5D=200&color%5BR%5D=100",
		// 	},
		// },
		// {
		// 	name: "deepObject_explode_array_object",
		// 	value: map[any]any{
		// 		"role": []any{
		// 			map[string]any{
		// 				"user": []any{
		// 					[]any{"admin", "anonymous"},
		// 				},
		// 			},
		// 		},
		// 	},
		// 	encoding: BaseParameter{
		// 		Name:          "id",
		// 		In:            oaschema.InQuery,
		// 		Style:         (oaschema.EncodingStyleDeepObject),
		// 		Explode:       new(true),
		// 		AllowReserved: true,
		// 	},
		// 	expected: []string{"id[role][0][user][0][]=admin&id[role][0][user][0][]=anonymous"},
		// },
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := EncodeFormURLEncoded(tc.value, tc.mediaType)
			assert.NoError(t, err)
			assert.Contains(t, tc.expected, result)
		})
	}
}

// BenchmarkEncodeURLEncode-11    	  800754	      1434 ns/op	    1560 B/op	      29 allocs/op
func BenchmarkEncodeURLEncode(b *testing.B) {
	value := map[any]any{
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

	for b.Loop() {
		EncodeFormURLEncoded(value, mediaType)
	}
}
