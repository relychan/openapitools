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

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/oasvalidator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- DecodePathValue ---

func TestDecodePathValue_EmptyValueReturnsError(t *testing.T) {
	def := pathParam("id", oaschema.EncodingStyleSimple, false, intSchema())
	_, errs := DecodePathValue(def, "")
	require.Len(t, errs, 1)
	assert.Equal(t, oasvalidator.ErrCodeInvalidURLParam, errs[0].Code)
	assert.Equal(t, "id", errs[0].Parameter)
}

func TestDecodePathValue_NoSchema_SimpleStyle(t *testing.T) {
	s := oaschema.EncodingStyleSimple
	e := false
	def := &oaschema.Parameter{Name: "id", In: oaschema.InPath, Style: &s, Explode: &e}

	result, errs := DecodePathValue(def, "3")
	assert.Empty(t, errs)
	assert.Equal(t, "3", result)
}

func TestDecodePathValue_NoSchema_SimpleStyle_CSV(t *testing.T) {
	s := oaschema.EncodingStyleSimple
	e := false
	def := &oaschema.Parameter{Name: "id", In: oaschema.InPath, Style: &s, Explode: &e}

	result, errs := DecodePathValue(def, "3,4,5")
	assert.Empty(t, errs)
	assert.Equal(t, []string{"3", "4", "5"}, result)
}

func TestDecodePathValue_SimpleStyle_Integer(t *testing.T) {
	def := pathParam("id", oaschema.EncodingStyleSimple, false, intSchema())
	result, errs := DecodePathValue(def, "42")
	assert.Empty(t, errs)
	assert.Equal(t, int64(42), result)
}

func TestDecodePathValue_SimpleStyle_String(t *testing.T) {
	def := pathParam("name", oaschema.EncodingStyleSimple, false, stringSchema())
	result, errs := DecodePathValue(def, "alice")
	assert.Empty(t, errs)
	assert.Equal(t, "alice", result)
}

func TestDecodePathValue_SimpleStyle_Number(t *testing.T) {
	def := pathParam("price", oaschema.EncodingStyleSimple, false, numberSchema())
	result, errs := DecodePathValue(def, "3.14")
	assert.Empty(t, errs)
	assert.InDelta(t, 3.14, result, 1e-9)
}

func TestDecodePathValue_SimpleStyle_Boolean(t *testing.T) {
	def := pathParam("flag", oaschema.EncodingStyleSimple, false, boolSchema())
	result, errs := DecodePathValue(def, "true")
	assert.Empty(t, errs)
	assert.Equal(t, true, result)
}

func TestDecodePathValue_SimpleStyle_InvalidInteger(t *testing.T) {
	def := pathParam("id", oaschema.EncodingStyleSimple, false, intSchema())
	_, errs := DecodePathValue(def, "not-a-number")
	require.NotEmpty(t, errs)
	assert.Equal(t, oasvalidator.ErrCodeInvalidURLParam, errs[0].Code)
	assert.Equal(t, "id", errs[0].Parameter)
}

// --- Label style ---

func TestDecodePathValue_LabelStyle_MissingDotPrefix(t *testing.T) {
	def := pathParam("id", oaschema.EncodingStyleLabel, false, intSchema())
	_, errs := DecodePathValue(def, "42")
	require.Len(t, errs, 1)
	assert.Equal(t, oasvalidator.ErrCodeInvalidURLParam, errs[0].Code)
}

func TestDecodePathValue_LabelStyle_Integer(t *testing.T) {
	def := pathParam("id", oaschema.EncodingStyleLabel, false, intSchema())
	result, errs := DecodePathValue(def, ".42")
	assert.Empty(t, errs)
	assert.Equal(t, int64(42), result)
}

func TestDecodePathValue_LabelStyle_Array_NonExplode(t *testing.T) {
	// /users/.3,4,5
	def := pathParam("id", oaschema.EncodingStyleLabel, false, intArraySchema())
	result, errs := DecodePathValue(def, ".3,4,5")
	assert.Empty(t, errs)
	assert.Equal(t, []any{int64(3), int64(4), int64(5)}, result)
}

