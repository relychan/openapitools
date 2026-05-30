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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- helpers (supplement the ones in path_decode_test.go) ---

func nullableSchema(types ...string) *base.Schema {
	t := true

	return &base.Schema{Type: types, Nullable: &t}
}

func numberArraySchema() *base.Schema {
	return &base.Schema{
		Type: []string{oaschema.Array},
		Items: &base.DynamicValue[*base.SchemaProxy, bool]{
			A: base.CreateSchemaProxy(numberSchema()),
		},
	}
}

func boolArraySchema() *base.Schema {
	return &base.Schema{
		Type: []string{oaschema.Array},
		Items: &base.DynamicValue[*base.SchemaProxy, bool]{
			A: base.CreateSchemaProxy(boolSchema()),
		},
	}
}

func objectArraySchema(props map[string]*base.Schema) *base.Schema {
	return &base.Schema{
		Type: []string{oaschema.Array},
		Items: &base.DynamicValue[*base.SchemaProxy, bool]{
			A: base.CreateSchemaProxy(objectSchema(props)),
		},
	}
}

func objectSchemaRequired(props map[string]*base.Schema, required ...string) *base.Schema {
	s := objectSchema(props)
	s.Required = required

	return s
}

func objectSchemaAdditional(props map[string]*base.Schema, additionalSchema *base.Schema) *base.Schema {
	s := objectSchema(props)
	s.AdditionalProperties = &base.DynamicValue[*base.SchemaProxy, bool]{
		A: base.CreateSchemaProxy(additionalSchema),
	}

	return s
}

func objectSchemaAdditionalBool(props map[string]*base.Schema) *base.Schema {
	s := objectSchema(props)
	s.AdditionalProperties = &base.DynamicValue[*base.SchemaProxy, bool]{
		B: true,
		N: 1,
	}

	return s
}

func objectSchemaPattern(pattern string, patternSchema *base.Schema) *base.Schema {
	m := orderedmap.New[string, *base.SchemaProxy]()
	m.Set(pattern, base.CreateSchemaProxy(patternSchema))

	return &base.Schema{
		Type:              []string{oaschema.Object},
		PatternProperties: m,
	}
}

func unionSchema(types ...string) *base.Schema {
	return &base.Schema{Type: types}
}

func makeNode(key ParamSelector, values []string, children ...*ParameterNode) *ParameterNode {
	return &ParameterNode{key: key, values: values, items: children}
}

// --- ParameterNodes.Insert ---

func TestParameterNodes_Insert_SingleLeaf(t *testing.T) {
	var nodes ParameterNodes

	err := nodes.Insert(ParamKeys{ParamKey("color")}, []string{"red"})
	require.Nil(t, err)
	require.Len(t, nodes, 1)
	assert.Equal(t, []string{"red"}, nodes[0].values)
}

func TestParameterNodes_Insert_DeduplicatesRoot(t *testing.T) {
	var nodes ParameterNodes

	require.Nil(t, nodes.Insert(ParamKeys{ParamKey("color"), ParamKey("r")}, []string{"100"}))
	require.Nil(t, nodes.Insert(ParamKeys{ParamKey("color"), ParamKey("g")}, []string{"200"}))

	require.Len(t, nodes, 1, "must reuse existing root node")
	assert.Len(t, nodes[0].items, 2)
}

func TestParameterNodes_Insert_MultipleRoots(t *testing.T) {
	var nodes ParameterNodes

	require.Nil(t, nodes.Insert(ParamKeys{ParamKey("a")}, []string{"1"}))
	require.Nil(t, nodes.Insert(ParamKeys{ParamKey("b")}, []string{"2"}))

	require.Len(t, nodes, 2)
}

func TestParameterNodes_Insert_EmptyKeys_NoOp(t *testing.T) {
	var nodes ParameterNodes

	err := nodes.Insert(ParamKeys{}, []string{"x"})
	require.Nil(t, err)
	assert.Empty(t, nodes)
}

func TestParameterNodes_Find_HitsCorrectNode(t *testing.T) {
	var nodes ParameterNodes

	require.Nil(t, nodes.Insert(ParamKeys{ParamKey("a")}, []string{"1"}))
	require.Nil(t, nodes.Insert(ParamKeys{ParamKey("b")}, []string{"2"}))

	found := nodes.Find(ParamKey("b"))
	require.NotNil(t, found)
	assert.Equal(t, []string{"2"}, found.values)
}

