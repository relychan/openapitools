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
	"strconv"
	"strings"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/relychan/goutils"
	"go.yaml.in/yaml/v4"
)

var (
	// ErrInvalidContentType occurs when the content type string is invalid.
	ErrInvalidContentType     = errors.New("invalid content type")
	errUnclosedTemplateString = errors.New("expected a closed curly bracket")
)

const (
	// ErrCodeRequestDecodeBodyError represents a code for an decoding error from request body.
	ErrCodeRequestDecodeBodyError = "request_decode_body_error"
	// ErrCodeResponseDecodeBodyError represents a code for an decoding error from response body.
	ErrCodeResponseDecodeBodyError = "response_decode_body_error"
	// ErrCodeRequestEncodeBodyError represents a code for an encoding error from request body.
	ErrCodeRequestEncodeBodyError = "request_encode_body_error"
	// ErrCodeResponseEncodeBodyError represents a code for an encoding error from response body.
	ErrCodeResponseEncodeBodyError = "response_encode_body_error"
	// ErrCodeMalformedJSON represents a code for a malformed JSON error.
	ErrCodeMalformedJSON = "malformed_json"
	// ErrCodeMalformedXML represents a code for a malformed XML error.
	ErrCodeMalformedXML = "malformed_xml"
	// ErrCodeXMLEncodeError represents a code for a XML encoding error.
	ErrCodeXMLEncodeError = "xml_encode_error"
	// ErrCodeMultipartFormEncodeError represents a code for a multipart form encoding error.
	ErrCodeMultipartFormEncodeError = "multipart_encode_error"
	// ErrCodeRequestTransformError represents a code for a request transformation error.
	ErrCodeRequestTransformError = "request_transform_error"
	// ErrCodeResponseTransformError represents a code for a response transformation error.
	ErrCodeResponseTransformError = "response_transform_error"
	// ErrCodeWriteResponseError represents a code for a response write error.
	ErrCodeWriteResponseError = "write_response_error"
	// ErrCodeInvalidRESTfulRequestConfig represents a code for invalid errors of ProxyRESTfulRequestConfig.
	ErrCodeInvalidRESTfulRequestConfig = "invalid_restful_request_config"
	// ErrCodeProxyRESTfulResponseConfig represents a code for invalid errors of ProxyRESTfulResponseConfig.
	ErrCodeProxyRESTfulResponseConfig = "invalid_restful_response_config"
	// ErrCodeInvalidServerURL represents a code for invalid server URL errors.
	ErrCodeInvalidServerURL = "invalid_server_url"
	// ErrCodeInvalidRequestURL represents a code for invalid request URL errors.
	ErrCodeInvalidRequestURL = "invalid_request_url"
	// ErrCodeInvalidURLParam represents a code for invalid request URL parameter errors.
	ErrCodeInvalidURLParam = "invalid_url_param"
	// ErrCodeInvalidQueryParam represents a code for invalid query parameter errors.
	ErrCodeInvalidQueryParam = "invalid_query_param"
	// ErrCodeValidationError represents a code for validation errors.
	ErrCodeValidationError = "validation_error"
	// ErrCodeGraphQLResponseEmpty represents a code for empty graphql response.
	ErrCodeGraphQLResponseEmpty = "graphql_response_empty"
	// ErrCodeRemoteServerError represents a code for remote server errors.
	ErrCodeRemoteServerError = "remote_server_error"
)

// ErrorFunc abstracts a function to create an error detail lazily.
type ErrorFunc func() *goutils.ErrorDetail

// CollectErrors collects error functions to error details.
func CollectErrors(errFuncs []ErrorFunc) []goutils.ErrorDetail {
	return CollectErrorsFunc(errFuncs, nil)
}

// CollectErrorsFunc collects error functions to error details.
func CollectErrorsFunc(
	errFuncs []ErrorFunc,
	modifyFunc func(*goutils.ErrorDetail),
) []goutils.ErrorDetail {
	if len(errFuncs) == 0 {
		return nil
	}

	results := make([]goutils.ErrorDetail, len(errFuncs))

	for i, fn := range errFuncs {
		err := fn()

		if modifyFunc != nil {
			modifyFunc(err)
		}

		results[i] = *err
	}

	return results
}

