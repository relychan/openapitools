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
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/pb33f/libopenapi/datamodel/high/base"
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
	Value     any
	Schema    *base.Schema
}

type queryParamObjectDecoder struct {
	RawValues map[string][]string
	Result    map[string]any
}

func createQueryParamObjectDecoder(rawValues map[string][]string) *queryParamObjectDecoder {
	return &queryParamObjectDecoder{
		RawValues: rawValues,
		Result:    map[string]any{},
	}
}

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
		decoder := createQueryParamObjectDecoder(values)

		decodeErrs := decoder.DecodeExplode(definition.Schema)
		if len(decodeErrs) > 0 {
			return nil, false, decodeErrs
		}

		return decoder.Result, true, nil
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

	if isObject {
		if !explode {
			values, isValidObject := splitNonExplodeDelimitedStyle(rawValues, style, isObject)
			if !isValidObject {
				return nil, false, []httperror.ValidationError{
					newInvalidQueryNonExplodedObjectError(definition.Name, style),
				}
			}

			rawValues = values
		}

		rawObjectValue, ok := parseNonExplodeQueryObject(rawValues)
		if !ok {
			return nil, false, []httperror.ValidationError{
				{
					Detail: "malformed object syntax for query style: " + style.String(),
					Code:   oasvalidator.ErrCodeInvalidQueryParam,
				},
			}
		}

		decoder := createQueryParamObjectDecoder(rawObjectValue)

		errs := decoder.Decode(definition.Schema)
		if len(errs) > 0 {
			return nil, false, errs
		}

		return decoder.Result, true, nil
	}

	decoder := &queryParamDecoder{
		Name:      definition.Name,
		Schema:    definition.Schema,
		Types:     schemaTypes,
		RawValues: rawValues,
	}

	if !explode {
		values, _ := splitNonExplodeDelimitedStyle(rawValues, style, false)
		decoder.RawValues = values
	}

	if oaschema.IsSchemaTypeEmpty(definition.Schema) {
		return decoder.RawValues, true, nil
	}

	itemResults, decodeErrs := decoder.Decode()
	if len(decodeErrs) > 0 {
		return nil, false, decodeErrs
	}

	if itemResults == nil && !nullable {
		err := oasvalidator.ParameterRequiredError(definition.Name)

		return nil, false, []httperror.ValidationError{*err}
	}

	return itemResults, true, nil
}

