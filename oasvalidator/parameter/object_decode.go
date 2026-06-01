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
	"log/slog"
	"maps"
	"slices"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/relychan/goutils/httperror"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/oasvalidator"
	"github.com/relychan/openapitools/oasvalidator/regexps"
)

// objectParamDecoder decodes a flat map[string][]string (from an exploded or non-exploded object
// parameter) into a typed map[string]any guided by an OpenAPI schema.
type objectParamDecoder struct {
	RawValues map[string][]string
	Result    map[string]any
}

// newObjectParamDecoder creates an objectParamDecoder from the given raw key→values map.
func newObjectParamDecoder(rawValues map[string][]string) *objectParamDecoder {
	return &objectParamDecoder{
		RawValues: rawValues,
		Result:    map[string]any{},
	}
}

// Decode decodes all raw values into Result using schema, resolving allOf, anyOf, and oneOf.
func (opd *objectParamDecoder) Decode(schema *base.Schema) []httperror.ValidationError {
	if oaschema.IsSchemaObjectEmpty(schema) {
		opd.Result = normalizeRawObjectValues(opd.RawValues)

		return nil
	}

	errs := opd.decodeProperties(schema)
	if len(errs) > 0 {
		return errs
	}

	for _, ao := range schema.AllOf {
		if ao == nil {
			continue
		}

		aoSchema := ao.Schema()
		if aoSchema == nil {
			continue
		}

		decodeErrs := opd.Decode(aoSchema)
		if len(decodeErrs) > 0 {
			errs = append(errs, decodeErrs...)
		}
	}

	if len(errs) > 0 {
		return errs
	}

	if len(schema.AnyOf) > 0 {
		var anyOfSuccess bool

		for _, ao := range schema.AnyOf {
			decodeErrs := opd.decodeOrUnionItem(ao)
			if len(decodeErrs) > 0 {
				errs = append(errs, decodeErrs...)

				continue
			}

			anyOfSuccess = true
		}

		if !anyOfSuccess {
			return errs
		}
	}

	if len(schema.OneOf) > 0 {
		var oneOfSuccess bool

		for _, oo := range schema.OneOf {
			decodeErrs := opd.decodeOrUnionItem(oo)
			if len(decodeErrs) > 0 {
				errs = append(errs, decodeErrs...)

				continue
			}

			oneOfSuccess = true

			break
		}

		if !oneOfSuccess {
			return errs
		}
	}

	errFuncs := oasvalidator.ValidateObject(schema, opd.Result)
	if len(errFuncs) > 0 {
		return oasvalidator.CollectErrors(errFuncs)
	}

	return nil
}

// decodeProperties decodes known schema properties then falls through to additional
// and pattern properties for any keys not yet accounted for.
func (opd *objectParamDecoder) decodeProperties(
	schema *base.Schema,
) []httperror.ValidationError {
	var (
		parsedKeys = make([]string, 0, len(opd.RawValues))
		errs       []httperror.ValidationError
	)

	if schema.Properties != nil {
		for iter := schema.Properties.First(); iter != nil; iter = iter.Next() {
			key := iter.Key()

			value, ok := opd.RawValues[key]
			if !ok {
				continue
			}

			parsedKeys = append(parsedKeys, key)

			propSchemaProxy := iter.Value()
			if propSchemaProxy == nil {
				opd.Result[key] = normalizeRawParamValue(value)

				continue
			}

			propSchema := propSchemaProxy.Schema()

			decodeErrs := opd.decodeProperty(key, value, propSchema)
			if len(decodeErrs) > 0 {
				errs = append(errs, decodeErrs...)
			}
		}
	}

	if len(errs) > 0 {
		return errs
	}

	errs = opd.decodeObjectAdditionalProperties(schema, parsedKeys)
	if len(errs) > 0 {
		return errs
	}

	errs = opd.decodeObjectPatternProperties(schema, parsedKeys)
	if len(errs) > 0 {
		return errs
	}

	return nil
}

