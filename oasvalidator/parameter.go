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

package oasvalidator

import (
	"slices"

	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/relychan/goutils/httperror"
	"github.com/relychan/openapitools/oaschema"
)

// ValidateParameterDefinitions validate a list of parameter definition and merge it with unique name and location.
func ValidateParameterDefinitions(
	params []*highv3.Parameter,
) ([]*oaschema.Parameter, []httperror.ValidationError) {
	if len(params) == 0 {
		return nil, nil
	}

	var (
		dest = make([]*oaschema.Parameter, 0, len(params))
		errs []httperror.ValidationError
	)

L:
	for _, item := range params {
		param, vErrs := ValidateParameterDefinition(item)
		if len(vErrs) > 0 {
			errs = append(errs, vErrs...)

			continue
		}

		for j, destParam := range dest {
			if destParam.Name == param.Name && destParam.In == param.In {
				dest[j] = param

				continue L
			}
		}

		dest = append(dest, param)
	}

	if len(errs) == 0 {
		return slices.Clip(dest), nil
	}

	return nil, errs
}

// ValidateParameterDefinition validates a single parameter definition and returns a normalized
// oaschema.Parameter, resolving location, style, and schema.
func ValidateParameterDefinition(
	param *highv3.Parameter,
) (*oaschema.Parameter, []httperror.ValidationError) {
	if param == nil {
		return nil, nil
	}

	var errs []httperror.ValidationError

	location, err := oaschema.ParseParameterLocation(param.In)
	if err != nil {
		ve := httperror.ValidationError{
			Code:      ErrCodeValidationError,
			Detail:    err.Error(),
			Parameter: param.Name,
		}

		errs = append(errs, ve)
	}

	result := &oaschema.Parameter{
		Name:          param.Name,
		Explode:       param.Explode,
		AllowReserved: param.AllowReserved,
		Required:      param.Required != nil && *param.Required,
		In:            location,
	}

	if param.Style != "" {
		style, err := oaschema.ParseParameterEncodingStyle(param.Style)
		if err != nil {
			ve := httperror.ValidationError{
				Code:      ErrCodeValidationError,
				Detail:    err.Error(),
				Parameter: param.Name,
			}

			setParameterErrorLocation(result, &ve)

			errs = append(errs, ve)
		} else {
			result.Style = &style
		}
	}

	if param.Schema != nil {
		schema, schemaErrs := ValidateSchemaProxy(param.Schema)
		if len(schemaErrs) > 0 {
			setParameterErrorsLocation(result, schemaErrs)

			errs = append(errs, schemaErrs...)
		} else {
			result.Schema = schema
		}
	}

	if len(errs) == 0 {
		return result, nil
	}

	return nil, errs
}

// setParameterErrorsLocation stamps each error with the parameter location and error code.
func setParameterErrorsLocation(param *oaschema.Parameter, errs []httperror.ValidationError) {
	for i := range errs {
		setParameterErrorLocation(param, &errs[i])
	}
}

// setParameterErrorLocation stamps a single error with the parameter name and the error code
// appropriate for the parameter's location (header, query, or path).
func setParameterErrorLocation(param *oaschema.Parameter, err *httperror.ValidationError) {
	switch param.In {
	case oaschema.InHeader:
		err.Header = param.Name
		err.Code = ErrCodeInvalidHeader
	case oaschema.InQuery:
		err.Parameter = param.Name
		err.Code = ErrCodeInvalidQueryParam
	case oaschema.InPath:
		err.Parameter = param.Name
		err.Code = ErrCodeInvalidURLParam
	default:
		err.Parameter = param.Name
		err.Code = ErrCodeValidationError
	}
}
