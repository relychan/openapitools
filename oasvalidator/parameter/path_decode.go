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
	"slices"
	"strings"

	"github.com/relychan/goutils/httperror"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/oasvalidator"
)

// DecodePathValue decodes the path parameter from a string value.
// The value is encoded differently on each style, according to the [OpenAPI specification].
//
// [OpenAPI specification](https://github.com/OAI/OpenAPI-Specification/blob/3.2.0/versions/3.2.0.md#style-examples)
func DecodePathValue(
	definition *oaschema.Parameter,
	value string,
) (any, []httperror.ValidationError) {
	if value == "" {
		return nil, []httperror.ValidationError{
			{
				Code:      oasvalidator.ErrCodeInvalidURLParam,
				Detail:    "URL parameter is required",
				Parameter: definition.Name,
			},
		}
	}

	style, explode := definition.GetStyleAndExplode()

	value, err := trimPathValueByStyle(definition.Name, value, style)
	if err != nil {
		return nil, []httperror.ValidationError{*err}
	}

	if definition == nil || definition.Schema == nil {
		rawResults, parseErr := parsePathArrayParam(definition.Name, value, style, explode)
		if parseErr != nil {
			return nil, []httperror.ValidationError{*parseErr}
		}

		return normalizeRawParamValue(rawResults), nil
	}

	schemaTypes, nullable := oaschema.GetSchemaTypes(definition.Schema)

	if value == "" {
		if nullable {
			return nil, nil
		}

		return nil, []httperror.ValidationError{
			{
				Code:      oasvalidator.ErrCodeInvalidURLParam,
				Detail:    "URL parameter must not be empty",
				Parameter: definition.Name,
			},
		}
	}

	if slices.Contains(schemaTypes, oaschema.Object) {
		result, errs := decodePathObjectParam(definition, value, style, explode)
		if len(errs) == 0 {
			return result, nil
		}

		if len(schemaTypes) == 1 {
			return nil, enrichPathParamErrors(errs, definition.Name)
		}

		schemaTypes = slices.DeleteFunc(schemaTypes, func(t string) bool {
			return t == oaschema.Object
		})
	}

	rawValues, parseErr := parsePathArrayParam(definition.Name, value, style, explode)
	if parseErr != nil {
		return nil, []httperror.ValidationError{*parseErr}
	}

	if slices.Contains(schemaTypes, oaschema.Array) {
		results, errs := decodePathArrayParam(definition, rawValues)
		if len(errs) == 0 {
			return results, nil
		}

		if len(schemaTypes) == 1 {
			return nil, enrichPathParamErrors(errs, definition.Name)
		}

		schemaTypes = slices.DeleteFunc(schemaTypes, func(t string) bool {
			return t == oaschema.Array
		})
	}

	decoder := paramDecoder{
		RawValues: rawValues,
		Schema:    definition.Schema,
	}

	result, errs := decoder.Decode(schemaTypes)
	if len(errs) > 0 {
		return nil, enrichPathParamErrors(errs, definition.Name)
	}

	return result, nil
}

// decodePathArrayParam decodes pre-split raw path values as an array and validates against schema.
func decodePathArrayParam(
	definition *oaschema.Parameter,
	rawValues []string,
) (any, []httperror.ValidationError) {
	results, errs := decodeArrayParam(rawValues, definition.Schema)
	if len(errs) == 0 {
		return results, nil
	}

	errFuncs := oasvalidator.ValidateValue(definition.Schema, results)
	if len(errFuncs) > 0 {
		return nil, oasvalidator.CollectErrorsFunc(errFuncs, func(ve *httperror.ValidationError) {
			ve.Code = oasvalidator.ErrCodeInvalidURLParam
			ve.Parameter = definition.Name
		})
	}

	return results, nil
}