func TestDecodePathValue_LabelStyle_Array_Explode(t *testing.T) {
	// /users/.3.4.5
	def := pathParam("id", oaschema.EncodingStyleLabel, true, intArraySchema())
	result, errs := DecodePathValue(def, ".3.4.5")
	assert.Empty(t, errs)
	assert.Equal(t, []any{int64(3), int64(4), int64(5)}, result)
}

func TestDecodePathValue_LabelStyle_Object_NonExplode(t *testing.T) {
	// /users/.role,admin,firstName,Alex
	def := pathParam("user", oaschema.EncodingStyleLabel, false, objectSchema(map[string]*base.Schema{
		"role":      stringSchema(),
		"firstName": stringSchema(),
	}))
	result, errs := DecodePathValue(def, ".role,admin,firstName,Alex")
	assert.Empty(t, errs)
	assert.Equal(t, map[string]any{"role": "admin", "firstName": "Alex"}, result)
}

func TestDecodePathValue_LabelStyle_Object_Explode(t *testing.T) {
	// /users/.role=admin.firstName=Alex
	def := pathParam("user", oaschema.EncodingStyleLabel, true, objectSchema(map[string]*base.Schema{
		"role":      stringSchema(),
		"firstName": stringSchema(),
	}))
	result, errs := DecodePathValue(def, ".role=admin.firstName=Alex")
	assert.Empty(t, errs)
	assert.Equal(t, map[string]any{"role": "admin", "firstName": "Alex"}, result)
}

// --- Matrix style ---

func TestDecodePathValue_MatrixStyle_MissingSemicolonPrefix(t *testing.T) {
	def := pathParam("id", oaschema.EncodingStyleMatrix, false, intSchema())
	_, errs := DecodePathValue(def, "id=42")
	require.Len(t, errs, 1)
	assert.Equal(t, oasvalidator.ErrCodeInvalidURLParam, errs[0].Code)
}

func TestDecodePathValue_MatrixStyle_Integer(t *testing.T) {
	// /users/;id=42
	def := pathParam("id", oaschema.EncodingStyleMatrix, false, intSchema())
	result, errs := DecodePathValue(def, ";id=42")
	assert.Empty(t, errs)
	assert.Equal(t, int64(42), result)
}

func TestDecodePathValue_MatrixStyle_Array_NonExplode(t *testing.T) {
	// /users/;id=3,4,5
	def := pathParam("id", oaschema.EncodingStyleMatrix, false, intArraySchema())
	result, errs := DecodePathValue(def, ";id=3,4,5")
	assert.Empty(t, errs)
	assert.Equal(t, []any{int64(3), int64(4), int64(5)}, result)
}

func TestDecodePathValue_MatrixStyle_Array_Explode(t *testing.T) {
	// /users/;id=3;id=4;id=5
	def := pathParam("id", oaschema.EncodingStyleMatrix, true, stringArraySchema())
	result, errs := DecodePathValue(def, ";id=3;id=4;id=5")
	assert.Empty(t, errs)
	assert.Equal(t, []any{"3", "4", "5"}, result)
}

func TestDecodePathValue_MatrixStyle_Object_NonExplode(t *testing.T) {
	// /users/;id=role,admin,firstName,Alex
	def := pathParam("id", oaschema.EncodingStyleMatrix, false, objectSchema(map[string]*base.Schema{
		"role":      stringSchema(),
		"firstName": stringSchema(),
	}))
	result, errs := DecodePathValue(def, ";id=role,admin,firstName,Alex")
	assert.Empty(t, errs)
	assert.Equal(t, map[string]any{"role": "admin", "firstName": "Alex"}, result)
}

func TestDecodePathValue_MatrixStyle_Object_Explode(t *testing.T) {
	// /users/;role=admin;firstName=Alex
	def := pathParam("id", oaschema.EncodingStyleMatrix, true, objectSchema(map[string]*base.Schema{
		"role":      stringSchema(),
		"firstName": stringSchema(),
	}))
	result, errs := DecodePathValue(def, ";role=admin;firstName=Alex")
	assert.Empty(t, errs)
	assert.Equal(t, map[string]any{"role": "admin", "firstName": "Alex"}, result)
}

