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
	"net/http"
	"testing"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/oasvalidator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeHeaderParameter(t *testing.T) {
	t.Run("missing_required_header", func(t *testing.T) {
		defs := []*oaschema.Parameter{
			{
				Name:     "X-Token",
				In:       oaschema.InHeader,
				Required: true,
			},
		}
		_, errs := DecodeHeaderParameters(defs, http.Header{})
		require.Len(t, errs, 1)
		assert.Equal(t, oasvalidator.ErrCodeInvalidHeader, errs[0].Code)
		assert.Equal(t, "X-Token", errs[0].Header)
	})

	t.Run("missing_optional_header_returns_nil", func(t *testing.T) {
		def := &oaschema.Parameter{
			Name:     "X-Optional",
			Required: false,
		}
		result, errs := decodeHeaderParameter(def, http.Header{})
		assert.Empty(t, errs)
		assert.Nil(t, result)
	})

	t.Run("no_schema_single_value", func(t *testing.T) {
		def := &oaschema.Parameter{
			Name: "X-Token",
		}
		headers := http.Header{"X-Token": {"abc"}}
		result, errs := decodeHeaderParameter(def, headers)
		assert.Empty(t, errs)
		assert.Equal(t, "abc", result)
	})

	t.Run("no_schema_comma_separated_splits_into_slice", func(t *testing.T) {
		def := &oaschema.Parameter{
			Name: "X-Ids",
		}
		headers := http.Header{"X-Ids": {"1,2,3"}}
		result, errs := decodeHeaderParameter(def, headers)
		assert.Empty(t, errs)
		assert.Equal(t, []string{"1", "2", "3"}, result)
	})

	t.Run("string_schema_single_value", func(t *testing.T) {
		def := &oaschema.Parameter{
			Name:   "X-Name",
			Schema: stringSchema(),
		}
		headers := http.Header{"X-Name": {"hello"}}
		result, errs := decodeHeaderParameter(def, headers)
		assert.Empty(t, errs)
		assert.Equal(t, "hello", result)
	})

	t.Run("integer_schema_valid", func(t *testing.T) {
		def := &oaschema.Parameter{
			Name:   "X-Count",
			Schema: intSchema(),
		}
		headers := http.Header{"X-Count": {"42"}}
		result, errs := decodeHeaderParameter(def, headers)
		assert.Empty(t, errs)
		assert.Equal(t, int64(42), result)
	})

	t.Run("integer_schema_invalid_value", func(t *testing.T) {
		def := &oaschema.Parameter{
			Name:   "X-Count",
			Schema: intSchema(),
		}
		headers := http.Header{"X-Count": {"not-a-number"}}
		_, errs := decodeHeaderParameter(def, headers)
		require.NotEmpty(t, errs)
		assert.Equal(t, oasvalidator.ErrCodeInvalidHeader, errs[0].Code)
		assert.Equal(t, "X-Count", errs[0].Header)
	})

	t.Run("boolean_schema_true", func(t *testing.T) {
		def := &oaschema.Parameter{
			Name:   "X-Flag",
			Schema: boolSchema(),
		}
		headers := http.Header{"X-Flag": {"true"}}
		result, errs := decodeHeaderParameter(def, headers)
		assert.Empty(t, errs)
		assert.Equal(t, true, result)
	})

	t.Run("number_schema_float", func(t *testing.T) {
		def := &oaschema.Parameter{
			Name: "X-Price",
			Schema: &base.Schema{
				Type: []string{oaschema.Number},
			},
		}
		headers := http.Header{"X-Price": {"3.14"}}
		result, errs := decodeHeaderParameter(def, headers)
		assert.Empty(t, errs)
		assert.InDelta(t, float64(3.14), result, 1e-9)
	})
}