// decodePathObjectParam parses the raw path segment into a key→value map and decodes it as an object.
func decodePathObjectParam(
	definition *oaschema.Parameter,
	rawValue string,
	style oaschema.ParameterEncodingStyle,
	explode bool,
) (any, []httperror.ValidationError) {
	rawObjectValues, err := parsePathObjectParam(definition.Name, rawValue, style, explode)
	if err != nil {
		return nil, []httperror.ValidationError{*err}
	}

	decoder := newObjectParamDecoder(rawObjectValues)

	errs := decoder.Decode(definition.Schema)
	if len(errs) > 0 {
		return nil, enrichPathParamErrors(errs, definition.Name)
	}

	return decoder.Result, nil
}

// trimPathValueByStyle strips the style-specific prefix character ("." for label, ";" for matrix)
// and returns an error when the prefix is missing.
func trimPathValueByStyle(
	name string,
	rawValue string,
	style oaschema.ParameterEncodingStyle,
) (string, *httperror.ValidationError) {
	// remove the symbol prefix from raw value string
	switch style {
	case oaschema.EncodingStyleLabel:
		if rawValue[0] != oaschema.Dot[0] {
			return "", &httperror.ValidationError{
				Code:      oasvalidator.ErrCodeInvalidURLParam,
				Detail:    "The label style of parameter value must start with a dot",
				Parameter: name,
			}
		}

		return rawValue[1:], nil
	case oaschema.EncodingStyleMatrix:
		if rawValue[0] != oaschema.SemiColon[0] {
			return "", &httperror.ValidationError{
				Code:      oasvalidator.ErrCodeInvalidURLParam,
				Detail:    "The matrix style of parameter value must start with a semicolon",
				Parameter: name,
			}
		}

		return rawValue[1:], nil
	default:
		return rawValue, nil
	}
}

// Splits RawValue into individual array elements according to the serialization style.
// The style prefix has already been stripped by DecodeFromSchemaTypes.
func parsePathArrayParam(
	name string,
	rawValue string,
	style oaschema.ParameterEncodingStyle,
	explode bool,
) ([]string, *httperror.ValidationError) {
	switch style {
	case oaschema.EncodingStyleLabel:
		if rawValue == "" {
			return nil, nil
		}

		// /users/.3.4.5
		// /users/.role=admin.firstName=Alex
		if explode {
			return strings.Split(rawValue, oaschema.Dot), nil
		}

		// /users/.3,4,5
		// /users/.role,admin,firstName,Alex
		return strings.Split(rawValue, oaschema.Comma), nil
	case oaschema.EncodingStyleMatrix:
		prefix := name + oaschema.Equals
		// /users/;id=3;id=4;id=5
		// /users/;role=admin;firstName=Alex
		if explode {
			parts := strings.Split(rawValue, oaschema.SemiColon)
			results := make([]string, len(parts))

			for i, part := range parts {
				value, found := strings.CutPrefix(strings.TrimSpace(part), prefix)
				if !found {
					return nil, &httperror.ValidationError{
						Code:      oasvalidator.ErrCodeInvalidURLParam,
						Detail:    "Invalid matrix style in parameter value. The array value must follow this format: ;key1=value1;key2=value2",
						Parameter: name,
					}
				}

				results[i] = value
			}

			return results, nil
		}

		// /users/;id=3,4,5
		// /users/;id=role,admin,firstName,Alex
		value, found := strings.CutPrefix(rawValue, prefix)
		if !found {
			return nil, &httperror.ValidationError{
				Code:      oasvalidator.ErrCodeInvalidURLParam,
				Detail:    "Invalid matrix style in parameter value. The array value must follow this format: ;key1=value1,value2",
				Parameter: name,
			}
		}

		return strings.Split(value, oaschema.Comma), nil
	default:
		// encode with the simple style
		return strings.Split(rawValue, oaschema.Comma), nil
	}
}

