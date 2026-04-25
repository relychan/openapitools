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
	"github.com/relychan/goutils/httperror"
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
	// ErrCodeOpenAPISchemaError represents a code for OpenAPI schema errors.
	ErrCodeOpenAPISchemaError = "openapi_schema_error"
)

// ErrorFunc abstracts a function to create an error detail lazily.
type ErrorFunc func() *httperror.ValidationError

// CollectErrors collects error functions to error details.
func CollectErrors(errFuncs []ErrorFunc) []httperror.ValidationError {
	return CollectErrorsFunc(errFuncs, nil)
}

// CollectErrorsFunc collects error functions to error details.
func CollectErrorsFunc(
	errFuncs []ErrorFunc,
	modifyFunc func(*httperror.ValidationError),
) []httperror.ValidationError {
	if len(errFuncs) == 0 {
		return nil
	}

	results := make([]httperror.ValidationError, len(errFuncs))

	for i, fn := range errFuncs {
		err := fn()

		if modifyFunc != nil {
			modifyFunc(err)
		}

		results[i] = *err
	}

	return results
}

func MinContainsError(expected int64, actual int64) *httperror.ValidationError {
	return &httperror.ValidationError{
		Code: ErrCodeValidationError,
		Detail: "Require at least " + strconv.FormatInt(expected, 10) +
			" items to match contains schema, but got: " +
			strconv.FormatInt(actual, 10),
	}
}

func MinContainsErrorFunc(expected int64, actual int64) ErrorFunc {
	return func() *httperror.ValidationError {
		return MinContainsError(expected, actual)
	}
}

func MaxContainsError(expected int64, actual int64) *httperror.ValidationError {
	return &httperror.ValidationError{
		Code: ErrCodeValidationError,
		Detail: "Require maximum " + strconv.FormatInt(expected, 10) +
			" items to match contains schema, but got: " +
			strconv.FormatInt(actual, 10),
	}
}

func MaxContainsErrorFunc(expected int64, actual int64) ErrorFunc {
	return func() *httperror.ValidationError {
		return MaxContainsError(expected, actual)
	}
}

func InvalidTypeError(
	expected []string,
	actual string,
) *httperror.ValidationError {
	return &httperror.ValidationError{
		Code: ErrCodeValidationError,
		Detail: "Invalid type or syntax. Expected the type of value to be one of [" +
			strings.Join(expected, ", ") + "], however the request provided '" +
			actual + "' type",
	}
}

// func TypeMismatchedError(expected []string, actual string) *httperror.ValidationError {
// 	return &httperror.ValidationError{
// 		Code:   ErrCodeInvalidQueryParam,
// 		Detail: "Invalid data types. Expected one of [" + strings.Join(expected, ", ") + "], but got: " + actual,
// 	}
// }

func InvalidTypeErrorFunc(
	expected []string,
	actual string,
) ErrorFunc {
	return func() *httperror.ValidationError {
		return InvalidTypeError(expected, actual)
	}
}

// NotNullError returns a validation error for not-null value.
func NotNullError() *httperror.ValidationError {
	return &httperror.ValidationError{
		Code:   ErrCodeValidationError,
		Detail: "The value must not be null",
	}
}

func EnumValidationError(
	typeSchema *base.Schema,
	actual string,
) *httperror.ValidationError {
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

	return &httperror.ValidationError{
		Code:   ErrCodeValidationError,
		Detail: detail,
	}
}

func EnumValidationErrorFunc(
	typeSchema *base.Schema,
	actual string,
) ErrorFunc {
	return func() *httperror.ValidationError {
		return EnumValidationError(typeSchema, actual)
	}
}

func MultipleOfValidationError(
	multipleOf float64,
	actual float64,
) *httperror.ValidationError {
	detail := "Value must be a multiple of '" +
		strconv.FormatFloat(multipleOf, 'f', -1, 64) +
		"', but got: " + strconv.FormatFloat(actual, 'f', -1, 64)

	return &httperror.ValidationError{
		Code:   ErrCodeValidationError,
		Detail: detail,
	}
}