func TestParameterNodes_Find_MissingKey_ReturnsNil(t *testing.T) {
	var nodes ParameterNodes

	require.Nil(t, nodes.Insert(ParamKeys{ParamKey("a")}, []string{"1"}))

	assert.Nil(t, nodes.Find(ParamKey("z")))
}

// --- ParameterNode.Normalize ---

func TestNormalize_SingleIndexChild_Flattens(t *testing.T) {
	// node[0] = "val"  →  node = "val"
	node := makeNode(ParamKey("root"), nil,
		makeNode(ParamIndex(0), []string{"val"}),
	)
	node.Normalize()

	assert.Equal(t, []string{"val"}, node.values)
	assert.Empty(t, node.items)
}

func TestNormalize_SingleNumericKeyChild_ConvertsToIndex(t *testing.T) {
	// node["0"] = "val"  →  node[0] = "val", then flattens
	node := makeNode(ParamKey("root"), nil,
		makeNode(ParamKey("0"), []string{"val"}),
	)
	node.Normalize()

	// After promoting ParamKey("0") → ParamIndex(0) it gets flattened too.
	assert.Equal(t, []string{"val"}, node.items[0].values)
	assert.Equal(t, ParamIndex(0), node.items[0].key)
}

func TestNormalize_MultipleIndexChildren_SortedByIndex(t *testing.T) {
	node := makeNode(ParamKey("ids"), nil,
		makeNode(ParamIndex(2), []string{"c"}),
		makeNode(ParamIndex(0), []string{"a"}),
		makeNode(ParamIndex(1), []string{"b"}),
	)
	node.Normalize()

	require.Len(t, node.items, 3)
	assert.Equal(t, ParamIndex(0), node.items[0].key)
	assert.Equal(t, ParamIndex(1), node.items[1].key)
	assert.Equal(t, ParamIndex(2), node.items[2].key)
}

func TestNormalize_ObjectChildren_NotSorted(t *testing.T) {
	node := makeNode(ParamKey("obj"), nil,
		makeNode(ParamKey("z"), []string{"1"}),
		makeNode(ParamKey("a"), []string{"2"}),
	)
	node.Normalize()

	// Object keys must not be reordered.
	require.Len(t, node.items, 2)
	assert.Equal(t, ParamKey("z"), node.items[0].key)
	assert.Equal(t, ParamKey("a"), node.items[1].key)
}

func TestNormalize_SingleIndexWithChildren_NotFlattened(t *testing.T) {
	// node[0].sub = "x"  — has nested items, must NOT flatten
	inner := makeNode(ParamIndex(0), nil,
		makeNode(ParamKey("sub"), []string{"x"}),
	)
	node := makeNode(ParamKey("root"), nil, inner)
	node.Normalize()

	require.NotEmpty(t, node.items, "should keep index child that itself has children")
}

// --- ParameterNode.InsertNode ---

func TestInsertNode_MixedKeyAndIndex_CoercesIndex(t *testing.T) {
	// First insert with ParamKey("0"), then insert with ParamIndex(1).
	node := makeNode(ParamKey("ids"), nil)

	require.Nil(t, node.InsertNode(ParamKeys{ParamKey("0")}, []string{"a"}))
	require.Nil(t, node.InsertNode(ParamKeys{ParamIndex(1)}, []string{"b"}))

	assert.IsType(t, ParamIndex(0), node.items[0].key)
}

func TestInsertNode_MixedNonNumericKey_ReturnsError(t *testing.T) {
	node := makeNode(ParamKey("ids"), nil)

	require.Nil(t, node.InsertNode(ParamKeys{ParamIndex(0)}, []string{"a"}))

	err := node.InsertNode(ParamKeys{ParamKey("notANumber")}, []string{"b"})
	require.NotNil(t, err)
}

// --- ParameterNode.Decode – arbitrary (no schema) ---

func TestDecode_NoSchema_Scalar(t *testing.T) {
	node := makeNode(ParamKey("name"), []string{"alice"})
	result, errs := node.Decode(nil)
	assert.Empty(t, errs)
	assert.Equal(t, "alice", result)
}

