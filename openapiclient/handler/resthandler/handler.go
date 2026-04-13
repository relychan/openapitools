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
	"log/slog"
	"net/http"

	"github.com/hasura/gotel"
	"github.com/hasura/gotel/otelutils"
	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/relychan/gohttpc"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/openapiclient/handler/proxyhandler"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
	"go.yaml.in/yaml/v4"
)

var tracer = gotel.NewTracer("openapitools/resthandler")

// RESTfulHandler implements the ProxyHandler interface for RESTful proxy.
type RESTfulHandler struct {
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
	return re.handleRequest(ctx, request, nil, options)
}

// Stream resolves the HTTP request and proxies that request to the remote server.
// The response is a stream.
func (re *RESTfulHandler) Stream(
	ctx context.Context,
	request *proxyhandler.Request,
	writer http.ResponseWriter,
	options *proxyhandler.ProxyHandleOptions,
) (*http.Response, error) {
	response, _, err := re.handleRequest(ctx, request, writer, options)

	return response, err
}

func (re *RESTfulHandler) handleRequest(
	ctx context.Context,
	request *proxyhandler.Request,
	writer http.ResponseWriter,
	options *proxyhandler.ProxyHandleOptions,
) (*http.Response, any, error) {
	logger := otelutils.GetLogger(ctx)
	span := trace.SpanFromContext(ctx)

	span.SetAttributes(
		attribute.KeyValue{
			Key:   semconv.HTTPRequestMethodKey,
			Value: attribute.StringValue(request.Method()),
		},
		semconv.URLPath(request.GetURL().Path),
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

	// Decode and return/stream the response directly if there is no custom response config.
	if re.customResponse == nil || re.customResponse.IsZero() ||
		(resp.StatusCode < 200 || resp.StatusCode >= 300) {
		var (
			respBody  any
			respError error
		)

		if writer == nil {
			respBody, respError = re.decodeRawResponse(ctx, resp)
		} else {
			respError = re.writeRawResponse(ctx, resp, writer, options)
		}

		re.printRequestLog(
			ctx,
			span,
			logger,
			request,
			req,
			resp,
			respError,
		)

		return resp, respBody, respError
	}

	transformedBody, err := re.transformResponse(ctx, logger, resp, writer)
	re.printRequestLog(
		ctx,
		span,
		logger,
		request,
		req,
		resp,
		err,
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

	requestHeaders := otelutils.ExtractTelemetryHeaders(originalRequest.Header(), nil)
	otelutils.SetSpanHeaderMatrixAttributes(span, "http.request.header", requestHeaders)

	requestAttrs = append(requestAttrs,
		slog.String("original_path", originalRequest.GetURL().Path),
		slog.String("original_method", originalRequest.Method()),
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
		respHeaders := otelutils.ExtractTelemetryHeaders(response.Header, nil)

		attrs = append(attrs, slog.GroupAttrs(
			"response",
			slog.Int("status", response.StatusCode),
			otelutils.NewHeaderMatrixLogGroupAttrs(
				"headers",
				respHeaders,
			),
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