func MultipleOfValidationErrorFunc(
	multipleOf float64,
	actual float64,
) ErrorFunc {
	return func() *httperror.ValidationError {
		return MultipleOfValidationError(multipleOf, actual)
	}
}

func MaximumValidationError(expected float64, actual float64, exclusive bool) *httperror.ValidationError {
	detail := "Number value must be less than "

	if !exclusive {
		detail += "or equal "
	}

	detail += strconv.FormatFloat(expected, 'f', -1, 64) +
		", but got: " + strconv.FormatFloat(actual, 'f', -1, 64)

	return &httperror.ValidationError{
		Code:   ErrCodeValidationError,
		Detail: detail,
	}
}

func MaximumValidationErrorFunc(expected float64, actual float64, exclusive bool) ErrorFunc {
	return func() *httperror.ValidationError {
		return MaximumValidationError(expected, actual, exclusive)
	}
}

func MinimumValidationError(expected float64, actual float64, exclusive bool) *httperror.ValidationError {
	detail := "Number value must be greater than "

	if !exclusive {
		detail += "or equal "
	}

	detail += strconv.FormatFloat(expected, 'f', -1, 64) +
		", but got: " + strconv.FormatFloat(actual, 'f', -1, 64)

	return &httperror.ValidationError{
		Code:   ErrCodeValidationError,
		Detail: detail,
	}
}

func MinimumValidationErrorFunc(expected float64, actual float64, exclusive bool) ErrorFunc {
	return func() *httperror.ValidationError {
		return MinimumValidationError(expected, actual, exclusive)
	}
}

func MaxLengthValidationError(expected int64, actual int64) *httperror.ValidationError {
	detail := "The length of the string value must be less than " +
		strconv.FormatInt(expected, 10) +
		", but got: " + strconv.FormatInt(actual, 10)

	return &httperror.ValidationError{
		Code:   ErrCodeValidationError,
		Detail: detail,
	}
}

func MaxLengthValidationErrorFunc(expected int64, actual int64) ErrorFunc {
	return func() *httperror.ValidationError {
		return MaxLengthValidationError(expected, actual)
	}
}

func MinLengthValidationError(expected int64, actual int64) *httperror.ValidationError {
	detail := "The length of the string value must be less than " +
		strconv.FormatInt(expected, 10) +
		", but got: " + strconv.FormatInt(actual, 10)

	return &httperror.ValidationError{
		Code:   ErrCodeValidationError,
		Detail: detail,
	}
}

func MinLengthValidationErrorFunc(expected int64, actual int64) ErrorFunc {
	return func() *httperror.ValidationError {
		return MinLengthValidationError(expected, actual)
	}
}

func PatternValidationError(expected string) *httperror.ValidationError {
	return &httperror.ValidationError{
		Code:   ErrCodeValidationError,
		Detail: "The value does not match pattern: " + expected,
	}
}

func PatternValidationErrorFunc(expected string) ErrorFunc {
	return func() *httperror.ValidationError {
		return PatternValidationError(expected)
	}
}

// ArrayMaxItemsValidationError returns a validation error for maximum items in array.
func ArrayMaxItemsValidationError(expected, actual int64) *httperror.ValidationError {
	return &httperror.ValidationError{
		Code: ErrCodeValidationError,
		Detail: "Array must have a maximum items length of " +
			strconv.FormatInt(expected, 10) + ", but got " +
			strconv.FormatInt(actual, 10) + " items",
	}
}

func ArrayMaxItemsValidationErrorFunc(expected, actual int64) ErrorFunc {
	return func() *httperror.ValidationError {
		return ArrayMaxItemsValidationError(expected, actual)
	}
}

