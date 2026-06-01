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
	"strconv"
	"strings"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/relychan/goutils"
	"github.com/relychan/goutils/httperror"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/oasvalidator"
	"github.com/relychan/openapitools/oasvalidator/regexps"
)

// ParameterNodes is a flat list of root-level ParameterNode entries, one per distinct top-level key.
// It is used as the working set when inserting and decoding deep-object query parameters.
type ParameterNodes []*ParameterNode

// Find returns the first node whose key equals key, or nil if not found.
func (pn ParameterNodes) Find(key ParamSelector) *ParameterNode {
	for _, node := range pn {
		if node.key.Equal(key) {
			return node
		}
	}

	return nil
}

// Insert inserts a leaf at the path described by keys, creating intermediate nodes as needed.
func (pn *ParameterNodes) Insert(keys ParamKeys, values []string) *httperror.ValidationError {
	if len(keys) == 0 {
		return nil
	}

	for _, vs := range *pn {
		if vs.key.Equal(keys[0]) {
			err := vs.InsertNode(keys[1:], values)
			if err != nil {
				err.Parameter = keys[0].String()

				return err
			}

			return nil
		}
	}

	node := &ParameterNode{
		key: keys[0],
	}

	*pn = append(*pn, node)

	err := node.InsertNode(keys[1:], values)
	if err != nil {
		err.Parameter = keys[0].String()

		return err
	}

	return nil
}

// String implements fmt.Stringer for debugging.
func (pn ParameterNodes) String() string {
	if len(pn) == 0 {
		return ""
	}

	var sb strings.Builder

	for i, node := range pn {
		if i > 0 {
			sb.WriteByte('\n')
		}

		sb.WriteString(node.String())
	}

	return sb.String()
}

// ParameterNode is a single node in the decoded parameter tree.
// Leaf nodes (no items) carry raw string values; branch nodes carry child nodes keyed by
// ParamKey (object property) or ParamIndex (array element).
type ParameterNode struct {
	key    ParamSelector
	values []string
	items  ParameterNodes
}

// FindNode returns the direct child node matching key, or nil if absent.
func (pn *ParameterNode) FindNode(key ParamSelector) *ParameterNode {
	if len(pn.items) == 0 {
		return nil
	}

	return pn.items.Find(key)
}

// Normalize resolves ambiguities that arise from deep-object query parsing:
//   - A single numeric ParamKey child ("0") is promoted to ParamIndex so the node is
//     treated as an array element rather than an object property.
//   - A single ParamIndex child with no further children is collapsed: its values are
//     hoisted into the parent, flattening a one-element array.
//   - Multi-element arrays are sorted by index so later schema validation sees items in order.
func (pn *ParameterNode) Normalize() {
	if len(pn.items) == 0 {
		return
	}

	if len(pn.items) == 1 {
		switch key := pn.items[0].key.(type) {
		case ParamIndex:
			if len(pn.items[0].items) == 0 {
				pn.values = pn.items[0].values
				pn.items = nil

				return
			}
		case ParamKey:
			index, err := strconv.Atoi(string(key))
			if err == nil {
				pn.items[0].key = ParamIndex(index)
			}
		default:
		}

		pn.items[0].Normalize()

		return
	}

	// skip sorting object keys.
	if IsParamIndex(pn.items[0].key) {
		slices.SortFunc(pn.items, compareParameterNodes)
	}

	for _, item := range pn.items {
		item.Normalize()
	}
}

// InsertNode walks (or creates) the path described by keys and stores values at the leaf.
// When mixing ParamKey and ParamIndex siblings it tries to coerce the odd type so that
// the whole level is consistently typed; it returns an error if coercion is impossible.
func (pn *ParameterNode) InsertNode(keys ParamKeys, values []string) *httperror.ValidationError {
	if len(keys) == 0 {
		pn.values = values

		return nil
	}

	// best-effort to converting the key to index if other keys in the list are indexes.
	switch selector := keys[0].(type) {
	case ParamIndex:
		if len(pn.items) >= 1 {
			key, ok := pn.items[0].key.(ParamKey)
			if ok {
				indexKey, err := strconv.Atoi(string(key))
				if err != nil {
					return newMixedArrayAndObjectError()
				}

				pn.items[0].key = ParamIndex(indexKey)
			}
		}
	case ParamKey:
		if len(pn.items) >= 1 {
			_, ok := pn.items[0].key.(ParamIndex)
			if ok {
				indexKey, err := strconv.Atoi(string(selector))
				if err != nil {
					return newMixedArrayAndObjectError()
				}

				keys[0] = ParamIndex(indexKey)
			}
		}
	default:
	}

	for _, item := range pn.items {
		if item.key.Equal(keys[0]) {
			return item.InsertNode(keys[1:], values)
		}
	}

	item := &ParameterNode{
		key: keys[0],
	}

	pn.items = append(pn.items, item)

	return item.InsertNode(keys[1:], values)
}

