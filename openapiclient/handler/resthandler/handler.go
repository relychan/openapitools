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

// Package resthandler evaluates and execute REST requests to the remote server.
package resthandler

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"

	"github.com/hasura/gotel"
	"github.com/hasura/gotel/otelutils"
	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/relychan/gohttpc"
	"github.com/relychan/goutils"
	"github.com/relychan/goutils/httpheader"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/openapiclient/handler/proxyhandler"
	"github.com/relychan/openapitools/openapiclient/handler/resthandler/contenttype"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
	"go.yaml.in/yaml/v4"
)

var tracer = gotel.NewTracer("openapitools/resthandler")

// RESTfulHandler implements the ProxyHandler interface for RESTful proxy.
type RESTfulHandler struct {
	operation           *highv3.Operation
	requestContentType  string
	responseContentType string
	customRequest       *customRESTRequest
	customResponse      *customRESTResponse
	parameters          []*highv3.Parameter
}

// NewRESTfulHandler creates a RESTHandler from operation.
func NewRESTfulHandler(
	operation *highv3.Operation,
	rawProxyAction *yaml.Node,
	options *proxyhandler.NewProxyHandlerOptions,
) (proxyhandler.ProxyHandler, error) {
	handler := &RESTfulHandler{
		operation:  operation,
		parameters: oaschema.MergeParameters(options.Parameters, operation.Parameters),
	}

	if rawProxyAction == nil {
		requestContentType, err := parseRequestContentType(operation, nil)
		if err != nil {
			return nil, err
		}

		responseContentType, err := parseResponseContentType(operation, nil)
		if err != nil {
			return nil, err
		}

		handler.requestContentType = requestContentType
		handler.responseContentType = responseContentType

		return handler, nil
	}

	var proxyAction ProxyRESTfulActionConfig

	err := rawProxyAction.Decode(&proxyAction)
	if err != nil {
		return nil, err
	}

	requestContentType, err := parseRequestContentType(operation, proxyAction.Request)
	if err != nil {
		return nil, err
	}

	responseContentType, err := parseResponseContentType(operation, proxyAction.Response)
	if err != nil {
		return nil, err
	}

	handler.requestContentType = requestContentType
	handler.responseContentType = responseContentType

	getEnvFunc := options.GetEnvFunc()

	handler.customRequest, err = newCustomRESTRequestFromConfig(proxyAction.Request, getEnvFunc)
	if err != nil {
		return nil, err
	}

	handler.customResponse, err = newCustomRESTResponse(
		proxyAction.Response,
		getEnvFunc,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize response config: %w", err)
	}

	return handler, nil
}

// Type returns type of the current handler.
func (*RESTfulHandler) Type() proxyhandler.ProxyActionType {
	return ProxyActionTypeREST
}

// Handle resolves the HTTP request and proxies that request to the remote server.
func (re *RESTfulHandler) Handle(
	ctx context.Context,
	request *proxyhandler.Request,
	options *proxyhandler.ProxyHandleOptions,
) (*http.Response, any, error) {
	response, responseBody, err := re.handleRequest(ctx, request, options)
	if err != nil || responseBody == nil {
		return response, responseBody, err
	}

	reader, ok := responseBody.(io.Reader)
	if !ok {
		return response, responseBody, nil
	}

	decodedBody, err := contenttype.Decode(response.Header.Get(httpheader.ContentType), reader)
	if err != nil {
		respErr := goutils.NewServerError()
		respErr.Detail = err.Error()

		return response, nil, err
	}

	return response, decodedBody, nil
}

// Stream resolves the HTTP request and proxies that request to the remote server.
// The response is a stream.
func (re *RESTfulHandler) Stream(
	ctx context.Context,
	request *proxyhandler.Request,
	options *proxyhandler.ProxyHandleOptions,
) (*http.Response, error) {
	response, responseBody, err := re.handleRequest(ctx, request, options)
	if err != nil || responseBody == nil {
		return response, err
	}

	switch reader := responseBody.(type) {
	case io.ReadCloser:
		response.Body = reader

		return response, nil
	case io.Reader:
		response.Body = io.NopCloser(reader)

		return response, nil
	default:
		contentType := re.responseContentType
		if contentType == "" {
			contentType = response.Header.Get(httpheader.ContentType)
		}

		respReader, err := contenttype.Encode(contentType, responseBody)
		if err != nil {
			return nil, &goutils.ErrorDetail{
				Detail: err.Error(),
				Code:   oaschema.ErrCodeWriteResponseError,
			}
		}

		response.Body = io.NopCloser(respReader)

		return response, nil
	}
}

