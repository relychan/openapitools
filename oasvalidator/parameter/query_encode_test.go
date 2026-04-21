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

package parameter

import (
	"net/url"
	"testing"

	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/oasvalidator"
	"github.com/stretchr/testify/assert"
)

func TestEncodingURLQueryParam(t *testing.T) {
	testCases := []struct {
		name     string
		value    any
		encoding BaseParameter
		expected string
	}{
		{
			name:  "empty",
			value: nil,
			encoding: BaseParameter{
				Name:    "id",
				In:      oaschema.InQuery,
				Explode: new(true),
				Style:   new(oaschema.EncodingStyleForm),
			},
			expected: "",
		},
		{
			name:  "form_explode_single",
			value: "3",
			encoding: BaseParameter{
				Name:    "id",
				In:      oaschema.InQuery,
				Explode: new(true),
				Style:   new(oaschema.EncodingStyleForm),
			},
			expected: "id=3",
		},
		{
			name:  "form_single",
			value: "3",
			encoding: BaseParameter{
				Name:    "id",
				In:      oaschema.InQuery,
				Explode: new(false),
				Style:   new(oaschema.EncodingStyleForm),
			},
			expected: "id=3",
		},
		{
			name:  "form_multiple",
			value: []string{"3", "4", "5"},
			encoding: BaseParameter{
				Name:          "id",
				In:            oaschema.InQuery,
				Explode:       new(false),
				Style:         new(oaschema.EncodingStyleForm),
				AllowReserved: true,
			},
			expected: "id=3,4,5",
		},
		{
			name:  "form_explode_multiple",
			value: []string{"3", "4", "5"},
			encoding: BaseParameter{
				Name:          "id",
				In:            oaschema.InQuery,
				Explode:       new(true),
				Style:         new(oaschema.EncodingStyleForm),
				AllowReserved: true,
			},
			expected: "id=3&id=4&id=5",
		},
		{
			name: "form_object",
			value: map[any]any{
				"role": "admin",
			},
			encoding: BaseParameter{
				Name:          "id",
				In:            oaschema.InQuery,
				Explode:       new(false),
				Style:         new(oaschema.EncodingStyleForm),
				AllowReserved: true,
			},
			expected: "id=role,admin",
		},
		{
			name: "form_explode_object",
			value: map[any]any{
				"role": "admin",
			},
			encoding: BaseParameter{
				Name:          "id",
				In:            oaschema.InQuery,
				Explode:       new(true),
				Style:         new(oaschema.EncodingStyleForm),
				AllowReserved: true,
			},
			expected: "role=admin",
		},
		{
			name: "form_array_object",
			value: map[any]any{
				"role": []any{
					map[string]any{
						"user": "admin",
					},
				},
			},
			encoding: BaseParameter{
				Name:          "id",
				In:            oaschema.InQuery,
				Explode:       new(false),
				Style:         new(oaschema.EncodingStyleForm),
				AllowReserved: true,
			},
			expected: "id=role[0][user],admin",
		},
		{
			name: "form_explode_array_object_multiple",
			value: map[any]any{
				"role": []any{
					map[string]any{
						"user": []any{
							[]any{"admin", "anonymous"},
						},
					},
				},
			},
			encoding: BaseParameter{
				Name:          "id",
				In:            oaschema.InQuery,
				Explode:       new(true),
				Style:         new(oaschema.EncodingStyleForm),
				AllowReserved: true,
			},
			expected: "role[0][user][0]=admin&role[0][user][0]=anonymous",
		},
		{
			name:  "spaceDelimited_array",
			value: []string{"3", "4", "5"},
			encoding: BaseParameter{
				Name:          "id",
				In:            oaschema.InQuery,
				Explode:       new(false),
				Style:         new(oaschema.EncodingStyleSpaceDelimited),
				AllowReserved: true,
			},
			expected: "id=3+4+5",
		},
		{
			name: "spaceDelimited_object",
			value: map[string]any{
				"R": "100",
				"G": "200",
			},
			encoding: BaseParameter{
				Name:          "color",
				In:            oaschema.InQuery,
				Style:         new(oaschema.EncodingStyleSpaceDelimited),
				Explode:       new(false),
				AllowReserved: false,
			},
			expected: "color=G+200+R+100",
		},
		{
			name:  "spaceDelimited_explode_array",
			value: []any{"3", "4", "5"},
			encoding: BaseParameter{
				Name:          "id",
				In:            oaschema.InQuery,
				Style:         new(oaschema.EncodingStyleSpaceDelimited),
				Explode:       new(true),
				AllowReserved: false,
			},
			expected: "id=3&id=4&id=5",
		},
		{
			name:  "pipeDelimited_array",
			value: []any{"3", "4", "5"},
			encoding: BaseParameter{
				Name:          "id",
				In:            oaschema.InQuery,
				Style:         new(oaschema.EncodingStylePipeDelimited),
				Explode:       new(false),
				AllowReserved: true,
			},
			expected: "id=3%7C4%7C5",
		},
		{
			name:  "pipeDelimited_explode_array",
			value: []any{"3", "4", "5"},
			encoding: BaseParameter{
				Name:          "id",
				In:            oaschema.InQuery,
				Style:         new(oaschema.EncodingStylePipeDelimited),
				Explode:       new(true),
				AllowReserved: true,
			},
			expected: "id=3&id=4&id=5",
		},
		{
			name: "pipeDelimited_object",
			value: map[string]any{
				"R": "100",
				"G": "200",
			},
			encoding: BaseParameter{
				Name:          "color",
				In:            oaschema.InQuery,
				Style:         new(oaschema.EncodingStylePipeDelimited),
				Explode:       new(false),
				AllowReserved: false,
			},
			expected: "color=G%7C200%7CR%7C100",
		},
		{
			name:  "deepObject_array_explode",
			value: []any{"3", "4", "5"},
			encoding: BaseParameter{
				Name:          "id",
				In:            oaschema.InQuery,
				Style:         new(oaschema.EncodingStyleDeepObject),
				Explode:       new(true),
				AllowReserved: true,
			},
			expected: "id[]=3&id[]=4&id[]=5",
		},
		{
			name: "deepObject_object_explode",
			value: map[string]any{
				"R": "100",
				"G": "200",
			},
			encoding: BaseParameter{
				Name:          "color",
				In:            oaschema.InQuery,
				Style:         new(oaschema.EncodingStyleDeepObject),
				Explode:       new(true),
				AllowReserved: false,
			},
			expected: "color%5BG%5D=200&color%5BR%5D=100",
		},
		{
			name: "deepObject_explode_array_object",
			value: map[any]any{
				"role": []any{
					map[string]any{
						"user": []any{
							[]any{"admin", "anonymous"},
						},
					},
				},
			},
			encoding: BaseParameter{
				Name:          "id",
				In:            oaschema.InQuery,
				Style:         new(oaschema.EncodingStyleDeepObject),
				Explode:       new(true),
				AllowReserved: true,
			},
			expected: "id[role][0][user][0][]=admin&id[role][0][user][0][]=anonymous",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			qValues := url.Values{}
			SetQueryParam(qValues, tc.encoding, tc.value)
			assert.Equal(t, tc.expected, oasvalidator.EncodeQueryValuesUnescape(qValues))
		})
	}
}

// BenchmarkSetQueryParam-11    	  741129	      1507 ns/op	    2136 B/op	      34 allocs/op
func BenchmarkSetQueryParam(b *testing.B) {
	value := map[any]any{
		"role": []any{
			map[string]any{
				"user": []any{
					[]any{"admin", "anonymous"},
				},
			},
		},
	}

	encoding := BaseParameter{
		Name:          "id",
		In:            oaschema.InQuery,
		Style:         new(oaschema.EncodingStyleForm),
		Explode:       new(false),
		AllowReserved: true,
	}

	for b.Loop() {
		qValues := url.Values{}
		SetQueryParam(qValues, encoding, value)
		oasvalidator.EncodeQueryValuesUnescape(qValues)
	}
}
