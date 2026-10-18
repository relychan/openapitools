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
	"slices"

	"github.com/hasura/goenvconf"
	"github.com/pb33f/go-yaml"
	"github.com/relychan/gohttpc"
	"github.com/relychan/openapitools/oaschema"
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
	Method string
	GetEnv goenvconf.GetEnvFunc
}

// GetEnvFunc returns a function to get environment variables.
func (nrp NewProxyHandlerOptions) GetEnvFunc() goenvconf.GetEnvFunc {
	if nrp.GetEnv == nil {
		return goenvconf.GetOSEnv
	}

	return nrp.GetEnv
}

// NewProxyHandlerFunc abstracts a function to create a new proxy handler.
type NewProxyHandlerFunc func(operation *oaschema.Operation, rawProxyAction *yaml.Node, options *NewProxyHandlerOptions) (ProxyHandler, error)

// NewRequestFunc abstracts a function to create an HTTP request.
type NewRequestFunc func(method string, uri string) *gohttpc.RequestWithClient

// ProxyHandleOptions hold request options for the proxy handler.
type ProxyHandleOptions struct {
	NewRequest NewRequestFunc
	Settings   *oaschema.OpenAPIResourceSettings
}

// ForwardResponseHeaders forward headers from http.Response to http.ResponseWriter.
// All values of multi-value headers (e.g. Set-Cookie) are copied.
// Framing headers (Content-Length, Content-Encoding, Transfer-Encoding) are never forwarded
// because the response body may be decompressed or transformed by the proxy.
func (pho *ProxyHandleOptions) ForwardResponseHeaders(
	writer http.ResponseWriter,
	response *http.Response,
) {
	if pho == nil || pho.Settings == nil || pho.Settings.ForwardHeaders == nil || response == nil {
		return
	}

	writerHeader := writer.Header()

	for _, header := range pho.Settings.ForwardHeaders.Response {
		key := http.CanonicalHeaderKey(header)

		if isNonForwardableResponseHeader(key) {
			continue
		}

		values := response.Header.Values(key)
		if len(values) == 0 {
			continue
		}

		writerHeader[key] = slices.Clone(values)
	}
}

func isNonForwardableResponseHeader(canonicalKey string) bool {
	switch canonicalKey {
	case "Content-Length", "Content-Encoding", "Transfer-Encoding", "Connection", "Trailer":
		return true
	default:
		return false
	}
}