// InvalidParamArrayMaxItemsError returns a validation error for max items array.
func InvalidParamArrayMaxItemsError(
	name string,
	code string,
	expected, actual int64,
) *goutils.ErrorDetail {
	return &goutils.ErrorDetail{
		Code:      code,
		Parameter: name,
		Detail: "Array parameter '" + name + "' has a maximum items length of " +
			strconv.FormatInt(expected, 10) + ", however the request provided " +
			strconv.FormatInt(actual, 10) + " items",
	}
}

// InvalidParamArrayMinItemsError returns a validation error for min items array.
func InvalidParamArrayMinItemsError(
	name string,
	code string,
	expected, actual int64,
) *goutils.ErrorDetail {
	return &goutils.ErrorDetail{
		Code:      code,
		Parameter: name,
		Detail: "Array parameter '" + name + "' has a minimum items length of " +
			strconv.FormatInt(expected, 10) + ", however the request provided " +
			strconv.FormatInt(actual, 10) + " items",
	}
}

// InvalidParamArrayUniqueItemsError returns a validation error for unique items array.
func InvalidParamArrayUniqueItemsError(
	name string,
	code string,
	duplicates []string,
) *goutils.ErrorDetail {
	return &goutils.ErrorDetail{
		Code:      code,
		Parameter: name,
		Detail: "Array parameter " + name + "' contains the following duplicates: " +
			strings.Join(duplicates, ", "),
	}
}

func MinContainsError(expected int64, actual int64) *goutils.ErrorDetail {
	return &goutils.ErrorDetail{
		Code: ErrCodeValidationError,
		Detail: "Require at least " + strconv.FormatInt(expected, 10) +
			" items to match contains schema, but got: " +
			strconv.FormatInt(actual, 10),
	}
}

func MinContainsErrorFunc(expected int64, actual int64) ErrorFunc {
	return func() *goutils.ErrorDetail {
		return MinContainsError(expected, actual)
	}
}

func MaxContainsError(expected int64, actual int64) *goutils.ErrorDetail {
	return &goutils.ErrorDetail{
		Code: ErrCodeValidationError,
		Detail: "Require maximum " + strconv.FormatInt(expected, 10) +
			" items to match contains schema, but got: " +
			strconv.FormatInt(actual, 10),
	}
}

func MaxContainsErrorFunc(expected int64, actual int64) ErrorFunc {
	return func() *goutils.ErrorDetail {
		return MaxContainsError(expected, actual)
	}
}

func InvalidTypeError(
	expected []string,
	actual string,
) *goutils.ErrorDetail {
	return &goutils.ErrorDetail{
		Code: ErrCodeValidationError,
		Detail: "Invalid type or syntax. Expected the type of value to be one of [" +
			strings.Join(expected, ", ") + "], however the request provided '" +
			actual + "' type",
	}
}

func InvalidTypeErrorFunc(
	expected []string,
	actual string,
) ErrorFunc {
	return func() *goutils.ErrorDetail {
		return InvalidTypeError(expected, actual)
	}
}

// NotNullError returns a validation error for not-null value.
func NotNullError() *goutils.ErrorDetail {
	return &goutils.ErrorDetail{
		Code:   ErrCodeValidationError,
		Detail: "The value must not be null",
	}
}

func EnumValidationError(
	typeSchema *base.Schema,
	actual string,
) *goutils.ErrorDetail {
	enums := typeSchema.Enum

	if typeSchema.Const != nil {
		enums = []*yaml.Node{typeSchema.Const}
	}

	enumValues := make([]string, 0, len(enums))

	for _, node := range enums {
		if node == nil {
			continue
		}

		enumValues = append(enumValues, node.Value)
	}

	detail := "Value '" + actual + "' does not match any enum values"

	if len(enumValues) > 0 {
		detail += ": [" + strings.Join(enumValues, ", ") + "]"
	}

	return &goutils.ErrorDetail{
		Code:   ErrCodeValidationError,
		Detail: detail,
	}
}

