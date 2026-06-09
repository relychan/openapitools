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
	"net/url"

	"github.com/relychan/openapitools/oaschema"
)

const escapedQuerySpace = "+"

var escapedPipe = url.QueryEscape("|")

// queryParamSetter encodes a single query parameter into a url.Values map using the
// style and explode settings resolved from the OpenAPI parameter definition.
type queryParamSetter struct {
	params        url.Values
	rootKey       string
	style         oaschema.ParameterEncodingStyle
	explode       bool
	allowReserved bool
}

// SetQueryParam encodes and set the query param into URL values.
// The value is encoded differently on each style, according to the [OpenAPI specification].
//
// [OpenAPI specification](https://github.com/OAI/OpenAPI-Specification/blob/3.2.0/versions/3.2.0.md#style-examples)
func SetQueryParam(params url.Values, definition *oaschema.Parameter, value any) {
	qre := &queryParamSetter{
		params:        params,
		rootKey:       definition.Name,
		allowReserved: definition.AllowReserved,
	}

	qre.style, qre.explode = definition.GetStyleAndExplode()
	queryParams := EvaluateParameterValue(value, ParamKeys{})

	qre.Set(queryParams)
}

// Set dispatches to the appropriate encoding strategy based on the resolved style and explode flag.
func (qre *queryParamSetter) Set(params ParameterItems) {
	if len(params) == 0 {
		return
	}

	switch qre.style {
	case oaschema.EncodingStyleSpaceDelimited:
		qre.setParamDelimitedStyle(params, escapedQuerySpace)
	case oaschema.EncodingStylePipeDelimited:
		qre.setParamDelimitedStyle(params, escapedPipe)
	case oaschema.EncodingStyleDeepObject:
		qre.setParamDeepObjects(params)
	default:
		// form style
		if qre.explode {
			for _, param := range params {
				qre.setParamFormExplode(param)
			}

			return
		}

		qre.setParamDelimitedStyleNonExplode(params, oaschema.Comma)
	}
}

// Set delimited-separated array values for simple params.
// For example: /users?id=3|4|5.
func (qre *queryParamSetter) setParamDelimitedStyle(params ParameterItems, separator string) {
	if qre.explode {
		// the same format with the form style
		for _, param := range params {
			qre.setParamFormExplode(param)
		}

		return
	}

	qre.setParamDelimitedStyleNonExplode(params, separator)
}

// ampersand-separated values, also known as form-style query expansion with explode=true.
func (qre *queryParamSetter) setParamFormExplode(param ParameterItem) {
	// The parameter does not have nested object. Encode with the simple form format.
	// /users?id=3&id=4&id=5
	queryKey := qre.rootKey

	if param.IsNested() {
		// The root key is ignored in nested fields.
		// /users?role=admin&firstName=Alex
		queryKey = param.keys.Format("", false)
	}

	qre.addParam(queryKey, param.value)
}

// Encode and set ampersand-separated values with explode=false.
func (qre *queryParamSetter) setParamDelimitedStyleNonExplode(
	params ParameterItems,
	separator string,
) {
	encodedValue := EncodeParamDelimitedStyleNonExplode(
		params,
		separator,
		separator,
		GetQueryEscapeFunc(qre.allowReserved),
	)

	qre.params.Set(qre.rootKey, encodedValue)
}

// simple non-nested objects are serialized as paramName[prop1]=value1&paramName[prop2]=value2&...
// setParamDeepObjects encodes each item using deep-object bracket notation.
func (qre *queryParamSetter) setParamDeepObjects(params ParameterItems) {
	for _, param := range params {
		qre.setParamDeepObject(param)
	}
}

// setParamDeepObject encodes a single item into bracket-notation query key, e.g. user[name]=Alex.
func (qre *queryParamSetter) setParamDeepObject(param ParameterItem) {
	if len(param.keys) == 0 {
		// The parameter does not have nested object. Encode with the simple form format.
		// /users?id=3&id=4&id=5
		qre.addParam(qre.rootKey, param.value)

		return
	}

	// The root key is ignored in nested fields.
	// /users?role=admin&firstName=Alex
	queryKey := param.keys.Format(qre.rootKey, true)

	qre.addParam(queryKey, param.value)
}

// addParam appends a key/value pair.  When allowReserved is set, RFC 3986 reserved
// characters (e.g. ":", "/", "?") are left unescaped as permitted by the OpenAPI spec.
func (qre *queryParamSetter) addParam(key string, value string) {
	queryEscape := GetQueryEscapeFunc(qre.allowReserved)

	qre.params.Add(queryEscape(key), queryEscape(value))
}
