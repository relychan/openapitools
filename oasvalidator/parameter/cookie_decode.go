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

	"github.com/relychan/goutils/httperror"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/oasvalidator"
)

// DecodeCookieParameters decodes header parameters from the header map value.
// Each header value is encoded differently on each style, according to the [OpenAPI specification].
//
// [OpenAPI specification](https://github.com/OAI/OpenAPI-Specification/blob/3.2.0/versions/3.2.0.md#style-examples)
func DecodeCookieParameters(
	definition []*oaschema.Parameter,
	cookies []*http.Cookie,
) (map[string]any, []httperror.ValidationError) {
	var (
		decodeErrors []httperror.ValidationError
		results      map[string]any
	)

	for _, def := range definition {
		if def == nil || def.In != oaschema.InCookie {
			continue
		}

		value, errs := decodeCookieParameter(def, cookies)
		if len(errs) > 0 {
			decodeErrors = append(decodeErrors, errs...)

			continue
		}

		if results == nil {
			results = make(map[string]any)
		}

		results[def.Name] = value
	}

	if len(decodeErrors) > 0 {
		return nil, decodeErrors
	}

	return results, nil
}

func decodeCookieParameter(
	definition *oaschema.Parameter,
	cookies []*http.Cookie,
) (any, []httperror.ValidationError) {
	if definition.Content != nil {
		rawValues, err := findCookies(cookies, definition)
		if err != nil {
			return nil, []httperror.ValidationError{*err}
		}

		result, errs := decodeParameterFromContent(definition, rawValues)
		if len(errs) > 0 {
			return nil, enrichCookieErrors(errs, definition.Name)
		}

		return result, nil
	}

	if definition.Schema == nil {
		rawValues, err := findCookies(cookies, definition)
		if err != nil {
			return nil, []httperror.ValidationError{*err}
		}

		rawResults := splitArrayParams(rawValues)

		return normalizeRawParamValue(rawResults), nil
	}

	schemaTypes, nullable := oaschema.GetSchemaTypes(definition.Schema)

	if len(cookies) == 0 {
		if nullable {
			return nil, nil
		}

		return nil, []httperror.ValidationError{
			{
				Code:   oasvalidator.ErrCodeCookieRequired,
				Detail: "Cookie must not be null",
				Header: definition.Name,
			},
		}
	}

	if slices.Contains(schemaTypes, oaschema.Object) {
		result, errs := decodeCookieObjectParameter(definition, cookies)
		if len(errs) == 0 {
			return result, nil
		}

		if len(schemaTypes) == 1 {
			return nil, enrichCookieErrors(errs, definition.Name)
		}

		schemaTypes = slices.DeleteFunc(schemaTypes, func(t string) bool {
			return t == oaschema.Object
		})
	}

	rawValues, err := findCookies(cookies, definition)
	if err != nil {
		return nil, []httperror.ValidationError{*err}
	}

	if slices.Contains(schemaTypes, oaschema.Array) {
		result, errs := splitAndDecodeArrayParam(definition, rawValues)
		if len(errs) == 0 {
			return result, nil
		}

		if len(schemaTypes) == 1 {
			return nil, enrichCookieErrors(errs, definition.Name)
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
		return nil, enrichCookieErrors(errs, definition.Name)
	}

	return result, nil
}

func decodeCookieObjectParameter(
	definition *oaschema.Parameter,
	cookies []*http.Cookie,
) (any, []httperror.ValidationError) {
	explode := definition.Explode == nil || *definition.Explode
	if explode {
		// R=100; G=200; B=150
		cookieMap := createCookieMap(cookies)
		decoder := newObjectParamDecoder(cookieMap)

		decodeErrs := decoder.Decode(definition.Schema)
		if len(decodeErrs) == 0 {
			return decoder.Result, nil
		}

		return nil, decodeErrs
	}

	rawValues, err := findCookies(cookies, definition)
	if err != nil {
		return nil, []httperror.ValidationError{*err}
	}

	// color=R,100,G,200,B,150
	rawObjectValues, ok := parseNonExplodeObject(splitArrayParams(rawValues))
	if !ok {
		return nil, []httperror.ValidationError{
			{
				Code:   oasvalidator.ErrCodeInvalidCookie,
				Detail: "Invalid syntax for non-exploded style in cookie value. The object value must follow this format: key1,value1,key2,value2",
			},
		}
	}

	decoder := newObjectParamDecoder(rawObjectValues)

	errs := decoder.Decode(definition.Schema)
	if len(errs) == 0 {
		return decoder.Result, nil
	}

	return nil, errs
}

func createCookieMap(cookies []*http.Cookie) map[string][]string {
	if len(cookies) == 0 {
		return nil
	}

	results := make(map[string][]string)

	for _, cookie := range cookies {
		_, present := results[cookie.Name]
		if !present {
			results[cookie.Name] = []string{cookie.Value}
		} else {
			results[cookie.Name] = append(results[cookie.Name], cookie.Value)
		}
	}

	return results
}

func findCookies(
	cookies []*http.Cookie,
	definition *oaschema.Parameter,
) ([]string, *httperror.ValidationError) {
	var results []string

	for _, cookie := range cookies {
		if cookie.Name == definition.Name {
			results = append(results, cookie.Value)
		}
	}

	if len(results) == 0 {
		if !definition.Required {
			return nil, nil
		}

		return nil, &httperror.ValidationError{
			Code:   oasvalidator.ErrCodeCookieRequired,
			Detail: "Cookie is required",
			Header: definition.Name,
		}
	}

	return results, nil
}

// enrichCookieErrors stamps each error with the header error code and the header name.
func enrichCookieErrors(errs []httperror.ValidationError, name string) []httperror.ValidationError {
	for i := range errs {
		errs[i].Code = oasvalidator.ErrCodeInvalidCookie
		errs[i].Parameter = name
	}

	return errs
}
