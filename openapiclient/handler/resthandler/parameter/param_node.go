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
	"strconv"
	"strings"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/relychan/goutils"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/oasvalidator"
)

type ParameterNodes []*ParameterNode

func (pn ParameterNodes) Find(key ParamKey) *ParameterNode {
	for _, node := range pn {
		if node.key.Equal(key) {
			return node
		}
	}

	return nil
}

func (pn *ParameterNodes) Insert(keys ParamKeys, values []string) *goutils.ErrorDetail {
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

type ParameterNode struct {
	key    ParamKey
	values []string
	items  ParameterNodes
}

func (pn *ParameterNode) FindNode(key ParamKey) *ParameterNode {
	if len(pn.items) == 0 {
		return nil
	}

	return pn.items.Find(key)
}

func (pn *ParameterNode) Normalize() {
	if len(pn.items) == 0 {
		return
	}

	if len(pn.items) == 1 {
		if (pn.items[0].key.index != nil || pn.items[0].key.key == nil) &&
			len(pn.items[0].items) == 0 {
			pn.values = pn.items[0].values
			pn.items = nil

			return
		}

		index, err := strconv.ParseInt(*pn.items[0].key.key, 10, 32)
		if err == nil {
			pn.items[0].key = NewIndex(int(index))
		}

		pn.items[0].Normalize()

		return
	}

	// skip sorting object keys.
	if pn.items[0].key.index != nil {
		slices.SortFunc(pn.items, compareParameterNodes)
	}

	for _, item := range pn.items {
		item.Normalize()
	}
}

func (pn *ParameterNode) InsertNode(keys ParamKeys, values []string) *goutils.ErrorDetail {
	if len(keys) == 0 {
		pn.values = values

		return nil
	}

	// best-effort to converting the key to index if other keys in the list are indexes.
	if len(pn.items) == 1 && pn.items[0].key.key != nil && keys[0].index != nil {
		indexKey, err := strconv.Atoi(*pn.items[0].key.key)
		if err != nil {
			return newMixedArrayAndObjectError()
		}

		pn.items[0].key = NewIndex(indexKey)
	} else if len(pn.items) > 1 && pn.items[0].key.index != nil && keys[0].key != nil {
		indexKey, err := strconv.Atoi(*keys[0].key)
		if err != nil {
			return newMixedArrayAndObjectError()
		}

		keys[0] = NewIndex(indexKey)
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

func (pn *ParameterNode) Decode(typeSchema *base.Schema) (any, []goutils.ErrorDetail) {
	if oaschema.IsSchemaEmpty(typeSchema) {
		return pn.decodeArbitrary(), nil
	}

	result, _, errs := pn.decodeFromSchemaTypes(typeSchema)

	return result, errs
}

func (pn *ParameterNode) decodeFromSchemaTypes(
	schemaDef *base.Schema,
) (any, string, []goutils.ErrorDetail) {
	var finalErrors []goutils.ErrorDetail

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
) (any, string, []goutils.ErrorDetail) {
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

func (pn *ParameterNode) decodeFromArray(schemaDef *base.Schema) (any, []goutils.ErrorDetail) {
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
	if oaschema.IsSchemaEmpty(itemSchema) {
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
) (map[string]any, []goutils.ErrorDetail) {
	var (
		results    = make(map[string]any)
		parsedKeys = make([]string, 0, len(pn.items))
		errs       []goutils.ErrorDetail
	)

	if schemaDef.Properties != nil {
		for iter := schemaDef.Properties.First(); iter != nil; iter = iter.Next() {
			key := iter.Key()

			propNode := pn.FindNode(NewKey(key))
			if propNode == nil {
				if len(schemaDef.Required) > 0 && slices.Contains(schemaDef.Required, key) {
					err := oasvalidator.ObjectRequiredPropertyError(key)
					err.Pointer = "/" + key

					errs = append(errs, *err)
				}

				continue
			}

			parsedKeys = append(parsedKeys, key)

			schemaProxy := iter.Value()
			if schemaProxy == nil {
				results[key] = propNode.decodeArbitrary()

				continue
			}

			propSchema := schemaProxy.Schema()
			if oaschema.IsSchemaEmpty(propSchema) {
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

	if schemaDef.AdditionalProperties != nil &&
		(schemaDef.AdditionalProperties.B || schemaDef.AdditionalProperties.A != nil) {
		var propSchema *base.Schema

		if schemaDef.AdditionalProperties.N == 0 && schemaDef.AdditionalProperties.A != nil {
			propSchema = schemaDef.AdditionalProperties.A.Schema()
		}

		for _, propNode := range pn.items {
			if propNode.key.key != nil && slices.Contains(parsedKeys, *propNode.key.key) {
				continue
			}

			if oaschema.IsSchemaEmpty(propSchema) {
				results[propNode.key.String()] = propNode.decodeArbitrary()

				continue
			}

			value, decodeErrs := propNode.Decode(propSchema)
			if len(decodeErrs) == 0 {
				results[propNode.key.String()] = value

				continue
			}

			errs = append(errs, decodeErrs...)
		}
	}

	// TODO: patternProperties

	if len(errs) > 0 {
		return nil, errs
	}

	return results, nil
}

func (pn *ParameterNode) decodeArbitrary() any {
	if len(pn.items) == 0 {
		return pn.getValue()
	}

	if pn.items[0].key.index != nil {
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

func (pn *ParameterNode) getValue() any {
	switch len(pn.values) {
	case 0:
		return nil
	case 1:
		return pn.values[0]
	default:
		return pn.values
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

func compareParameterNodes(a, b *ParameterNode) int {
	if a.key.index == nil && b.key.index != nil {
		return 1
	}

	if a.key.index != nil && b.key.index == nil {
		return -1
	}

	if a.key.index != nil && b.key.index != nil {
		if *a.key.index == -1 {
			return 1
		}

		if *b.key.index == -1 {
			return 1
		}

		return *a.key.index - *b.key.index
	}

	if a.key.key == nil && b.key.key != nil {
		return 1
	}

	if a.key.key != nil && b.key.key == nil {
		return -1
	}

	if a.key.key != nil && b.key.key != nil {
		return strings.Compare(*a.key.key, *b.key.key)
	}

	return 0
}
