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
	"net/http"

	"github.com/relychan/openapitools/openapiclient/handler/proxyhandler"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
)

func (pc *ProxyClient) ServeHTTP(
	writer http.ResponseWriter,
	request *http.Request,
) {
	spanName := pc.buildSpanName("Stream", request.URL.Path)

	ctx, span := tracer.Start(request.Context(), spanName)
	defer span.End()

	span.SetAttributes(
		semconv.HTTPRequestMethodKey.String(request.Method),
		semconv.URLOriginal(request.URL.String()),
	)

	req := proxyhandler.NewRequest(request.Method, request.URL, request.Header, nil)

	route, notFoundErr := pc.findRoute(span, req)
	if notFoundErr != nil {
		writeErrorResponse(writer, notFoundErr.Status, notFoundErr)

		return
	}

	requestBody, err := parseHTTPRequestBody(route, writer, request)
	if err != nil {
		span.SetStatus(codes.Error, "failed to parse request body")
		span.RecordError(err)

		return
	}

	req.SetBody(requestBody)

	validationErr := validateRequest(route, req, request.Cookies())
	if validationErr != nil {
		span.SetStatus(codes.Error, "Failed to validate request")
		span.RecordError(validationErr)

		writeErrorResponse(writer, validationErr.Status, validationErr)

		return
	}

	options := &proxyhandler.ProxyHandleOptions{
		Settings:   pc.settings,
		NewRequest: pc.newRequestFunc(req, route),
	}

	_, streamErr := route.Method.Handler.Stream(ctx, req, writer, options) //nolint:bodyclose
	if streamErr != nil {
		status, respErr := pc.handleError(span, streamErr, request.URL.Path)

		writeErrorResponse(writer, status, respErr)

		return
	}

	span.SetStatus(codes.Ok, "")
}

// Stream routes the request to the remote server. The response will be transformed and written into the stream.
func (pc *ProxyClient) Stream(
	ctx context.Context,
	writer http.ResponseWriter,
	request *proxyhandler.Request,
) (*http.Response, error) {
	spanName := pc.buildSpanName("Stream", request.Path())

	ctx, span := tracer.Start(ctx, spanName)
	defer span.End()

	span.SetAttributes(
		semconv.HTTPRequestMethodKey.String(request.Method()),
		semconv.URLOriginal(request.URL()),
	)

	route, notFoundErr := pc.findRoute(span, request)
	if notFoundErr != nil {
		writeErrorResponse(writer, notFoundErr.Status, notFoundErr)

		return nil, notFoundErr
	}

	validationErr := validateRequest(route, request, nil)
	if validationErr != nil {
		span.SetStatus(codes.Error, "Failed to validate request")
		span.RecordError(validationErr)

		writeErrorResponse(writer, validationErr.Status, validationErr)

		return nil, validationErr
	}

	options := &proxyhandler.ProxyHandleOptions{
		Settings:   pc.settings,
		NewRequest: pc.newRequestFunc(request, route),
	}

	response, err := route.Method.Handler.Stream(ctx, request, writer, options)
	if err != nil {
		status, respErr := pc.handleError(span, err, request.Path())

		writeErrorResponse(writer, status, respErr)

		return response, respErr
	}

	span.SetStatus(codes.Ok, "")

	return response, nil
}