// --- Simple style array / object ---

func TestDecodePathValue_SimpleStyle_Array_NonExplode(t *testing.T) {
	// /users/3,4,5
	def := pathParam("id", oaschema.EncodingStyleSimple, false, intArraySchema())
	result, errs := DecodePathValue(def, "3,4,5")
	assert.Empty(t, errs)
	assert.Equal(t, []any{int64(3), int64(4), int64(5)}, result)
}

func TestDecodePathValue_SimpleStyle_Object_NonExplode(t *testing.T) {
	// /users/role,admin,firstName,Alex
	def := pathParam("user", oaschema.EncodingStyleSimple, false, objectSchema(map[string]*base.Schema{
		"role":      stringSchema(),
		"firstName": stringSchema(),
	}))
	result, errs := DecodePathValue(def, "role,admin,firstName,Alex")
	assert.Empty(t, errs)
	assert.Equal(t, map[string]any{"role": "admin", "firstName": "Alex"}, result)
}

func TestDecodePathValue_SimpleStyle_Object_Explode(t *testing.T) {
	// /users/role=admin,firstName=Alex
	def := pathParam("user", oaschema.EncodingStyleSimple, true, objectSchema(map[string]*base.Schema{
		"role":      stringSchema(),
		"firstName": stringSchema(),
	}))
	result, errs := DecodePathValue(def, "role=admin,firstName=Alex")
	assert.Empty(t, errs)
	assert.Equal(t, map[string]any{"role": "admin", "firstName": "Alex"}, result)
}

// --- trimPathValueByStyle ---

