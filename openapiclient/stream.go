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

	"github.com/relychan/goutils"
	"github.com/relychan/goutils/httpheader"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/openapiclient/handler/proxyhandler"
	"github.com/relychan/openapitools/openapiclient/handler/resthandler/contenttype"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
)

func (pc *ProxyClient) ServeHTTP(
	writer http.ResponseWriter,
	request *http.Request,
) {
	spanName := pc.buildSpanName("Stream", request.URL)

	ctx, span := tracer.Start(request.Context(), spanName)
	defer span.End()

	span.SetAttributes(
		semconv.HTTPRequestMethodKey.String(request.Method),
		semconv.URLOriginal(request.URL.String()),
	)

	req := proxyhandler.NewRequest(request.Method, request.URL, request.Header, nil)

	// req, err := validateHTTPRequest(writer, request)
	// if err != nil {
	// 	span.SetStatus(codes.Error, "failed to validate request body")
	// 	span.RecordError(err)

	// 	return nil, err
	// }

	route, options, notFoundErr := pc.findRoute(span, req)
	if notFoundErr != nil {
		writeErrorResponse(writer, notFoundErr.Status, notFoundErr)

		return
	}

	_, err := route.Method.Handler.Stream(ctx, req, writer, options) //nolint:bodyclose
	if err != nil {
		status, respErr := pc.handleError(span, err, request.URL.Path)

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
	spanName := pc.buildSpanName("Stream", request.GetURL())

	ctx, span := tracer.Start(ctx, spanName)
	defer span.End()

	span.SetAttributes(
		semconv.HTTPRequestMethodKey.String(request.Method()),
		semconv.URLOriginal(request.URL()),
	)

	route, options, notFoundErr := pc.findRoute(span, request)
	if notFoundErr != nil {
		writeErrorResponse(writer, notFoundErr.Status, notFoundErr)

		return nil, notFoundErr
	}

	response, err := route.Method.Handler.Stream(ctx, request, writer, options)
	if err != nil {
		status, respErr := pc.handleError(span, err, request.GetURL().Path)

		writeErrorResponse(writer, status, respErr)

		return response, respErr
	}

	span.SetStatus(codes.Ok, "")

	return response, nil
}

// validateHTTPRequest validates and create a new proxy request from an HTTP request.
func validateHTTPRequest(
	writer http.ResponseWriter,
	request *http.Request,
) (*proxyhandler.Request, error) {
	req := proxyhandler.NewRequest(request.Method, request.URL, request.Header, nil)

	if request.Body == nil || request.Body == http.NoBody {
		return req, nil
	}

	contentType := request.Header.Get(httpheader.ContentType)

	decodedBody, err := contenttype.Decode(contentType, request.Body)
	if err == nil {
		req.SetBody(decodedBody)

		return req, nil
	}

	errorDetail, ok := errors.AsType[*goutils.ErrorDetail](err)
	if !ok {
		errorDetail = &goutils.ErrorDetail{
			Detail: err.Error(),
			Code:   oaschema.ErrCodeRequestDecodeBodyError,
		}
	}

	respErr := goutils.NewBadRequestError(*errorDetail)
	respErr.Detail = "failed to decode request"

	if writer != nil {
		writeErrorResponse(writer, respErr.Status, respErr)
	}

	return nil, respErr
}
