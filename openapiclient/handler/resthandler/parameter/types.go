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
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/relychan/goutils"
	"github.com/relychan/openapitools/oaschema"
)

var (
	errParamNameRequired = errors.New("parameter name is required")
	errInvalidParamIn    = fmt.Errorf(
		"invalid parameter location. Accept one of [%s, %s, %s]",
		oaschema.InHeader,
		oaschema.InQuery,
		oaschema.InPath,
	)
	errInvalidParamHeaderStyle = fmt.Errorf(
		"invalid style of the header parameter. Accept one of [%s]",
		oaschema.EncodingStyleSimple,
	)
	errInvalidParamPathStyle = fmt.Errorf(
		"invalid style of the path parameter. Accept one of [%s, %s, %s]",
		oaschema.EncodingStyleLabel,
		oaschema.EncodingStyleMatrix,
		oaschema.EncodingStyleSimple,
	)
	errInvalidParamQueryStyle = fmt.Errorf(
		"invalid style of the query parameter. Accept one of [%s, %s, %s, %s]",
		oaschema.EncodingStyleForm,
		oaschema.EncodingStyleSpaceDelimited,
		oaschema.EncodingStylePipeDelimited,
		oaschema.EncodingStyleDeepObject,
	)
)

// BaseParameter represents an object of common configurations for a parameter.
type BaseParameter struct {
	// The name of the parameter.
	Name string `json:"name" yaml:"name"`
	// When this is true, parameter values of type array or object generate separate parameters for each value of the array or key-value pair of the map.
	Explode *bool `json:"explode,omitempty" yaml:"explode,omitempty"`
	// When this is true, parameter values are serialized using reserved expansion.
	AllowReserved bool `json:"allowReserved,omitempty" yaml:"allowReserved,omitempty"`
	// The location of the parameter.
	In oaschema.ParameterLocation `json:"in" yaml:"in" jsonschema:"type=string,enum=header,enum=query,enum=cookie,enum=path"`
	// Describes how the parameter value will be serialized depending on the type of the parameter value.
	Style *oaschema.ParameterEncodingStyle `json:"style,omitempty" yaml:"style,omitempty" jsonschema:"enum=simple,enum=label,enum=matrix,enum=form,enum=spaceDelimited,enum=pipeDelimited,enum=deepObject"`
}

// Validate checks if the current parameter config is valid.
func (conf BaseParameter) Validate() error {
	if conf.Name == "" {
		return errParamNameRequired
	}

	switch conf.In {
	case oaschema.InPath:
		if conf.Style != nil && (*conf.Style != oaschema.EncodingStyleMatrix &&
			*conf.Style != oaschema.EncodingStyleLabel &&
			*conf.Style != oaschema.EncodingStyleSimple) {
			return fmt.Errorf("%w, got %s", errInvalidParamPathStyle, *conf.Style)
		}
	case oaschema.InHeader:
		if conf.Style != nil && *conf.Style != oaschema.EncodingStyleSimple {
			return fmt.Errorf("%w, got %s", errInvalidParamHeaderStyle, conf.Style)
		}
	case oaschema.InQuery:
		if conf.Style != nil && (*conf.Style != oaschema.EncodingStyleForm &&
			*conf.Style != oaschema.EncodingStyleSpaceDelimited &&
			*conf.Style != oaschema.EncodingStylePipeDelimited &&
			*conf.Style != oaschema.EncodingStyleDeepObject) {
			return errInvalidParamQueryStyle
		}
	default:
		return fmt.Errorf("%w, got: %s", errInvalidParamIn, conf.In)
	}

	return nil
}

// GetStyleAndExplode gets the matched explode value of the parameter location.
func (conf BaseParameter) GetStyleAndExplode() (oaschema.ParameterEncodingStyle, bool) {
	return evalParamStyleAndExplode(conf.In, conf.Style, conf.Explode)
}

// ParamKeys represent a key slice.
type ParamKeys []ParamKey

// Equal checks if the target value is equal.
func (ks ParamKeys) Equal(target ParamKeys) bool {
	return slices.EqualFunc(ks, target, func(a, b ParamKey) bool {
		return a.Equal(b)
	})
}

// Format prints parameter keys with format.
func (ks ParamKeys) Format(root string, isDeepObject bool) string {
	lenKeys := len(ks)
	if lenKeys == 0 {
		return root
	}

	var sb strings.Builder

	sb.Grow(len(root) + len(ks))

	if root != "" {
		sb.WriteString(root)
	}

	for i, key := range ks {
		// skip the last array element except the deep object style
		if i == lenKeys-1 && key.index != nil {
			if isDeepObject {
				sb.WriteString("[]")
			}

			break
		}

		if i == 0 && root == "" {
			sb.WriteString(key.String())

			continue
		}

		sb.WriteByte('[')
		sb.WriteString(key.String())
		sb.WriteByte(']')
	}

	return sb.String()
}

// String implements fmt.Stringer interface.
func (ks ParamKeys) String() string {
	return ks.Format("", false)
}

// ParamKey represents a key string or index.
type ParamKey struct {
	key   *string
	index *int
}

// NewIndex creates an index key.
func NewIndex(index int) ParamKey {
	return ParamKey{index: &index}
}

// NewKey creates a string key.
func NewKey(key string) ParamKey {
	return ParamKey{key: &key}
}

// Equal checks if the target value is equal.
func (k ParamKey) Equal(target ParamKey) bool {
	return goutils.EqualComparablePtr(k.key, target.key) &&
		goutils.EqualComparablePtr(k.index, target.index)
}

// IsZero checks if the key is empty.
func (k ParamKey) IsZero() bool {
	return k.key != nil && k.index == nil
}

// Key gets the string key.
func (k ParamKey) Key() *string {
	return k.key
}

// Index gets the integer key.
func (k ParamKey) Index() *int {
	return k.index
}

// String implements fmt.Stringer interface.
func (k ParamKey) String() string {
	if k.index != nil {
		return strconv.Itoa(*k.index)
	}

	if k.key != nil {
		return *k.key
	}

	return ""
}

type ParameterItems []ParameterItem

// Build build parameter items to a key-values map and estimate the length of the string will be built.
func (ssp ParameterItems) Build(
	prefix string,
	isDeepObject bool,
) (map[string][]string, int) {
	if len(ssp) == 0 {
		return nil, 0
	}

	var count int

	results := make(map[string][]string)

	for _, item := range ssp {
		key := item.keys.Format(prefix, isDeepObject)
		count += len(key)
		count += len(item.value)
		results[key] = append(results[key], item.value)
	}

	return results, count
}

// ParameterItem represents the key-value pair.
type ParameterItem struct {
	keys  ParamKeys
	value string
}

// NewParameterItem creates a parameter value pair.
func NewParameterItem(keys ParamKeys, value string) ParameterItem {
	return ParameterItem{
		keys:  keys,
		value: value,
	}
}

// IsNested checks if the parameter is a nested object field.
func (pi ParameterItem) IsNested() bool {
	keyLength := len(pi.keys)

	return keyLength > 1 || (keyLength == 1 && pi.keys[0].index == nil)
}

// Keys return key fragments of the parameter item.
func (pi ParameterItem) Keys() ParamKeys {
	return pi.keys
}

// Value return the value of the item.
func (pi ParameterItem) Value() string {
	return pi.value
}

type ParameterNodes []*ParameterNode

func (pn *ParameterNodes) InsertNode(keys ParamKeys, values []string) *goutils.ErrorDetail {
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

type ParameterNode struct {
	key    ParamKey
	values []string
	items  []*ParameterNode
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
