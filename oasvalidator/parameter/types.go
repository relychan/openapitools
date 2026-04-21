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

// ParamIndex represents a parameter index.
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
