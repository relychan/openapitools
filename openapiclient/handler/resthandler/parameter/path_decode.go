// Copyright 2026 RelyChan Pte. Ltd
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package parameter

import (
	"fmt"
	"slices"
	"strings"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/relychan/goutils"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/oasvalidator"
)

type pathParamDecoder struct {
	Name     string
	RawValue string
	Schema   *base.Schema
	Style    oaschema.ParameterEncodingStyle
	Explode  bool
}

// DecodePathValue decodes the path parameter from a string value.
// The value is encoded differently on each style, according to the [OpenAPI specification].
//
// [OpenAPI specification](https://github.com/OAI/OpenAPI-Specification/blob/3.2.0/versions/3.2.0.md#style-examples)
func DecodePathValue(definition *highv3.Parameter, value string) (any, *goutils.ErrorDetail) {
	if value == "" {
		return nil, &goutils.ErrorDetail{
			Code:      oaschema.ErrCodeInvalidURLParam,
			Detail:    "URL parameter is required",
			Parameter: definition.Name,
		}
	}

	if definition == nil || definition.Schema == nil {
		return value, nil
	}

	style, explode, err := getParamStyleAndExplodeFromRawStyle(
		oaschema.InPath,
		definition.Style,
		definition.Explode,
	)
	if err != nil {
		return nil, &goutils.ErrorDetail{
			Code:      oaschema.ErrCodeInvalidURLParam,
			Detail:    err.Error(),
			Parameter: definition.Name,
		}
	}

	decoder := &pathParamDecoder{
		Name:     definition.Name,
		Style:    style,
		Explode:  explode,
		RawValue: value,
		Schema:   definition.Schema.Schema(),
	}

	return decoder.Decode()
}

// Decode evaluates and decodes URL parameters.
func (ppe *pathParamDecoder) Decode() (any, *goutils.ErrorDetail) {
	result, _, err := ppe.DecodeFromSchemaTypes()

	return result, err
}

// DecodeFromSchemaTypes decode a path parameter value from types of schema.
// Returns the decoded value, a matched type and an error.
// Prefer string if exists.
func (ppe *pathParamDecoder) DecodeFromSchemaTypes() (any, string, *goutils.ErrorDetail) {
	// remove the symbol prefix from raw value string
	switch ppe.Style {
	case oaschema.EncodingStyleLabel:
		if ppe.RawValue[0] != oaschema.Dot[0] {
			return nil, "", &goutils.ErrorDetail{
				Code:      oaschema.ErrCodeInvalidURLParam,
				Detail:    "The label style of parameter value must start with a dot",
				Parameter: ppe.Name,
			}
		}

		ppe.RawValue = ppe.RawValue[1:]
	case oaschema.EncodingStyleMatrix:
		if ppe.RawValue[0] != oaschema.SemiColon[0] {
			return nil, "", &goutils.ErrorDetail{
				Code:      oaschema.ErrCodeInvalidURLParam,
				Detail:    "The matrix style of parameter value must start with a semicolon",
				Parameter: ppe.Name,
			}
		}

		ppe.RawValue = ppe.RawValue[1:]
	default:
	}

	if slices.Contains(ppe.Schema.Type, oaschema.String) {
		return ppe.RawValue, oaschema.String, nil
	}

	var finalError *goutils.ErrorDetail

	for _, typeName := range ppe.Schema.Type {
		if typeName == "" {
			continue
		}

		result, primitiveType, err := ppe.DecodeFromSchemaType(typeName)
		if err == nil {
			return result, primitiveType, nil
		}

		finalError = &goutils.ErrorDetail{
			Code:      oaschema.ErrCodeInvalidURLParam,
			Detail:    err.Error(),
			Parameter: ppe.Name,
		}
	}

	return nil, "", finalError
}

// DecodeFromSchemaType decodes a path parameter value from a type of the schema.
// Returns the decoded value, a matched type and an error.
func (ppe *pathParamDecoder) DecodeFromSchemaType(
	typeName string,
) (any, string, *goutils.ErrorDetail) {
	result, primitiveType, err := oasvalidator.DecodePrimitiveValueFromType(
		ppe.RawValue,
		typeName,
	)
	if err != nil {
		return nil, "", &goutils.ErrorDetail{
			Code:      oaschema.ErrCodeInvalidURLParam,
			Detail:    err.Error(),
			Parameter: ppe.Name,
		}
	}

	if primitiveType != "" {
		return result, primitiveType, nil
	}

	switch typeName {
	case oaschema.Array:
		result, err := ppe.DecodeFromArray()

		return result, typeName, err
	case oaschema.Object:
		result, err := ppe.DecodeFromObject()

		return result, typeName, err
	default:
		return ppe.RawValue, typeName, nil
	}
}

