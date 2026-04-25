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
	"github.com/relychan/goutils/httperror"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/oasvalidator"
	"github.com/relychan/openapitools/oasvalidator/regexps"
)

// queryParamDecoder holds the resolved configuration and raw string values for a single
// query parameter and drives all style-aware decoding.
type queryParamDecoder struct {
	Name      string
	Types     []string
	RawValues []string
	Schema    *base.Schema
}

// DecodeQueryFromParameters decodes the query parameters from string values.
// The value is encoded differently on each style, according to the [OpenAPI specification].
//
// [OpenAPI specification](https://github.com/OAI/OpenAPI-Specification/blob/3.2.0/versions/3.2.0.md#style-examples)
func DecodeQueryFromParameters(
	definitions []*highv3.Parameter,
	values map[string][]string,
) (map[string]any, []httperror.ValidationError) {
	if len(definitions) == 0 {
		return goutils.ToAnyMap(values), nil
	}

	var (
		results = make(map[string]any)
		errs    []httperror.ValidationError
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
			return nil, []httperror.ValidationError{
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

		value, present, decodeErrors := decodeQueryFromParameter(definition, values, style, explode)
		if len(decodeErrors) > 0 {
			errs = append(errs, decodeErrors...)

			continue
		}

		if present {
			results[definition.Name] = value
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

func decodeQueryFromParameter(
	definition *highv3.Parameter,
	values map[string][]string,
	style oaschema.ParameterEncodingStyle,
	explode bool,
) (any, bool, []httperror.ValidationError) {
	var paramSchema *base.Schema

	if definition.Schema != nil {
		paramSchema = definition.Schema.Schema()
	}

	isObject := paramSchema != nil && slices.Contains(paramSchema.Type, oaschema.Object)

	decoder := &queryParamDecoder{
		Name:   definition.Name,
		Schema: paramSchema,
		Types:  paramSchema.Type,
	}

	// Properties in exploded object are flatten.
	// Because the schema can not have enough information, this parameter should be optional.
	if explode && isObject {
		itemResults, decodeErrs := decoder.decodeExplodeObject(values)
		if len(decodeErrs) > 0 {
			return nil, false, decodeErrs
		}

		return itemResults, true, nil
	}

	rawValues, present := values[definition.Name]
	if !present {
		if definition.Required != nil && *definition.Required {
			err := oasvalidator.ParameterRequiredError(definition.Name)
			err.Code = oasvalidator.ErrCodeInvalidQueryParam

			return nil, false, []httperror.ValidationError{*err}
		}

		return nil, false, nil
	}

	decoder.RawValues = rawValues

	if !explode {
		values, isValidObject := decoder.splitNonExplodeDelimitedStyle(style, isObject)
		if !isValidObject {
			return nil, false, []httperror.ValidationError{
				newInvalidQueryNonExplodedObjectError(definition.Name, style),
			}
		}

		decoder.RawValues = values
	}

	if oaschema.IsSchemaTypeEmpty(paramSchema) {
		return decoder.RawValues, true, nil
	}

	itemResults, decodeErrs := decoder.Decode()
	if len(decodeErrs) > 0 {
		return nil, false, decodeErrs
	}

	return itemResults, true, nil
}

// Decode evaluates and decodes URL parameters.
func (qpe *queryParamDecoder) Decode() (any, []httperror.ValidationError) {
	result, resultType, errs := qpe.decodeFromSchemaTypes()
	if len(errs) > 0 {
		return nil, errs
	}

	if len(qpe.Schema.AllOf) > 0 {
		allOf := oaschema.ExtractSchemaProxies(qpe.Schema.AllOf)
		schemaTypes, _ := oaschema.GetUnionSchemaTypes(allOf)

		if resultType != "" && len(schemaTypes) > 0 && !slices.Contains(schemaTypes, resultType) {
			return nil, []httperror.ValidationError{
				{
					Code: oasvalidator.ErrCodeOpenAPISchemaError,
					Detail: "Mismatched types in allOf [" + strings.Join(schemaTypes, ", ") +
						"] and schema types [" + strings.Join(qpe.Schema.Type, ", ") + "]",
					Parameter: qpe.Name,
				},
			}
		}

		if len(schemaTypes) == 0 {
			schemaTypes = []string{resultType}
		}
	}

	return result, errs
}

// decodeFromSchemaTypes decodes the raw values by trying each type declared in the schema.
// String is given priority: if the schema allows string the raw value is returned as-is
// to avoid lossy parsing (e.g. a numeric string "007" would become 7).
func (qpe *queryParamDecoder) decodeFromSchemaTypes() (any, string, []httperror.ValidationError) {
	if len(qpe.RawValues) == 0 {
		return qpe.RawValues, "", nil
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

	var finalErrors []httperror.ValidationError

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
) (any, string, []httperror.ValidationError) {
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

func (qpe *queryParamDecoder) decodeFromArray() ([]any, []httperror.ValidationError) {
	errFuncs := oasvalidator.ValidateArray(qpe.Schema, qpe.RawValues, cmp.Compare)
	errs := oasvalidator.CollectErrorsFunc(errFuncs, func(ed *httperror.ValidationError) {
		ed.Code = oasvalidator.ErrCodeInvalidQueryParam
		ed.Parameter = qpe.Name
	})

	if len(qpe.RawValues) == 0 || qpe.Schema.Items == nil || qpe.Schema.Items.A == nil {
		return goutils.ToAnySlice(qpe.RawValues), errs
	}

	itemSchema := qpe.Schema.Items.A.Schema()
	if oaschema.IsSchemaTypeEmpty(itemSchema) {
		return goutils.ToAnySlice(qpe.RawValues), errs
	}

	results := make([]any, len(qpe.RawValues))

	for i, value := range qpe.RawValues {
		itemValue, _, err := qpe.decodeItemValueFromSchemaTypes(itemSchema, value)
		if err != nil {
			errs = append(errs, *err)

			return nil, errs
		}

		results[i] = itemValue
	}

	return results, errs
}

func (qpe *queryParamDecoder) decodeFromObject() (map[string]any, []httperror.ValidationError) {
	rawValues, _ := qpe.parseNonExplodeObject()

	errFuncs := oasvalidator.ValidateObject(qpe.Schema, rawValues)
	if len(errFuncs) > 0 {
		return nil, oasvalidator.CollectErrors(errFuncs)
	}

	var (
		results = make(map[string]any)
		errs    []httperror.ValidationError
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
			if oaschema.IsSchemaTypeEmpty(propSchema) {
				results[key] = value

				continue
			}

			propDecoder := &queryParamDecoder{
				Name:      key,
				RawValues: value,
				Schema:    propSchema,
				Types:     propSchema.Type,
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

func (qpe *queryParamDecoder) splitNonExplodeDelimitedStyle(
	style oaschema.ParameterEncodingStyle,
	isObject bool,
) ([]string, bool) {
	if len(qpe.RawValues) == 0 {
		return qpe.RawValues, true
	}

	switch style {
	case oaschema.EncodingStyleSpaceDelimited:
		// /users?id=3 4 5
		return qpe.parseDelimitedStyle(oaschema.Space, isObject)
	case oaschema.EncodingStylePipeDelimited:
		// /users?id=3|4|5
		return qpe.parseDelimitedStyle(oaschema.Pipe, isObject)
	default:
		// /users?id=3,4,5
		return qpe.parseDelimitedStyle(oaschema.Comma, isObject)
	}
}

// Set delimited-separated array values for array params.
// For example: /users?id=3|4|5.
func (qpe *queryParamDecoder) parseDelimitedStyle(
	separator string,
	isObject bool,
) ([]string, bool) {
	results := make([]string, 0, len(qpe.RawValues))

	for _, value := range qpe.RawValues {
		if value == "" {
			continue
		}

		items := strings.Split(value, separator)
		if isObject && len(items)%2 != 0 {
			return nil, false
		}

		if len(results) == 0 {
			results = items
		} else {
			results = append(results, items...)
		}
	}

	return slices.Clip(results), true
}

// DecodeItemValueFromSchemaTypes decode a path parameter value from types of schema.
// Returns the decoded value, a matched type and an error.
// Prefer string if exists.
func (qpe *queryParamDecoder) decodeItemValueFromSchemaTypes(
	itemSchema *base.Schema,
	value any,
) (any, string, *httperror.ValidationError) {
	if len(itemSchema.Type) == 0 {
		return value, "", nil
	}

	if slices.Contains(itemSchema.Type, oaschema.String) {
		return value, oaschema.String, nil
	}

	var finalError *httperror.ValidationError

	for _, typeName := range itemSchema.Type {
		if typeName == "" {
			continue
		}

		result, primitiveType, err := oasvalidator.DecodePrimitiveValueFromType(
			value,
			typeName,
		)
		if err != nil {
			finalError = &httperror.ValidationError{
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

	return nil, "", &httperror.ValidationError{
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
) (map[string]any, []httperror.ValidationError) {
	// /users?role=admin&firstName=Alex
	var (
		result     = make(map[string]any)
		parsedKeys = make([]string, 0, len(queryValues))
		errs       []httperror.ValidationError
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
			if oaschema.IsSchemaTypeEmpty(propSchema) {
				result[key] = rawValues

				continue
			}

			propDecoder := &queryParamDecoder{
				Name:      key,
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
) []httperror.ValidationError {
	if qpe.Schema.PatternProperties == nil || qpe.Schema.PatternProperties.Len() == 0 {
		return nil
	}

	var errs []httperror.ValidationError

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

			if oaschema.IsSchemaTypeEmpty(propSchema) {
				results[key] = values

				continue
			}

			propDecoder := &queryParamDecoder{
				Name:      key,
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
) []httperror.ValidationError {
	if qpe.Schema.AdditionalProperties == nil ||
		(!qpe.Schema.AdditionalProperties.B && qpe.Schema.AdditionalProperties.A == nil) {
		return nil
	}

	var (
		propSchema *base.Schema
		errs       []httperror.ValidationError
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

		if oaschema.IsSchemaTypeEmpty(propSchema) {
			result[key] = rawValues

			continue
		}

		propDecoder := &queryParamDecoder{
			Name:      key,
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

func (qpe *queryParamDecoder) parseNonExplodeObject() (map[string][]string, bool) {
	result := make(map[string][]string)

	for i := 0; i < len(qpe.RawValues); i += 2 {
		if qpe.RawValues[i] == "" {
			return nil, false
		}

		result[qpe.RawValues[i]] = append(result[qpe.RawValues[i]], qpe.RawValues[i+1])
	}

	return result, true
}

// decodeQueryDeepObjectFromParameters decodes all deepObject-style parameters from the
// raw query map and merges decoded values into results.  If definitions is empty the
// entire raw map is decoded without schema guidance.
func decodeQueryDeepObjectFromParameters(
	definitions []*highv3.Parameter,
	queryValues map[string][]string,
	results map[string]any,
) []httperror.ValidationError {
	rawNodes, errs := parseDeepObjectNodes(queryValues)
	if len(errs) > 0 {
		return errs
	}

	if len(definitions) == 0 {
		for _, node := range rawNodes {
			node.decodeArbitraryObject(results)
		}

		return nil
	}

	for _, def := range definitions {
		value, decodeErrs := decodeQueryDeepObjectFromParameter(def, rawNodes)
		if len(decodeErrs) > 0 {
			errs = append(errs, decodeErrs...)
		} else {
			results[def.Name] = value
		}
	}

	return errs
}

func decodeQueryDeepObjectFromParameter(
	definition *highv3.Parameter,
	rawNodes ParameterNodes,
) (any, []httperror.ValidationError) {
	node := rawNodes.Find(ParamKey(definition.Name))
	if node == nil {
		if definition.Required != nil && *definition.Required {
			err := oasvalidator.ParameterRequiredError(definition.Name)
			err.Code = oasvalidator.ErrCodeInvalidQueryParam

			return nil, []httperror.ValidationError{*err}
		}

		return nil, nil
	}

	if definition.Schema == nil {
		return node.decodeArbitrary(), nil
	}

	schemaDef := definition.Schema.Schema()
	if schemaDef == nil {
		return node.decodeArbitrary(), nil
	}

	return node.Decode(schemaDef)
}

// parseDeepObjectNodes converts the flat map[string][]string from net/url into a
// ParameterNodes tree by parsing bracket-notation keys (e.g. "user[name]") and then
// calling Normalize to resolve any index/key ambiguities.
func parseDeepObjectNodes(queryValues map[string][]string) (ParameterNodes, []httperror.ValidationError) {
	var (
		rawNodes = make(ParameterNodes, 0, len(queryValues))
		errs     []httperror.ValidationError
	)

	for key, values := range queryValues {
		if key == "" {
			continue
		}

		parsedKeys, ok := parseDeepObjectKey(key)
		if !ok {
			errs = append(errs, httperror.ValidationError{
				Code:      oasvalidator.ErrCodeInvalidQueryParam,
				Detail:    "Invalid syntax from query key",
				Parameter: key,
			})

			continue
		}

		err := rawNodes.Insert(parsedKeys, values)
		if err != nil {
			err.Parameter = key

			errs = append(errs, *err)
		}
	}

	if len(errs) > 0 {
		return nil, errs
	}

	// Normalize array elements in the tree.
	for _, node := range rawNodes {
		node.Normalize()
	}

	return slices.Clip(rawNodes), nil
}

func newMixedArrayAndObjectError() *httperror.ValidationError {
	return &httperror.ValidationError{
		Code:   oasvalidator.ErrCodeInvalidQueryParam,
		Detail: "Query parameters can not contain both array and object",
	}
}

// decodePrimitiveQueryValuesFromSchemaType decodes the first non-empty string in values
// using the given type name.  An empty or absent value is treated as null rather than
// an error because the string-type fast path in decodeFromSchemaTypes already handled
// legitimate empty strings.
func decodePrimitiveQueryValuesFromSchemaType(
	typeName string,
	values []string,
) (any, string, []httperror.ValidationError) {
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
			return nil, "", []httperror.ValidationError{
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

// addParameterErrors appends src errors into dest, promoting the nested Parameter
// name to Pointer so the path context is preserved, then setting Parameter to the
// parent name.  This lets callers reconstruct a full JSON-pointer error path.
func addParameterErrors(
	dest []httperror.ValidationError,
	src []httperror.ValidationError,
	name string,
) []httperror.ValidationError {
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

func newInvalidQueryNonExplodedObjectError(
	name string,
	style oaschema.ParameterEncodingStyle,
) httperror.ValidationError {
	detail := "Invalid syntax for the form style in parameter value. The object value must follow this format: queryKey=key1,value1,key2,value2"

	switch style {
	case oaschema.EncodingStyleSpaceDelimited:
		detail = "Invalid syntax for non-exploded spaceDelimited style in parameter value. The object value must follow this format: queryKey=key1 value1 key2 value2"
	case oaschema.EncodingStylePipeDelimited:
		detail = "Invalid syntax for non-exploded pipeDelimited style in parameter value. The object value must follow this format: queryKey=key1|value1|key2|value2"
	default:
	}

	return httperror.ValidationError{
		Code:      oasvalidator.ErrCodeInvalidQueryParam,
		Detail:    detail,
		Parameter: name,
	}
}