// ArrayMinItemsValidationError returns a validation error for minimum items in array.
func ArrayMinItemsValidationError(expected, actual int64) *httperror.ValidationError {
	return &httperror.ValidationError{
		Code: ErrCodeValidationError,
		Detail: "Array must have a minimum items length of " +
			strconv.FormatInt(expected, 10) + ", but got " +
			strconv.FormatInt(actual, 10) + " items",
	}
}

func ArrayMinItemsValidationErrorFunc(expected, actual int64) ErrorFunc {
	return func() *httperror.ValidationError {
		return ArrayMinItemsValidationError(expected, actual)
	}
}

// ArrayUniqueItemsValidationError returns a validation error for unique items array.
func ArrayUniqueItemsValidationError[T any](duplicates []T) *httperror.ValidationError {
	return &httperror.ValidationError{
		Code:   ErrCodeValidationError,
		Detail: fmt.Sprintf("Array contains the following duplicates: %v", duplicates),
	}
}

func ArrayUniqueItemsValidationErrorFunc[T any](duplicates []T) ErrorFunc {
	return func() *httperror.ValidationError {
		return ArrayUniqueItemsValidationError(duplicates)
	}
}

// ObjectMinPropertiesValidationError returns a validation error for minimum properties in object.
func ObjectMinPropertiesValidationError(expected, actual int64) *httperror.ValidationError {
	return &httperror.ValidationError{
		Code: ErrCodeValidationError,
		Detail: "Object must have a minimum properties of " +
			strconv.FormatInt(expected, 10) + ", but got " +
			strconv.FormatInt(actual, 10) + " items",
	}
}

func ObjectMinPropertiesValidationErrorFunc(expected, actual int64) ErrorFunc {
	return func() *httperror.ValidationError {
		return ObjectMinPropertiesValidationError(expected, actual)
	}
}

// ObjectMaxPropertiesValidationError returns a validation error for maximum properties in object.
func ObjectMaxPropertiesValidationError(expected, actual int64) *httperror.ValidationError {
	return &httperror.ValidationError{
		Code: ErrCodeValidationError,
		Detail: "Object must have a maximum properties of " +
			strconv.FormatInt(expected, 10) + ", but got " +
			strconv.FormatInt(actual, 10) + " items",
	}
}

func ObjectMaxPropertiesValidationErrorFunc(expected, actual int64) ErrorFunc {
	return func() *httperror.ValidationError {
		return ObjectMinPropertiesValidationError(expected, actual)
	}
}

// ObjectRequiredPropertyError returns a validation error for a missing required property in object.
func ObjectRequiredPropertyError(name string) *httperror.ValidationError {
	return &httperror.ValidationError{
		Code:    ErrCodeValidationError,
		Pointer: "/" + name,
		Detail:  "Required property '" + name + "' is missing in the object",
	}
}

func ObjectRequiredPropertyErrorFunc(name string) ErrorFunc {
	return func() *httperror.ValidationError {
		return ObjectRequiredPropertyError(name)
	}
}

// ObjectDependentRequiredError returns a validation error for a missing dependent required property in object.
func ObjectDependentRequiredError(name string, dependent string) *httperror.ValidationError {
	return &httperror.ValidationError{
		Code:    ErrCodeValidationError,
		Pointer: "/" + dependent,
		Detail:  "Property '" + dependent + "' is required if '" + name + "' exists in the object",
	}
}

func ObjectDependentRequiredErrorFunc(name string, dependent string) ErrorFunc {
	return func() *httperror.ValidationError {
		return ObjectDependentRequiredError(name, dependent)
	}
}

// ParameterRequiredError returns a validation error for a missing required parameter.
func ParameterRequiredError(name string) *httperror.ValidationError {
	return &httperror.ValidationError{
		Code:      ErrCodeValidationError,
		Parameter: name,
		Detail:    "Required parameter '" + name + "' is missing in the object",
	}
}

func ParameterRequiredErrorFunc(name string) ErrorFunc {
	return func() *httperror.ValidationError {
		return ParameterRequiredError(name)
	}
}