func (ppe *pathParamDecoder) DecodeFromArray() ([]any, *goutils.ErrorDetail) {
	strValues, err := ppe.SplitArrayFromString()
	if err != nil {
		return nil, err
	}

	valueLength := int64(len(strValues))
	// array length validations
	if ppe.Schema.MaxItems != nil && valueLength > *ppe.Schema.MaxItems {
		return nil, oasvalidator.InvalidParamArrayMaxItemsError(
			ppe.Name,
			oaschema.ErrCodeInvalidURLParam,
			*ppe.Schema.MaxItems,
			valueLength,
		)
	}

	if ppe.Schema.MinItems != nil && valueLength < *ppe.Schema.MinItems {
		return nil, oasvalidator.InvalidParamArrayMinItemsError(
			ppe.Name,
			oaschema.ErrCodeInvalidURLParam,
			*ppe.Schema.MinItems,
			valueLength,
		)
	}

	if len(strValues) == 0 || ppe.Schema.Items.A == nil {
		return []any{}, nil
	}

	itemSchema := ppe.Schema.Items.A.Schema()
	if oaschema.IsSchemaEmpty(itemSchema) {
		return goutils.ToAnySlice(strValues), nil
	}

	results := make([]any, len(strValues))

	for i, value := range strValues {
		itemValue, _, err := ppe.DecodeItemValueFromSchemaTypes(itemSchema, value)
		if err != nil {
			return nil, err
		}

		results[i] = itemValue
	}

	return results, nil
}

func (ppe *pathParamDecoder) DecodeFromObject() (map[string]any, *goutils.ErrorDetail) {
	values, err := ppe.SplitObjectFromString()
	if err != nil {
		return nil, err
	}

	if len(values) == 0 || ppe.Schema.Properties == nil || ppe.Schema.Properties.Len() == 0 {
		return values, nil
	}

	for iter := ppe.Schema.Properties.First(); iter != nil; iter = iter.Next() {
		key := iter.Key()

		propSchemaProxy := iter.Value()
		if propSchemaProxy == nil {
			continue
		}

		propSchema := propSchemaProxy.Schema()
		if propSchema == nil {
			continue
		}

		value, ok := values[key]
		if !ok {
			continue
		}

		parsedValue, _, err := ppe.DecodeItemValueFromSchemaTypes(propSchema, value)
		if err != nil {
			return nil, err
		}

		values[key] = parsedValue
	}

	return values, nil
}

func (ppe *pathParamDecoder) SplitArrayFromString() ([]string, *goutils.ErrorDetail) {
	switch ppe.Style {
	case oaschema.EncodingStyleLabel:
		if ppe.RawValue == "" {
			return []string{}, nil
		}

		// /users/.3.4.5
		// /users/.role=admin.firstName=Alex
		if ppe.Explode {
			return strings.Split(ppe.RawValue, oaschema.Dot), nil
		}

		// /users/.3,4,5
		// /users/.role,admin,firstName,Alex
		return strings.Split(ppe.RawValue, oaschema.Comma), nil
	case oaschema.EncodingStyleMatrix:
		prefix := ppe.Name + oaschema.Equals
		// /users/;id=3;id=4;id=5
		// /users/;role=admin;firstName=Alex
		if ppe.Explode {
			parts := strings.Split(ppe.RawValue, oaschema.SemiColon)
			results := make([]string, len(parts))

			for i, part := range parts {
				value, found := strings.CutPrefix(part, prefix)
				if !found {
					return nil, &goutils.ErrorDetail{
						Code:      oaschema.ErrCodeInvalidURLParam,
						Detail:    "Invalid matrix style in parameter value. The array value must follow this format: ;key1=value1;key2=value2",
						Parameter: ppe.Name,
					}
				}

				results[i] = value
			}

			return results, nil
		}

		// /users/;id=3,4,5
		// /users/;id=role,admin,firstName,Alex
		value, found := strings.CutPrefix(ppe.RawValue, prefix)
		if !found {
			return nil, &goutils.ErrorDetail{
				Code:      oaschema.ErrCodeInvalidURLParam,
				Detail:    "Invalid matrix style in parameter value. The array value must follow this format: ;key1=value1,value2",
				Parameter: ppe.Name,
			}
		}

		return strings.Split(value, oaschema.Comma), nil
	default:
		// encode with the simple style
		return strings.Split(ppe.RawValue, oaschema.Comma), nil
	}
}