// Decode evaluates and decodes URL parameters.
func (qpe *queryParamDecoder) Decode() (any, []httperror.ValidationError) { //nolint:gocognit,cyclop,funlen
	result, resultType, errs := qpe.decodeFromSchemaTypes()
	if len(errs) > 0 {
		return nil, errs
	}

	if len(qpe.Schema.AllOf) > 0 {
		allOf := oaschema.ExtractSchemaProxies(qpe.Schema.AllOf)
		schemaTypes, _ := oaschema.GetUnionSchemaTypes(allOf)

		if resultType != "" && len(schemaTypes) > 0 {
			if !slices.Contains(schemaTypes, resultType) {
				return nil, []httperror.ValidationError{
					{
						Code: oasvalidator.ErrCodeOpenAPISchemaError,
						Detail: "Mismatched types in allOf [" + strings.Join(schemaTypes, ", ") +
							"] and schema types [" + strings.Join(qpe.Types, ", ") + "]",
						Parameter: qpe.Name,
					},
				}
			}

			schemaTypes = []string{resultType}
		}

		for _, unionSchema := range allOf {
			decoder := &queryParamDecoder{
				Name:   qpe.Name,
				Schema: unionSchema,
				Types:  schemaTypes,
				Value:  result,
			}

			aor, errs := decoder.Decode()
			if len(errs) > 0 {
				return nil, errs
			}

			qpe.Value = aor
		}
	}

	if len(qpe.Schema.AnyOf) > 0 {
		anyOf := oaschema.ExtractSchemaProxies(qpe.Schema.AnyOf)
		schemaTypes, _ := oaschema.GetUnionSchemaTypes(anyOf)

		if resultType != "" && len(schemaTypes) > 0 {
			if !slices.Contains(schemaTypes, resultType) {
				return nil, []httperror.ValidationError{
					{
						Code: oasvalidator.ErrCodeOpenAPISchemaError,
						Detail: "Mismatched types in anyOf [" + strings.Join(schemaTypes, ", ") +
							"] and schema types [" + strings.Join(qpe.Types, ", ") + "]",
						Parameter: qpe.Name,
					},
				}
			}

			schemaTypes = []string{resultType}
		}

		var (
			anyOfErrs      []httperror.ValidationError
			isAnyOfSuccess bool
		)

		for _, unionSchema := range anyOf {
			decoder := &queryParamDecoder{
				Name:   qpe.Name,
				Schema: unionSchema,
				Types:  schemaTypes,
				Value:  result,
			}

			aor, errs := decoder.Decode()
			if len(errs) > 0 {
				anyOfErrs = append(anyOfErrs, errs...)

				continue
			}

			qpe.Value = aor
			isAnyOfSuccess = true
		}

		if !isAnyOfSuccess {
			return nil, anyOfErrs
		}
	}

	if len(qpe.Schema.OneOf) > 0 {
		oneOf := oaschema.ExtractSchemaProxies(qpe.Schema.OneOf)
		schemaTypes, _ := oaschema.GetUnionSchemaTypes(oneOf)

		if resultType != "" && len(schemaTypes) > 0 {
			if !slices.Contains(schemaTypes, resultType) {
				return nil, []httperror.ValidationError{
					{
						Code: oasvalidator.ErrCodeOpenAPISchemaError,
						Detail: "Mismatched types in oneOf [" + strings.Join(schemaTypes, ", ") +
							"] and schema types [" + strings.Join(qpe.Types, ", ") + "]",
						Parameter: qpe.Name,
					},
				}
			}

			schemaTypes = []string{resultType}
		}

		var (
			oneOfErrs      []httperror.ValidationError
			isOneOfSuccess bool
		)

		for _, unionSchema := range oneOf {
			decoder := &queryParamDecoder{
				Name:   qpe.Name,
				Schema: unionSchema,
				Types:  schemaTypes,
				Value:  result,
			}

			oor, errs := decoder.Decode()
			if len(errs) > 0 {
				oneOfErrs = append(oneOfErrs, errs...)

				continue
			}

			qpe.Value = oor
			isOneOfSuccess = true

			break
		}

		if !isOneOfSuccess {
			return nil, oneOfErrs
		}
	}

	return result, errs
}

