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
	Path        string
	ParamValues map[string]string
}

// Request represents an HTTP request to be proxying.
type Request struct {
	// Method specifies the HTTP method (GET, POST, PUT, etc.).
	Method string
	// URL specifies either the URI being proxied.
	URL *url.URL
	// Header contains the request header fields.
	Header http.Header
	// Body is the request's body.
	Body any
}
