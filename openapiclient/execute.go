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
	"net/url"
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
	writer http.ResponseWriter,
	request *http.Request,
) (*http.Response, error) {
	spanName := pc.buildSpanName("Stream", request.URL)

	ctx, span := tracer.Start(request.Context(), spanName)
	defer span.End()

	span.SetAttributes(
		semconv.HTTPRequestMethodKey.String(request.Method),
		semconv.URLOriginal(request.URL.String()),
	)

	req, err := newRequest(writer, request)
	if err != nil {
		span.SetStatus(codes.Error, "failed to decode request body")
		span.RecordError(err)

		return nil, err
	}

	route, options, err := pc.prepareRequest(span, writer, req)
	if err != nil {
		return nil, err
	}

	response, err := route.Method.Handler.Stream(ctx, req, writer, options)
	if err != nil {
		status, respErr := pc.handleError(span, err, request.URL.Path)

		writeErrorResponse(writer, status, respErr)

		return response, respErr
	}

	span.SetStatus(codes.Ok, "")

	return response, nil
}

// Execute routes and proxies the request to the remote server.
func (pc *ProxyClient) Execute(
	ctx context.Context,
	method string,
	requestPath string,
	header http.Header,
	body any,
) (*http.Response, any, error) {
	requestURL, err := goutils.ParsePathOrHTTPURL(requestPath)
	if err != nil {
		respErr := goutils.NewBadRequestError()
		respErr.Detail = err.Error()

		return nil, nil, respErr
	}

	ctx, span := tracer.Start(ctx, pc.buildSpanName("Proxy", requestURL))
	defer span.End()

	span.SetAttributes(
		semconv.HTTPRequestMethodKey.String(method),
		semconv.URLOriginal(requestPath),
	)

	request := proxyhandler.NewRequest(method, requestURL, header, body)

	route, options, err := pc.prepareRequest(span, nil, request)
	if err != nil {
		return nil, nil, err
	}

	response, responseBody, err := route.Method.Handler.Handle(ctx, request, options)
	if err != nil {
		_, respError := pc.handleError(span, err, requestURL.Path)

		return nil, nil, respError
	}

	span.SetStatus(codes.Ok, "")

	return response, responseBody, nil
}

func (pc *ProxyClient) prepareRequest(
	span trace.Span,
	writer http.ResponseWriter,
	request *proxyhandler.Request,
) (*internal.Route, *proxyhandler.ProxyHandleOptions, error) {
	if pc.CustomAttributesFunc != nil {
		span.SetAttributes(pc.CustomAttributesFunc(request)...)
	}

	requestURL := request.GetURL()
	originalPath := requestURL.Path

	if pc.settings != nil &&
		pc.settings.BasePath != "" &&
		pc.settings.BasePath != "/" &&
		requestURL.Path != "" {
		// The URL path may omit the slash character
		basePath := pc.settings.BasePath
		if requestURL.Path[0] != '/' {
			basePath = basePath[1:]
		}

		requestURL.Path = strings.TrimPrefix(requestURL.Path, basePath)
	}

	route := pc.node.FindRoute(requestURL.Path, request.Method())
	if route == nil {
		span.SetStatus(codes.Error, "request path or method does not exist")

		err := goutils.NewNotFoundError()
		err.Instance = originalPath

		if writer != nil {
			writeErrorResponse(writer, err.Status, err)
		}

		return nil, nil, err
	}

	span.SetAttributes(semconv.URLPath(requestURL.Path))

	span.SetAttributes(
		attribute.String("http.request.proxy.type", string(route.Method.Handler.Type())),
	)

	options := &proxyhandler.ProxyHandleOptions{
		Settings:    pc.settings,
		ParamValues: route.ParamValues,
		NewRequest:  pc.newRequestFunc(request, route),
	}

	return route, options, nil
}

func (*ProxyClient) handleError(
	span trace.Span,
	err error,
	requestPath string,
) (int, error) {
	span.SetStatus(codes.Error, "proxy failed")
	span.RecordError(err)

	rfc9457Error, ok := errors.AsType[*goutils.RFC9457Error](err)
	if ok {
		rfc9457Error.Instance = requestPath

		return rfc9457Error.Status, rfc9457Error
	}

	exError, ok := errors.AsType[*goutils.RFC9457ErrorWithExtensions](err)
	if ok {
		exError.Instance = requestPath

		return exError.Status, exError
	}

	respError := goutils.NewServerError()
	respError.Detail = err.Error()
	respError.Instance = requestPath

	return respError.Status, respError
}

func (pc *ProxyClient) newRequestFunc(
	request *proxyhandler.Request,
	route *internal.Route,
) proxyhandler.NewRequestFunc {
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

		headers := request.Header()

		if len(headers) > 0 &&
			pc.settings != nil &&
			pc.settings.ForwardHeaders != nil {
			for _, key := range pc.settings.ForwardHeaders.Request {
				value := headers.Get(key)
				if value != "" {
					reqHeader.Set(key, value)
				}
			}
		}

		return req
	}
}

func (pc *ProxyClient) buildSpanName(prefix string, requestURL *url.URL) string {
	if pc.TraceHighCardinalityPath {
		return prefix + " " + requestURL.String()
	}

	return prefix
}