func (ppe *pathParamDecoder) SplitObjectFromString() (map[string]any, *goutils.ErrorDetail) {
	switch ppe.Style {
	case oaschema.EncodingStyleLabel:
		if ppe.RawValue == "" {
			return map[string]any{}, nil
		}

		// /users/.role=admin.firstName=Alex
		if ppe.Explode {
			return ppe.parseExplodeObject(ppe.RawValue, oaschema.Dot)
		}

		// /users/.role,admin,firstName,Alex
		return ppe.parseNonExplodeObject(ppe.RawValue, oaschema.Comma)
	case oaschema.EncodingStyleMatrix:
		if ppe.RawValue == "" {
			return map[string]any{}, nil
		}

		// /users/;role=admin;firstName=Alex
		if ppe.Explode {
			return ppe.parseExplodeObject(ppe.RawValue, oaschema.SemiColon)
		}

		// /users/;id=role,admin,firstName,Alex
		value, found := strings.CutPrefix(ppe.RawValue, ppe.Name+oaschema.Equals)
		if !found {
			return nil, ppe.newInvalidObjectError()
		}

		return ppe.parseNonExplodeObject(value, oaschema.Comma)
	default:
		// /users/role=admin,firstName=Alex
		if ppe.Explode {
			return ppe.parseExplodeObject(ppe.RawValue, oaschema.Comma)
		}

		// /users/role,admin,firstName,Alex
		return ppe.parseNonExplodeObject(ppe.RawValue, oaschema.Comma)
	}
}

// DecodeItemValueFromSchemaTypes decode a path parameter value from types of schema.
// Returns the decoded value, a matched type and an error.
// Prefer string if exists.
func (ppe *pathParamDecoder) DecodeItemValueFromSchemaTypes(
	itemSchema *base.Schema,
	value any,
) (any, string, *goutils.ErrorDetail) {
	if len(itemSchema.Type) == 0 {
		return value, "", nil
	}

	if slices.Contains(itemSchema.Type, oaschema.String) {
		return value, oaschema.String, nil
	}

	var finalError *goutils.ErrorDetail

	for _, typeName := range ppe.Schema.Type {
		if typeName == "" {
			continue
		}

		result, primitiveType, err := oasvalidator.DecodePrimitiveValueFromType(
			ppe.RawValue,
			typeName,
		)
		if err != nil {
			finalError = &goutils.ErrorDetail{
				Code:      oaschema.ErrCodeInvalidURLParam,
				Detail:    err.Error(),
				Parameter: ppe.Name,
			}
		} else if primitiveType != "" {
			return result, primitiveType, nil
		}
	}

	if finalError != nil {
		return nil, "", finalError
	}

	return nil, "", &goutils.ErrorDetail{
		Code: oaschema.ErrCodeInvalidURLParam,
		Detail: fmt.Sprintf(
			"Unsupported types or nested fields in URL path parameter: %v",
			itemSchema.Type,
		),
		Parameter: ppe.Name,
	}
}

func (ppe *pathParamDecoder) parseExplodeObject(
	rawValue string,
	separator string,
) (map[string]any, *goutils.ErrorDetail) {
	result := make(map[string]any)

	for part := range strings.SplitSeq(rawValue, separator) {
		key, value, found := strings.Cut(part, oaschema.Equals)
		if !found || key == "" {
			return nil, ppe.newInvalidObjectError()
		}

		result[key] = value
	}

	return result, nil
}

func (ppe *pathParamDecoder) parseNonExplodeObject(
	rawValue string,
	separator string,
) (map[string]any, *goutils.ErrorDetail) {
	result := make(map[string]any)

	parts := strings.Split(rawValue, separator)
	if len(parts)%2 != 0 {
		return nil, ppe.newInvalidObjectError()
	}

	for i := 0; i < len(parts); i += 2 {
		if parts[i] == "" {
			return nil, ppe.newInvalidObjectError()
		}

		result[parts[i]] = parts[i+1]
	}

	return result, nil
}

func (ppe *pathParamDecoder) newInvalidObjectError() *goutils.ErrorDetail {
	message := "Invalid syntax for simple style in parameter value. The object value must follow this format: key1,value1,key2,value2"

	switch ppe.Style {
	case oaschema.EncodingStyleLabel:
		if ppe.Explode {
			message = "Invalid syntax for exploded label style in parameter value. The object value must follow this format: .key1=value1.key2=value2"
		} else {
			message = "Invalid syntax for non-exploded label style in parameter value. The object value must follow this format: .key1,value1,key2,value2"
		}
	case oaschema.EncodingStyleMatrix:
		if ppe.Explode {
			message = "Invalid syntax for exploded matrix style in parameter value. The object value must follow this format: ;key1=value1;key2=value2"
		} else {
			message = "Invalid syntax for non-exploded matrix style in parameter value. The object value must follow this format: ;id=key1,value1,key2,value2"
		}
	default:
		if ppe.Explode {
			message = "Invalid syntax for simple style in parameter value. The object value must follow this format: role=admin,firstName=Alex"
		}
	}

	return &goutils.ErrorDetail{
		Code:      oaschema.ErrCodeInvalidURLParam,
		Detail:    message,
		Parameter: ppe.Name,
	}
}
