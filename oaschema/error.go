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
)

var (
	// ErrInvalidParameterLocation occurs when the parameter location is invalid.
	ErrInvalidParameterLocation = fmt.Errorf(
		"invalid ParameterLocation. Expected one of %v",
		enumValueParameterLocations,
	)
	// ErrInvalidParameterEncodingStyle occurs when the parameter location is invalid.
	ErrInvalidParameterEncodingStyle = fmt.Errorf(
		"invalid ParameterEncodingStyle. Expected one of %v",
		enumValueEncodingStyles,
	)
	// ErrInvalidSecuritySchemeType occurs when the security scheme type is invalid.
	ErrInvalidSecuritySchemeType = fmt.Errorf(
		"invalid SecuritySchemeType. Expected one of %v",
		SupportedSecuritySchemeTypes(),
	)
	// ErrResourceSpecRequired occurs when the spec field of resource is empty.
	ErrResourceSpecRequired = errors.New("either spec or ref is required for the OpenAPI document")
	// ErrInvalidOpenAPIResourceDefinition occurs when failing to parse a OpenAPIResourceDefinition from YAML string.
	ErrInvalidOpenAPIResourceDefinition = errors.New(
		"invalid OpenAPIResourceDefinition",
	)
	// ErrInvalidContentType occurs when the content type string is invalid.
	ErrInvalidContentType = errors.New("invalid content type")
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
	// ErrCodeGraphQLResponseEmpty represents a code for empty graphql response.
	ErrCodeGraphQLResponseEmpty = "graphql_response_empty"
	// ErrCodeRemoteServerError represents a code for remote server errors.
	ErrCodeRemoteServerError = "remote_server_error"
)
