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

// ParamKeys is an ordered path of selectors that locates a value within a nested
// parameter tree, e.g. [ParamKey("address"), ParamKey("city")] or
// [ParamKey("ids"), ParamIndex(0)].
type ParamKeys []ParamSelector

// Equal checks if the target value is equal.
func (ks ParamKeys) Equal(target ParamKeys) bool {
	return slices.Equal(ks, target)
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
		if i == lenKeys-1 && IsParamIndex(key) {
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

// ParamSelector is the discriminated union for a single path segment: either a
// string key (ParamKey) for object properties or a numeric index (ParamIndex) for
// array elements.
type ParamSelector interface {
	goutils.Equaler[ParamSelector]
	goutils.IsZeroer
	fmt.Stringer
}

// ParamKey represents a parameter key string.
type ParamKey string

var _ ParamSelector = ParamKey("")

// Equal checks if the target value is equal.
func (k ParamKey) Equal(target ParamSelector) bool {
	value, isString := target.(ParamKey)

	return isString && k == value
}

// IsZero checks if the key is empty.
func (k ParamKey) IsZero() bool {
	return k == ""
}

// String implements fmt.Stringer interface.
func (k ParamKey) String() string {
	return string(k)
}

// ParamIndex represents a parameter index. The sentinel value -1 is used when a
// bare "[]" suffix is parsed from a deep-object key (e.g. "ids[]"), meaning the
// position within the array is not known until Normalize resolves it.
type ParamIndex int

var _ ParamSelector = ParamIndex(0)

// Equal checks if the target value is equal.
func (k ParamIndex) Equal(target ParamSelector) bool {
	value, isIndex := target.(ParamIndex)

	return isIndex && k == value
}

// IsZero checks if the key is empty.
func (k ParamIndex) IsZero() bool {
	return k == -1
}

// String implements fmt.Stringer interface.
func (k ParamIndex) String() string {
	return strconv.Itoa(int(k))
}

func IsParamKey(selector ParamSelector) bool {
	_, ok := selector.(ParamKey)

	return ok
}

func IsParamIndex(selector ParamSelector) bool {
	_, ok := selector.(ParamIndex)

	return ok
}

// ParameterItems is the flattened representation of a (potentially nested) parameter
// value produced by EvaluateParameterValue.  Each item carries the full key path and
// the serialized leaf value so encoders can reconstruct any OpenAPI serialization style.
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
	switch len(pi.keys) {
	case 0:
		return false
	case 1:
		return IsParamKey(pi.keys[0])
	default:
		return true
	}
}

// Keys return key fragments of the parameter item.
func (pi ParameterItem) Keys() ParamKeys {
	return pi.keys
}

// Value return the value of the item.
func (pi ParameterItem) Value() string {
	return pi.value
}
