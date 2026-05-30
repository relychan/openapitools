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

	"github.com/relychan/goutils"
	"github.com/relychan/goutils/httperror"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/oasvalidator"
)

// DecodeQueryFromParameters decodes the query parameters from string values.
// The value is encoded differently on each style, according to the [OpenAPI specification].
//
// [OpenAPI specification](https://github.com/OAI/OpenAPI-Specification/blob/3.2.0/versions/3.2.0.md#style-examples)
func DecodeQueryFromParameters(
	definitions []*oaschema.Parameter,
	values map[string][]string,
) (map[string]any, []httperror.ValidationError) {
	if len(definitions) == 0 {
		return goutils.ToAnyMap(values), nil
	}

	var (
		results = make(map[string]any)
		errs    []httperror.ValidationError
	)

	var deepObjectParams []*oaschema.Parameter

	for _, definition := range definitions {
		if definition.In != oaschema.InQuery {
			continue
		}

		style, explode := definition.GetStyleAndExplode()
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
	definition *oaschema.Parameter,
	values map[string][]string,
	style oaschema.ParameterEncodingStyle,
	explode bool,
) (any, bool, []httperror.ValidationError) {
	var (
		schemaTypes []string
		nullable    bool
		isObject    bool
	)

	if definition.Schema != nil {
		schemaTypes, nullable = oaschema.GetSchemaTypes(definition.Schema)
		isObject = slices.Contains(definition.Schema.Type, oaschema.Object)
	}

	// Properties in exploded object are flatten.
	// Because the schema can not have enough information, this parameter should be optional.
	if explode && isObject {
		decoder := newObjectParamDecoder(values)

		decodeErrs := decoder.Decode(definition.Schema)
		if len(decodeErrs) == 0 {
			return decoder.Result, true, nil
		}

		if len(schemaTypes) == 1 {
			return nil, false, enrichQueryErrors(decodeErrs, definition.Name)
		}

		schemaTypes = slices.DeleteFunc(schemaTypes, func(t string) bool {
			return t == oaschema.Object
		})

		isObject = false
	}

	rawValues, present := values[definition.Name]
	if !present {
		if definition.Required {
			err := oasvalidator.ParameterRequiredError(definition.Name)
			err.Code = oasvalidator.ErrCodeInvalidQueryParam

			return nil, false, []httperror.ValidationError{*err}
		}

		return nil, false, nil
	}

	if len(rawValues) == 0 {
		if nullable {
			return nil, true, nil
		}

		return nil, false, []httperror.ValidationError{
			{
				Code:   oasvalidator.ErrCodeInvalidQueryParam,
				Detail: "Value must not be null",
				Header: definition.Name,
			},
		}
	}

	if definition.Schema == nil {
		if !explode {
			rawValues, _ = splitNonExplodeDelimitedStyle(rawValues, style, false)
		}

		return normalizeRawParamValue(rawValues), true, nil
	}

	if isObject {
		result, errs := decodeQueryObjectNonExplodeParam(definition, rawValues, style)
		if len(errs) == 0 {
			return result, true, nil
		}

		if len(schemaTypes) == 1 {
			return nil, false, errs
		}

		schemaTypes = slices.DeleteFunc(schemaTypes, func(t string) bool {
			return t == oaschema.Object
		})
	}

	if slices.Contains(schemaTypes, oaschema.Array) {
		result, errs := decodeQueryArrayParam(definition, rawValues, style, explode)
		if len(errs) == 0 {
			return result, true, nil
		}

		if len(schemaTypes) == 1 {
			return nil, false, enrichQueryErrors(errs, definition.Name)
		}

		schemaTypes = slices.DeleteFunc(schemaTypes, func(t string) bool {
			return t == oaschema.Array
		})
	}

	decoder := &paramDecoder{
		Schema:    definition.Schema,
		RawValues: rawValues,
	}

	result, errs := decoder.Decode(schemaTypes)
	if len(errs) > 0 {
		return nil, false, enrichQueryErrors(errs, definition.Name)
	}

	return result, true, nil
}

func decodeQueryArrayParam(
	definition *oaschema.Parameter,
	rawValues []string,
	style oaschema.ParameterEncodingStyle,
	explode bool,
) (any, []httperror.ValidationError) {
	if !explode {
		rawValues, _ = splitNonExplodeDelimitedStyle(rawValues, style, false)
	}

	results, errs := decodeParamFromArray(rawValues, definition.Schema)
	if len(errs) > 0 {
		return nil, enrichQueryErrors(errs, definition.Name)
	}

	errFuncs := oasvalidator.ValidateValue(definition.Schema, results)
	if len(errFuncs) > 0 {
		return nil, oasvalidator.CollectErrorsFunc(errFuncs, func(ve *httperror.ValidationError) {
			ve.Code = oasvalidator.ErrCodeInvalidHeader
			ve.Header = definition.Name
		})
	}

	return results, nil
}

func decodeQueryObjectNonExplodeParam(
	definition *oaschema.Parameter,
	rawValues []string,
	style oaschema.ParameterEncodingStyle,
) (any, []httperror.ValidationError) {
	values, isValidObject := splitNonExplodeDelimitedStyle(rawValues, style, true)
	if !isValidObject {
		return nil, []httperror.ValidationError{
			newInvalidQueryNonExplodedObjectError(definition.Name, style),
		}
	}

	rawObjectValue, ok := parseNonExplodeQueryObject(values)
	if !ok {
		return nil, []httperror.ValidationError{
			{
				Detail:    "malformed object syntax for query style: " + style.String(),
				Code:      oasvalidator.ErrCodeInvalidQueryParam,
				Parameter: definition.Name,
			},
		}
	}

	decoder := newObjectParamDecoder(rawObjectValue)

	errs := decoder.Decode(definition.Schema)
	if len(errs) == 0 {
		return decoder.Result, nil
	}

	return nil, enrichQueryErrors(errs, definition.Name)
}

func parseNonExplodeQueryObject(rawValues []string) (map[string][]string, bool) {
	result := make(map[string][]string)

	for i := 0; i < len(rawValues); i += 2 {
		if rawValues[i] == "" {
			return nil, false
		}

		result[rawValues[i]] = append(result[rawValues[i]], rawValues[i+1])
	}

	return result, true
}

// decodeQueryDeepObjectFromParameters decodes all deepObject-style parameters from the
// raw query map and merges decoded values into results.  If definitions is empty the
// entire raw map is decoded without schema guidance.
func decodeQueryDeepObjectFromParameters(
	definitions []*oaschema.Parameter,
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
	definition *oaschema.Parameter,
	rawNodes ParameterNodes,
) (any, []httperror.ValidationError) {
	node := rawNodes.Find(ParamKey(definition.Name))
	if node == nil {
		if definition.Required {
			err := oasvalidator.ParameterRequiredError(definition.Name)
			err.Code = oasvalidator.ErrCodeInvalidQueryParam

			return nil, []httperror.ValidationError{*err}
		}

		return nil, nil
	}

	if definition.Schema == nil {
		return node.decodeArbitrary(), nil
	}

	return node.Decode(definition.Schema)
}

// parseDeepObjectNodes converts the flat map[string][]string from net/url into a
// ParameterNodes tree by parsing bracket-notation keys (e.g. "user[name]") and then
// calling Normalize to resolve any index/key ambiguities.
func parseDeepObjectNodes(
	queryValues map[string][]string,
) (ParameterNodes, []httperror.ValidationError) {
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
	rawValues []string,
) (any, string, []httperror.ValidationError) {
	if len(rawValues) == 0 {
		return nil, typeName, nil
	}

	if typeName == oaschema.String {
		var result string

		for _, rawValue := range rawValues {
			if rawValue != "" {
				result = rawValue

				break
			}
		}

		return result, oaschema.String, nil
	}

	for _, rawValue := range rawValues {
		if rawValue == "" {
			continue
		}

		result, primitiveType, err := oasvalidator.DecodePrimitiveValueFromType(
			rawValue,
			typeName,
		)
		if err != nil {
			return nil, "", []httperror.ValidationError{
				{
					Detail: err.Error(),
				},
			}
		}

		return result, primitiveType, nil
	}
	// Return raw values with unknown type.
	return rawValues, "", nil
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

func enrichQueryErrors(errs []httperror.ValidationError, name string) []httperror.ValidationError {
	for i := range errs {
		errs[i].Code = oasvalidator.ErrCodeInvalidQueryParam
		errs[i].Header = name
	}

	return errs
}
