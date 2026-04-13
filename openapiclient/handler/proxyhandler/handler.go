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

// Package proxyhandler defines types for the proxy handler.
package proxyhandler

import (
	"context"
	"net/http"
	"net/url"

	"github.com/hasura/goenvconf"
	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/relychan/gohttpc"
	"github.com/relychan/openapitools/oaschema"
	"go.yaml.in/yaml/v4"
)

// ProxyHandler abstracts the executor to proxy HTTP requests.
type ProxyHandler interface {
	// Type returns type of the current handler.
	Type() ProxyActionType
	// Handle resolves the HTTP request and proxies that request to the remote server.
	// The response is decoded to a native Go type.
	Handle(
		ctx context.Context,
		request *Request,
		options *ProxyHandleOptions,
	) (*http.Response, any, error)
	// Stream resolves the HTTP request and proxies that request to the remote server.
	// The response body can be a raw stream or transformed reader.
	Stream(
		ctx context.Context,
		request *Request,
		writer http.ResponseWriter,
		options *ProxyHandleOptions,
	) (*http.Response, error)
}

// NewProxyHandlerOptions hold request options for the proxy handler.
type NewProxyHandlerOptions struct {
	Method     string
	Parameters []*highv3.Parameter
	GetEnv     goenvconf.GetEnvFunc
}

// GetEnvFunc returns a function to get environment variables.
func (nrp NewProxyHandlerOptions) GetEnvFunc() goenvconf.GetEnvFunc {
	if nrp.GetEnv == nil {
		return goenvconf.GetOSEnv
	}

	return nrp.GetEnv
}

// NewProxyHandlerFunc abstracts a function to create a new proxy handler.
type NewProxyHandlerFunc func(operation *highv3.Operation, rawProxyAction *yaml.Node, options *NewProxyHandlerOptions) (ProxyHandler, error)

// NewRequestFunc abstracts a function to create an HTTP request.
type NewRequestFunc func(method string, uri string) *gohttpc.RequestWithClient

// ProxyHandleOptions hold request options for the proxy handler.
type ProxyHandleOptions struct {
	NewRequest  NewRequestFunc
	Settings    *oaschema.OpenAPIResourceSettings
	ParamValues map[string]string
}

// ForwardResponseHeaders forward headers from http.Response to http.ResponseWriter.
func (pho *ProxyHandleOptions) ForwardResponseHeaders(
	writer http.ResponseWriter,
	response *http.Response,
) {
	if pho.Settings == nil || pho.Settings.ForwardHeaders == nil {
		return
	}

	for _, header := range pho.Settings.ForwardHeaders.Response {
		value := response.Header.Get(header)
		if value != "" {
			writer.Header().Set(header, value)
		}
	}
}

// Request represents an HTTP request to be proxying.
type Request struct {
	// Method specifies the HTTP method (GET, POST, PUT, etc.).
	method string
	// URL specifies either the URI being proxied.
	url *url.URL
	// Header contains the request header fields.
	header http.Header
	// Body is the request's body.
	body any
}

// NewRequest creates a new [Request] instance.
func NewRequest(method string, uri *url.URL, header http.Header, body any) *Request {
	return &Request{
		method: method,
		url:    uri,
		header: header,
		body:   body,
	}
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
	return r.url.String()
}

// GetURL returns the URL of the request.
func (r *Request) GetURL() *url.URL {
	return r.url
}

// SetURL sets the URL to the request.
func (r *Request) SetURL(u *url.URL) {
	r.url = u
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