func TestDecode_NoSchema_MultiValue(t *testing.T) {
	node := makeNode(ParamKey("ids"), []string{"1", "2", "3"})
	result, errs := node.Decode(nil)
	assert.Empty(t, errs)
	assert.Equal(t, []string{"1", "2", "3"}, result)
}

func TestDecode_NoSchema_NestedObject(t *testing.T) {
	node := makeNode(ParamKey("user"), nil,
		makeNode(ParamKey("name"), []string{"alice"}),
		makeNode(ParamKey("age"), []string{"30"}),
	)
	result, errs := node.Decode(nil)
	assert.Empty(t, errs)
	m, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "alice", m["name"])
	assert.Equal(t, "30", m["age"])
}

func TestDecode_NoSchema_NestedArray(t *testing.T) {
	node := makeNode(ParamKey("ids"), nil,
		makeNode(ParamIndex(0), []string{"10"}),
		makeNode(ParamIndex(1), []string{"20"}),
	)
	result, errs := node.Decode(nil)
	assert.Empty(t, errs)
	arr, ok := result.([]any)
	require.True(t, ok)
	assert.Equal(t, []any{"10", "20"}, arr)
}

// --- ParameterNode.Decode – primitive types ---

func TestDecode_Integer_Valid(t *testing.T) {
	node := makeNode(ParamKey("count"), []string{"42"})
	result, errs := node.Decode(intSchema())
	assert.Empty(t, errs)
	assert.Equal(t, int64(42), result)
}

func TestDecode_Integer_Invalid(t *testing.T) {
	node := makeNode(ParamKey("count"), []string{"not-a-number"})
	_, errs := node.Decode(intSchema())
	require.NotEmpty(t, errs)
}

func TestDecode_Number_Float(t *testing.T) {
	node := makeNode(ParamKey("price"), []string{"3.14"})
	result, errs := node.Decode(numberSchema())
	assert.Empty(t, errs)
	assert.InDelta(t, 3.14, result, 1e-9)
}

func TestDecode_Boolean_True(t *testing.T) {
	node := makeNode(ParamKey("active"), []string{"true"})
	result, errs := node.Decode(boolSchema())
	assert.Empty(t, errs)
	assert.Equal(t, true, result)
}

func TestDecode_Boolean_False(t *testing.T) {
	node := makeNode(ParamKey("active"), []string{"false"})
	result, errs := node.Decode(boolSchema())
	assert.Empty(t, errs)
	assert.Equal(t, false, result)
}

func TestDecode_String_Valid(t *testing.T) {
	node := makeNode(ParamKey("name"), []string{"alice"})
	result, errs := node.Decode(stringSchema())
	assert.Empty(t, errs)
	assert.Equal(t, "alice", result)
}

func TestDecode_Nullable_EmptyValues_ReturnsNil(t *testing.T) {
	node := makeNode(ParamKey("opt"), nil)
	result, errs := node.Decode(nullableSchema(oaschema.Integer))
	assert.Empty(t, errs)
	assert.Nil(t, result)
}

func TestDecode_NonNullable_EmptyValues_ReturnsError(t *testing.T) {
	node := makeNode(ParamKey("id"), nil)
	_, errs := node.Decode(intSchema())
	require.NotEmpty(t, errs)
}

// --- ParameterNode.Decode – union types ---

func TestDecode_UnionIntOrString_IntInput(t *testing.T) {
	schema := unionSchema(oaschema.Integer, oaschema.String)
	node := makeNode(ParamKey("val"), []string{"99"})
	result, errs := node.Decode(schema)
	assert.Empty(t, errs)
	assert.Equal(t, int64(99), result)
}

func TestDecode_UnionIntOrString_StringInput(t *testing.T) {
	schema := unionSchema(oaschema.Integer, oaschema.String)
	node := makeNode(ParamKey("val"), []string{"hello"})
	result, errs := node.Decode(schema)
	assert.Empty(t, errs)
	assert.Equal(t, "hello", result)
}

func TestDecode_UnionObjectOrString_ObjectInput(t *testing.T) {
	schema := &base.Schema{
		Type: []string{oaschema.Object, oaschema.String},
		Properties: func() *orderedmap.Map[string, *base.SchemaProxy] {
			m := orderedmap.New[string, *base.SchemaProxy]()
			m.Set("name", base.CreateSchemaProxy(stringSchema()))

			return m
		}(),
	}
	node := makeNode(ParamKey("filter"), nil,
		makeNode(ParamKey("name"), []string{"alice"}),
	)
	result, errs := node.Decode(schema)
	assert.Empty(t, errs)
	m, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "alice", m["name"])
}

