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
	"cmp"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/relychan/goutils"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/oasvalidator"
	"github.com/relychan/openapitools/oasvalidator/regexps"
)

type queryParamDecoder struct {
	Name      string
	RawValues []string
	Schema    *base.Schema
	Style     oaschema.ParameterEncodingStyle
	Explode   bool
}

// DecodeQueryFromParameters decodes the query parameters from string values.
// The value is encoded differently on each style, according to the [OpenAPI specification].
//
// [OpenAPI specification](https://github.com/OAI/OpenAPI-Specification/blob/3.2.0/versions/3.2.0.md#style-examples)
func DecodeQueryFromParameters(
	definitions []*highv3.Parameter,
	values map[string][]string,
) (map[string]any, []goutils.ErrorDetail) {
	if len(definitions) == 0 {
		return goutils.ToAnyMap(values), nil
	}

	var (
		results = make(map[string]any)
		errs    []goutils.ErrorDetail
	)

	var deepObjectParams []*highv3.Parameter

	for _, definition := range definitions {
		if definition.In != oaschema.InQuery.String() {
			continue
		}

		style, explode, styleErr := getParamStyleAndExplodeFromRawStyle(
			oaschema.InQuery,
			definition.Style,
			definition.Explode,
		)
		if styleErr != nil {
			return nil, []goutils.ErrorDetail{
				{
					Code:      oasvalidator.ErrCodeInvalidQueryParam,
					Detail:    styleErr.Error(),
					Parameter: definition.Name,
				},
			}
		}

		if style == oaschema.EncodingStyleDeepObject {
			deepObjectParams = append(deepObjectParams, definition)

			continue
		}

		var paramSchema *base.Schema

		if definition.Schema != nil {
			paramSchema = definition.Schema.Schema()
		}

		decoder := &queryParamDecoder{
			Name:    definition.Name,
			Style:   style,
			Explode: explode,
			Schema:  paramSchema,
		}

		// Properties in exploded object are flatten.
		// Because the schema can not have enough information, this parameter should be optional.
		if explode && paramSchema != nil && slices.Contains(paramSchema.Type, oaschema.Object) {
			itemResults, decodeErr := decoder.decodeExplodeObject(values)
			if len(decodeErr) > 0 {
				errs = append(errs, decodeErr...)
			} else {
				results[definition.Name] = itemResults
			}

			continue
		}

		rawValues, present := values[definition.Name]
		if !present {
			if definition.Required != nil && *definition.Required {
				err := oasvalidator.ParameterRequiredError(definition.Name)
				err.Code = oasvalidator.ErrCodeInvalidQueryParam
				errs = append(errs, *err)
			}

			continue
		}

		decoder.RawValues = rawValues

		if oaschema.IsSchemaEmpty(paramSchema) {
			results[definition.Name] = decoder.splitArrayFromString()

			continue
		}

		itemResults, decodeErr := decoder.Decode()
		if len(decodeErr) > 0 {
			errs = append(errs, decodeErr...)
		} else {
			results[definition.Name] = itemResults
		}
	}

	if len(deepObjectParams) > 0 {
		deErrs := decodeQueryDeepObjectFromParameters(deepObjectParams, values, results)
		if len(deErrs) > 0 {
			errs = append(errs, deErrs...)
		}
	}

	if len(errs) > 0 {
		return nil, errs
	}

	if len(results) == 0 {
		results = goutils.ToAnyMap(values)
	}

	return results, nil
}

// Decode evaluates and decodes URL parameters.
func (qpe *queryParamDecoder) Decode() (any, []goutils.ErrorDetail) {
	result, _, errs := qpe.decodeFromSchemaTypes()

	return result, errs
}

