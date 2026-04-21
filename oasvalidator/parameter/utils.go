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
	"reflect"
	"strconv"
	"strings"

	"github.com/relychan/goutils"
	"github.com/relychan/openapitools/oaschema"
)

// EvaluateParameterValue evaluates the type of the value and encode it into a string map.
func EvaluateParameterValue(value any, parentKeys ParamKeys) ParameterItems {
	scalarValue, ok := goutils.FormatScalar(value)
	if ok {
		if scalarValue == "" || scalarValue == goutils.NullStr {
			return nil
		}

		return []ParameterItem{
			NewParameterItem(parentKeys, scalarValue),
		}
	}

	switch v := value.(type) {
	case []byte:
		return []ParameterItem{
			NewParameterItem(parentKeys, string(v)),
		}
	case []string:
		results := make(ParameterItems, 0, len(v))

		for i, item := range v {
			if item != "" {
				results = append(results, NewParameterItem(
					ParamKeys{ParamIndex(i)},
					item,
				))
			}
		}

		return results
	case []any:
		results := make(ParameterItems, 0, len(v))

		for i, item := range v {
			params := EvaluateParameterValue(item, append(parentKeys, ParamIndex(i)))
			if len(params) > 0 {
				results = append(results, params...)
			}
		}

		return results
	case map[string]any:
		results := make(ParameterItems, 0, len(v))

		for key, item := range v {
			params := EvaluateParameterValue(item, append(parentKeys, ParamKey(key)))
			if len(params) > 0 {
				results = append(results, params...)
			}
		}

		return results
	case map[any]any:
		results := make(ParameterItems, 0, len(v))

		for anyKey, item := range v {
			key, ok := anyKey.(string)
			if !ok {
				continue
			}

			params := EvaluateParameterValue(item, append(parentKeys, ParamKey(key)))
			if len(params) > 0 {
				results = append(results, params...)
			}
		}

		return results
	default:
		return evaluateParameterValueReflection(reflect.ValueOf(value), parentKeys)
	}
}