// --- ParameterNode.Decode – simple array ---

func TestDecode_Array_Integer_FromValues(t *testing.T) {
	// Flat values decoded against array[integer] schema.
	node := makeNode(ParamKey("ids"), []string{"1", "2", "3"})
	result, errs := node.Decode(intArraySchema())
	assert.Empty(t, errs)
	assert.Equal(t, []any{int64(1), int64(2), int64(3)}, result)
}

func TestDecode_Array_String_FromValues(t *testing.T) {
	node := makeNode(ParamKey("tags"), []string{"go", "openapi"})
	result, errs := node.Decode(stringArraySchema())
	assert.Empty(t, errs)
	assert.Equal(t, []any{"go", "openapi"}, result)
}

func TestDecode_Array_Number_FromValues(t *testing.T) {
	node := makeNode(ParamKey("scores"), []string{"1.1", "2.2"})
	result, errs := node.Decode(numberArraySchema())
	assert.Empty(t, errs)
	arr, ok := result.([]any)
	require.True(t, ok)
	assert.InDelta(t, 1.1, arr[0], 1e-9)
	assert.InDelta(t, 2.2, arr[1], 1e-9)
}

func TestDecode_Array_Bool_FromValues(t *testing.T) {
	node := makeNode(ParamKey("flags"), []string{"true", "false", "true"})
	result, errs := node.Decode(boolArraySchema())
	assert.Empty(t, errs)
	assert.Equal(t, []any{true, false, true}, result)
}

func TestDecode_Array_InvalidItemType(t *testing.T) {
	node := makeNode(ParamKey("ids"), []string{"1", "bad", "3"})
	_, errs := node.Decode(intArraySchema())
	require.NotEmpty(t, errs)
}

func TestDecode_Array_NoItemSchema_ArbitraryDecoded(t *testing.T) {
	schema := &base.Schema{Type: []string{oaschema.Array}}
	node := makeNode(ParamKey("items"), []string{"a", "b"})
	result, errs := node.Decode(schema)
	assert.Empty(t, errs)
	assert.NotNil(t, result)
}

// --- ParameterNode.Decode – nested array (tree nodes) ---

func TestDecode_Array_NestedIndexedNodes(t *testing.T) {
	node := makeNode(ParamKey("ids"), nil,
		makeNode(ParamIndex(0), []string{"10"}),
		makeNode(ParamIndex(1), []string{"20"}),
		makeNode(ParamIndex(2), []string{"30"}),
	)
	result, errs := node.Decode(intArraySchema())
	assert.Empty(t, errs)
	assert.Equal(t, []any{int64(10), int64(20), int64(30)}, result)
}

func TestDecode_Array_NestedObjectItems(t *testing.T) {
	// ids[0] = {name: "alice"}, ids[1] = {name: "bob"}
	schema := objectArraySchema(map[string]*base.Schema{
		"name": stringSchema(),
	})
	node := makeNode(ParamKey("users"), nil,
		makeNode(ParamIndex(0), nil,
			makeNode(ParamKey("name"), []string{"alice"}),
		),
		makeNode(ParamIndex(1), nil,
			makeNode(ParamKey("name"), []string{"bob"}),
		),
	)
	result, errs := node.Decode(schema)
	assert.Empty(t, errs)
	arr, ok := result.([]any)
	require.True(t, ok)
	require.Len(t, arr, 2)
	assert.Equal(t, map[string]any{"name": "alice"}, arr[0])
	assert.Equal(t, map[string]any{"name": "bob"}, arr[1])
}

// --- ParameterNode.Decode – object ---

func TestDecode_Object_FlatProperties(t *testing.T) {
	schema := objectSchema(map[string]*base.Schema{
		"name": stringSchema(),
		"age":  intSchema(),
	})
	node := makeNode(ParamKey("user"), nil,
		makeNode(ParamKey("name"), []string{"alice"}),
		makeNode(ParamKey("age"), []string{"30"}),
	)
	result, errs := node.Decode(schema)
	assert.Empty(t, errs)
	assert.Equal(t, map[string]any{
		"name": "alice",
		"age":  int64(30),
	}, result)
}

