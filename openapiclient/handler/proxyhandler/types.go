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

package proxyhandler

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/hasura/goenvconf"
	"github.com/relychan/openapitools/oasvalidator/parameter"
)

var (
	errOAuth2ClientCredentialsRequired = errors.New("clientId and clientSecret must not be empty")
	errOAuth2TokenURLRequired          = errors.New(
		"tokenUrl is required in the OAuth2 Client Credentials flow",
	)
)

// ProxyActionType represents enums of proxy types.
type ProxyActionType string

// InsertRouteOptions represents options for inserting routes.
type InsertRouteOptions struct {
	GetEnv goenvconf.GetEnvFunc
}

// APIKeyCredentials holds apiKey credentials of the security scheme.
type APIKeyCredentials struct {
	APIKey *goenvconf.EnvString `json:"apiKey,omitempty" yaml:"apiKey,omitempty"`
}

// BasicCredentials holds basic credentials of the security scheme.
type BasicCredentials struct {
	Username *goenvconf.EnvString `json:"username,omitempty" yaml:"username,omitempty"`
	Password *goenvconf.EnvString `json:"password,omitempty" yaml:"password,omitempty"`
}

// OAuth2Credentials holds OAuth2 credentials of the security scheme.
type OAuth2Credentials struct {
	ClientID     *goenvconf.EnvString `json:"clientId,omitempty" yaml:"clientId,omitempty"`
	ClientSecret *goenvconf.EnvString `json:"clientSecret,omitempty" yaml:"clientSecret,omitempty"`
	// Optional query parameters for the token and refresh URLs.
	EndpointParams map[string]goenvconf.EnvString `json:"endpointParams,omitempty" yaml:"endpointParams,omitempty"`
}

// Request represents an HTTP request to be proxying.
type Request struct {
	// Method specifies the HTTP method (GET, POST, PUT, etc.).
	method string
	// URL path of the request.
	path string
	// Header contains the request header fields.
	header http.Header
	// The body of the request.
	body any
	// Query parameters of the request.
	query url.Values
	// Parameter values of the request.
	urlParams map[string]any
	// Evaluated query parameters of the request.
	queryParams map[string]any
	// Evaluated headers of the request.
	headerParams map[string]any
	// URL fragment.
	fragment string
}

// NewRequest creates a new [Request] instance.
func NewRequest(method string, uri *url.URL, headers http.Header, body any) *Request {
	result := &Request{
		method: method,
		header: headers,
		body:   body,
	}

	if uri != nil {
		uriPath := uri.Path

		if len(uriPath) == 0 {
			uriPath = "/"
		} else if uriPath[0] != '/' {
			uriPath = "/" + uriPath
		}

		result.path = uriPath
		result.fragment = uri.RawFragment
		result.query = uri.Query()
	}

	return result
}

// Method returns the method of the request.
func (r *Request) Method() string {
	return r.method
}

// SetMethod sets a new method to the request.
func (r *Request) SetMethod(method string) {
	r.method = method
}

// URL returns the URL string of the request.
func (r *Request) URL() string {
	result := r.path

	if len(r.query) == 0 && r.fragment == "" {
		return r.path
	}

	var sb strings.Builder

	sb.WriteString(result)

	if len(r.query) > 0 {
		sb.WriteByte('?')
		sb.WriteString(r.query.Encode())
	}

	if r.fragment != "" {
		sb.WriteByte('#')
		sb.WriteString(r.fragment)
	}

	return sb.String()
}

// Path returns the request path of the request.
func (r *Request) Path() string {
	return r.path
}

// SetPath sets the URL path to the request.
func (r *Request) SetPath(value string) {
	r.path = value
}

// Header returns the headers of the request.
func (r *Request) Header() http.Header {
	return r.header
}

// Body returns the body of the request.
func (r *Request) Body() any {
	return r.body
}

// SetBody sets the body of the request.
func (r *Request) SetBody(value any) {
	r.body = value
}

// URLParams returns parameter values of the request URL.
func (r *Request) URLParams() map[string]any {
	return r.urlParams
}

// SetURLParams sets the URL parameters of the request.
func (r *Request) SetURLParams(value map[string]any) {
	r.urlParams = value
}

// Query returns query parameter values of the request URL.
func (r *Request) Query() url.Values {
	return r.query
}

// SetQuery sets query parameters for the request.
func (r *Request) SetQuery(values url.Values) {
	r.query = values
}

// QueryParams returns query parameter values of the request URL.
func (r *Request) QueryParams() map[string]any {
	return r.queryParams
}

// SetQueryParams sets query parameters for the request.
func (r *Request) SetQueryParams(values map[string]any) {
	r.queryParams = values
}

// SetHeaderParams sets decoded header parameters for the request.
func (r *Request) SetHeaderParams(values map[string]any) {
	r.headerParams = values
}

// ToMap converts the struct to map.
func (r *Request) ToMap() map[string]any {
	result := map[string]any{
		"param":   r.urlParams,
		"query":   r.queryParams,
		"headers": r.headerParams,
	}

	if len(r.headerParams) == 0 && len(r.header) > 0 {
		result["headers"] = parameter.NormalizeHeaders(r.header)
	}

	if r.body != nil {
		result["body"] = r.body
	}

	return result
}
