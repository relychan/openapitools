package oasvalidator

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildTestSchemas builds the component schemas of an OpenAPI 3.1 document from YAML.
// The schemas input is indented under components.schemas.
func buildTestSchemas(t *testing.T, schemas string) map[string]*base.Schema {
	t.Helper()

	spec := "openapi: 3.1.0\ninfo:\n  title: test\n  version: 1.0.0\npaths: {}\ncomponents:\n  schemas:\n" + schemas

	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)

	model, err := doc.BuildV3Model()
	require.NoError(t, err)

	results := map[string]*base.Schema{}

	for iter := model.Model.Components.Schemas.First(); iter != nil; iter = iter.Next() {
		results[iter.Key()] = iter.Value().Schema()
	}

	return results
}

func buildTestSchema(t *testing.T, schema string) *base.Schema {
	t.Helper()

	lines := strings.Split(strings.TrimSpace(schema), "\n")
	for i, line := range lines {
		lines[i] = "      " + line
	}

	return buildTestSchemas(t, "    Test:\n"+strings.Join(lines, "\n"))["Test"]
}

func decodeJSON(t *testing.T, raw string) any {
	t.Helper()

	var result any

	require.NoError(t, json.Unmarshal([]byte(raw), &result))

	return result
}

func TestValidateValue_JSONIntegers(t *testing.T) {
	schema := buildTestSchema(t, `
type: object
properties:
  count:
    type: integer
    minimum: 1
    maximum: 10
    multipleOf: 2
  status:
    type: integer
    enum: [1, 2]
  price:
    type: number`)

	assert.Empty(t, ValidateValue(schema, decodeJSON(t, `{"count": 4, "status": 2, "price": 3}`)))
	// Go integers are valid numbers.
	assert.Empty(t, ValidateValue(schema, map[string]any{"price": 3}))

	errs := ValidateValue(schema, decodeJSON(t, `{"count": 4.5}`))
	require.Len(t, errs, 1)
	assert.Equal(t, "/count", errs[0].Pointer)
	assert.Contains(t, errs[0].Detail, "Invalid type")

	// integer rules still apply to JSON floats.
	errs = ValidateValue(schema, decodeJSON(t, `{"count": 12}`))
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Detail, "12")

	assert.Len(t, ValidateValue(schema, decodeJSON(t, `{"count": 3}`)), 1)
	assert.Len(t, ValidateValue(schema, decodeJSON(t, `{"status": 3}`)), 1)
}

func TestValidateValue_ExclusiveMinMaxNumeric(t *testing.T) {
	schema := buildTestSchema(t, `
type: number
exclusiveMinimum: 0
exclusiveMaximum: 10`)

	assert.Empty(t, ValidateValue(schema, 5.0))
	assert.Len(t, ValidateValue(schema, 0.0), 1)
	assert.Len(t, ValidateValue(schema, -1.0), 1)
	assert.Len(t, ValidateValue(schema, 10.0), 1)

	// both bounds are checked independently.
	minOnly := buildTestSchema(t, `
type: number
exclusiveMinimum: 1
maximum: 10`)

	assert.Len(t, ValidateValue(minOnly, 1.0), 1)
	assert.Len(t, ValidateValue(minOnly, 11.0), 1)
	assert.Empty(t, ValidateValue(minOnly, 10.0))
}

func TestValidateValue_AdditionalPropertiesFalse(t *testing.T) {
	schema := buildTestSchema(t, `
type: object
additionalProperties: false
properties:
  name:
    type: string
patternProperties:
  "^x-":
    type: string`)

	assert.Empty(t, ValidateValue(schema, map[string]any{"name": "a", "x-foo": "b"}))

	errs := ValidateValue(schema, map[string]any{"name": "a", "extra": 1})
	require.Len(t, errs, 1)
	assert.Equal(t, "/extra", errs[0].Pointer)

	// pattern properties are still validated.
	assert.Len(t, ValidateValue(schema, map[string]any{"x-foo": 1}), 1)

	// nil additionalProperties allows extra keys.
	open := buildTestSchema(t, `
type: object
properties:
  name:
    type: string`)
	assert.Empty(t, ValidateValue(open, map[string]any{"name": "a", "extra": 1}))

	// additionalProperties schema applies only to keys not matching patternProperties.
	typed := buildTestSchema(t, `
type: object
additionalProperties:
  type: integer
patternProperties:
  "^s_":
    type: string`)
	assert.Empty(t, ValidateValue(typed, map[string]any{"s_name": "a", "count": 1}))
	assert.Len(t, ValidateValue(typed, map[string]any{"count": "a"}), 1)
}