// decodeFromSchemaTypes decodes the raw values by trying each type declared in the schema.
// String is given priority: if the schema allows string the raw value is returned as-is
// to avoid lossy parsing (e.g. a numeric string "007" would become 7).
func (qpe *queryParamDecoder) decodeFromSchemaTypes() (any, string, []httperror.ValidationError) {
	if len(qpe.RawValues) == 0 {
		return qpe.Value, "", nil
	}

	var finalErrors []httperror.ValidationError

	for _, typeName := range qpe.Types {
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
		rawObjectValue, ok := parseNonExplodeQueryObject(qpe.RawValues)
		if !ok {
			return nil, "", []httperror.ValidationError{
				{
					Detail: "malformed object syntax for query style",
					Code:   oasvalidator.ErrCodeInvalidQueryParam,
				},
			}
		}

		decoder := createQueryParamObjectDecoder(rawObjectValue)

		errs := decoder.Decode(qpe.Schema)
		if len(errs) > 0 {
			return nil, oaschema.Object, errs
		}

		return decoder.Result, oaschema.Object, nil
	default:
		if qpe.Value != nil {
			_, ok := qpe.Value.(string)
			if !ok {
				return nil, "", []httperror.ValidationError{
					*oasvalidator.InvalidTypeError([]string{typeName}, reflect.TypeOf(qpe.Value).String()),
				}
			}

			return qpe.Value, oaschema.String, nil
		}

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

func (qpod *queryParamObjectDecoder) Decode(schema *base.Schema) []httperror.ValidationError {
	if oaschema.IsSchemaObjectEmpty(schema) {
		for key, values := range qpod.RawValues {
			qpod.Result[key] = values
		}

		return nil
	}

	errs := qpod.decodeProperties(schema)
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

		decodeErrs := qpod.Decode(aoSchema)
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
			decodeErrs := qpod.decodeOrUnionItem(ao)
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

	for _, oo := range schema.OneOf {
		decodeErrs := qpod.decodeOrUnionItem(oo)
		if len(decodeErrs) > 0 {
			errs = append(errs, decodeErrs...)

			continue
		}

		break
	}

	errFuncs := oasvalidator.ValidateObject(schema, qpod.Result)
	if len(errFuncs) > 0 {
		return oasvalidator.CollectErrors(errFuncs)
	}

	return nil
}

func (qpod *queryParamObjectDecoder) DecodeExplode(
	schema *base.Schema,
) []httperror.ValidationError {
	// /users?role=admin&firstName=Alex
	var (
		parsedKeys = make([]string, 0, len(qpod.RawValues))
		errs       []httperror.ValidationError
	)

	if schema.Properties != nil {
		for iter := schema.Properties.First(); iter != nil; iter = iter.Next() {
			key := iter.Key()

			rawValues, present := qpod.RawValues[key]
			if !present {
				if len(schema.Required) > 0 && slices.Contains(schema.Required, key) {
					err := oasvalidator.ObjectRequiredPropertyError(key)

					errs = append(errs, *err)
				}

				continue
			}

			parsedKeys = append(parsedKeys, key)

			schemaProxy := iter.Value()
			if schemaProxy == nil {
				qpod.Result[key] = rawValues

				continue
			}

			propSchema := schemaProxy.Schema()
			if oaschema.IsSchemaTypeEmpty(propSchema) {
				qpod.Result[key] = rawValues

				continue
			}

			schemaTypes, _ := oaschema.GetSchemaTypes(propSchema)
			propDecoder := &queryParamDecoder{
				Name:      key,
				RawValues: rawValues,
				Schema:    propSchema,
				Types:     schemaTypes,
			}

			value, decodeErrs := propDecoder.Decode()
			if len(decodeErrs) == 0 {
				qpod.Result[key] = value

				continue
			}

			errs = addParameterErrors(errs, decodeErrs, key)
		}
	}

	if len(errs) > 0 {
		return errs
	}

	errs = qpod.decodeObjectAdditionalProperties(schema, parsedKeys)
	if len(errs) > 0 {
		return errs
	}

	errs = qpod.decodeObjectPatternProperties(schema)
	if len(errs) > 0 {
		return errs
	}

	return nil
}

func (qpod *queryParamObjectDecoder) decodeOrUnionItem(
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
		errFuncs := oasvalidator.ValidateObject(aoSchema, qpod.Result)
		if len(errFuncs) > 0 {
			return oasvalidator.CollectErrors(errFuncs)
		}

		return nil
	}

	ooDecoder := createQueryParamObjectDecoder(qpod.RawValues)

	decodeErrs := ooDecoder.Decode(aoSchema)
	if len(decodeErrs) > 0 {
		return decodeErrs
	}

	if len(qpod.Result) > 0 {
		maps.Copy(qpod.Result, ooDecoder.Result)
	} else {
		qpod.Result = ooDecoder.Result
	}

	return nil
}

func (qpod *queryParamObjectDecoder) decodeProperties(
	schema *base.Schema,
) []httperror.ValidationError {
	var errs []httperror.ValidationError

	if schema.Properties != nil {
		for iter := schema.Properties.First(); iter != nil; iter = iter.Next() {
			key := iter.Key()

			value, ok := qpod.RawValues[key]
			if !ok {
				continue
			}

			propSchemaProxy := iter.Value()
			if propSchemaProxy == nil {
				qpod.Result[key] = value

				continue
			}

			propSchema := propSchemaProxy.Schema()
			if oaschema.IsSchemaTypeEmpty(propSchema) {
				qpod.Result[key] = value

				continue
			}

			schemaTypes, _ := oaschema.GetSchemaTypes(propSchema)
			propDecoder := &queryParamDecoder{
				Name:      key,
				RawValues: value,
				Schema:    propSchema,
				Types:     schemaTypes,
			}

			propValue, decodeErrs := propDecoder.Decode()
			if len(decodeErrs) == 0 {
				qpod.Result[key] = propValue

				continue
			}

			errs = append(errs, decodeErrs...)
		}
	}

	if len(errs) > 0 {
		return errs
	}

	errs = qpod.decodeObjectAdditionalProperties(schema, nil)
	if len(errs) > 0 {
		return errs
	}

	errs = qpod.decodeObjectPatternProperties(schema)
	if len(errs) > 0 {
		return errs
	}

	return nil
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

func (qpod *queryParamObjectDecoder) decodeObjectPatternProperties(
	schema *base.Schema,
) []httperror.ValidationError {
	if schema.PatternProperties == nil || schema.PatternProperties.Len() == 0 {
		return nil
	}

	var errs []httperror.ValidationError

	for iter := schema.PatternProperties.First(); iter != nil; iter = iter.Next() {
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

		for key, values := range qpod.RawValues {
			_, present := qpod.Result[key]
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
				qpod.Result[key] = values

				continue
			}

			schemaTypes, _ := oaschema.GetSchemaTypes(propSchema)
			propDecoder := &queryParamDecoder{
				Name:      key,
				RawValues: values,
				Schema:    propSchema,
				Types:     schemaTypes,
			}

			value, decodeErrs := propDecoder.Decode()
			if len(decodeErrs) == 0 {
				qpod.Result[key] = value

				continue
			}

			errs = addParameterErrors(errs, decodeErrs, key)
		}
	}

	return errs
}

func (qpod *queryParamObjectDecoder) decodeObjectAdditionalProperties(
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

	for key, rawValues := range qpod.RawValues {
		if len(parsedKeys) > 0 && slices.Contains(parsedKeys, key) {
			continue
		}

		_, present := qpod.Result[key]
		if present {
			continue
		}

		if oaschema.IsSchemaTypeEmpty(propSchema) {
			qpod.Result[key] = rawValues

			continue
		}

		schemaTypes, _ := oaschema.GetSchemaTypes(propSchema)
		propDecoder := &queryParamDecoder{
			Name:      key,
			RawValues: rawValues,
			Schema:    propSchema,
			Types:     schemaTypes,
		}

		value, decodeErrs := propDecoder.Decode()
		if len(decodeErrs) == 0 {
			qpod.Result[key] = value

			continue
		}

		errs = addParameterErrors(errs, decodeErrs, key)
	}

	return errs
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
	if len(rawValues) == 0 || (len(rawValues) == 1 && rawValues[0] == "") {
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
					Code:   oasvalidator.ErrCodeInvalidQueryParam,
					Detail: err.Error(),
				},
			}
		}

		return result, primitiveType, nil
	}

	return rawValues, typeName, nil
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

func splitNonExplodeDelimitedStyle(
	rawValues []string,
	style oaschema.ParameterEncodingStyle,
	isObject bool,
) ([]string, bool) {
	if len(rawValues) == 0 {
		return rawValues, true
	}

	switch style {
	case oaschema.EncodingStyleSpaceDelimited:
		// /users?id=3 4 5
		return parseDelimitedStyle(rawValues, oaschema.Space, isObject)
	case oaschema.EncodingStylePipeDelimited:
		// /users?id=3|4|5
		return parseDelimitedStyle(rawValues, oaschema.Pipe, isObject)
	default:
		// /users?id=3,4,5
		return parseDelimitedStyle(rawValues, oaschema.Comma, isObject)
	}
}

// Set delimited-separated array values for array params.
// For example: /users?id=3|4|5.
func parseDelimitedStyle(
	rawValues []string,
	separator string,
	isObject bool,
) ([]string, bool) {
	results := make([]string, 0, len(rawValues))

	for _, value := range rawValues {
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