func TestDecodeHeaderParameter_Array(t *testing.T) {
	t.Run("array_comma_separated", func(t *testing.T) {
		def := &oaschema.Parameter{
			Name:   "X-Ids",
			Schema: intArraySchema(),
		}
		headers := http.Header{"X-Ids": {"1,2,3"}}
		result, errs := decodeHeaderParameter(def, headers)
		assert.Empty(t, errs)
		assert.Equal(t, []any{int64(1), int64(2), int64(3)}, result)
	})

	t.Run("array_multiple_header_values_merged", func(t *testing.T) {
		def := &oaschema.Parameter{
			Name: "X-Tags",
			Schema: &base.Schema{
				Type: []string{oaschema.Array},
			},
		}
		// HTTP allows multiple header lines for the same key
		headers := http.Header{"X-Tags": {"foo,bar", "baz"}}
		result, errs := decodeHeaderParameter(def, headers)
		assert.Empty(t, errs)
		assert.Equal(t, []any{"foo", "bar", "baz"}, result)
	})

	t.Run("array_item_type_error", func(t *testing.T) {
		def := &oaschema.Parameter{
			Name:   "X-Ids",
			Schema: intArraySchema(),
		}
		headers := http.Header{"X-Ids": {"1,two,3"}}
		_, errs := decodeHeaderParameter(def, headers)
		require.NotEmpty(t, errs)
		assert.Equal(t, oasvalidator.ErrCodeInvalidHeader, errs[0].Code)
		assert.Equal(t, "X-Ids", errs[0].Header)
	})
}

func TestDecodeHeaderParameter_Object(t *testing.T) {
	t.Run("non_explode_object", func(t *testing.T) {
		// X-Color: R,100,G,200  (key,value,key,value)
		def := &oaschema.Parameter{
			Name:    "X-Color",
			Explode: new(false),
			Schema: &base.Schema{
				Type: []string{oaschema.Object},
				Properties: func() *orderedmap.Map[string, *base.SchemaProxy] {
					m := orderedmap.New[string, *base.SchemaProxy]()
					m.Set("R", base.CreateSchemaProxy(intSchema()))
					m.Set("G", base.CreateSchemaProxy(intSchema()))

					return m
				}(),
			},
		}
		headers := http.Header{"X-Color": {"R,100,G,200"}}
		result, errs := decodeHeaderParameter(def, headers)
		assert.Empty(t, errs)
		assert.Equal(t, map[string]any{"R": int64(100), "G": int64(200)}, result)
	})

	t.Run("explode_object", func(t *testing.T) {
		// X-User: role=admin,firstName=Alex
		def := &oaschema.Parameter{
			Name:    "X-User",
			Explode: new(true),
			Schema: &base.Schema{
				Type: []string{oaschema.Object},
			},
		}
		headers := http.Header{"X-User": {"role=admin,firstName=Alex"}}
		result, errs := decodeHeaderParameter(def, headers)
		assert.Empty(t, errs)
		assert.Equal(t, map[string]any{
			"role":      "admin",
			"firstName": "Alex",
		}, result)
	})

	t.Run("non_explode_object_invalid_syntax_odd_count", func(t *testing.T) {
		def := &oaschema.Parameter{
			Name:    "X-Color",
			Explode: new(false),
			Schema: &base.Schema{
				Type: []string{oaschema.Object},
			},
		}
		// Odd number of comma-separated parts → not a valid key,value sequence
		headers := http.Header{"X-Color": {"R,100,G"}}
		_, errs := decodeHeaderParameter(def, headers)
		require.NotEmpty(t, errs)
		assert.Equal(t, oasvalidator.ErrCodeInvalidHeader, errs[0].Code)
		assert.Equal(t, "X-Color", errs[0].Header)
	})

	t.Run("explode_object_invalid_syntax_missing_equals", func(t *testing.T) {
		def := &oaschema.Parameter{
			Name:    "X-User",
			Explode: new(true),
			Schema: &base.Schema{
				Type: []string{oaschema.Object},
			},
		}
		headers := http.Header{"X-User": {"roleadmin"}}
		_, errs := decodeHeaderParameter(def, headers)
		require.NotEmpty(t, errs)
		assert.Equal(t, oasvalidator.ErrCodeInvalidHeader, errs[0].Code)
		assert.Equal(t, "X-User", errs[0].Header)
	})
}