// decodeOrUnionItem decodes a single anyOf/oneOf branch and merges its result into opd.Result.
func (opd *objectParamDecoder) decodeOrUnionItem(
	proxy *base.SchemaProxy,
) []httperror.ValidationError {
	if proxy == nil {
		return nil
	}

	aoSchema := proxy.Schema()
	if aoSchema == nil {
		return nil
	}

	if oaschema.IsSchemaObjectEmpty(aoSchema) {
		errFuncs := oasvalidator.ValidateObject(aoSchema, opd.Result)
		if len(errFuncs) > 0 {
			return oasvalidator.CollectErrors(errFuncs)
		}

		return nil
	}

	ooDecoder := newObjectParamDecoder(opd.RawValues)

	decodeErrs := ooDecoder.Decode(aoSchema)
	if len(decodeErrs) > 0 {
		return decodeErrs
	}

	if len(opd.Result) > 0 {
		maps.Copy(opd.Result, ooDecoder.Result)
	} else {
		opd.Result = ooDecoder.Result
	}

	return nil
}

// decodeObjectPatternProperties matches raw keys against schema patternProperties and decodes
// matching entries not already present in Result. Regex compile failures are logged and skipped.
func (opd *objectParamDecoder) decodeObjectPatternProperties(
	schema *base.Schema,
	parsedKeys []string,
) []httperror.ValidationError {
	if schema.PatternProperties == nil || schema.PatternProperties.Len() == 0 {
		return nil
	}

	var errs []httperror.ValidationError

	for iter := schema.PatternProperties.First(); iter != nil; iter = iter.Next() {
		rawPattern := iter.Key()

		pattern, err := regexps.Get(rawPattern)
		if err != nil {
			// ignore compile error on runtime.
			slog.Warn(
				"failed to compile regular expression: "+err.Error(),
				slog.String("pattern", rawPattern),
			)

			continue
		}

		var propSchema *base.Schema

		schemaProxy := iter.Value()
		if schemaProxy != nil {
			propSchema = schemaProxy.Schema()
		}

		for key, values := range opd.RawValues {
			if len(parsedKeys) > 0 && slices.Contains(parsedKeys, key) {
				continue
			}

			_, present := opd.Result[key]
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

			decodeErrs := opd.decodeProperty(key, values, propSchema)
			if len(decodeErrs) > 0 {
				errs = append(errs, decodeErrs...)
			}
		}
	}

	return errs
}

// decodeObjectAdditionalProperties decodes raw keys not already parsed by explicit or pattern
// properties. Skips when AdditionalProperties is absent or false.
func (opd *objectParamDecoder) decodeObjectAdditionalProperties(
	schema *base.Schema,
	parsedKeys []string,
) []httperror.ValidationError {
	if schema.AdditionalProperties == nil ||
		(!schema.AdditionalProperties.B && schema.AdditionalProperties.A == nil) {
		return nil
	}

	var (
		propSchema *base.Schema
		errs       []httperror.ValidationError
	)

	if schema.AdditionalProperties.N == 0 && schema.AdditionalProperties.A != nil {
		propSchema = schema.AdditionalProperties.A.Schema()
	}

	for key, rawValues := range opd.RawValues {
		if len(parsedKeys) > 0 && slices.Contains(parsedKeys, key) {
			continue
		}

		_, present := opd.Result[key]
		if present {
			continue
		}

		decodeErrs := opd.decodeProperty(key, rawValues, propSchema)
		if len(decodeErrs) > 0 {
			errs = append(errs, decodeErrs...)
		}
	}

	return errs
}

// decodeProperty decodes a single named property using paramDecoder and stores the result.
// Pointer prefixes are prepended to validation errors for precise error location.
func (opd *objectParamDecoder) decodeProperty(
	key string,
	rawValues []string,
	schema *base.Schema,
) []httperror.ValidationError {
	propDecoder := paramDecoder{
		RawValues: rawValues,
		Schema:    schema,
	}

	propValue, decodeErrs := propDecoder.Decode(nil)
	if len(decodeErrs) == 0 {
		opd.Result[key] = propValue

		return nil
	}

	for i := range decodeErrs {
		decodeErrs[i].PrependPointer("/" + key)
	}

	return decodeErrs
}
