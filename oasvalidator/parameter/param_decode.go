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
	"slices"
	"strconv"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/relychan/goutils"
	"github.com/relychan/goutils/httperror"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/oasvalidator"
)

// paramDecoder holds the resolved configuration and raw string values for a single parameter.
type paramDecoder struct {
	RawValues []string
	Schema    *base.Schema
}

// Decode evaluates and decodes URL parameters.
func (pd *paramDecoder) Decode( //nolint:cyclop
	types []string,
) (any, []httperror.ValidationError) {
	if pd.Schema == nil {
		return normalizeRawParamValue(pd.RawValues), nil
	}

	ts, allOf, oneOf, anyOf, isNullable := oaschema.ExtractSchemaTypes(pd.Schema)
	if len(ts) == 0 {
		if len(pd.RawValues) > 0 || isNullable {
			return normalizeRawParamValue(pd.RawValues), nil
		}

		return nil, []httperror.ValidationError{
			{
				Detail: "Field is required",
			},
		}
	}

	if len(types) == 0 {
		types = ts
	}

	result, resultType, errs := pd.decodeFromSchemaTypes(types)
	if len(errs) > 0 {
		return nil, errs
	}

	errs = oasvalidator.ValidateSchema(pd.Schema)
	if len(errs) > 0 {
		return nil, errs
	}

	for _, ao := range allOf {
		errs = oasvalidator.ValidateSchema(ao)
		if len(errs) > 0 {
			return nil, errs
		}
	}

	if len(anyOf) > 0 {
		var (
			anyOfErrs      []httperror.ValidationError
			isAnyOfSuccess bool
		)

		for _, ao := range anyOf {
			if len(ao.Type) > 0 && slices.Contains(ao.Type, resultType) {
				continue
			}

			errs := oasvalidator.ValidateSchema(ao)
			if len(errs) > 0 {
				anyOfErrs = append(anyOfErrs, errs...)

				continue
			}

			isAnyOfSuccess = true
		}

		if !isAnyOfSuccess {
			return nil, anyOfErrs
		}
	}

	if len(oneOf) > 0 {
		var (
			oneOfErrs      []httperror.ValidationError
			isOneOfSuccess bool
		)

		for _, oo := range oneOf {
			if len(oo.Type) > 0 && slices.Contains(oo.Type, resultType) {
				continue
			}

			errs := oasvalidator.ValidateSchema(oo)
			if len(errs) > 0 {
				oneOfErrs = append(oneOfErrs, errs...)

				continue
			}

			isOneOfSuccess = true

			break
		}

		if !isOneOfSuccess {
			return nil, oneOfErrs
		}
	}

	return result, nil
}

// decodeFromSchemaTypes decodes the raw values by trying each type declared in the schema.
// String is given priority: if the schema allows string the raw value is returned as-is
// to avoid lossy parsing (e.g. a numeric string "007" would become 7).
func (pd *paramDecoder) decodeFromSchemaTypes(
	types []string,
) (any, string, []httperror.ValidationError) {
	var (
		finalErrors []httperror.ValidationError
		hasObject   bool
	)

	for _, typeName := range types {
		if typeName == oaschema.Object {
			hasObject = true

			continue
		}

		result, primitiveType, errs := pd.decodeFromSchemaType(typeName)
		if len(errs) == 0 {
			return result, primitiveType, nil
		}

		finalErrors = errs
	}

	if len(finalErrors) > 0 {
		return nil, "", finalErrors
	}

	if hasObject {
		return nil, "", []httperror.ValidationError{
			{
				Detail: "Unsupported object type or nested fields in parameter",
			},
		}
	}

	return nil, "", finalErrors
}

// DecodeFromSchemaType decodes a path parameter value from a type of the schema.
// Returns the decoded value, a matched type and an error.
func (pd *paramDecoder) decodeFromSchemaType(
	typeName string,
) (any, string, []httperror.ValidationError) {
	switch typeName {
	case oaschema.Array:
		result, err := decodeArrayParam(pd.RawValues, pd.Schema)

		return result, typeName, err
	default:
		result, typeName, errs := decodePrimitiveQueryValuesFromSchemaType(
			typeName,
			pd.RawValues,
		)

		if len(errs) > 0 {
			return nil, "", errs
		}

		if typeName == "" {
			return nil, "", []httperror.ValidationError{
				{
					Detail: "Unsupported type: " + typeName,
				},
			}
		}

		return result, typeName, nil
	}
}

func decodeArrayParam(
	rawValues []string,
	schema *base.Schema,
) ([]any, []httperror.ValidationError) {
	if len(rawValues) == 0 || schema.Items == nil || schema.Items.A == nil {
		return goutils.ToAnySlice(rawValues), nil
	}

	itemSchema := schema.Items.A.Schema()
	if oaschema.IsSchemaTypeEmpty(itemSchema) {
		return goutils.ToAnySlice(rawValues), nil
	}

	return decodeArrayParamWithItemSchema(rawValues, itemSchema)
}

func decodeArrayParamWithItemSchema(
	rawValues []string,
	itemSchema *base.Schema,
) ([]any, []httperror.ValidationError) {
	results := make([]any, 0, len(rawValues))

	for i, value := range rawValues {
		decoder := paramDecoder{
			RawValues: []string{value},
			Schema:    itemSchema,
		}

		itemValue, errs := decoder.Decode(nil)
		if len(errs) > 0 {
			for j, e := range errs {
				e.Pointer = "/" + strconv.Itoa(i)
				errs[j] = e
			}

			return nil, errs
		}

		results = append(results, itemValue)
	}

	return results, nil
}