func TestDecode_Object_RequiredPropertyPresent(t *testing.T) {
	schema := objectSchemaRequired(
		map[string]*base.Schema{"name": stringSchema()},
		"name",
	)
	node := makeNode(ParamKey("user"), nil,
		makeNode(ParamKey("name"), []string{"alice"}),
	)
	result, errs := node.Decode(schema)
	assert.Empty(t, errs)
	assert.Equal(t, map[string]any{"name": "alice"}, result)
}

func TestDecode_Object_RequiredPropertyMissing(t *testing.T) {
	schema := objectSchemaRequired(
		map[string]*base.Schema{
			"name": stringSchema(),
			"age":  intSchema(),
		},
		"age",
	)
	node := makeNode(ParamKey("user"), nil,
		makeNode(ParamKey("name"), []string{"alice"}),
	)
	_, errs := node.Decode(schema)
	require.NotEmpty(t, errs)
}

func TestDecode_Object_OptionalPropertyAbsent(t *testing.T) {
	schema := objectSchema(map[string]*base.Schema{
		"name": stringSchema(),
		"age":  intSchema(),
	})
	// Only "name" supplied; "age" is optional.
	node := makeNode(ParamKey("user"), nil,
		makeNode(ParamKey("name"), []string{"bob"}),
	)
	result, errs := node.Decode(schema)
	assert.Empty(t, errs)
	m, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "bob", m["name"])
	_, hasAge := m["age"]
	assert.False(t, hasAge)
}

func TestDecode_Object_NestedObject(t *testing.T) {
	schema := objectSchema(map[string]*base.Schema{
		"address": objectSchema(map[string]*base.Schema{
			"city":    stringSchema(),
			"country": stringSchema(),
		}),
	})
	node := makeNode(ParamKey("user"), nil,
		makeNode(ParamKey("address"), nil,
			makeNode(ParamKey("city"), []string{"Singapore"}),
			makeNode(ParamKey("country"), []string{"SG"}),
		),
	)
	result, errs := node.Decode(schema)
	assert.Empty(t, errs)
	m, ok := result.(map[string]any)
	require.True(t, ok)
	addr, ok := m["address"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Singapore", addr["city"])
	assert.Equal(t, "SG", addr["country"])
}

func TestDecode_Object_NestedObjectWithArray(t *testing.T) {
	schema := objectSchema(map[string]*base.Schema{
		"name": stringSchema(),
		"roles": {
			Type: []string{oaschema.Array},
			Items: &base.DynamicValue[*base.SchemaProxy, bool]{
				A: base.CreateSchemaProxy(stringSchema()),
			},
		},
	})
	node := makeNode(ParamKey("user"), nil,
		makeNode(ParamKey("name"), []string{"alice"}),
		makeNode(ParamKey("roles"), nil,
			makeNode(ParamIndex(0), []string{"admin"}),
			makeNode(ParamIndex(1), []string{"viewer"}),
		),
	)
	result, errs := node.Decode(schema)
	assert.Empty(t, errs)
	m, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "alice", m["name"])
	assert.Equal(t, []any{"admin", "viewer"}, m["roles"])
}

// --- AdditionalProperties ---

func TestDecode_Object_AdditionalProperties_SchemaTyped(t *testing.T) {
	schema := objectSchemaAdditional(
		map[string]*base.Schema{"name": stringSchema()},
		intSchema(),
	)
	node := makeNode(ParamKey("obj"), nil,
		makeNode(ParamKey("name"), []string{"alice"}),
		makeNode(ParamKey("score"), []string{"99"}),
	)
	result, errs := node.Decode(schema)
	assert.Empty(t, errs)
	m, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "alice", m["name"])
	assert.Equal(t, int64(99), m["score"])
}

func TestDecode_Object_AdditionalProperties_Bool_True(t *testing.T) {
	schema := objectSchemaAdditionalBool(map[string]*base.Schema{"name": stringSchema()})
	node := makeNode(ParamKey("obj"), nil,
		makeNode(ParamKey("name"), []string{"alice"}),
		makeNode(ParamKey("extra"), []string{"anything"}),
	)
	result, errs := node.Decode(schema)
	assert.Empty(t, errs)
	m, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "anything", m["extra"])
}

