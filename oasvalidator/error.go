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
	"errors"
	"fmt"

	"github.com/relychan/goutils"
)

var (
	// ErrInvalidContentType occurs when the content type string is invalid.
	ErrInvalidContentType     = errors.New("invalid content type")
	errUnclosedTemplateString = errors.New("expected a closed curly bracket")
)

// InvalidParamArrayMaxItemsError returns a validation error for max items array.
func InvalidParamArrayMaxItemsError(name string, code string, expected, actual int64) *goutils.ErrorDetail {
	return &goutils.ErrorDetail{
		Code:      code,
		Parameter: name,
		Detail: fmt.Sprintf(
			"The parameter (which is an array) '%s' has a maximum item length of %d, however the request provided %d items",
			name,
			expected,
			actual,
		),
	}
}

// InvalidParamArrayMinItemsError returns a validation error for min items array.
func InvalidParamArrayMinItemsError(name string, code string, expected, actual int64) *goutils.ErrorDetail {
	return &goutils.ErrorDetail{
		Code:      code,
		Parameter: name,
		Detail: fmt.Sprintf(
			"Array parameter '%s' has a minimum items length of %d, however the request provided %d items",
			name,
			expected,
			actual,
		),
	}
}

// InvalidParamArrayUniqueItemsError returns a validation error for unique items array.
func InvalidParamArrayUniqueItemsError(name string, code string, duplicates []string) *goutils.ErrorDetail {
	return &goutils.ErrorDetail{
		Code:      code,
		Parameter: name,
		Detail: fmt.Sprintf(
			"Array parameter '%s' contains the following duplicates: %v",
			name,
			duplicates,
		),
	}
}

// func IncorrectParamArrayUniqueItems(param *v3.Parameter, sch *base.Schema, duplicates string, pathTemplate string, operation string, renderedSchema string) *ValidationError {
// 	keywordLocation := helpers.ConstructParameterJSONPointer(pathTemplate, operation, param.Name, "uniqueItems")
// 	specLine, specCol := schemaItemsTypeLineCol(sch)

// 	return &ValidationError{
// 		ValidationType:    helpers.ParameterValidation,
// 		ValidationSubType: helpers.ParameterValidationQuery,
// 		Message:           fmt.Sprintf("Query array parameter '%s' contains non-unique items", param.Name),
// 		Reason:            fmt.Sprintf("The query parameter (which is an array) '%s' contains the following duplicates: '%s'", param.Name, duplicates),
// 		SpecLine:          specLine,
// 		SpecCol:           specCol,
// 		ParameterName:     param.Name,
// 		Context:           sch,
// 		HowToFix:          "Ensure the array values are all unique",
// 		SchemaValidationErrors: []*SchemaValidationFailure{{
// 			Reason:          fmt.Sprintf("Array contains duplicate values: %s", duplicates),
// 			FieldName:       param.Name,
// 			InstancePath:    []string{param.Name},
// 			KeywordLocation: keywordLocation,
// 			ReferenceSchema: renderedSchema,
// 		}},
// 	}
// }