func TestTrimPathValueByStyle(t *testing.T) {
	cases := []struct {
		name    string
		style   oaschema.ParameterEncodingStyle
		raw     string
		want    string
		wantErr bool
	}{
		{
			name:  "simple_passthrough",
			style: oaschema.EncodingStyleSimple,
			raw:   "value",
			want:  "value",
		},
		{
			name:  "label_strips_dot",
			style: oaschema.EncodingStyleLabel,
			raw:   ".value",
			want:  "value",
		},
		{
			name:    "label_missing_dot",
			style:   oaschema.EncodingStyleLabel,
			raw:     "value",
			wantErr: true,
		},
		{
			name:  "matrix_strips_semicolon",
			style: oaschema.EncodingStyleMatrix,
			raw:   ";value",
			want:  "value",
		},
		{
			name:    "matrix_missing_semicolon",
			style:   oaschema.EncodingStyleMatrix,
			raw:     "value",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := trimPathValueByStyle("p", tc.raw, tc.style)
			if tc.wantErr {
				require.NotNil(t, err)
				assert.Equal(t, oasvalidator.ErrCodeInvalidURLParam, err.Code)
			} else {
				require.Nil(t, err)
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

// --- parsePathArrayParam ---

func TestParsePathArrayParam(t *testing.T) {
	cases := []struct {
		name    string
		style   oaschema.ParameterEncodingStyle
		explode bool
		raw     string
		want    []string
		wantErr bool
	}{
		{
			name:  "simple_csv",
			style: oaschema.EncodingStyleSimple,
			raw:   "3,4,5",
			want:  []string{"3", "4", "5"},
		},
		{
			name:  "simple_single",
			style: oaschema.EncodingStyleSimple,
			raw:   "42",
			want:  []string{"42"},
		},
		{
			name:    "label_non_explode_comma",
			style:   oaschema.EncodingStyleLabel,
			explode: false,
			raw:     "3,4,5",
			want:    []string{"3", "4", "5"},
		},
		{
			name:    "label_explode_dot",
			style:   oaschema.EncodingStyleLabel,
			explode: true,
			raw:     "3.4.5",
			want:    []string{"3", "4", "5"},
		},
		{
			name:  "label_empty_raw",
			style: oaschema.EncodingStyleLabel,
			raw:   "",
			want:  nil,
		},
		{
			name:    "matrix_non_explode",
			style:   oaschema.EncodingStyleMatrix,
			explode: false,
			raw:     "id=3,4,5",
			want:    []string{"3", "4", "5"},
		},
		{
			name:    "matrix_non_explode_missing_prefix",
			style:   oaschema.EncodingStyleMatrix,
			explode: false,
			raw:     "3,4,5",
			wantErr: true,
		},
		{
			name:    "matrix_explode",
			style:   oaschema.EncodingStyleMatrix,
			explode: true,
			raw:     "id=3;id=4;id=5",
			want:    []string{"3", "4", "5"},
		},
		{
			name:    "matrix_explode_missing_key_prefix",
			style:   oaschema.EncodingStyleMatrix,
			explode: true,
			raw:     "3;4;5",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parsePathArrayParam("id", tc.raw, tc.style, tc.explode)
			if tc.wantErr {
				require.NotNil(t, err)
				assert.Equal(t, oasvalidator.ErrCodeInvalidURLParam, err.Code)
			} else {
				require.Nil(t, err)
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

// --- parsePathObjectParam ---

func TestParsePathObjectParam_Empty(t *testing.T) {
	got, err := parsePathObjectParam("id", "", oaschema.EncodingStyleSimple, false)
	require.Nil(t, err)
	assert.Nil(t, got)
}

func TestParsePathObjectParam_SimpleNonExplode(t *testing.T) {
	// role,admin,firstName,Alex
	got, err := parsePathObjectParam("id", "role,admin,firstName,Alex", oaschema.EncodingStyleSimple, false)
	require.Nil(t, err)
	assert.Equal(t, map[string][]string{
		"role":      {"admin"},
		"firstName": {"Alex"},
	}, got)
}

func TestParsePathObjectParam_SimpleNonExplode_OddParts(t *testing.T) {
	_, err := parsePathObjectParam("id", "role,admin,firstName", oaschema.EncodingStyleSimple, false)
	require.NotNil(t, err)
	assert.Equal(t, oasvalidator.ErrCodeInvalidURLParam, err.Code)
}

func TestParsePathObjectParam_SimpleExplode(t *testing.T) {
	// role=admin,firstName=Alex
	got, err := parsePathObjectParam("id", "role=admin,firstName=Alex", oaschema.EncodingStyleSimple, true)
	require.Nil(t, err)
	assert.Equal(t, map[string][]string{
		"role":      {"admin"},
		"firstName": {"Alex"},
	}, got)
}

func TestParsePathObjectParam_SimpleExplode_MissingEquals(t *testing.T) {
	_, err := parsePathObjectParam("id", "roleadmin", oaschema.EncodingStyleSimple, true)
	require.NotNil(t, err)
	assert.Equal(t, oasvalidator.ErrCodeInvalidURLParam, err.Code)
}

func TestParsePathObjectParam_LabelNonExplode(t *testing.T) {
	// already stripped of the leading dot by trimPathValueByStyle
	// /users/.role,admin,firstName,Alex → rawValue = "role,admin,firstName,Alex"
	got, err := parsePathObjectParam("id", "role,admin,firstName,Alex", oaschema.EncodingStyleLabel, false)
	require.Nil(t, err)
	assert.Equal(t, map[string][]string{
		"role":      {"admin"},
		"firstName": {"Alex"},
	}, got)
}

func TestParsePathObjectParam_LabelExplode(t *testing.T) {
	// rawValue after dot-strip: "role=admin.firstName=Alex"
	got, err := parsePathObjectParam("id", "role=admin.firstName=Alex", oaschema.EncodingStyleLabel, true)
	require.Nil(t, err)
	assert.Equal(t, map[string][]string{
		"role":      {"admin"},
		"firstName": {"Alex"},
	}, got)
}

func TestParsePathObjectParam_LabelExplode_MissingEquals(t *testing.T) {
	_, err := parsePathObjectParam("id", "roleadmin.firstNameAlex", oaschema.EncodingStyleLabel, true)
	require.NotNil(t, err)
	assert.Equal(t, oasvalidator.ErrCodeInvalidURLParam, err.Code)
}

func TestParsePathObjectParam_MatrixNonExplode(t *testing.T) {
	// rawValue after semicolon-strip: "id=role,admin,firstName,Alex"
	got, err := parsePathObjectParam("id", "id=role,admin,firstName,Alex", oaschema.EncodingStyleMatrix, false)
	require.Nil(t, err)
	assert.Equal(t, map[string][]string{
		"role":      {"admin"},
		"firstName": {"Alex"},
	}, got)
}

func TestParsePathObjectParam_MatrixNonExplode_MissingPrefix(t *testing.T) {
	_, err := parsePathObjectParam("id", "role,admin,firstName,Alex", oaschema.EncodingStyleMatrix, false)
	require.NotNil(t, err)
	assert.Equal(t, oasvalidator.ErrCodeInvalidURLParam, err.Code)
}

func TestParsePathObjectParam_MatrixExplode(t *testing.T) {
	// rawValue after semicolon-strip: "role=admin;firstName=Alex"
	got, err := parsePathObjectParam("id", "role=admin;firstName=Alex", oaschema.EncodingStyleMatrix, true)
	require.Nil(t, err)
	assert.Equal(t, map[string][]string{
		"role":      {"admin"},
		"firstName": {"Alex"},
	}, got)
}

func TestParsePathObjectParam_MatrixExplode_MissingEquals(t *testing.T) {
	_, err := parsePathObjectParam("id", "roleadmin;firstNameAlex", oaschema.EncodingStyleMatrix, true)
	require.NotNil(t, err)
	assert.Equal(t, oasvalidator.ErrCodeInvalidURLParam, err.Code)
}

// --- newInvalidPathObjectErrorMessage ---

func TestNewInvalidPathObjectErrorMessage(t *testing.T) {
	cases := []struct {
		style   oaschema.ParameterEncodingStyle
		explode bool
		substr  string
	}{
		{oaschema.EncodingStyleLabel, true, ".key1=value1.key2=value2"},
		{oaschema.EncodingStyleLabel, false, ".key1,value1,key2,value2"},
		{oaschema.EncodingStyleMatrix, true, ";key1=value1;key2=value2"},
		{oaschema.EncodingStyleMatrix, false, ";id=key1,value1,key2,value2"},
		{oaschema.EncodingStyleSimple, true, "role=admin,firstName=Alex"},
		{oaschema.EncodingStyleSimple, false, "key1,value1,key2,value2"},
	}

	for _, tc := range cases {
		msg := newInvalidPathObjectErrorMessage(tc.style, tc.explode)
		assert.Contains(t, msg, tc.substr, "style=%s explode=%v", tc.style, tc.explode)
	}
}

// helpers

func pathParam(name string, style oaschema.ParameterEncodingStyle, explode bool, schema *base.Schema) *oaschema.Parameter {
	s := style
	e := explode

	return &oaschema.Parameter{
		Name:    name,
		In:      oaschema.InPath,
		Style:   &s,
		Explode: &e,
		Schema:  schema,
	}
}

func intSchema() *base.Schema {
	return &base.Schema{Type: []string{oaschema.Integer}}
}

func stringSchema() *base.Schema {
	return &base.Schema{Type: []string{oaschema.String}}
}

func numberSchema() *base.Schema {
	return &base.Schema{Type: []string{oaschema.Number}}
}

func boolSchema() *base.Schema {
	return &base.Schema{Type: []string{oaschema.Boolean}}
}

func intArraySchema() *base.Schema {
	return &base.Schema{
		Type: []string{oaschema.Array},
		Items: &base.DynamicValue[*base.SchemaProxy, bool]{
			A: base.CreateSchemaProxy(intSchema()),
		},
	}
}

func stringArraySchema() *base.Schema {
	return &base.Schema{
		Type: []string{oaschema.Array},
		Items: &base.DynamicValue[*base.SchemaProxy, bool]{
			A: base.CreateSchemaProxy(stringSchema()),
		},
	}
}

func objectSchema(props map[string]*base.Schema) *base.Schema {
	m := orderedmap.New[string, *base.SchemaProxy]()
	for k, v := range props {
		m.Set(k, base.CreateSchemaProxy(v))
	}

	return &base.Schema{
		Type:       []string{oaschema.Object},
		Properties: m,
	}
}