func TestDecode_Object_AdditionalProperties_InvalidType(t *testing.T) {
	schema := objectSchemaAdditional(
		map[string]*base.Schema{"name": stringSchema()},
		intSchema(),
	)
	node := makeNode(ParamKey("obj"), nil,
		makeNode(ParamKey("name"), []string{"alice"}),
		makeNode(ParamKey("score"), []string{"not-a-number"}),
	)
	_, errs := node.Decode(schema)
	require.NotEmpty(t, errs)
}

// --- PatternProperties ---

func TestDecode_Object_PatternProperties_Match(t *testing.T) {
	schema := objectSchemaPattern(`^x_`, intSchema())
	node := makeNode(ParamKey("obj"), nil,
		makeNode(ParamKey("x_count"), []string{"5"}),
		makeNode(ParamKey("x_total"), []string{"100"}),
	)
	result, errs := node.Decode(schema)
	assert.Empty(t, errs)
	m, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, int64(5), m["x_count"])
	assert.Equal(t, int64(100), m["x_total"])
}

func TestDecode_Object_PatternProperties_NoMatch_Skipped(t *testing.T) {
	schema := objectSchemaPattern(`^prefix_`, stringSchema())
	// "other" does not match "^prefix_" — not captured in any property.
	node := makeNode(ParamKey("obj"), nil,
		makeNode(ParamKey("other"), []string{"val"}),
	)
	result, errs := node.Decode(schema)
	assert.Empty(t, errs)
	m, ok := result.(map[string]any)
	require.True(t, ok)
	_, present := m["other"]
	assert.False(t, present)
}

func TestDecode_Object_PatternProperties_ExplicitPropertyTakesPrecedence(t *testing.T) {
	// "name" is in both explicit properties and matches the pattern "^n" — explicit wins.
	explicitM := orderedmap.New[string, *base.SchemaProxy]()
	explicitM.Set("name", base.CreateSchemaProxy(stringSchema()))

	patternM := orderedmap.New[string, *base.SchemaProxy]()
	patternM.Set("^n", base.CreateSchemaProxy(intSchema()))

	schema := &base.Schema{
		Type:              []string{oaschema.Object},
		Properties:        explicitM,
		PatternProperties: patternM,
	}
	node := makeNode(ParamKey("obj"), nil,
		makeNode(ParamKey("name"), []string{"alice"}),
	)
	result, errs := node.Decode(schema)
	assert.Empty(t, errs)
	m, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "alice", m["name"])
}

// --- allOf ---

func TestDecode_Object_AllOf_MergesProperties(t *testing.T) {
	base1 := objectSchema(map[string]*base.Schema{"name": stringSchema()})
	base2 := objectSchema(map[string]*base.Schema{"age": intSchema()})

	schema := &base.Schema{
		Type: []string{oaschema.Object},
		AllOf: []*base.SchemaProxy{
			base.CreateSchemaProxy(base1),
			base.CreateSchemaProxy(base2),
		},
	}
	node := makeNode(ParamKey("user"), nil,
		makeNode(ParamKey("name"), []string{"alice"}),
		makeNode(ParamKey("age"), []string{"30"}),
	)
	result, errs := node.Decode(schema)
	assert.Empty(t, errs)
	m, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "alice", m["name"])
	assert.Equal(t, int64(30), m["age"])
}

func TestDecode_Object_AllOf_PartialMissing_Error(t *testing.T) {
	base1 := objectSchemaRequired(map[string]*base.Schema{"name": stringSchema()}, "name")
	base2 := objectSchemaRequired(map[string]*base.Schema{"age": intSchema()}, "age")

	schema := &base.Schema{
		Type: []string{oaschema.Object},
		AllOf: []*base.SchemaProxy{
			base.CreateSchemaProxy(base1),
			base.CreateSchemaProxy(base2),
		},
	}
	// Only "name" is supplied; "age" is required by the second allOf.
	node := makeNode(ParamKey("user"), nil,
		makeNode(ParamKey("name"), []string{"alice"}),
	)
	_, errs := node.Decode(schema)
	require.NotEmpty(t, errs)
}

// --- anyOf ---