func (pn ParameterNode) String() string {
	return pn.printIndent(0)
}

// Decode resolves the node's raw values into a typed Go value guided by schema.
// Object and array schema types are tried first; remaining scalar types are tried in declaration order.
func (pn *ParameterNode) Decode(schema *base.Schema) (any, []httperror.ValidationError) {
	if oaschema.IsSchemaTypeEmpty(schema) {
		return pn.decodeArbitrary(), nil
	}

	schemaTypes, nullable := oaschema.GetSchemaTypes(schema)
	if len(pn.values) == 0 && len(pn.items) == 0 {
		if nullable {
			return nil, nil
		}

		return nil, []httperror.ValidationError{
			{
				Code:      oasvalidator.ErrCodeInvalidQueryParam,
				Parameter: pn.key.String(),
				Detail:    "Parameter must not be null",
			},
		}
	}

	if slices.Contains(schemaTypes, oaschema.Object) {
		result, errs := pn.decodeObject(schema)
		if len(errs) == 0 {
			return result, nil
		}

		if len(schemaTypes) == 1 {
			return nil, errs
		}

		schemaTypes = slices.DeleteFunc(schemaTypes, func(t string) bool {
			return t == oaschema.Object
		})
	}

	if slices.Contains(schemaTypes, oaschema.Array) {
		result, errs := pn.decodeArray(schema)
		if len(errs) == 0 {
			return result, nil
		}

		if len(schemaTypes) == 1 {
			return nil, errs
		}

		schemaTypes = slices.DeleteFunc(schemaTypes, func(t string) bool {
			return t == oaschema.Array
		})
	}

	result, _, errs := pn.decodeFromSchemaTypes(schemaTypes)
	if len(errs) > 0 {
		return nil, errs
	}

	errFuncs := oasvalidator.ValidateValue(schema, result)
	if len(errFuncs) > 0 {
		return nil, enrichQueryErrors(errs, pn.key.String())
	}

	return result, nil
}

// decodeFromSchemaTypes tries each declared schema type in order and returns the first
// successful decode. The empty type and "null" are skipped; errors from the last failing
// type are surfaced when all types fail.
func (pn *ParameterNode) decodeFromSchemaTypes(
	schemaTypes []string,
) (any, string, []httperror.ValidationError) {
	var finalErrors []httperror.ValidationError

	for _, typeName := range schemaTypes {
		result, resultType, errs := decodePrimitiveQueryValuesFromSchemaType(typeName, pn.values)
		if len(errs) == 0 {
			return result, resultType, nil
		}

		finalErrors = errs
	}

	return nil, "", finalErrors
}

// decodeArray decodes the node as an array value and runs schema-level array validation.
func (pn *ParameterNode) decodeArray(
	schemaDef *base.Schema,
) (any, []httperror.ValidationError) {
	results, errs := pn.decodeArrayValues(schemaDef)
	if len(errs) > 0 {
		return nil, errs
	}

	errs = oasvalidator.ValidateValue(schemaDef, results)
	if len(errs) > 0 {
		return nil, enrichQueryErrors(errs, pn.key.String())
	}

	return results, nil
}

// decodeArrayValues picks the decoding strategy (arbitrary vs. schema-guided) and
// recurses into child nodes for nested deep-object arrays.
func (pn *ParameterNode) decodeArrayValues(
	schemaDef *base.Schema,
) (any, []httperror.ValidationError) {
	if schemaDef.Items == nil || schemaDef.Items.IsB() || schemaDef.Items.A == nil {
		return pn.decodeArbitraryArray(), nil
	}

	itemSchema := schemaDef.Items.A.Schema()
	if oaschema.IsSchemaTypeEmpty(itemSchema) {
		return pn.decodeArbitraryArray(), nil
	}

	if len(pn.items) == 0 {
		return decodeArrayParamWithItemSchema(pn.values, itemSchema)
	}

	var (
		results = make([]any, 0, len(pn.items))
		errs    []httperror.ValidationError
	)

	for _, item := range pn.items {
		itemValue, decodeErrors := item.Decode(itemSchema)
		if len(decodeErrors) > 0 {
			errs = append(errs, decodeErrors...)
		} else {
			results = append(results, itemValue)
		}
	}

	return results, errs
}