// DecodeFromSchemaTypes decode a path parameter value from types of schema.
// Returns the decoded value, a matched type and an error.
// Prefer string if exists.
func (qpe *queryParamDecoder) decodeFromSchemaTypes() (any, string, []goutils.ErrorDetail) {
	if len(qpe.RawValues) == 0 {
		return nil, "", nil
	}

	if slices.Contains(qpe.Schema.Type, oaschema.String) {
		var result string

		for _, value := range qpe.RawValues {
			if value != "" {
				result = value

				break
			}
		}

		return result, oaschema.String, nil
	}

	var finalErrors []goutils.ErrorDetail

	for _, typeName := range qpe.Schema.Type {
		if typeName == "" || typeName == oaschema.Null {
			continue
		}

		result, primitiveType, errs := qpe.decodeFromSchemaType(typeName)
		if len(errs) == 0 {
			return result, primitiveType, nil
		}

		finalErrors = errs
	}

	return nil, "", finalErrors
}

// DecodeFromSchemaType decodes a path parameter value from a type of the schema.
// Returns the decoded value, a matched type and an error.
func (qpe *queryParamDecoder) decodeFromSchemaType(
	typeName string,
) (any, string, []goutils.ErrorDetail) {
	switch typeName {
	case oaschema.Array:
		result, err := qpe.decodeFromArray()

		return result, typeName, err
	case oaschema.Object:
		result, err := qpe.decodeFromObject()

		return result, typeName, err
	default:
		result, resultType, errs := decodePrimitiveQueryValuesFromSchemaType(
			typeName,
			qpe.RawValues,
		)
		for i, err := range errs {
			err.Parameter = qpe.Name
			errs[i] = err
		}

		return result, resultType, errs
	}
}

func (qpe *queryParamDecoder) decodeFromArray() ([]any, []goutils.ErrorDetail) {
	strValues := qpe.splitArrayFromString()

	errFuncs := oasvalidator.ValidateArray(qpe.Schema, strValues, cmp.Compare)
	errs := oasvalidator.CollectErrorsFunc(errFuncs, func(ed *goutils.ErrorDetail) {
		ed.Code = oasvalidator.ErrCodeInvalidQueryParam
		ed.Parameter = qpe.Name
	})

	if len(strValues) == 0 || qpe.Schema.Items == nil || qpe.Schema.Items.A == nil {
		return goutils.ToAnySlice(strValues), errs
	}

	itemSchema := qpe.Schema.Items.A.Schema()
	if oaschema.IsSchemaEmpty(itemSchema) {
		return goutils.ToAnySlice(strValues), errs
	}

	results := make([]any, len(strValues))

	for i, value := range strValues {
		itemValue, _, err := qpe.decodeItemValueFromSchemaTypes(itemSchema, value)
		if err != nil {
			errs = append(errs, *err)

			return nil, errs
		}

		results[i] = itemValue
	}

	return results, errs
}

func (qpe *queryParamDecoder) decodeFromObject() (map[string]any, []goutils.ErrorDetail) {
	rawValues, err := qpe.splitObjectFromString()
	if err != nil {
		return nil, []goutils.ErrorDetail{*err}
	}

	errFuncs := oasvalidator.ValidateObject(qpe.Schema, rawValues)
	if len(errFuncs) > 0 {
		return nil, oasvalidator.CollectErrors(errFuncs)
	}

	var (
		results = make(map[string]any)
		errs    []goutils.ErrorDetail
	)

	if qpe.Schema.Properties != nil {
		for iter := qpe.Schema.Properties.First(); iter != nil; iter = iter.Next() {
			key := iter.Key()

			value, ok := rawValues[key]
			if !ok {
				continue
			}

			propSchemaProxy := iter.Value()
			if propSchemaProxy == nil {
				results[key] = value

				continue
			}

			propSchema := propSchemaProxy.Schema()
			if oaschema.IsSchemaEmpty(propSchema) {
				results[key] = value

				continue
			}

			propDecoder := &queryParamDecoder{
				Name:      key,
				Style:     qpe.Style,
				Explode:   qpe.Explode,
				RawValues: value,
				Schema:    propSchema,
			}

			propValue, decodeErrs := propDecoder.Decode()
			if len(decodeErrs) == 0 {
				results[key] = propValue

				continue
			}

			errs = append(errs, decodeErrs...)
		}
	}

	if len(errs) > 0 {
		return nil, errs
	}

	errs = qpe.decodeObjectAdditionalProperties(rawValues, results, nil)
	if len(errs) > 0 {
		return nil, errs
	}

	errs = qpe.decodeObjectPatternProperties(rawValues, results)
	if len(errs) > 0 {
		return nil, errs
	}

	if len(results) == 0 {
		// fallback to return all raw values.
		for key, values := range rawValues {
			results[key] = getValue(values)
		}
	}

	return results, nil
}

