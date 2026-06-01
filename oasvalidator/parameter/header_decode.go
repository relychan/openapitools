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
	"net/http"
	"slices"
	"strings"

	"github.com/relychan/goutils/httperror"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/oasvalidator"
)

// DecodeHeaderParameter decodes a header parameter from the header map value.
// The value is encoded differently on each style, according to the [OpenAPI specification].
//
// [OpenAPI specification](https://github.com/OAI/OpenAPI-Specification/blob/3.2.0/versions/3.2.0.md#style-examples)
func DecodeHeaderParameter(
	definition *oaschema.Parameter,
	headers http.Header,
) (any, []httperror.ValidationError) {
	rawValues, ok := headers[http.CanonicalHeaderKey(definition.Name)]
	if !ok || len(rawValues) == 0 {
		if !definition.Required {
			return nil, nil
		}

		return nil, []httperror.ValidationError{
			{
				Code:   oasvalidator.ErrCodeInvalidHeader,
				Detail: "Header is required",
				Header: definition.Name,
			},
		}
	}

	if definition == nil || definition.Schema == nil {
		rawResults := parseHeaderArrayParam(rawValues)

		return normalizeRawParamValue(rawResults), nil
	}

	schemaTypes, nullable := oaschema.GetSchemaTypes(definition.Schema)

	if len(rawValues) == 0 {
		if nullable {
			return nil, nil
		}

		return nil, []httperror.ValidationError{
			{
				Code:   oasvalidator.ErrCodeInvalidHeader,
				Detail: "Header must not be null",
				Header: definition.Name,
			},
		}
	}

	if slices.Contains(schemaTypes, oaschema.Object) {
		result, errs := decodeHeaderObjectParam(definition, rawValues)
		if len(errs) == 0 {
			return result, nil
		}

		if len(schemaTypes) == 1 {
			return nil, enrichHeaderErrors(errs, definition.Name)
		}

		schemaTypes = slices.DeleteFunc(schemaTypes, func(t string) bool {
			return t == oaschema.Object
		})
	}

	if slices.Contains(schemaTypes, oaschema.Array) {
		result, errs := decodeHeaderArrayParam(definition, rawValues)
		if len(errs) == 0 {
			return result, nil
		}

		if len(schemaTypes) == 1 {
			return nil, enrichHeaderErrors(errs, definition.Name)
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
		return nil, enrichHeaderErrors(errs, definition.Name)
	}

	return result, nil
}

// decodeHeaderArrayParam splits comma-joined header values and decodes the resulting array.
func decodeHeaderArrayParam(
	definition *oaschema.Parameter,
	rawValues []string,
) (any, []httperror.ValidationError) {
	rawParts := parseHeaderArrayParam(rawValues)

	results, errs := decodeArrayParam(rawParts, definition.Schema)
	if len(errs) > 0 {
		return nil, enrichHeaderErrors(errs, definition.Name)
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

// decodeHeaderObjectParam splits header values and decodes them as an object, handling
// exploded (key=value,key=value) and non-exploded (key,value,key,value) forms.
func decodeHeaderObjectParam(
	definition *oaschema.Parameter,
	rawValues []string,
) (any, []httperror.ValidationError) {
	rawParts := parseHeaderArrayParam(rawValues)

	explode := definition.Explode != nil && *definition.Explode

	rawObjectValues, err := splitObjectFromHeaderValues(rawParts, explode)
	if err != nil {
		err.Header = definition.Name

		return nil, []httperror.ValidationError{*err}
	}

	decoder := newObjectParamDecoder(rawObjectValues)

	errs := decoder.Decode(definition.Schema)
	if len(errs) > 0 {
		return nil, enrichHeaderErrors(errs, definition.Name)
	}

	return decoder.Result, nil
}

// parseHeaderArrayParam splits comma-joined values in rawValues into individual items.
// Returns rawValues unchanged when no commas are present, avoiding an allocation.
func parseHeaderArrayParam(rawValues []string) []string {
	valueCount := 0

	for _, value := range rawValues {
		valueCount++

		for i := range value {
			if value[i] == oaschema.Comma[0] {
				valueCount++
			}
		}
	}

	if valueCount == len(rawValues) {
		return rawValues
	}

	results := make([]string, 0, valueCount)

	for _, value := range rawValues {
		if value == "" {
			continue
		}

		for item := range strings.SplitSeq(value, oaschema.Comma) {
			results = append(results, item)
		}
	}

	if len(results) == 0 {
		results = []string{""}
	}

	return slices.Clip(results)
}

// Splits header values into a key→value map according to the serialization style.
func splitObjectFromHeaderValues(
	rawValues []string,
	explode bool,
) (map[string][]string, *httperror.ValidationError) {
	// X-MyHeader: role=admin,firstName=Alex
	if explode {
		values := make(map[string][]string)

		for _, rawValue := range rawValues {
			if rawValue == "" {
				continue
			}

			if !setExplodeObjectProperties(values, rawValue, oaschema.Comma) {
				return nil, &httperror.ValidationError{
					Code:   oasvalidator.ErrCodeInvalidHeader,
					Detail: "Invalid syntax for exploded simple style in parameter value. The object value must follow this format: key1=value1,key2=value2",
				}
			}
		}

		return values, nil
	}

	values, ok := parseNonExplodeObject(rawValues)
	if !ok {
		return nil, &httperror.ValidationError{
			Code:   oasvalidator.ErrCodeInvalidHeader,
			Detail: "Invalid syntax for non-exploded simple style in parameter value. The object value must follow this format: key1,value1,key2,value2",
		}
	}

	return values, nil
}

// enrichHeaderErrors stamps each error with the header error code and the header name.
func enrichHeaderErrors(errs []httperror.ValidationError, name string) []httperror.ValidationError {
	for i := range errs {
		errs[i].Code = oasvalidator.ErrCodeInvalidHeader
		errs[i].Header = name
	}

	return errs
}
