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

package resthandler

import (
	"net/http"

	"github.com/hasura/goenvconf"
	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/relychan/gotransform"
	"github.com/relychan/gotransform/jmes"
	"github.com/relychan/goutils"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/oasvalidator"
	"github.com/relychan/openapitools/openapiclient/handler/proxyhandler"
	"github.com/relychan/openapitools/openapiclient/handler/resthandler/parameter"
)

// ProxyActionTypeREST represents a constant value for REST proxy action.
const ProxyActionTypeREST proxyhandler.ProxyActionType = "rest"

// ProxyRESTfulActionConfig represents a proxy action config for REST operation.
type ProxyRESTfulActionConfig struct {
	// Type of the proxy action which is always rest.
	Type proxyhandler.ProxyActionType `json:"type" yaml:"type" jsonschema:"enum=rest"`
	// Configurations for the REST proxy request.
	Request *ProxyRESTfulRequestConfig `json:"request,omitempty" yaml:"request,omitempty"`
	// Configurations for evaluating REST responses.
	Response *ProxyCustomRESTfulResponseConfig `json:"response,omitempty" yaml:"response,omitempty"`
}

// ProxyCustomRESTfulResponseConfig represents configurations for the proxy response.
type ProxyCustomRESTfulResponseConfig struct {
	// Content type of the response to be transformed to.
	ContentType string `json:"contentType,omitempty" yaml:"contentType,omitempty"`
	// Configurations for transforming response data.
	Body *gotransform.TemplateTransformerConfig `json:"body,omitempty" yaml:"body,omitempty"`
}

// IsZero checks if the configuration is empty.
func (conf ProxyCustomRESTfulResponseConfig) IsZero() bool {
	return conf.Body == nil || conf.Body.IsZero()
}

type customRESTResponse struct {
	// Configurations for transforming response body data.
	Body gotransform.TemplateTransformer
}

// newCustomRESTResponse creates a [ProxyCustomResponse] from raw configurations.
func newCustomRESTResponse(
	config *ProxyCustomRESTfulResponseConfig,
	getEnv goenvconf.GetEnvFunc,
) (*customRESTResponse, error) {
	if config == nil || config.IsZero() {
		return nil, nil
	}

	transformer, err := gotransform.NewTransformerFromConfig("", *config.Body, getEnv)
	if err != nil {
		return nil, err
	}

	result := &customRESTResponse{
		Body: transformer,
	}

	return result, nil
}

// IsZero checks if the configuration is empty.
func (conf customRESTResponse) IsZero() bool {
	return conf.Body == nil || conf.Body.IsZero()
}

// ProxyRESTfulParameterConfig represents  an object of transformation configurations for a parameter.
type ProxyRESTfulParameterConfig struct {
	jmes.FieldMappingEntryConfig `yaml:",inline"`
	parameter.BaseParameter      `yaml:",inline"`
}

// ProxyRESTfulParameter represents  an object of evaluated configurations for a parameter.
type ProxyRESTfulParameter struct {
	jmes.FieldMappingEntry
	parameter.BaseParameter
}

// ProxyRESTfulRequestConfig represents configurations for the proxy request.
type ProxyRESTfulRequestConfig struct {
	// Overrides the request URL. Use the original request path if empty.
	URL string `json:"url,omitempty" yaml:"url,omitempty"`
	// Overrides the request method. Use the original request method if empty.
	Method string `json:"method,omitempty" jsonschema:"enum=GET,enum=POST,enum=PATCH,enum=PUT,enum=DELETE" yaml:"method,omitempty"`
	// The configuration to transform query, path, header and cookie parameters.
	Parameters []ProxyRESTfulParameterConfig `json:"parameters,omitempty" yaml:"parameters" jsonschema:"nullable"`
	// Content type of the body to be transformed to.
	ContentType string `json:"contentType,omitempty" yaml:"contentType,omitempty"`
	// The configuration to transform request body.
	Body *gotransform.TemplateTransformerConfig `json:"body,omitempty" yaml:"body"`
	// If this is true, all query parameters will be forwarded.
	// The default value is true if there is no query parameter is configured.
	ForwardAllQueryParams *bool `json:"forwardAllQueryParams,omitempty" yaml:"forwardAllQueryParams"`
}