// decodeObject decodes the node as a schema-typed object, processing allOf, anyOf,
// and oneOf sub-schemas, then running object-level validation.
func (pn *ParameterNode) decodeObject(
	schemaDef *base.Schema,
) (map[string]any, []httperror.ValidationError) {
	results := make(map[string]any)

	errs := pn.decodeObjectProperties(schemaDef, results)
	if len(errs) > 0 {
		return nil, errs
	}

	for _, ao := range schemaDef.AllOf {
		if ao == nil {
			continue
		}

		aoSchema := ao.Schema()
		if aoSchema == nil {
			continue
		}

		decodeErrs := pn.decodeObjectProperties(aoSchema, results)
		if len(decodeErrs) > 0 {
			errs = append(errs, decodeErrs...)
		}
	}

	if len(errs) > 0 {
		return nil, errs
	}

	if len(schemaDef.AnyOf) > 0 {
		var anyOfSuccess bool

		for _, ao := range schemaDef.AnyOf {
			decodeErrs := pn.decodeObjectOr(ao, results)
			if len(decodeErrs) > 0 {
				errs = append(errs, decodeErrs...)

				continue
			}

			anyOfSuccess = true
		}

		if !anyOfSuccess {
			return nil, errs
		}
	}

	if len(schemaDef.OneOf) > 0 {
		var oneOfSuccess bool

		for _, oo := range schemaDef.OneOf {
			decodeErrs := pn.decodeObjectOr(oo, results)
			if len(decodeErrs) > 0 {
				errs = append(errs, decodeErrs...)

				continue
			}

			oneOfSuccess = true

			break
		}

		if !oneOfSuccess {
			return nil, errs
		}
	}

	errs = oasvalidator.ValidateObjectWithoutProperties(schemaDef, results)
	if len(errs) > 0 {
		return nil, enrichQueryErrors(errs, pn.key.String())
	}

	return results, nil
}

// decodeObjectOr decodes a single anyOf/oneOf branch and merges successful properties into results.
func (pn *ParameterNode) decodeObjectOr(
	proxy *base.SchemaProxy,
	results map[string]any,
) []httperror.ValidationError {
	if proxy == nil {
		return nil
	}

	aoSchema := proxy.Schema()
	if aoSchema == nil {
		return nil
	}

	if oaschema.IsSchemaObjectEmpty(aoSchema) {
		errs := oasvalidator.ValidateObjectWithoutProperties(aoSchema, results)
		if len(errs) > 0 {
			return errs
		}

		return nil
	}

	orResults, errs := pn.decodeObject(aoSchema)
	if len(errs) > 0 {
		return errs
	}

	maps.Copy(results, orResults)

	return nil
}

// decodeObjectProperties decodes all known schema properties, then delegates to additional
// and pattern property decoders for any remaining child nodes.
func (pn *ParameterNode) decodeObjectProperties(
	schemaDef *base.Schema,
	results map[string]any,
) []httperror.ValidationError {
	var errs []httperror.ValidationError

	if schemaDef.Properties != nil {
		for iter := schemaDef.Properties.First(); iter != nil; iter = iter.Next() {
			key := iter.Key()

			propNode := pn.FindNode(ParamKey(key))
			if propNode == nil {
				if len(schemaDef.Required) > 0 && slices.Contains(schemaDef.Required, key) {
					err := oasvalidator.ObjectRequiredPropertyError(key)
					err.Pointer = "/" + key

					errs = append(errs, *err)
				}

				continue
			}

			schemaProxy := iter.Value()
			if schemaProxy == nil {
				results[key] = propNode.decodeArbitrary()

				continue
			}

			propSchema := schemaProxy.Schema()
			if oaschema.IsSchemaTypeEmpty(propSchema) {
				results[key] = propNode.decodeArbitrary()

				continue
			}

			value, decodeErrs := propNode.Decode(propSchema)
			if len(decodeErrs) == 0 {
				results[key] = value

				continue
			}

			errs = append(errs, decodeErrs...)
		}
	}

	if len(pn.items) == 0 || len(errs) > 0 {
		return errs
	}

	errs = pn.decodeObjectAdditionalProperties(schemaDef, results)
	if len(errs) > 0 {
		return errs
	}

	errs = pn.decodeObjectPatternProperties(schemaDef, results)
	if len(errs) > 0 {
		return errs
	}

	return nil
}

// decodeObjectPatternProperties matches node children against schema patternProperties.
// Keys already present in results (decoded from explicit properties) are skipped to avoid
// double-processing. Regex compile failures are logged and the pattern is skipped rather
// than failing the entire request.
func (pn *ParameterNode) decodeObjectPatternProperties(
	schemaDef *base.Schema,
	results map[string]any,
) []httperror.ValidationError {
	if schemaDef.PatternProperties == nil || schemaDef.PatternProperties.Len() == 0 {
		return nil
	}

	var errs []httperror.ValidationError

	for iter := schemaDef.PatternProperties.First(); iter != nil; iter = iter.Next() {
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

		for _, propNode := range pn.items {
			maybePropKey, ok := propNode.key.(ParamKey)
			if !ok {
				continue
			}

			propKey := string(maybePropKey)

			_, present := results[propKey]
			if present {
				continue
			}

			matched, err := pattern.MatchString(propKey)
			if err != nil {
				slog.Warn(
					"failed to compile pattern property: "+err.Error(),
					slog.String("pattern", key),
					slog.String("name", propKey),
				)

				continue
			}

			if !matched {
				continue
			}

			if oaschema.IsSchemaTypeEmpty(propSchema) {
				results[propKey] = propNode.decodeArbitrary()

				continue
			}

			value, decodeErrs := propNode.Decode(propSchema)
			if len(decodeErrs) == 0 {
				results[propKey] = value

				continue
			}

			errs = append(errs, decodeErrs...)
		}
	}

	return errs
}