func TestValidateSchema_AllOfMergesMaps(t *testing.T) {
	schemas := buildTestSchemas(t, `
    Base:
      type: object
      properties:
        id:
          type: string
      dependentRequired:
        id: [name]
    Other:
      type: object
      properties:
        name:
          type: string
    Test:
      allOf:
        - $ref: '#/components/schemas/Base'
        - $ref: '#/components/schemas/Other'`)

	schema := schemas["Test"]
	require.Empty(t, ValidateSchema(schema))
	require.NotNil(t, schema.Properties)

	_, hasID := schema.Properties.Get("id")
	_, hasName := schema.Properties.Get("name")
	assert.True(t, hasID)
	assert.True(t, hasName)
	require.NotNil(t, schema.DependentRequired)
	assert.Equal(t, 1, schema.DependentRequired.Len())

	// The referenced component schemas must not be mutated by the merge.
	assert.Equal(t, 1, schemas["Base"].Properties.Len())
	assert.Equal(t, 1, schemas["Other"].Properties.Len())

	assert.Len(t, ValidateValue(schema, map[string]any{"id": 1, "name": "a"}), 1)
	assert.Len(t, ValidateValue(schema, map[string]any{"id": "a"}), 1)
}

func TestValidateValue_OneOfExactlyOne(t *testing.T) {
	schema := buildTestSchema(t, `
oneOf:
  - type: number
    minimum: 0
  - type: number
    maximum: 10`)

	assert.Empty(t, ValidateValue(schema, -5.0))
	assert.Empty(t, ValidateValue(schema, 15.0))

	errs := ValidateValue(schema, 5.0)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Detail, "exactly one")
}

func TestValidateString_RuneLength(t *testing.T) {
	schema := buildTestSchema(t, `
type: string
minLength: 2
maxLength: 3`)

	assert.Empty(t, ValidateValue(schema, "日本語"))
	assert.Empty(t, ValidateValue(schema, "éé"))
	assert.Len(t, ValidateValue(schema, "日本語日"), 1)
	assert.Len(t, ValidateValue(schema, "日"), 1)
}

func TestValidateNumber_MultipleOfFloat(t *testing.T) {
	schema := buildTestSchema(t, `
type: number
multipleOf: 0.1`)

	for _, value := range []float64{0.3, 0.7, 1.1, 19.99 * 10, 123456.7} {
		assert.Empty(t, ValidateValue(schema, value), value)
	}

	assert.Len(t, ValidateValue(schema, 0.35), 1)
	assert.Len(t, ValidateValue(buildTestSchema(t, "type: integer\nmultipleOf: 3"), 7), 1)
}

func TestValidateValue_Null(t *testing.T) {
	schema := buildTestSchema(t, `
type: object
required: [name]
properties:
  name:
    type: string
  nickname:
    type: string
  note:
    type: [string, "null"]
  any: {}`)

	// absent optional properties are fine.
	assert.Empty(t, ValidateValue(schema, decodeJSON(t, `{"name": "a"}`)))
	assert.Empty(t, ValidateValue(schema, decodeJSON(t, `{"name": "a", "note": null, "any": null}`)))

	errs := ValidateValue(schema, decodeJSON(t, `{"name": "a", "nickname": null}`))
	require.Len(t, errs, 1)
	assert.Equal(t, "/nickname", errs[0].Pointer)

	assert.Len(t, ValidateValue(schema, nil), 1)
	assert.Len(t, ValidateValue(buildTestSchema(t, "type: integer"), (*int)(nil)), 1)

	nullable := &base.Schema{Type: []string{"string"}, Nullable: new(true)}
	assert.Empty(t, ValidateValue(nullable, nil))
	assert.Empty(t, ValidateValue(nullable, (*string)(nil)))
	assert.Empty(t, ValidateNullableBoolean(&base.Schema{Type: []string{"boolean"}, Nullable: new(true)}, nil))

	items := buildTestSchema(t, "type: array\nitems:\n  type: string")
	assert.Len(t, ValidateValue(items, decodeJSON(t, `["a", null]`)), 1)
}

func TestValidateValue_DependentRequiredConditional(t *testing.T) {
	schema := buildTestSchema(t, `
type: object
dependentRequired:
  credit_card: [billing_address]`)

	assert.Empty(t, ValidateValue(schema, map[string]any{"name": "a"}))
	assert.Empty(t, ValidateValue(schema, map[string]any{"credit_card": "1", "billing_address": "x"}))
	assert.Len(t, ValidateValue(schema, map[string]any{"credit_card": "1"}), 1)

	// reflection path
	assert.Empty(t, ValidateValue(schema, map[string]int{"name": 1}))
	assert.Len(t, ValidateValue(schema, map[string]int{"credit_card": 1}), 1)
}