func TestDecode_Object_AnyOf_FirstMatchSuffices(t *testing.T) {
	opt1 := objectSchema(map[string]*base.Schema{"name": stringSchema()})
	opt2 := objectSchema(map[string]*base.Schema{"code": intSchema()})

	schema := &base.Schema{
		Type: []string{oaschema.Object},
		AnyOf: []*base.SchemaProxy{
			base.CreateSchemaProxy(opt1),
			base.CreateSchemaProxy(opt2),
		},
	}
	node := makeNode(ParamKey("obj"), nil,
		makeNode(ParamKey("name"), []string{"alice"}),
	)
	result, errs := node.Decode(schema)
	assert.Empty(t, errs)
	m, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "alice", m["name"])
}

func TestDecode_Object_AnyOf_BothMatch_MergesAll(t *testing.T) {
	opt1 := objectSchema(map[string]*base.Schema{"name": stringSchema()})
	opt2 := objectSchema(map[string]*base.Schema{"age": intSchema()})

	schema := &base.Schema{
		Type: []string{oaschema.Object},
		AnyOf: []*base.SchemaProxy{
			base.CreateSchemaProxy(opt1),
			base.CreateSchemaProxy(opt2),
		},
	}
	node := makeNode(ParamKey("obj"), nil,
		makeNode(ParamKey("name"), []string{"alice"}),
		makeNode(ParamKey("age"), []string{"30"}),
	)
	result, errs := node.Decode(schema)
	assert.Empty(t, errs)
	m, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "alice", m["name"])
	assert.Equal(t, int64(30), m["age"])
}

// --- oneOf ---

func TestDecode_Object_OneOf_FirstMatch(t *testing.T) {
	opt1 := objectSchema(map[string]*base.Schema{"name": stringSchema()})
	opt2 := objectSchema(map[string]*base.Schema{"code": intSchema()})

	schema := &base.Schema{
		Type: []string{oaschema.Object},
		OneOf: []*base.SchemaProxy{
			base.CreateSchemaProxy(opt1),
			base.CreateSchemaProxy(opt2),
		},
	}
	node := makeNode(ParamKey("obj"), nil,
		makeNode(ParamKey("name"), []string{"alice"}),
	)
	result, errs := node.Decode(schema)
	assert.Empty(t, errs)
	m, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "alice", m["name"])
}

// --- deep nesting ---

func TestDecode_DeepNested_ObjectArrayObject(t *testing.T) {
	// Mirrors the query-string shape:
	// role[0][user][0][]= admin&role[0][user][0][]=anonymous
	// → {role: [{user: [["admin","anonymous"]]}]}
	userValuesSchema := &base.Schema{
		Type: []string{oaschema.Array},
		Items: &base.DynamicValue[*base.SchemaProxy, bool]{
			A: base.CreateSchemaProxy(stringSchema()),
		},
	}
	userItemSchema := objectSchema(map[string]*base.Schema{
		"user": {
			Type: []string{oaschema.Array},
			Items: &base.DynamicValue[*base.SchemaProxy, bool]{
				A: base.CreateSchemaProxy(userValuesSchema),
			},
		},
	})
	schema := objectSchema(map[string]*base.Schema{
		"role": {
			Type: []string{oaschema.Array},
			Items: &base.DynamicValue[*base.SchemaProxy, bool]{
				A: base.CreateSchemaProxy(userItemSchema),
			},
		},
	})

	// Build the tree manually the way Normalize() would leave it after parsing
	// role[0][user][0][] = ["admin","anonymous"].
	inner := makeNode(ParamIndex(-1), []string{"admin", "anonymous"})
	userInner := makeNode(ParamIndex(0), nil, inner)
	userNode := makeNode(ParamKey("user"), nil, userInner)
	roleItem := makeNode(ParamIndex(0), nil, userNode)
	node := makeNode(ParamKey("role"), nil, roleItem)

	// Normalize first (mirrors production path).
	node.Normalize()

	root := makeNode(ParamKey("root"), nil, node)
	result, errs := root.Decode(schema)
	assert.Empty(t, errs)

	m, ok := result.(map[string]any)
	require.True(t, ok)

	roles, ok := m["role"].([]any)
	require.True(t, ok)
	require.Len(t, roles, 1)

	roleMap, ok := roles[0].(map[string]any)
	require.True(t, ok)

	users, ok := roleMap["user"].([]any)
	require.True(t, ok)
	require.Len(t, users, 1)
	assert.Equal(t, []any{"admin", "anonymous"}, users[0])
}