func EnumValidationErrorFunc(
	typeSchema *base.Schema,
	actual string,
) ErrorFunc {
	return func() *goutils.ErrorDetail {
		return EnumValidationError(typeSchema, actual)
	}
}

func MultipleOfValidationError(
	multipleOf float64,
	actual float64,
) *goutils.ErrorDetail {
	detail := "Value must be a multiple of '" +
		strconv.FormatFloat(multipleOf, 'f', -1, 64) +
		"', but got: " + strconv.FormatFloat(actual, 'f', -1, 64)

	return &goutils.ErrorDetail{
		Code:   ErrCodeValidationError,
		Detail: detail,
	}
}

func MultipleOfValidationErrorFunc(
	multipleOf float64,
	actual float64,
) ErrorFunc {
	return func() *goutils.ErrorDetail {
		return MultipleOfValidationError(multipleOf, actual)
	}
}

func MaximumValidationError(expected float64, actual float64, exclusive bool) *goutils.ErrorDetail {
	detail := "Number value must be less than "

	if !exclusive {
		detail += "or equal "
	}

	detail += strconv.FormatFloat(expected, 'f', -1, 64) +
		", but got: " + strconv.FormatFloat(actual, 'f', -1, 64)

	return &goutils.ErrorDetail{
		Code:   ErrCodeValidationError,
		Detail: detail,
	}
}

func MaximumValidationErrorFunc(expected float64, actual float64, exclusive bool) ErrorFunc {
	return func() *goutils.ErrorDetail {
		return MaximumValidationError(expected, actual, exclusive)
	}
}

func MinimumValidationError(expected float64, actual float64, exclusive bool) *goutils.ErrorDetail {
	detail := "Number value must be greater than "

	if !exclusive {
		detail += "or equal "
	}

	detail += strconv.FormatFloat(expected, 'f', -1, 64) +
		", but got: " + strconv.FormatFloat(actual, 'f', -1, 64)

	return &goutils.ErrorDetail{
		Code:   ErrCodeValidationError,
		Detail: detail,
	}
}

func MinimumValidationErrorFunc(expected float64, actual float64, exclusive bool) ErrorFunc {
	return func() *goutils.ErrorDetail {
		return MinimumValidationError(expected, actual, exclusive)
	}
}

func MaxLengthValidationError(expected int64, actual int64) *goutils.ErrorDetail {
	detail := "The length of the string value must be less than " +
		strconv.FormatInt(expected, 10) +
		", but got: " + strconv.FormatInt(actual, 10)

	return &goutils.ErrorDetail{
		Code:   ErrCodeValidationError,
		Detail: detail,
	}
}

func MaxLengthValidationErrorFunc(expected int64, actual int64) ErrorFunc {
	return func() *goutils.ErrorDetail {
		return MaxLengthValidationError(expected, actual)
	}
}

func MinLengthValidationError(expected int64, actual int64) *goutils.ErrorDetail {
	detail := "The length of the string value must be less than " +
		strconv.FormatInt(expected, 10) +
		", but got: " + strconv.FormatInt(actual, 10)

	return &goutils.ErrorDetail{
		Code:   ErrCodeValidationError,
		Detail: detail,
	}
}

func MinLengthValidationErrorFunc(expected int64, actual int64) ErrorFunc {
	return func() *goutils.ErrorDetail {
		return MinLengthValidationError(expected, actual)
	}
}

func PatternValidationError(expected string) *goutils.ErrorDetail {
	return &goutils.ErrorDetail{
		Code:   ErrCodeValidationError,
		Detail: "The value does not match pattern: " + expected,
	}
}

func PatternValidationErrorFunc(expected string) ErrorFunc {
	return func() *goutils.ErrorDetail {
		return PatternValidationError(expected)
	}
}

// ArrayMaxItemsValidationError returns a validation error for maximum items in array.
func ArrayMaxItemsValidationError(expected, actual int64) *goutils.ErrorDetail {
	return &goutils.ErrorDetail{
		Code: ErrCodeValidationError,
		Detail: "Array must have a maximum items length of " +
			strconv.FormatInt(expected, 10) + ", but got " +
			strconv.FormatInt(actual, 10) + " items",
	}
}