// IsZero checks if the configuration is empty.
func (rr ProxyRESTfulRequestConfig) IsZero() bool {
	return rr.URL == "" &&
		rr.Method == "" &&
		len(rr.Parameters) == 0 &&
		(rr.Body == nil || rr.Body.IsZero()) &&
		rr.ForwardAllQueryParams == nil
}

type customRESTRequest struct {
	URL                   string
	Method                string
	Parameters            []ProxyRESTfulParameter
	Body                  gotransform.TemplateTransformer
	ForwardAllQueryParams *bool
}

// IsZero checks if the configuration is empty.
func (rr customRESTRequest) IsZero() bool {
	return rr.URL == "" &&
		rr.Method == "" &&
		len(rr.Parameters) == 0 &&
		(rr.Body == nil || rr.Body.IsZero()) &&
		rr.ForwardAllQueryParams == nil
}

func newCustomRESTRequestFromConfig(
	conf *ProxyRESTfulRequestConfig,
	getEnvFunc goenvconf.GetEnvFunc,
) (*customRESTRequest, error) {
	if conf == nil || conf.IsZero() {
		return nil, nil
	}

	result := &customRESTRequest{
		URL:                   conf.URL,
		Method:                conf.Method,
		ForwardAllQueryParams: conf.ForwardAllQueryParams,
		Parameters:            make([]ProxyRESTfulParameter, len(conf.Parameters)),
	}

	switch result.Method {
	case "", http.MethodGet, http.MethodPost, http.MethodDelete, http.MethodPatch, http.MethodPut:
	default:
		return nil, &goutils.ErrorDetail{
			Detail:  "invalid HTTP method to transform. Expected one of GET, POST, PUT, PATCH, DELETE, got: " + result.Method,
			Code:    oaschema.ErrCodeInvalidRESTfulRequestConfig,
			Pointer: "/method",
		}
	}

	for i, param := range conf.Parameters {
		field, err := param.EvaluateEntry(getEnvFunc)
		if err != nil {
			return nil, &goutils.ErrorDetail{
				Detail:  "failed to evaluate the parameter: " + err.Error(),
				Code:    oaschema.ErrCodeInvalidRESTfulRequestConfig,
				Pointer: "/parameters/" + param.Name,
			}
		}

		result.Parameters[i] = ProxyRESTfulParameter{
			FieldMappingEntry: field,
			BaseParameter:     param.BaseParameter,
		}
	}

	if conf.Body != nil {
		customBody, err := gotransform.NewTransformerFromConfig("", *conf.Body, getEnvFunc)
		if err != nil {
			return nil, &goutils.ErrorDetail{
				Detail:  "failed to transform custom request body: " + err.Error(),
				Code:    oaschema.ErrCodeInvalidRESTfulRequestConfig,
				Pointer: "/body",
			}
		}

		result.Body = customBody
	}

	return result, nil
}

func parseRequestContentType(
	operation *highv3.Operation,
	conf *ProxyRESTfulRequestConfig,
) (string, error) {
	var contentType string

	if conf != nil && conf.ContentType != "" {
		contentType = conf.ContentType
	} else if operation.RequestBody != nil {
		contentType = oaschema.GetDefaultContentType(operation.RequestBody.Content)
	}

	result, err := oasvalidator.ValidateContentType(contentType)
	if err != nil {
		return "", &goutils.ErrorDetail{
			Detail:  err.Error() + " " + contentType,
			Pointer: "/contentType",
			Code:    oaschema.ErrCodeInvalidRESTfulRequestConfig,
		}
	}

	return result, nil
}

func parseResponseContentType(
	operation *highv3.Operation,
	conf *ProxyCustomRESTfulResponseConfig,
) (string, error) {
	var contentType string

	if conf != nil && conf.ContentType != "" {
		contentType = conf.ContentType
	} else {
		contentType = oaschema.GetResponseContentTypeFromOperation(operation)
	}

	result, err := oasvalidator.ValidateContentType(contentType)
	if err != nil {
		return "", &goutils.ErrorDetail{
			Detail:  err.Error() + " " + contentType,
			Pointer: "/contentType",
			Code:    oaschema.ErrCodeProxyRESTfulResponseConfig,
		}
	}

	return result, nil
}
