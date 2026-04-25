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
	"slices"
	"strconv"
	"strings"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/relychan/goutils/httperror"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/oasvalidator"
	"github.com/relychan/openapitools/oasvalidator/regexps"
)

// ParameterNodes is a flat list of root-level ParameterNode entries, one per distinct top-level key.
// It is used as the working set when inserting and decoding deep-object query parameters.
type ParameterNodes []*ParameterNode

func (pn ParameterNodes) Find(key ParamSelector) *ParameterNode {
	for _, node := range pn {
		if node.key.Equal(key) {
			return node
		}
	}

	return nil
}

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
		if len(pn.items) == 1 {
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
		if len(pn.items) > 1 {
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

func (pn *ParameterNode) Decode(typeSchema *base.Schema) (any, []httperror.ValidationError) {
	if oaschema.IsSchemaTypeEmpty(typeSchema) {
		return pn.decodeArbitrary(), nil
	}

	result, _, errs := pn.decodeFromSchemaTypes(typeSchema)

	return result, errs
}

// decodeFromSchemaTypes tries each declared schema type in order and returns the first
// successful decode. The empty type and "null" are skipped; errors from the last failing
// type are surfaced when all types fail.
func (pn *ParameterNode) decodeFromSchemaTypes(
	schemaDef *base.Schema,
) (any, string, []httperror.ValidationError) {
	var finalErrors []httperror.ValidationError

	for _, typeName := range schemaDef.Type {
		if typeName == "" || typeName == oaschema.Null {
			continue
		}

		result, resultType, errs := pn.decodeFromSchemaType(schemaDef, typeName)
		if len(errs) == 0 {
			return result, resultType, nil
		}

		finalErrors = errs
	}

	return nil, "", finalErrors
}

func (pn *ParameterNode) decodeFromSchemaType(
	schemaDef *base.Schema,
	typeName string,
) (any, string, []httperror.ValidationError) {
	switch typeName {
	case oaschema.Array:
		result, errs := pn.decodeFromArray(schemaDef)
		for _, ed := range errs {
			ed.Code = oasvalidator.ErrCodeInvalidQueryParam
			ed.Parameter = pn.key.String()
		}

		return result, typeName, errs
	case oaschema.Object:
		result, err := pn.decodeFromObject(schemaDef)

		return result, typeName, err
	default:
		return decodePrimitiveQueryValuesFromSchemaType(typeName, pn.values)
	}
}

func (pn *ParameterNode) decodeFromArray(schemaDef *base.Schema) (any, []httperror.ValidationError) {
	errFuncs := oasvalidator.ValidateArray(schemaDef, pn.items, compareParameterNodes)

	errs := oasvalidator.CollectErrors(errFuncs)
	if len(errs) > 0 {
		return nil, errs
	}

	if len(pn.items) == 0 {
		return pn.values, nil
	}

	if schemaDef.Items.A == nil {
		return pn.decodeArbitraryArray(), nil
	}

	itemSchema := schemaDef.Items.A.Schema()
	if oaschema.IsSchemaTypeEmpty(itemSchema) {
		return pn.decodeArbitraryArray(), nil
	}

	results := make([]any, len(pn.items))

	for i, item := range pn.items {
		itemValue, decodeErrors := item.Decode(itemSchema)
		if len(decodeErrors) > 0 {
			errs = append(errs, decodeErrors...)
		} else {
			results[i] = itemValue
		}
	}

	return results, errs
}

func (pn *ParameterNode) decodeFromObject(
	schemaDef *base.Schema,
) (map[string]any, []httperror.ValidationError) {
	var (
		results = make(map[string]any)
		errs    []httperror.ValidationError
	)

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
		return nil, slices.Clip(errs)
	}

	errs = pn.decodeObjectAdditionalProperties(schemaDef, results)
	if len(errs) > 0 {
		return nil, errs
	}

	errs = pn.decodeObjectPatternProperties(schemaDef, results)
	if len(errs) > 0 {
		return nil, errs
	}

	return results, nil
}

// decodeObjectPatternProperties matches node children against schema patternProperties.
// Keys already present in results (decoded from explicit properties) are skipped to avoid
// double-processing. Regex compile failures are logged and the pattern is skipped rather
// than failing the entire request.
func (pn *ParameterNode) decodeObjectPatternProperties(
	schemaDef *base.Schema,
	results map[string]any,
) []httperror.ValidationError {
	if schemaDef.PatternProperties == nil && schemaDef.PatternProperties.Len() == 0 {
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

func (pn *ParameterNode) decodeArbitrary() any {
	if len(pn.items) == 0 {
		return getValue(pn.values)
	}

	if IsParamIndex(pn.items[0].key) {
		return pn.decodeArbitraryArray()
	}

	results := make(map[string]any)

	pn.decodeArbitraryObject(results)

	return results
}

func (pn *ParameterNode) decodeArbitraryArray() []any {
	results := make([]any, 0, len(pn.items))

	for _, item := range pn.items {
		results = append(results, item.decodeArbitrary())
	}

	return results
}

func (pn *ParameterNode) decodeArbitraryObject(results map[string]any) {
	for _, node := range pn.items {
		results[node.key.String()] = node.decodeArbitrary()
	}
}

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