func (re *RESTfulHandler) handleRequest(
	ctx context.Context,
	request *proxyhandler.Request,
	options *proxyhandler.ProxyHandleOptions,
) (*http.Response, any, error) {
	logger := otelutils.GetLogger(ctx)
	isDebug := logger.Enabled(ctx, slog.LevelDebug)
	span := trace.SpanFromContext(ctx)

	span.SetAttributes(
		attribute.KeyValue{
			Key:   semconv.HTTPRequestMethodKey,
			Value: attribute.StringValue(request.Method),
		},
		semconv.URLPath(request.URL.Path),
		attribute.String("proxy.type", string(re.Type())),
	)

	req, err := re.prepareRequest(request, options)
	if err != nil {
		re.printRequestLog(
			ctx,
			span,
			logger,
			request,
			nil,
			nil,
			err,
		)

		return nil, nil, err
	}

	resp, err := req.Execute(ctx)
	if err != nil {
		re.printRequestLog(
			ctx,
			span,
			logger,
			request,
			req,
			resp,
			err,
		)

		return resp, nil, err
	}

	if re.customResponse == nil || re.customResponse.IsZero() ||
		(resp.StatusCode < 200 || resp.StatusCode >= 300) {
		re.printRequestLog(
			ctx,
			span,
			logger,
			request,
			req,
			resp,
			nil,
		)

		return resp, resp.Body, nil
	}

	transformedBody, responseAttrs, err := re.transformResponse(ctx, resp, isDebug)
	re.printRequestLog(
		ctx,
		span,
		logger,
		request,
		req,
		resp,
		err,
		responseAttrs...,
	)

	return resp, transformedBody, err
}

func (*RESTfulHandler) printRequestLog(
	ctx context.Context,
	span trace.Span,
	logger *slog.Logger,
	originalRequest *proxyhandler.Request,
	request *gohttpc.RequestWithClient,
	response *http.Response,
	err error,
	responseAttrs ...slog.Attr,
) {
	isDebug := logger.Enabled(ctx, slog.LevelDebug)

	if !isDebug && err == nil {
		return
	}

	requestAttrs := make([]slog.Attr, 0, 5)

	if request != nil {
		requestAttrs = append(
			requestAttrs,
			slog.String("url", request.URL()),
			slog.String("method", request.Method()),
		)
	}

	requestHeaders := otelutils.ExtractTelemetryHeaders(originalRequest.Header)
	otelutils.SetSpanHeaderMatrixAttributes(span, "http.request.header", requestHeaders)

	requestAttrs = append(requestAttrs,
		slog.String("original_path", originalRequest.URL.Path),
		slog.String("original_method", originalRequest.Method),
		otelutils.NewHeaderMatrixLogGroupAttrs(
			"headers",
			requestHeaders,
		),
	)

	attrs := make([]slog.Attr, 0, 4)
	attrs = append(
		attrs,
		slog.String("type", "proxy-handler"),
		slog.String("handler_type", "rest"),
		slog.GroupAttrs("request", requestAttrs...),
	)

	if response != nil {
		responseAttrs = slices.Grow(responseAttrs, 2)
		respHeaders := otelutils.ExtractTelemetryHeaders(response.Header)

		responseAttrs := append(
			responseAttrs,
			slog.Int("status", response.StatusCode),
			otelutils.NewHeaderMatrixLogGroupAttrs(
				"headers",
				respHeaders,
			),
		)

		attrs = append(attrs, slog.GroupAttrs(
			"response",
			responseAttrs...,
		))

		otelutils.SetSpanHeaderMatrixAttributes(span, "http.response.header", respHeaders)
	}

	var message string

	logLevel := slog.LevelDebug

	if err != nil {
		message = err.Error()
	}

	logger.LogAttrs(ctx, logLevel, message, attrs...)
}
