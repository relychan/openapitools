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

package oaschema

import (
	"errors"
	"fmt"

	"github.com/pb33f/libopenapi/datamodel/high/base"
)

var (
	errParamNameRequired = errors.New("parameter name is required")
	errInvalidParamIn    = fmt.Errorf(
		"invalid parameter location. Accept one of [%s, %s, %s]",
		InHeader,
		InQuery,
		InPath,
	)
	errInvalidParamHeaderStyle = fmt.Errorf(
		"invalid style of the header parameter. Accept one of [%s]",
		EncodingStyleSimple,
	)
	errInvalidParamPathStyle = fmt.Errorf(
		"invalid style of the path parameter. Accept one of [%s, %s, %s]",
		EncodingStyleLabel,
		EncodingStyleMatrix,
		EncodingStyleSimple,
	)
	errInvalidParamQueryStyle = fmt.Errorf(
		"invalid style of the query parameter. Accept one of [%s, %s, %s, %s]",
		EncodingStyleForm,
		EncodingStyleSpaceDelimited,
		EncodingStylePipeDelimited,
		EncodingStyleDeepObject,
	)
)

// Parameter represents an object of common configurations for a parameter.
type Parameter struct {
	// The name of the parameter.
	Name string `json:"name" yaml:"name"`
	// When this is true, parameter values of type array or object generate separate parameters for each value of the array or key-value pair of the map.
	Explode *bool `json:"explode,omitempty" yaml:"explode,omitempty"`
	// When this is true, parameter values are serialized using reserved expansion.
	AllowReserved bool `json:"allowReserved,omitempty" yaml:"allowReserved,omitempty"`
	// Whether the parameter is required.
	Required bool `json:"required,omitempty" yaml:"required,omitempty"`
	// The location of the parameter.
	In ParameterLocation `json:"in" yaml:"in" jsonschema:"type=string,enum=header,enum=query,enum=cookie,enum=path"`
	// Describes how the parameter value will be serialized depending on the type of the parameter value.
	Style *ParameterEncodingStyle `json:"style,omitempty" yaml:"style,omitempty" jsonschema:"enum=simple,enum=label,enum=matrix,enum=form,enum=spaceDelimited,enum=pipeDelimited,enum=deepObject"`
	// Schema of the parameter.
	Schema *base.Schema `json:"-" yaml:"-"`
	// Optional con
	Content *MediaType `json:"-" yaml:"-"`
}

// Validate checks if the current parameter config is valid.
func (param Parameter) Validate() error {
	if param.Name == "" {
		return errParamNameRequired
	}

	switch param.In {
	case InPath:
		if param.Style != nil && (*param.Style != EncodingStyleMatrix &&
			*param.Style != EncodingStyleLabel &&
			*param.Style != EncodingStyleSimple) {
			return fmt.Errorf("%w, got %s", errInvalidParamPathStyle, *param.Style)
		}
	case InHeader:
		if param.Style != nil && *param.Style != EncodingStyleSimple {
			return fmt.Errorf("%w, got %s", errInvalidParamHeaderStyle, param.Style)
		}
	case InQuery:
		if param.Style != nil && (*param.Style != EncodingStyleForm &&
			*param.Style != EncodingStyleSpaceDelimited &&
			*param.Style != EncodingStylePipeDelimited &&
			*param.Style != EncodingStyleDeepObject) {
			return errInvalidParamQueryStyle
		}
	default:
		return fmt.Errorf("%w, got: %s", errInvalidParamIn, param.In)
	}

	return nil
}

// GetStyleAndExplode gets the matched explode value of the parameter location.
func (param Parameter) GetStyleAndExplode() (ParameterEncodingStyle, bool) {
	return GetParameterStyleAndExplode(param.In, param.Style, param.Explode)
}

// GetParameterStyleAndExplode applies the OpenAPI defaults for style and explode per location:
//   - path / header: default style=simple, default explode=false
//   - query / cookie: default style=form,  default explode=true
func GetParameterStyleAndExplode(
	location ParameterLocation,
	style *ParameterEncodingStyle,
	explode *bool,
) (ParameterEncodingStyle, bool) {
	switch location {
	case InPath, InHeader:
		explodeValue := explode != nil && *explode

		if style == nil {
			return EncodingStyleSimple, explodeValue
		}

		return *style, explodeValue
	case InQuery, InCookie:
		explodeValue := explode == nil || *explode

		if style == nil {
			return EncodingStyleForm, explodeValue
		}

		return *style, explodeValue
	default:
		return 255, false
	}
}