// Splits RawValue into a key→value map according to the serialization style.
// The style prefix has already been stripped by DecodeFromSchemaTypes.
func parsePathObjectParam(
	name string,
	rawValue string,
	style oaschema.ParameterEncodingStyle,
	explode bool,
) (map[string][]string, *httperror.ValidationError) {
	if rawValue == "" {
		return nil, nil
	}

	switch style {
	case oaschema.EncodingStyleLabel:
		// /users/.role=admin.firstName=Alex
		if explode {
			result, ok := parseExplodeObjectParam(rawValue, oaschema.Dot)
			if !ok {
				return nil, newInvalidPathObjectError(name, style, explode)
			}

			return result, nil
		}

		// /users/.role,admin,firstName,Alex
		parts := strings.Split(rawValue, oaschema.Comma)

		results, ok := parseNonExplodeObject(parts)
		if !ok {
			return nil, newInvalidPathObjectError(name, style, explode)
		}

		return results, nil
	case oaschema.EncodingStyleMatrix:
		// /users/;role=admin;firstName=Alex
		if explode {
			result, ok := parseExplodeObjectParam(rawValue, oaschema.SemiColon)
			if !ok {
				return nil, newInvalidPathObjectError(name, style, explode)
			}

			return result, nil
		}

		// /users/;id=role,admin,firstName,Alex
		value, found := strings.CutPrefix(rawValue, name+oaschema.Equals)
		if !found {
			return nil, newInvalidPathObjectError(name, style, explode)
		}

		parts := strings.Split(value, oaschema.Comma)

		results, ok := parseNonExplodeObject(parts)
		if !ok {
			return nil, newInvalidPathObjectError(name, style, explode)
		}

		return results, nil
	default:
		// /users/role=admin,firstName=Alex
		if explode {
			result, ok := parseExplodeObjectParam(rawValue, oaschema.Comma)
			if !ok {
				return nil, newInvalidPathObjectError(name, style, explode)
			}

			return result, nil
		}

		// /users/role,admin,firstName,Alex
		parts := strings.Split(rawValue, oaschema.Comma)

		results, ok := parseNonExplodeObject(parts)
		if !ok {
			return nil, newInvalidPathObjectError(name, style, explode)
		}

		return results, nil
	}
}

// newInvalidPathObjectError wraps newInvalidPathObjectErrorMessage in a ValidationError.
func newInvalidPathObjectError(
	name string,
	style oaschema.ParameterEncodingStyle,
	explode bool,
) *httperror.ValidationError {
	return &httperror.ValidationError{
		Code:      oasvalidator.ErrCodeInvalidURLParam,
		Detail:    newInvalidPathObjectErrorMessage(style, explode),
		Parameter: name,
	}
}

// newInvalidPathObjectErrorMessage returns a style- and explode-specific human-readable message
// describing the expected path object serialization format.
func newInvalidPathObjectErrorMessage(style oaschema.ParameterEncodingStyle, explode bool) string {
	switch style {
	case oaschema.EncodingStyleLabel:
		if explode {
			return "Invalid syntax for exploded label style in parameter value. The object value must follow this format: .key1=value1.key2=value2"
		}

		return "Invalid syntax for non-exploded label style in parameter value. The object value must follow this format: .key1,value1,key2,value2"
	case oaschema.EncodingStyleMatrix:
		if explode {
			return "Invalid syntax for exploded matrix style in parameter value. The object value must follow this format: ;key1=value1;key2=value2"
		}

		return "Invalid syntax for non-exploded matrix style in parameter value. The object value must follow this format: ;id=key1,value1,key2,value2"
	default:
		if explode {
			return "Invalid syntax for simple style in parameter value. The object value must follow this format: role=admin,firstName=Alex"
		}

		return "Invalid syntax for simple style in parameter value. The object value must follow this format: key1,value1,key2,value2"
	}
}

// enrichPathParamErrors stamps each error with the URL param error code and the parameter name.
func enrichPathParamErrors(
	errs []httperror.ValidationError,
	name string,
) []httperror.ValidationError {
	for i := range errs {
		errs[i].Code = oasvalidator.ErrCodeInvalidURLParam
		errs[i].Parameter = name
	}

	return errs
}