func TestValidateValue_UniqueItemsAny(t *testing.T) {
	schema := buildTestSchema(t, "type: array\nuniqueItems: true")

	assert.Empty(t, ValidateValue(schema, decodeJSON(t, `[1, "1", {"a": 1}, {"a": 2}]`)))
	assert.Len(t, ValidateValue(schema, decodeJSON(t, `[{"a": 1, "b": 2}, {"b": 2, "a": 1}]`)), 1)
	assert.Len(t, ValidateValue(schema, []any{1, 1.0}), 1)
	assert.Len(t, ValidateValue(schema, []any{"x", "x"}), 1)
}

func TestCompareNullableBoolean(t *testing.T) {
	assert.Equal(t, 0, CompareNullableBoolean(new(true), new(true)))
	assert.Equal(t, 0, CompareNullableBoolean(new(false), new(false)))
	assert.Equal(t, 1, CompareNullableBoolean(new(true), new(false)))
	assert.Equal(t, -1, CompareNullableBoolean(new(false), new(true)))
	assert.Equal(t, -1, CompareNullableBoolean(nil, new(false)))
	assert.Equal(t, 1, CompareNullableBoolean(new(false), nil))
	assert.Equal(t, 0, CompareNullableBoolean(nil, nil))

	schema := &base.Schema{Type: []string{"array"}, UniqueItems: new(true)}
	assert.Len(t, ValidateValue(schema, []*bool{new(true), new(true)}), 1)
	assert.Empty(t, ValidateValue(schema, []*bool{new(true), new(false), nil}))
}

func TestValidateContains_MaxContains(t *testing.T) {
	schema := buildTestSchema(t, `
type: array
contains:
  type: string
maxContains: 1`)

	// minContains is absent: must not panic and defaults to 1.
	err := ValidateContains(schema, []any{"a", "b"})
	require.NotNil(t, err)
	assert.Contains(t, err.Detail, "maximum 1")

	assert.Nil(t, ValidateContains(schema, []any{"a", 1}))
	assert.NotNil(t, ValidateContains(schema, []any{1}))
}

func TestValidateContentType_FormAndOctetStream(t *testing.T) {
	for _, contentType := range []string{"application/x-www-form-urlencoded", "application/octet-stream"} {
		result, err := ValidateContentType(contentType)
		require.NoError(t, err)
		assert.Equal(t, contentType, result)
	}

	result, err := ValidateContentType("application/octet-stream, application/json")
	require.NoError(t, err)
	assert.Equal(t, "application/json", result)

	_, err = ValidateContentType("image/png")
	require.ErrorIs(t, err, ErrInvalidContentType)

	content := orderedmap.New[string, *highv3.MediaType]()
	content.Set("application/x-www-form-urlencoded", &highv3.MediaType{})

	op, errs := ValidateOperation(nil, &highv3.Operation{
		RequestBody: &highv3.RequestBody{Content: content},
	}, nil)
	require.Empty(t, errs)
	assert.Equal(t, "application/x-www-form-urlencoded", op.RequestBody.Content.ContentType)
}

func TestValidateResponseContentType_Normalized(t *testing.T) {
	testCases := map[string]string{
		"Image/PNG":                       "image/png",
		"Application/PDF; Version=1.7":    "application/pdf; version=1.7",
		"Application/JSON; Charset=UTF-8": "application/json; charset=UTF-8",
		"*/*":                             "*/*",
	}

	for input, expected := range testCases {
		result, err := ValidateResponseContentType(input)
		require.NoError(t, err, input)
		assert.Equal(t, expected, result, input)
	}

	content := orderedmap.New[string, *highv3.MediaType]()
	content.Set("Image/PNG", &highv3.MediaType{})

	codes := orderedmap.New[string, *highv3.Response]()
	codes.Set("200", &highv3.Response{Content: content})

	op, errs := ValidateOperation(nil, &highv3.Operation{
		Responses: &highv3.Responses{Codes: codes},
	}, nil)
	require.Empty(t, errs)

	_, ok := op.Responses["image/png"]
	assert.True(t, ok)
}

func TestValidateSchema_AllOfEnumIntersection(t *testing.T) {
	schema := buildTestSchema(t, `
type: string
enum: [a, b, c]
allOf:
  - enum: [b, c, d]`)

	require.Empty(t, ValidateSchema(schema))
	require.Len(t, schema.Enum, 2)
	assert.Equal(t, "b", schema.Enum[0].Value)
	assert.Equal(t, "c", schema.Enum[1].Value)
	assert.Len(t, ValidateValue(schema, "a"), 1)
	assert.Empty(t, ValidateValue(schema, "b"))

	disjoint := buildTestSchema(t, `
type: string
enum: [a]
allOf:
  - enum: [b]`)
	assert.NotEmpty(t, ValidateSchema(disjoint))
}