func evaluateParameterValueReflection(
	value reflect.Value,
	parentKeys ParamKeys,
) ParameterItems {
	reflectValue, notNull := goutils.UnwrapPointerFromReflectValue(value)
	if !notNull {
		return nil
	}

	switch reflectValue.Kind() {
	case reflect.Bool:
		return []ParameterItem{
			NewParameterItem(parentKeys, strconv.FormatBool(reflectValue.Bool())),
		}
	case reflect.String:
		return []ParameterItem{
			NewParameterItem(parentKeys, reflectValue.String()),
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return []ParameterItem{
			NewParameterItem(parentKeys, strconv.FormatInt(reflectValue.Int(), 10)),
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return []ParameterItem{
			NewParameterItem(parentKeys, strconv.FormatUint(reflectValue.Uint(), 10)),
		}
	case reflect.Float32:
		return []ParameterItem{
			NewParameterItem(
				parentKeys,
				strconv.FormatFloat(reflectValue.Float(), 'f', -1, 32),
			),
		}
	case reflect.Float64:
		return []ParameterItem{
			NewParameterItem(
				parentKeys,
				strconv.FormatFloat(reflectValue.Float(), 'f', -1, 64),
			),
		}
	case reflect.Complex64:
		return []ParameterItem{
			NewParameterItem(
				parentKeys,
				strconv.FormatComplex(reflectValue.Complex(), 'f', -1, 64),
			),
		}
	case reflect.Complex128:
		return []ParameterItem{
			NewParameterItem(
				parentKeys,
				strconv.FormatComplex(reflectValue.Complex(), 'f', -1, 128),
			),
		}
	case reflect.Slice, reflect.Array:
		valueLength := reflectValue.Len()
		results := make(ParameterItems, 0, valueLength)

		for i := range valueLength {
			elem := reflectValue.Index(i)

			params := evaluateParameterValueReflection(elem, append(parentKeys, ParamIndex(i)))
			if len(params) > 0 {
				results = append(results, params...)
			}
		}

		return results
	case reflect.Map:
		keys := reflectValue.MapKeys()
		results := make(ParameterItems, 0, len(keys))

		for _, key := range keys {
			if key.Kind() != reflect.String {
				// the key of JSON objects must be string.
				continue
			}

			keyStr := key.String()
			elem := reflectValue.MapIndex(key)

			params := evaluateParameterValueReflection(elem, append(parentKeys, ParamKey(keyStr)))
			if len(params) > 0 {
				results = append(results, params...)
			}
		}

		return results
	default:
		// Skip unserializable fields.
		return nil
	}
}

// EncodeParamDelimitedStyleNonExplode encodes ampersand-separated values with explode=false.
func EncodeParamDelimitedStyleNonExplode(
	params ParameterItems,
	separator byte,
	assignSymbol byte,
) string {
	builtParams, count := params.Build("", false)
	if len(builtParams) == 0 {
		return ""
	}

	var sb strings.Builder

	sb.Grow(count)

	buildParamDelimitedStyleNonExplode(&sb, builtParams, separator, assignSymbol)

	return sb.String()
}

func buildParamDelimitedStyleNonExplode(
	sb *strings.Builder,
	builtParams map[string][]string,
	separator byte,
	assignSymbol byte,
) {
	first := true
	keys := goutils.GetSortedKeys(builtParams)

	for _, key := range keys {
		values := builtParams[key]

		if !first {
			sb.WriteByte(separator)
		} else {
			first = false
		}

		if key == "" {
			// /users?id=3,4,5
			for j, value := range values {
				if j > 0 {
					sb.WriteByte(separator)
				}

				sb.WriteString(value)
			}

			continue
		}

		// Nested fields are flattened.
		// /users?id=role,admin,firstName,Alex
		sb.WriteString(key)
		sb.WriteByte(assignSymbol)

		for i, value := range values {
			if i > 0 {
				sb.WriteByte(separator)
			}

			sb.WriteString(value)
		}
	}
}

func getParamStyleAndExplodeFromRawStyle(
	location oaschema.ParameterLocation,
	rawStyle string,
	rawExplode *bool,
) (oaschema.ParameterEncodingStyle, bool, error) {
	var style *oaschema.ParameterEncodingStyle

	if rawStyle != "" {
		parsedStyle, err := oaschema.ParseParameterEncodingStyle(rawStyle)
		if err != nil {
			return 255, false, err
		}

		style = &parsedStyle
	}

	result, explode := evalParamStyleAndExplode(location, style, rawExplode)

	return result, explode, nil
}

// Evaluate the style and explode of a parameter from the location.
func evalParamStyleAndExplode(
	location oaschema.ParameterLocation,
	style *oaschema.ParameterEncodingStyle,
	explode *bool,
) (oaschema.ParameterEncodingStyle, bool) {
	switch location {
	case oaschema.InPath, oaschema.InHeader:
		explodeValue := explode != nil && *explode

		if style == nil {
			return oaschema.EncodingStyleSimple, explodeValue
		}

		return *style, explodeValue
	case oaschema.InQuery, oaschema.InCookie:
		explodeValue := explode == nil || *explode

		if style == nil {
			return oaschema.EncodingStyleForm, explodeValue
		}

		return *style, explodeValue
	default:
		return 255, false
	}
}

// parses the deep object string to param paths.
func parseDeepObjectKey(input string) (ParamKeys, bool) {
	var results ParamKeys

	inputLength := len(input)
	tempKey := make([]byte, 0, inputLength)
	isOpen := false

	for i := 0; i < inputLength; i++ {
		c := input[i]
		if c == '[' {
			if len(tempKey) > 0 {
				results = append(results, ParamKey(tempKey))
				tempKey = tempKey[:0]
			}

			isOpen = true

			continue
		}

		if c == ']' {
			// close bracket must not be in the head.
			if !isOpen {
				return nil, false
			}

			if len(tempKey) > 0 {
				results = append(results, ParamKey(tempKey))
				tempKey = tempKey[:0]
			} else {
				results = append(results, ParamIndex(-1))
			}

			// close bracket at the end.
			if i == inputLength-1 {
				isOpen = false

				break
			}

			if input[i+1] != '[' {
				return nil, false
			}

			i++

			continue
		}

		tempKey = append(tempKey, c)
	}

	if isOpen {
		return nil, false
	}

	if len(tempKey) > 0 {
		results = append(results, ParamKey(tempKey))
	}

	return results, true
}

func getValue(values []string) any {
	switch len(values) {
	case 0:
		return nil
	case 1:
		return values[0]
	default:
		return values
	}
}
