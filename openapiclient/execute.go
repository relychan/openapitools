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
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/hasura/gotel"
	"github.com/relychan/goutils"
	"github.com/relychan/goutils/httperror"
	"github.com/relychan/openapitools/openapiclient/handler/proxyhandler"
	"github.com/relychan/openapitools/openapiclient/internal"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

// Execute routes and proxies the request to the remote server.
func (pc *ProxyClient) Execute(
	ctx context.Context,
	method string,
	requestPath string,
	header http.Header,
	body any,
) (*http.Response, any, error) {
	requestURL, err := goutils.ParseRelativeURI(requestPath)
	if err != nil {
		respErr := httperror.NewBadRequestError()

		ve, ok := errors.AsType[*httperror.ValidationError](err)
		if ok {
			respErr.Errors = []httperror.ValidationError{*ve}
		} else {
			respErr.Detail = err.Error()
		}

		return nil, nil, respErr
	}

	// Decode the URL path the same way as the HTTP server does for ServeHTTP.
	// The upstream request builder escapes the path again, so it must not be escaped twice.
	decodeErr := decodeURLPath(requestURL)
	if decodeErr != nil {
		respErr := httperror.NewBadRequestError(httperror.ValidationError{
			Code:   goutils.ErrCodeInvalidPath,
			Detail: "Invalid URL path encoding",
		})
		respErr.Instance = requestURL.Path

		return nil, nil, respErr
	}

	ctx, span := tracer.Start(ctx, pc.buildSpanName("Proxy", requestURL.Path))
	defer span.End()

	span.SetAttributes(
		semconv.HTTPRequestMethodKey.String(method),
		semconv.URLOriginal(requestPath),
	)

	request := proxyhandler.NewRequest(method, requestURL, header, body)

	route, routeErr := pc.findRoute(span, request)
	if routeErr != nil {
		return nil, nil, routeErr
	}

	validationErr := validateRequest(route, request, nil)
	if validationErr != nil {
		span.SetStatus(codes.Error, "Failed to validate request")
		span.RecordError(validationErr)

		return nil, nil, validationErr
	}

	options := &proxyhandler.ProxyHandleOptions{
		Settings:   pc.settings,
		NewRequest: pc.newRequestFunc(request, route),
	}

	response, responseBody, err := route.Method.Handler.Handle(ctx, request, options)
	if err != nil {
		_, respError := pc.handleError(ctx, span, err, requestURL.Path)

		return nil, nil, respError
	}

	span.SetStatus(codes.Ok, "")

	return response, responseBody, nil
}

func (pc *ProxyClient) findRoute(
	span trace.Span,
	request *proxyhandler.Request,
) (*internal.Route, *httperror.HTTPError) {
	if pc.CustomAttributesFunc != nil {
		span.SetAttributes(pc.CustomAttributesFunc(request)...)
	}

	requestPath := request.Path()

	pathErr := goutils.ValidateURLPath(requestPath)
	if pathErr != nil {
		respErr := httperror.NewBadRequestError(*pathErr)
		respErr.Instance = requestPath

		span.SetStatus(codes.Error, "invalid request path")
		span.RecordError(respErr)

		return nil, respErr
	}

	if pc.settings != nil {
		var ok bool

		requestPath, ok = trimBasePath(requestPath, pc.settings.BasePath)
		if !ok {
			respErr := httperror.NewNotFoundError()
			respErr.Instance = request.Path()

			span.SetStatus(codes.Error, respErr.Detail)
			span.RecordError(respErr)

			return nil, respErr
		}
	}

	route, routeError := pc.node.FindRoute(requestPath, request.Method())
	if routeError != nil {
		span.SetStatus(codes.Error, routeError.Detail)
		span.RecordError(routeError)

		routeError.Instance = request.Path()

		return nil, routeError
	}

	span.SetAttributes(
		semconv.URLTemplate(route.Pattern),
		attribute.String("http.request.proxy.type", string(route.Method.Handler.Type())),
	)

	request.SetPath(requestPath)

	return route, nil
}

func (*ProxyClient) handleError(
	ctx context.Context,
	span trace.Span,
	err error,
	requestPath string,
) (int, error) {
	span.SetStatus(codes.Error, "proxy failed")
	span.RecordError(err)

	rfc9457Error, ok := errors.AsType[*httperror.HTTPError](err)
	if ok {
		rfc9457Error.Instance = requestPath

		return rfc9457Error.Status, rfc9457Error
	}

	httpError, ok := errors.AsType[httperror.HTTPError](err)
	if ok {
		httpError.Instance = requestPath

		return httpError.Status, &httpError
	}

	exError, ok := errors.AsType[*goutils.HTTPErrorWithExtensions](err)
	if ok {
		exError.Instance = requestPath

		return exError.Status, exError
	}

	validationErr, ok := errors.AsType[*httperror.ValidationError](err)
	if ok && validationErr != nil {
		respError := httperror.NewBadRequestError(*validationErr)
		respError.Instance = requestPath

		return respError.Status, respError
	}

	validationErrValue, ok := errors.AsType[httperror.ValidationError](err)
	if ok {
		respError := httperror.NewBadRequestError(validationErrValue)
		respError.Instance = requestPath

		return respError.Status, respError
	}

	// Do not leak internal error details to the client.
	gotel.GetLogger(ctx).ErrorContext(
		ctx,
		"failed to proxy the request",
		slog.String("error", err.Error()),
		slog.String("path", requestPath),
	)

	respError := httperror.NewServerError()
	respError.Detail = "failed to proxy the request"
	respError.Instance = requestPath

	return respError.Status, respError
}

// trimBasePath trims the base path from the request path.
// The base path must match the whole request path or be followed by a slash.
// Returns false if the request path is outside the base path.
func trimBasePath(requestPath string, basePath string) (string, bool) {
	basePath = strings.TrimRight(basePath, "/")
	if basePath == "" {
		return requestPath, true
	}

	if basePath[0] != '/' {
		basePath = "/" + basePath
	}

	if requestPath == basePath {
		return "/", true
	}

	if strings.HasPrefix(requestPath, basePath+"/") {
		return requestPath[len(basePath):], true
	}

	return "", false
}

// decodeURLPath decodes the escaped path of the URL into Path and keeps the original one in RawPath.
func decodeURLPath(uri *url.URL) error {
	rawPath := uri.Path

	decodedPath, err := url.PathUnescape(rawPath)
	if err != nil {
		return err
	}

	uri.Path = decodedPath
	uri.RawPath = ""

	if decodedPath != rawPath {
		uri.RawPath = rawPath
	}

	return nil
}

func (pc *ProxyClient) buildSpanName(prefix string, requestPath string) string {
	if pc.TraceHighCardinalityPath {
		return prefix + " " + requestPath
	}

	return prefix
}