func (qpe *queryParamDecoder) splitArrayFromString() []string {
	if qpe.Explode || len(qpe.RawValues) == 0 {
		// The format of array queries is the same for all style if explode=true
		// The url library already parsed the query values. Therefore, no action here.
		// /users?id=3&id=4&id=5
		return qpe.RawValues
	}

	switch qpe.Style {
	case oaschema.EncodingStyleSpaceDelimited:
		// /users?id=3 4 5
		return qpe.parseDelimitedStyle(oaschema.Space)
	case oaschema.EncodingStylePipeDelimited:
		// /users?id=3|4|5
		return qpe.parseDelimitedStyle(oaschema.Pipe)
	default:
		// /users?id=3,4,5
		return qpe.parseDelimitedStyle(oaschema.Comma)
	}
}

// Set delimited-separated array values for array params.
// For example: /users?id=3|4|5.
func (qpe *queryParamDecoder) parseDelimitedStyle(separator string) []string {
	results := make([]string, 0, len(qpe.RawValues))

	for _, value := range qpe.RawValues {
		if value == "" {
			continue
		}

		items := strings.Split(value, separator)

		if len(results) == 0 {
			results = items
		} else {
			results = append(results, items...)
		}
	}

	return slices.Clip(results)
}

func (qpe *queryParamDecoder) splitObjectFromString() (map[string][]string, *goutils.ErrorDetail) {
	switch qpe.Style {
	case oaschema.EncodingStyleSpaceDelimited:
		// color=R%20100%20G%20200%20B%20150
		return qpe.parseNonExplodeObject(oaschema.Space)
	case oaschema.EncodingStylePipeDelimited:
		// color=R|100|G|200|B|150
		return qpe.parseNonExplodeObject(oaschema.Pipe)
	default:
		// color=blue,black,brown
		return qpe.parseNonExplodeObject(oaschema.Comma)
	}
}

// DecodeItemValueFromSchemaTypes decode a path parameter value from types of schema.
// Returns the decoded value, a matched type and an error.
// Prefer string if exists.
func (qpe *queryParamDecoder) decodeItemValueFromSchemaTypes(
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

	for _, typeName := range itemSchema.Type {
		if typeName == "" {
			continue
		}

		result, primitiveType, err := oasvalidator.DecodePrimitiveValueFromType(
			value,
			typeName,
		)
		if err != nil {
			finalError = &goutils.ErrorDetail{
				Code:      oasvalidator.ErrCodeInvalidQueryParam,
				Detail:    err.Error(),
				Parameter: qpe.Name,
			}
		} else if primitiveType != "" {
			return result, primitiveType, nil
		}
	}

	if finalError != nil {
		return nil, "", finalError
	}

	return nil, "", &goutils.ErrorDetail{
		Code: oasvalidator.ErrCodeInvalidQueryParam,
		Detail: fmt.Sprintf(
			"Unsupported types or nested fields in URL query parameter: %v",
			itemSchema.Type,
		),
		Parameter: qpe.Name,
	}
}

