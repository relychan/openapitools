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
	"testing"

	"github.com/relychan/openapitools/oaschema"
	"github.com/stretchr/testify/assert"
)

func TestEncodingURLPathParam(t *testing.T) {
	testCases := []struct {
		name     string
		value    any
		encoding BaseParameter
		expected []string
	}{
		{
			name:  "simple_empty",
			value: nil,
			encoding: BaseParameter{
				Name:    "id",
				In:      oaschema.InPath,
				Explode: new(true),
				Style:   new(oaschema.EncodingStyleSimple),
			},
			expected: []string{""},
		},
		{
			name:  "simple_empty_explode",
			value: nil,
			encoding: BaseParameter{
				Name:    "id",
				In:      oaschema.InPath,
				Explode: new(true),
				Style:   new(oaschema.EncodingStyleSimple),
			},
			expected: []string{""},
		},
		{
			name:  "simple_single",
			value: "3",
			encoding: BaseParameter{
				Name:    "id",
				In:      oaschema.InPath,
				Explode: new(false),
				Style:   new(oaschema.EncodingStyleSimple),
			},
			expected: []string{"3"},
		},
		{
			name:  "simple_single_explode",
			value: "3",
			encoding: BaseParameter{
				Name:    "id",
				In:      oaschema.InPath,
				Explode: new(true),
				Style:   new(oaschema.EncodingStyleSimple),
			},
			expected: []string{"3"},
		},
		{
			name:  "simple_array",
			value: []int{3, 4, 5},
			encoding: BaseParameter{
				Name:    "id",
				In:      oaschema.InPath,
				Explode: new(false),
				Style:   new(oaschema.EncodingStyleSimple),
			},
			expected: []string{"3,4,5"},
		},
		{
			name:  "simple_array_explode",
			value: []int{3, 4, 5},
			encoding: BaseParameter{
				Name:    "id",
				In:      oaschema.InPath,
				Explode: new(true),
				Style:   new(oaschema.EncodingStyleSimple),
			},
			expected: []string{"3,4,5"},
		},
		{
			name: "simple_object",
			value: map[string]any{
				"role":      "admin",
				"firstName": "Alex",
			},
			encoding: BaseParameter{
				Name:    "id",
				In:      oaschema.InPath,
				Explode: new(false),
				Style:   new(oaschema.EncodingStyleSimple),
			},
			expected: []string{
				"firstName,Alex,role,admin",
				"role,admin,firstName,Alex",
			},
		},
		{
			name: "simple_object_explode",
			value: map[string]any{
				"role":      "admin",
				"firstName": "Alex",
			},
			encoding: BaseParameter{
				Name:    "id",
				In:      oaschema.InPath,
				Explode: new(true),
				Style:   new(oaschema.EncodingStyleSimple),
			},
			expected: []string{
				"firstName=Alex,role=admin",
				"role=admin,firstName=Alex",
			},
		},
		{
			name:  "label_empty",
			value: nil,
			encoding: BaseParameter{
				Name:    "id",
				In:      oaschema.InPath,
				Explode: new(false),
				Style:   new(oaschema.EncodingStyleLabel),
			},
			expected: []string{"."},
		},
		{
			name:  "label_empty_explode",
			value: nil,
			encoding: BaseParameter{
				Name:    "id",
				In:      oaschema.InPath,
				Explode: new(true),
				Style:   new(oaschema.EncodingStyleLabel),
			},
			expected: []string{"."},
		},
		{
			name:  "label_single",
			value: "3",
			encoding: BaseParameter{
				Name:    "id",
				In:      oaschema.InPath,
				Explode: new(false),
				Style:   new(oaschema.EncodingStyleLabel),
			},
			expected: []string{".3"},
		},
		{
			name:  "label_single_explode",
			value: "3",
			encoding: BaseParameter{
				Name:    "id",
				In:      oaschema.InPath,
				Explode: new(true),
				Style:   new(oaschema.EncodingStyleLabel),
			},
			expected: []string{".3"},
		},
		{
			name:  "label_array",
			value: []int{3, 4, 5},
			encoding: BaseParameter{
				Name:    "id",
				In:      oaschema.InPath,
				Explode: new(false),
				Style:   new(oaschema.EncodingStyleLabel),
			},
			expected: []string{".3,4,5"},
		},
		{
			name:  "label_array_explode",
			value: []int{3, 4, 5},
			encoding: BaseParameter{
				Name:    "id",
				In:      oaschema.InPath,
				Explode: new(true),
				Style:   new(oaschema.EncodingStyleLabel),
			},
			expected: []string{".3.4.5"},
		},
		{
			name: "label_object",
			value: map[string]any{
				"role":      "admin",
				"firstName": "Alex",
			},
			encoding: BaseParameter{
				Name:    "id",
				In:      oaschema.InPath,
				Explode: new(false),
				Style:   new(oaschema.EncodingStyleLabel),
			},
			expected: []string{
				".firstName,Alex,role,admin",
				".role,admin,firstName,Alex",
			},
		},
		{
			name: "label_object_explode",
			value: map[string]any{
				"role":      "admin",
				"firstName": "Alex",
			},
			encoding: BaseParameter{
				Name:    "id",
				In:      oaschema.InPath,
				Explode: new(true),
				Style:   new(oaschema.EncodingStyleLabel),
			},
			expected: []string{
				".firstName=Alex.role=admin",
				".role=admin.firstName=Alex",
			},
		},
		{
			name:  "matrix_empty",
			value: nil,
			encoding: BaseParameter{
				Name:    "id",
				In:      oaschema.InPath,
				Explode: new(false),
				Style:   new(oaschema.EncodingStyleMatrix),
			},
			expected: []string{";id="},
		},
		{
			name:  "matrix_empty_explode",
			value: nil,
			encoding: BaseParameter{
				Name:    "id",
				In:      oaschema.InPath,
				Explode: new(true),
				Style:   new(oaschema.EncodingStyleMatrix),
			},
			expected: []string{";id="},
		},
		{
			name:  "matrix_single",
			value: "3",
			encoding: BaseParameter{
				Name:    "id",
				In:      oaschema.InPath,
				Explode: new(false),
				Style:   new(oaschema.EncodingStyleMatrix),
			},
			expected: []string{";id=3"},
		},
		{
			name:  "matrix_single_explode",
			value: "3",
			encoding: BaseParameter{
				Name:    "id",
				In:      oaschema.InPath,
				Explode: new(true),
				Style:   new(oaschema.EncodingStyleMatrix),
			},
			expected: []string{";id=3"},
		},
		{
			name:  "matrix_array",
			value: []int{3, 4, 5},
			encoding: BaseParameter{
				Name:    "id",
				In:      oaschema.InPath,
				Explode: new(false),
				Style:   new(oaschema.EncodingStyleMatrix),
			},
			expected: []string{";id=3,4,5"},
		},
		{
			name:  "matrix_array_explode",
			value: []int{3, 4, 5},
			encoding: BaseParameter{
				Name:    "id",
				In:      oaschema.InPath,
				Explode: new(true),
				Style:   new(oaschema.EncodingStyleMatrix),
			},
			expected: []string{";id=3;id=4;id=5"},
		},
		{
			name: "matrix_object",
			value: map[string]any{
				"role":      "admin",
				"firstName": "Alex",
			},
			encoding: BaseParameter{
				Name:    "id",
				In:      oaschema.InPath,
				Explode: new(false),
				Style:   new(oaschema.EncodingStyleMatrix),
			},
			expected: []string{
				";id=firstName,Alex,role,admin",
				";id=role,admin,firstName,Alex",
			},
		},
		{
			name: "matrix_object_explode",
			value: map[string]any{
				"role":      "admin",
				"firstName": "Alex",
			},
			encoding: BaseParameter{
				Name:    "id",
				In:      oaschema.InPath,
				Explode: new(true),
				Style:   new(oaschema.EncodingStyleMatrix),
			},
			expected: []string{
				";firstName=Alex;role=admin",
				";role=admin;firstName=Alex",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Contains(t, tc.expected, EncodePathValue(tc.encoding, tc.value))
		})
	}
}

// BenchmarkEncodePath-11    	 3355124	       351.6 ns/op	     336 B/op	      11 allocs/op
func BenchmarkEncodePath(b *testing.B) {
	value := map[string]any{
		"role":      "admin",
		"firstName": "Alex",
	}

	encoding := BaseParameter{
		Name:          "thisisalongid",
		In:            oaschema.InPath,
		Style:         new(oaschema.EncodingStyleMatrix),
		Explode:       new(true),
		AllowReserved: true,
	}

	for b.Loop() {
		EncodePathValue(encoding, value)
	}
}
