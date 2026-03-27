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

package openapiclient

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/relychan/gohttpc"
	"github.com/relychan/goutils"
	"github.com/relychan/openapitools/openapiclient/handler/proxyhandler"
	"github.com/relychan/openapitools/openapiclient/internal"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
)

// Stream routes the request to the remote server. The response will be transformed and written into the stream.
func (pc *ProxyClient) Stream(
	request *http.Request,
	w http.ResponseWriter,
) (*http.Response, error) {
	ctx, span := tracer.Start(request.Context(), "stream_request")
	defer span.End()

	req, err := NewRequest(request)
	if err != nil {
		span.SetStatus(codes.Error, "failed to decode request body")
		span.RecordError(err)

		return nil, err
	}

	route, options, err := pc.prepareRequest(span, req)
	if err != nil {
		return nil, err
	}

	response, err := route.Method.Handler.Stream(ctx, req, w, options)
	if err != nil {
		return response, pc.handleError(span, err, options.Path)
	}

	span.SetStatus(codes.Ok, "")

	return response, nil
}

// Execute routes and proxies the request to the remote server.
func (pc *ProxyClient) Execute(
	ctx context.Context,
	request *proxyhandler.Request,
) (*http.Response, any, error) {
	ctx, span := tracer.Start(ctx, "proxy_request")
	defer span.End()

	route, options, err := pc.prepareRequest(span, request)
	if err != nil {
		return nil, nil, err
	}

	response, responseBody, err := route.Method.Handler.Handle(ctx, request, options)
	if err != nil {
		return nil, nil, pc.handleError(span, err, options.Path)
	}

	span.SetStatus(codes.Ok, "")

	return response, responseBody, nil
}

func (pc *ProxyClient) prepareRequest(
	span trace.Span,
	request *proxyhandler.Request,
) (*internal.Route, *proxyhandler.ProxyHandleOptions, error) {
	span.SetAttributes(
		semconv.HTTPRequestMethodKey.String(request.Method),
		semconv.URLOriginal(request.URL.String()),
	)

	if pc.metadata.Settings != nil && pc.metadata.Settings.Expose != nil &&
		!*pc.metadata.Settings.Expose {
		// This API isn't exposed. Returns HTTP 404
		return nil, nil, goutils.RFC9457Error{
			Status:   http.StatusNotFound,
			Title:    "Resource Not Found",
			Instance: request.URL.Path,
		}
	}

	requestPath := request.URL.Path

	if pc.metadata.Settings != nil && pc.metadata.Settings.BasePath != "" &&
		request.URL.Path != "" {
		// The URL path may omit the slash character
		basePath := pc.metadata.Settings.BasePath
		if requestPath[0] != '/' {
			basePath = basePath[1:]
		}

		requestPath = strings.TrimPrefix(requestPath, basePath)
	}

	span.SetAttributes(semconv.URLPath(requestPath))

	route := pc.node.FindRoute(requestPath, request.Method)
	if route == nil {
		span.SetStatus(codes.Error, "request path or method does not exist")

		return nil, nil, goutils.RFC9457Error{
			Status:   http.StatusNotFound,
			Title:    "Resource Not Found",
			Instance: requestPath,
		}
	}

	span.SetAttributes(
		attribute.String("http.request.proxy.type", string(route.Method.Handler.Type())),
	)

	options := &proxyhandler.ProxyHandleOptions{
		Settings:    pc.metadata.Settings,
		ParamValues: route.ParamValues,
		NewRequest:  pc.newRequestFunc(route),
		Path:        requestPath,
	}

	return route, options, nil
}

func (*ProxyClient) handleError(
	span trace.Span,
	err error,
	requestPath string,
) *goutils.RFC9457Error {
	span.SetStatus(codes.Error, "proxy failed")
	span.RecordError(err)

	rfc9457Error, ok := errors.AsType[*goutils.RFC9457Error](err)
	if ok {
		rfc9457Error.Instance = requestPath

		return rfc9457Error
	}

	return &goutils.RFC9457Error{
		Status:   http.StatusInternalServerError,
		Title:    http.StatusText(http.StatusInternalServerError),
		Detail:   err.Error(),
		Instance: requestPath,
	}
}

func (pc *ProxyClient) newRequestFunc(route *internal.Route) proxyhandler.NewRequestFunc {
	return func(method string, url string) *gohttpc.RequestWithClient {
		req := pc.lbClient.R(method, url)
		reqHeader := req.Header()

		authenticator := pc.authenticators.GetAuthenticator(route.Method.Security)
		if authenticator != nil {
			req.SetAuthenticator(authenticator)
		}

		for key, value := range pc.defaultHeaders {
			reqHeader.Set(key, value)
		}

		if pc.metadata.Settings != nil && pc.metadata.Settings.ForwardHeaders != nil {
			for _, key := range pc.metadata.Settings.ForwardHeaders.Request {
				value := reqHeader.Get(key)
				if value != "" {
					reqHeader.Set(key, value)
				}
			}
		}

		return req
	}
}