func (qpe *queryParamDecoder) decodeExplodeObject(
	queryValues map[string][]string,
) (map[string]any, []goutils.ErrorDetail) {
	// /users?role=admin&firstName=Alex
	var (
		result     = make(map[string]any)
		parsedKeys = make([]string, 0, len(queryValues))
		errs       []goutils.ErrorDetail
	)

	if qpe.Schema.Properties != nil {
		for iter := qpe.Schema.Properties.First(); iter != nil; iter = iter.Next() {
			key := iter.Key()

			rawValues, present := queryValues[key]
			if !present {
				if len(qpe.Schema.Required) > 0 && slices.Contains(qpe.Schema.Required, key) {
					err := oasvalidator.ObjectRequiredPropertyError(key)
					err.Parameter = qpe.Name

					errs = append(errs, *err)
				}

				continue
			}

			parsedKeys = append(parsedKeys, key)

			schemaProxy := iter.Value()
			if schemaProxy == nil {
				result[key] = rawValues

				continue
			}

			propSchema := schemaProxy.Schema()
			if oaschema.IsSchemaEmpty(propSchema) {
				result[key] = rawValues

				continue
			}

			propDecoder := &queryParamDecoder{
				Name:      key,
				Style:     qpe.Style,
				Explode:   qpe.Explode,
				RawValues: rawValues,
				Schema:    propSchema,
			}

			value, decodeErrs := propDecoder.Decode()
			if len(decodeErrs) == 0 {
				result[key] = value

				continue
			}

			errs = addParameterErrors(errs, decodeErrs, key)
		}
	}

	if len(errs) > 0 {
		return nil, errs
	}

	errs = qpe.decodeObjectAdditionalProperties(queryValues, result, parsedKeys)
	if len(errs) > 0 {
		return nil, errs
	}

	errs = qpe.decodeObjectPatternProperties(queryValues, result)
	if len(errs) > 0 {
		return nil, errs
	}

	return result, nil
}

func (qpe *queryParamDecoder) decodeObjectPatternProperties(
	queryValues map[string][]string,
	results map[string]any,
) []goutils.ErrorDetail {
	if qpe.Schema.PatternProperties == nil || qpe.Schema.PatternProperties.Len() == 0 {
		return nil
	}

	var errs []goutils.ErrorDetail

	for iter := qpe.Schema.PatternProperties.First(); iter != nil; iter = iter.Next() {
		key := iter.Key()

		pattern, err := regexps.Get(key)
		if err != nil {
			// ignore compile error on runtime.
			slog.Warn(
				"failed to compile regular expression: "+err.Error(),
				slog.String("pattern", key),
			)

			continue
		}

		var propSchema *base.Schema

		schemaProxy := iter.Value()
		if schemaProxy != nil {
			propSchema = schemaProxy.Schema()
		}

		for key, values := range queryValues {
			_, present := results[key]
			if present {
				continue
			}

			matched, err := pattern.MatchString(key)
			if err != nil {
				slog.Warn(
					"failed to compile pattern property: "+err.Error(),
					slog.String("pattern", key),
					slog.String("name", key),
				)

				continue
			}

			if !matched {
				continue
			}

			if oaschema.IsSchemaEmpty(propSchema) {
				results[key] = values

				continue
			}

			propDecoder := &queryParamDecoder{
				Name:      key,
				Style:     qpe.Style,
				Explode:   qpe.Explode,
				RawValues: values,
				Schema:    propSchema,
			}

			value, decodeErrs := propDecoder.Decode()
			if len(decodeErrs) == 0 {
				results[key] = value

				continue
			}

			errs = addParameterErrors(errs, decodeErrs, key)
		}
	}

	return errs
}