func TestSplitObjectFromHeaderValues(t *testing.T) {
	t.Run("non_explode_valid", func(t *testing.T) {
		result, err := splitObjectFromHeaderValues([]string{"role", "admin", "age", "30"}, false)
		require.Nil(t, err)
		assert.Equal(t, map[string][]string{
			"role": {"admin"},
			"age":  {"30"},
		}, result)
	})

	t.Run("non_explode_odd_parts_error", func(t *testing.T) {
		_, err := splitObjectFromHeaderValues([]string{"role", "admin", "age"}, false)
		require.NotNil(t, err)
		assert.Equal(t, oasvalidator.ErrCodeInvalidHeader, err.Code)
	})

	t.Run("explode_valid", func(t *testing.T) {
		result, err := splitObjectFromHeaderValues([]string{"role=admin", "age=30"}, true)
		require.Nil(t, err)
		assert.Equal(t, map[string][]string{
			"role": {"admin"},
			"age":  {"30"},
		}, result)
	})

	t.Run("explode_missing_equals_error", func(t *testing.T) {
		_, err := splitObjectFromHeaderValues([]string{"roleadmin"}, true)
		require.NotNil(t, err)
		assert.Equal(t, oasvalidator.ErrCodeInvalidHeader, err.Code)
	})

	t.Run("explode_multiple_values_same_key", func(t *testing.T) {
		result, err := splitObjectFromHeaderValues([]string{"role=admin,role=superuser"}, true)
		require.Nil(t, err)
		assert.Equal(t, []string{"admin", "superuser"}, result["role"])
	})

	t.Run("non_explode_empty_key_error", func(t *testing.T) {
		_, err := splitObjectFromHeaderValues([]string{"", "value"}, false)
		require.NotNil(t, err)
		assert.Equal(t, oasvalidator.ErrCodeInvalidHeader, err.Code)
	})
}

func TestDecodeHeaderParameter_MultiType(t *testing.T) {
	t.Run("string_or_integer_prefers_string", func(t *testing.T) {
		def := &oaschema.Parameter{
			Name: "X-Val",
			Schema: &base.Schema{
				Type: []string{oaschema.String, oaschema.Integer},
			},
		}
		headers := http.Header{"X-Val": {"007"}}
		result, errs := decodeHeaderParameter(def, headers)
		assert.Empty(t, errs)
		// String type wins — no lossy parse
		assert.Equal(t, "007", result)
	})
}