// decodeObjectAdditionalProperties decodes node children that were not matched by
// explicit properties or patternProperties. When AdditionalProperties carries a schema
// (N==0, A!=nil) each extra key is decoded against it; when it is simply true (B) keys
// are decoded arbitrarily.
func (pn *ParameterNode) decodeObjectAdditionalProperties(
	schemaDef *base.Schema,
	results map[string]any,
) []httperror.ValidationError {
	if schemaDef.AdditionalProperties == nil ||
		(!schemaDef.AdditionalProperties.B && schemaDef.AdditionalProperties.A == nil) {
		return nil
	}

	var (
		propSchema *base.Schema
		errs       []httperror.ValidationError
	)

	if schemaDef.AdditionalProperties.N == 0 && schemaDef.AdditionalProperties.A != nil {
		propSchema = schemaDef.AdditionalProperties.A.Schema()
	}

	for _, propNode := range pn.items {
		maybePropKey, ok := propNode.key.(ParamKey)
		if !ok {
			continue
		}

		propKey := string(maybePropKey)

		_, present := results[propKey]
		if present {
			continue
		}

		if oaschema.IsSchemaTypeEmpty(propSchema) {
			results[propKey] = propNode.decodeArbitrary()

			continue
		}

		value, decodeErrs := propNode.Decode(propSchema)
		if len(decodeErrs) == 0 {
			results[propKey] = value

			continue
		}

		errs = append(errs, decodeErrs...)
	}

	return errs
}

// decodeArbitrary decodes without schema guidance: arrays when children are indexed,
// objects when children are keyed, and raw scalar/slice otherwise.
func (pn *ParameterNode) decodeArbitrary() any {
	if len(pn.items) == 0 {
		return normalizeRawParamValue(pn.values)
	}

	if IsParamIndex(pn.items[0].key) {
		return pn.decodeArbitraryArray()
	}

	results := make(map[string]any)

	pn.decodeArbitraryObject(results)

	return results
}

// decodeArbitraryArray collects child or leaf values into a []any without schema guidance.
func (pn *ParameterNode) decodeArbitraryArray() []any {
	if len(pn.items) == 0 {
		return goutils.ToAnySlice(pn.values)
	}

	results := make([]any, 0, len(pn.items))

	for _, item := range pn.items {
		results = append(results, item.decodeArbitrary())
	}

	return results
}

// decodeArbitraryObject populates results with string-keyed children decoded arbitrarily.
func (pn *ParameterNode) decodeArbitraryObject(results map[string]any) {
	for _, node := range pn.items {
		results[node.key.String()] = node.decodeArbitrary()
	}
}

// printIndent renders the node as an indented tree string for debugging.
func (pn ParameterNode) printIndent(indent int) string {
	if len(pn.items) == 0 {
		return strings.Repeat(" ", indent) + pn.key.String() +
			": [" + strings.Join(pn.values, ", ") + "]"
	}

	var sb strings.Builder

	if indent > 0 {
		sb.WriteString(strings.Repeat(" ", indent))
	}

	sb.WriteString(pn.key.String())
	sb.WriteByte(':')

	for _, node := range pn.items {
		sb.WriteByte('\n')
		sb.WriteString(node.printIndent(indent + 2))
	}

	return sb.String()
}

// compareParameterNodes sorts ParameterNodes so that ParamIndex nodes are ordered
// numerically (index -1, used as a placeholder for bare "[]", sorts last) and
// ParamKey nodes are ordered lexicographically.  Mixed-type siblings should not
// occur after normalization, but a ParamIndex always sorts before a ParamKey.
func compareParameterNodes(a, b *ParameterNode) int {
	switch sa := a.key.(type) {
	case ParamIndex:
		indexB, ok := b.key.(ParamIndex)
		if !ok {
			return -1
		}

		if sa == -1 {
			return 1
		}

		if indexB == -1 {
			return -1
		}

		return int(sa - indexB)
	case ParamKey:
		keyB, ok := b.key.(ParamKey)
		if !ok {
			return 1
		}

		return strings.Compare(string(sa), string(keyB))
	default:
		return 0
	}
}