func (qpe *queryParamDecoder) decodeObjectAdditionalProperties(
	queryValues map[string][]string,
	result map[string]any,
	parsedKeys []string,
) []goutils.ErrorDetail {
	if qpe.Schema.AdditionalProperties == nil ||
		(!qpe.Schema.AdditionalProperties.B && qpe.Schema.AdditionalProperties.A == nil) {
		return nil
	}

	var (
		propSchema *base.Schema
		errs       []goutils.ErrorDetail
	)

	if qpe.Schema.AdditionalProperties.N == 0 && qpe.Schema.AdditionalProperties.A != nil {
		propSchema = qpe.Schema.AdditionalProperties.A.Schema()
	}

	for key, rawValues := range queryValues {
		if len(parsedKeys) > 0 && slices.Contains(parsedKeys, key) {
			continue
		}

		_, present := result[key]
		if present {
			continue
		}

		if oaschema.IsSchemaEmpty(propSchema) {
			result[key] = rawValues

			continue
		}

		propDecoder := &queryParamDecoder{
			Name:      key,
			Style:     qpe.Style,
			Explode:   qpe.Explode,
			RawValues: rawValues,
			Schema:    propSchema,
		}

		value, decodeErrs := propDecoder.Decode()
		if len(decodeErrs) == 0 {
			result[key] = value

			continue
		}

		errs = addParameterErrors(errs, decodeErrs, key)
	}

	return errs
}

func (qpe *queryParamDecoder) parseNonExplodeObject(
	separator string,
) (map[string][]string, *goutils.ErrorDetail) {
	result := make(map[string][]string)

	for _, rawValue := range qpe.RawValues {
		if rawValue == "" {
			continue
		}

		parts := strings.Split(rawValue, separator)
		if len(parts)%2 != 0 {
			return nil, qpe.newInvalidObjectError()
		}

		for i := 0; i < len(parts); i += 2 {
			if parts[i] == "" {
				return nil, qpe.newInvalidObjectError()
			}

			result[parts[i]] = append(result[parts[i]], parts[i+1])
		}
	}

	return result, nil
}

func (qpe *queryParamDecoder) newInvalidObjectError() *goutils.ErrorDetail {
	message := "Invalid syntax for the form style in parameter value. The object value must follow this format: queryKey=key1,value1,key2,value2"

	switch qpe.Style {
	case oaschema.EncodingStyleSpaceDelimited:
		message = "Invalid syntax for non-exploded label style in parameter value. The object value must follow this format: queryKey=key1 value1 key2 value2"
	case oaschema.EncodingStylePipeDelimited:
		message = "Invalid syntax for non-exploded matrix style in parameter value. The object value must follow this format: ;id=key1|value1|key2|value2"
	default:
	}

	return &goutils.ErrorDetail{
		Code:      oasvalidator.ErrCodeInvalidQueryParam,
		Detail:    message,
		Parameter: qpe.Name,
	}
}

func decodePrimitiveQueryValuesFromSchemaType(
	typeName string,
	values []string,
) (any, string, []goutils.ErrorDetail) {
	// Because the DecodeFromSchemaTypes function already checked the string type,
	// the value should be converted to null for the empty string.
	if len(values) == 0 || (len(values) == 1 && values[0] == "") {
		normalizedType, _ := oaschema.NormalizeType(typeName)

		return nil, normalizedType, nil
	}

	for _, value := range values {
		if value == "" {
			continue
		}

		result, primitiveType, err := oasvalidator.DecodePrimitiveValueFromType(
			value,
			typeName,
		)
		if err != nil {
			return nil, "", []goutils.ErrorDetail{
				{
					Code:   oasvalidator.ErrCodeInvalidQueryParam,
					Detail: err.Error(),
				},
			}
		}

		return result, primitiveType, nil
	}

	return values, typeName, nil
}

func addParameterErrors(
	dest []goutils.ErrorDetail,
	src []goutils.ErrorDetail,
	name string,
) []goutils.ErrorDetail {
	dest = slices.Grow(dest, len(src))

	for _, de := range src {
		if de.Parameter != "" {
			de.Pointer = "/" + de.Parameter
		}

		de.Parameter = name

		dest = append(dest, de)
	}

	return dest
}