// goos: darwin
// goarch: arm64
// pkg: github.com/relychan/openapitools/oasvalidator/parameter
// cpu: Apple M3 Pro
// BenchmarkDecodeHeaderParameter/MissingOptional-11         	50759810	        23.77 ns/op	       0 B/op	       0 allocs/op
// BenchmarkDecodeHeaderParameter/NoSchema_Single-11         	26734170	        44.24 ns/op	      16 B/op	       1 allocs/op
// BenchmarkDecodeHeaderParameter/NoSchema_CommaSeparated-11 	 9048547	       130.5 ns/op	     104 B/op	       2 allocs/op
// BenchmarkDecodeHeaderParameter/String-11                  	 5783558	       210.7 ns/op	      65 B/op	       5 allocs/op
// BenchmarkDecodeHeaderParameter/Integer-11                 	 5070550	       239.0 ns/op	      65 B/op	       5 allocs/op
// BenchmarkDecodeHeaderParameter/Boolean-11                 	 5640459	       213.4 ns/op	      65 B/op	       5 allocs/op
// BenchmarkDecodeHeaderParameter/Number-11                  	 4718089	       253.1 ns/op	      80 B/op	       6 allocs/op
// BenchmarkDecodeHeaderParameter/Array_CommaSeparated-11    	  935188	        1284 ns/op	     568 B/op	      34 allocs/op
// BenchmarkDecodeHeaderParameter/Array_MultipleHeaderValues-11  4354665	       275.8 ns/op	     232 B/op	       8 allocs/op
// BenchmarkDecodeHeaderParameter/Object_NonExplode-11            995085	        1198 ns/op	    1115 B/op	      25 allocs/op
// BenchmarkDecodeHeaderParameter/Object_Explode-11              1853228	       651.7 ns/op	     944 B/op	      13 allocs/op
func BenchmarkDecodeHeaderParameter(b *testing.B) {
	headers := http.Header{
		"X-Token": {"abc"},
		"X-Ids":   {"1,2,3,4,5"},
		"X-Name":  {"hello"},
		"X-Count": {"42"},
		"X-Flag":  {"true"},
		"X-Price": {"3.14"},
		"X-Tags":  {"foo,bar", "baz,qux"},
		"X-Color": {"R,100,G,200,B,50"},
		"X-User":  {"role=admin,firstName=Alex,age=30"},
	}

	b.Run("MissingOptional", func(b *testing.B) {
		def := &oaschema.Parameter{Name: "X-Optional", Required: false}

		for b.Loop() {
			decodeHeaderParameter(def, headers)
		}
	})

	b.Run("NoSchema_Single", func(b *testing.B) {
		def := &oaschema.Parameter{Name: "X-Token", Required: true}

		for b.Loop() {
			decodeHeaderParameter(def, headers)
		}
	})

	b.Run("NoSchema_CommaSeparated", func(b *testing.B) {
		def := &oaschema.Parameter{Name: "X-Ids"}

		for b.Loop() {
			decodeHeaderParameter(def, headers)
		}
	})

	b.Run("String", func(b *testing.B) {
		def := &oaschema.Parameter{Name: "X-Name", Schema: stringSchema()}

		for b.Loop() {
			decodeHeaderParameter(def, headers)
		}
	})

	b.Run("Integer", func(b *testing.B) {
		def := &oaschema.Parameter{Name: "X-Count", Schema: intSchema()}

		for b.Loop() {
			decodeHeaderParameter(def, headers)
		}
	})

	b.Run("Boolean", func(b *testing.B) {
		def := &oaschema.Parameter{Name: "X-Flag", Schema: boolSchema()}

		for b.Loop() {
			decodeHeaderParameter(def, headers)
		}
	})

	b.Run("Number", func(b *testing.B) {
		def := &oaschema.Parameter{
			Name:   "X-Price",
			Schema: &base.Schema{Type: []string{oaschema.Number}},
		}

		for b.Loop() {
			decodeHeaderParameter(def, headers)
		}
	})

	b.Run("Array_CommaSeparated", func(b *testing.B) {
		def := &oaschema.Parameter{Name: "X-Ids", Schema: intArraySchema()}
		for b.Loop() {
			decodeHeaderParameter(def, headers)
		}
	})

	b.Run("Array_MultipleHeaderValues", func(b *testing.B) {
		def := &oaschema.Parameter{
			Name:   "X-Tags",
			Schema: &base.Schema{Type: []string{oaschema.Array}},
		}

		for b.Loop() {
			decodeHeaderParameter(def, headers)
		}
	})

	b.Run("Object_NonExplode", func(b *testing.B) {
		props := orderedmap.New[string, *base.SchemaProxy]()
		props.Set("R", base.CreateSchemaProxy(intSchema()))
		props.Set("G", base.CreateSchemaProxy(intSchema()))
		props.Set("B", base.CreateSchemaProxy(intSchema()))

		explode := false
		def := &oaschema.Parameter{
			Name:    "X-Color",
			Explode: &explode,
			Schema:  &base.Schema{Type: []string{oaschema.Object}, Properties: props},
		}

		b.ResetTimer()

		for b.Loop() {
			decodeHeaderParameter(def, headers)
		}
	})

	b.Run("Object_Explode", func(b *testing.B) {
		explode := true
		def := &oaschema.Parameter{
			Name:    "X-User",
			Explode: &explode,
			Schema:  &base.Schema{Type: []string{oaschema.Object}},
		}
		headers := http.Header{"X-User": {"role=admin,firstName=Alex,age=30"}}

		b.ResetTimer()

		for b.Loop() {
			decodeHeaderParameter(def, headers)
		}
	})
}
