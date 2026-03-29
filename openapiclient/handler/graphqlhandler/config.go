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

package graphqlhandler

import (
	"github.com/hasura/goenvconf"
	"github.com/relychan/gotransform"
	"github.com/relychan/gotransform/jmes"
	"github.com/relychan/openapitools/openapiclient/handler/proxyhandler"
)

// ProxyTypeGraphQL represents a constant value for GraphQL proxy action.
const ProxyTypeGraphQL proxyhandler.ProxyActionType = "graphql"

// GraphQLRequestBody represents a request body to a GraphQL server.
type GraphQLRequestBody struct {
	Query         string         `json:"query"`
	OperationName string         `json:"operationName,omitempty"`
	Variables     map[string]any `json:"variables,omitempty"`
	Extensions    map[string]any `json:"extensions,omitempty"`
}

// ProxyGraphQLActionConfig represents a proxy action config for GraphQL.
type ProxyGraphQLActionConfig struct {
	// Type of the proxy action which is always graphql.
	Type proxyhandler.ProxyActionType `json:"type" yaml:"type" jsonschema:"enum=graphql"`
	// Configurations for the GraphQL proxy request.
	Request *ProxyGraphQLRequestConfig `json:"request,omitempty" yaml:"request,omitempty"`
	// Configurations for evaluating graphql responses.
	Response *ProxyCustomGraphQLResponseConfig `json:"response,omitempty" yaml:"response,omitempty"`
}

// ProxyGraphQLRequestConfig represents configurations for the proxy request.
type ProxyGraphQLRequestConfig struct {
	// Overrides the request URL. Use the original request path if empty.
	URL string `json:"url,omitempty" yaml:"url,omitempty"`
	// The configuration to transform request headers.
	Headers map[string]jmes.FieldMappingEntryStringConfig `json:"headers,omitempty" yaml:"headers,omitempty"`
	// GraphQL query to be sent.
	Query string `json:"query" yaml:"query"`
	// Definition of GraphQL variables.
	Variables map[string]jmes.FieldMappingEntryConfig `json:"variables,omitempty" yaml:"variables,omitempty"`
	// Definition of GraphQL extensions.
	Extensions map[string]jmes.FieldMappingEntryConfig `json:"extensions,omitempty" yaml:"extensions,omitempty"`
}

// ProxyCustomGraphQLResponseConfig represents configurations for the proxy response.
type ProxyCustomGraphQLResponseConfig struct {
	// HTTP error code will be used if the response body has errors.
	// If not set, forward the HTTP status from the upstream response which is usually 200 OK.
	HTTPErrorCode *int `json:"httpErrorCode,omitempty" yaml:"httpErrorCode,omitempty" jsonschema:"minimum=400,maximum=599,default=400"`
	// Configurations for transforming response data.
	Body *gotransform.TemplateTransformerConfig `json:"body,omitempty" yaml:"body,omitempty"`
}

// IsZero checks if the configuration is empty.
func (conf ProxyCustomGraphQLResponseConfig) IsZero() bool {
	return conf.HTTPErrorCode == nil &&
		(conf.Body == nil || conf.Body.IsZero())
}

// ProxyCustomGraphQLResponse represents configurations for the proxy response.
type ProxyCustomGraphQLResponse struct {
	// HTTP error code will be used if the response body has errors.
	// If not set, forward the HTTP status from the upstream response which is usually 200 OK.
	HTTPErrorCode *int
	// Configurations for transforming response body data.
	Body gotransform.TemplateTransformer
}

// NewProxyCustomGraphQLResponse creates a [ProxyCustomGraphQLResponse] from raw configurations.
func NewProxyCustomGraphQLResponse(
	config *ProxyCustomGraphQLResponseConfig,
	getEnv goenvconf.GetEnvFunc,
) (*ProxyCustomGraphQLResponse, error) {
	if config == nil || config.IsZero() {
		return nil, nil
	}

	result := &ProxyCustomGraphQLResponse{
		HTTPErrorCode: config.HTTPErrorCode,
	}

	if config.Body != nil {
		transformer, err := gotransform.NewTransformerFromConfig("", *config.Body, getEnv)
		if err != nil {
			return result, err
		}

		result.Body = transformer
	}

	return result, nil
}

// IsZero checks if the configuration is empty.
func (conf ProxyCustomGraphQLResponse) IsZero() bool {
	return conf.HTTPErrorCode == nil &&
		(conf.Body == nil || conf.Body.IsZero())
}
