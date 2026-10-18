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
	"slices"
	"testing"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/oasvalidator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func deepObjectQueryParam(name string, schema *base.Schema) *oaschema.Parameter {
	style := oaschema.EncodingStyleDeepObject
	explode := true

	return &oaschema.Parameter{
		Name:    name,
		In:      oaschema.InQuery,
		Style:   &style,
		Explode: &explode,
		Schema:  schema,
	}
}

func TestDecodeQueryDeepObject_ScalarValidationErrorNotDropped(t *testing.T) {
	maximum := 100.0
	ageSchema := intSchema()
	ageSchema.Maximum = &maximum

	def := deepObjectQueryParam("filter", objectSchema(map[string]*base.Schema{
		"age": ageSchema,
	}))

	result, errs := DecodeQueryFromParameters(
		[]*oaschema.Parameter{def},
		url.Values{"filter[age]": {"500"}},
	)
	require.NotEmpty(t, errs)
	assert.Nil(t, result)
	assert.Equal(t, oasvalidator.ErrCodeInvalidQueryParam, errs[0].Code)

	result, errs = DecodeQueryFromParameters(
		[]*oaschema.Parameter{def},
		url.Values{"filter[age]": {"50"}},
	)
	require.Empty(t, errs)
	assert.Equal(t, map[string]any{"filter": map[string]any{"age": int64(50)}}, result)
}

func TestEvaluateParameterValue_NoSiblingKeyAliasing(t *testing.T) {
	value := map[string]any{
		"a": map[string]any{
			"b": map[string]any{
				"c": map[string]any{"x": "1", "y": "2", "z": "3"},
			},
		},
	}

	// Pre-allocate spare capacity in the parent keys to trigger aliasing with append.
	parentKeys := make(ParamKeys, 0, 16)

	for range 20 {
		items := EvaluateParameterValue(value, parentKeys)
		require.Len(t, items, 3)

		got := make([]string, 0, len(items))
		for _, item := range items {
			got = append(got, item.Keys().Format("f", true)+"="+item.Value())
		}

		slices.Sort(got)
		assert.Equal(t, []string{"f[a][b][c][x]=1", "f[a][b][c][y]=2", "f[a][b][c][z]=3"}, got)
	}

	params := url.Values{}
	SetQueryParam(params, deepObjectQueryParam("f", nil), value)
	assert.Equal(t, url.Values{
		url.QueryEscape("f[a][b][c][x]"): {"1"},
		url.QueryEscape("f[a][b][c][y]"): {"2"},
		url.QueryEscape("f[a][b][c][z]"): {"3"},
	}, params)
}

func TestEvaluateParameterValue_StringSlicePrefixesParentKeys(t *testing.T) {
	items := EvaluateParameterValue(
		map[string]any{"tags": []string{"a", "b"}},
		ParamKeys{},
	)
	require.Len(t, items, 2)
	assert.Equal(t, ParamKeys{ParamKey("tags"), ParamIndex(0)}, items[0].Keys())
	assert.Equal(t, "a", items[0].Value())
	assert.Equal(t, ParamKeys{ParamKey("tags"), ParamIndex(1)}, items[1].Keys())
	assert.Equal(t, "b", items[1].Value())

	params := url.Values{}
	SetQueryParam(params, deepObjectQueryParam("f", nil), map[string]any{"tags": []string{"a", "b"}})
	assert.Equal(t, url.Values{url.QueryEscape("f[tags][]"): {"a", "b"}}, params)
}

func TestDecodeQueryDeepObject_NumericIndexKeysOrdered(t *testing.T) {
	def := deepObjectQueryParam("ids", stringArraySchema())
	values := url.Values{}
	expected := make([]any, 0, 12)

	for i := range 12 {
		v := string(rune('a' + i))
		values.Set("ids["+ParamIndex(i).String()+"]", v)
		expected = append(expected, v)
	}

	for range 20 {
		result, errs := DecodeQueryFromParameters([]*oaschema.Parameter{def}, values)
		require.Empty(t, errs)
		assert.Equal(t, map[string]any{"ids": expected}, result)
	}

	// arbitrary (no item schema) arrays are ordered as well.
	arbitraryDef := deepObjectQueryParam("ids", &base.Schema{Type: []string{oaschema.Array}})

	for range 20 {
		result, errs := DecodeQueryFromParameters([]*oaschema.Parameter{arbitraryDef}, values)
		require.Empty(t, errs)
		assert.Equal(t, map[string]any{"ids": expected}, result)
	}
}

func TestDecodePathValue_MatrixStyle_StringStripsName(t *testing.T) {
	for _, explode := range []bool{false, true} {
		def := pathParam("id", oaschema.EncodingStyleMatrix, explode, stringSchema())

		result, errs := DecodePathValue(def, ";id=5")
		require.Empty(t, errs)
		assert.Equal(t, "5", result)

		result, errs = DecodePathValue(def, ";id")
		require.Empty(t, errs)
		assert.Empty(t, result)

		_, errs = DecodePathValue(def, ";other=5")
		require.Len(t, errs, 1)
		assert.Equal(t, oasvalidator.ErrCodeInvalidURLParam, errs[0].Code)
	}
}

func TestEncodePathValue_MatrixExplodeEscapes(t *testing.T) {
	def := pathParam("id", oaschema.EncodingStyleMatrix, true, nil)

	assert.Equal(t, ";id=a%2Fb;id=c%20d", EncodePathValue(def, []any{"a/b", "c d"}))
	assert.Equal(t, ";na%20me=x%2Fy", EncodePathValue(def, map[string]any{"na me": "x/y"}))
}

func TestDecodeQueryDeepObject_IgnoresUnrelatedMalformedKeys(t *testing.T) {
	def := deepObjectQueryParam("filter", objectSchema(map[string]*base.Schema{
		"age": intSchema(),
	}))

	result, errs := DecodeQueryFromParameters(
		[]*oaschema.Parameter{def},
		url.Values{"filter[age]": {"5"}, "cb]": {"2"}, "filterx[": {"1"}},
	)
	require.Empty(t, errs)
	assert.Equal(t, map[string]any{"filter": map[string]any{"age": int64(5)}}, result)

	// malformed keys that belong to the parameter still fail.
	_, errs = DecodeQueryFromParameters(
		[]*oaschema.Parameter{def},
		url.Values{"filter[age": {"5"}},
	)
	require.NotEmpty(t, errs)
}