func TestDecode_DeepNested_ObjectWithMixedProperties(t *testing.T) {
	// payment_method_options[card][setup_future_usage] = on_session
	// payment_method_options[card][request_three_d_secure] = any
	cardSchema := objectSchema(map[string]*base.Schema{
		"setup_future_usage":     stringSchema(),
		"request_three_d_secure": stringSchema(),
	})
	schema := objectSchema(map[string]*base.Schema{
		"card": cardSchema,
	})

	node := makeNode(ParamKey("payment_method_options"), nil,
		makeNode(ParamKey("card"), nil,
			makeNode(ParamKey("setup_future_usage"), []string{"on_session"}),
			makeNode(ParamKey("request_three_d_secure"), []string{"any"}),
		),
	)
	result, errs := node.Decode(schema)
	assert.Empty(t, errs)
	m, ok := result.(map[string]any)
	require.True(t, ok)
	card, ok := m["card"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "on_session", card["setup_future_usage"])
	assert.Equal(t, "any", card["request_three_d_secure"])
}

func TestDecode_DeepNested_ArrayOfObjectsWithNestedArrays(t *testing.T) {
	// line_items[0][price_data][unit_amount] = 945322526
	// line_items[0][quantity] = 968305911
	priceDataSchema := objectSchema(map[string]*base.Schema{
		"unit_amount": intSchema(),
		"currency":    stringSchema(),
	})
	lineItemSchema := objectSchema(map[string]*base.Schema{
		"price_data": priceDataSchema,
		"quantity":   intSchema(),
	})
	schema := &base.Schema{
		Type: []string{oaschema.Array},
		Items: &base.DynamicValue[*base.SchemaProxy, bool]{
			A: base.CreateSchemaProxy(lineItemSchema),
		},
	}

	node := makeNode(ParamKey("line_items"), nil,
		makeNode(ParamIndex(0), nil,
			makeNode(ParamKey("price_data"), nil,
				makeNode(ParamKey("unit_amount"), []string{"945322526"}),
				makeNode(ParamKey("currency"), []string{"usd"}),
			),
			makeNode(ParamKey("quantity"), []string{"5"}),
		),
	)
	result, errs := node.Decode(schema)
	assert.Empty(t, errs)

	arr, ok := result.([]any)
	require.True(t, ok)
	require.Len(t, arr, 1)

	item, ok := arr[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, int64(5), item["quantity"])

	pd, ok := item["price_data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, int64(945322526), pd["unit_amount"])
	assert.Equal(t, "usd", pd["currency"])
}

// --- ParameterNodes.String ---

func TestParameterNodes_String_EmptyReturnsEmpty(t *testing.T) {
	var nodes ParameterNodes
	assert.Equal(t, "", nodes.String())
}

func TestParameterNodes_String_SingleNode(t *testing.T) {
	node := makeNode(ParamKey("color"), []string{"red"})
	nodes := ParameterNodes{node}
	assert.Contains(t, nodes.String(), "color")
}

// --- compareParameterNodes ---

func TestCompareParameterNodes_IndexBeforeKey(t *testing.T) {
	a := makeNode(ParamIndex(0), nil)
	b := makeNode(ParamKey("z"), nil)
	assert.Negative(t, compareParameterNodes(a, b))
}

func TestCompareParameterNodes_KeyAfterIndex(t *testing.T) {
	a := makeNode(ParamKey("a"), nil)
	b := makeNode(ParamIndex(0), nil)
	assert.Positive(t, compareParameterNodes(a, b))
}

func TestCompareParameterNodes_IndexNegativeSentinelSortsLast(t *testing.T) {
	a := makeNode(ParamIndex(-1), nil)
	b := makeNode(ParamIndex(0), nil)
	assert.Positive(t, compareParameterNodes(a, b))
}

func TestCompareParameterNodes_TwoIndexesSortedNumerically(t *testing.T) {
	a := makeNode(ParamIndex(2), nil)
	b := makeNode(ParamIndex(10), nil)
	assert.Negative(t, compareParameterNodes(a, b))
}

func TestCompareParameterNodes_TwoKeysLexicographic(t *testing.T) {
	a := makeNode(ParamKey("apple"), nil)
	b := makeNode(ParamKey("banana"), nil)
	assert.Negative(t, compareParameterNodes(a, b))
}