func ArrayMaxItemsValidationErrorFunc(expected, actual int64) ErrorFunc {
	return func() *goutils.ErrorDetail {
		return ArrayMaxItemsValidationError(expected, actual)
	}
}

// ArrayMinItemsValidationError returns a validation error for minimum items in array.
func ArrayMinItemsValidationError(expected, actual int64) *goutils.ErrorDetail {
	return &goutils.ErrorDetail{
		Code: ErrCodeValidationError,
		Detail: "Array must have a minimum items length of " +
			strconv.FormatInt(expected, 10) + ", but got " +
			strconv.FormatInt(actual, 10) + " items",
	}
}

func ArrayMinItemsValidationErrorFunc(expected, actual int64) ErrorFunc {
	return func() *goutils.ErrorDetail {
		return ArrayMinItemsValidationError(expected, actual)
	}
}

// ArrayUniqueItemsValidationError returns a validation error for unique items array.
func ArrayUniqueItemsValidationError[T any](duplicates []T) *goutils.ErrorDetail {
	return &goutils.ErrorDetail{
		Code:   ErrCodeValidationError,
		Detail: fmt.Sprintf("Array contains the following duplicates: %v", duplicates),
	}
}

func ArrayUniqueItemsValidationErrorFunc[T any](duplicates []T) ErrorFunc {
	return func() *goutils.ErrorDetail {
		return ArrayUniqueItemsValidationError(duplicates)
	}
}

// ObjectMinPropertiesValidationError returns a validation error for minimum properties in object.
func ObjectMinPropertiesValidationError(expected, actual int64) *goutils.ErrorDetail {
	return &goutils.ErrorDetail{
		Code: ErrCodeValidationError,
		Detail: "Object must have a minimum properties of " +
			strconv.FormatInt(expected, 10) + ", but got " +
			strconv.FormatInt(actual, 10) + " items",
	}
}

func ObjectMinPropertiesValidationErrorFunc(expected, actual int64) ErrorFunc {
	return func() *goutils.ErrorDetail {
		return ObjectMinPropertiesValidationError(expected, actual)
	}
}

// ObjectMaxPropertiesValidationError returns a validation error for maximum properties in object.
func ObjectMaxPropertiesValidationError(expected, actual int64) *goutils.ErrorDetail {
	return &goutils.ErrorDetail{
		Code: ErrCodeValidationError,
		Detail: "Object must have a maximum properties of " +
			strconv.FormatInt(expected, 10) + ", but got " +
			strconv.FormatInt(actual, 10) + " items",
	}
}

func ObjectMaxPropertiesValidationErrorFunc(expected, actual int64) ErrorFunc {
	return func() *goutils.ErrorDetail {
		return ObjectMinPropertiesValidationError(expected, actual)
	}
}

// ObjectRequiredPropertyError returns a validation error for a missing required property in object.
func ObjectRequiredPropertyError(name string) *goutils.ErrorDetail {
	return &goutils.ErrorDetail{
		Code:    ErrCodeValidationError,
		Pointer: "/" + name,
		Detail:  "Required property '" + name + "' is missing in the object",
	}
}

func ObjectRequiredPropertyErrorFunc(name string) ErrorFunc {
	return func() *goutils.ErrorDetail {
		return ObjectRequiredPropertyError(name)
	}
}

// ObjectDependentRequiredError returns a validation error for a missing dependent required property in object.
func ObjectDependentRequiredError(name string, dependent string) *goutils.ErrorDetail {
	return &goutils.ErrorDetail{
		Code:    ErrCodeValidationError,
		Pointer: "/" + dependent,
		Detail:  "Property '" + dependent + "' is required if '" + name + "' exists in the object",
	}
}

func ObjectDependentRequiredErrorFunc(name string, dependent string) ErrorFunc {
	return func() *goutils.ErrorDetail {
		return ObjectDependentRequiredError(name, dependent)
	}
}
