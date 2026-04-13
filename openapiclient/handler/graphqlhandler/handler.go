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

// Package graphqlhandler evaluates and execute GraphQL requests to the remote server.
package graphqlhandler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/hasura/gotel"
	"github.com/hasura/gotel/otelutils"
	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/relychan/gotransform/jmes"
	"github.com/relychan/goutils"
	"github.com/relychan/goutils/httpheader"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/openapiclient/handler/proxyhandler"
	"github.com/relychan/openapitools/openapiclient/handler/resthandler/contenttype"
	"github.com/vektah/gqlparser/v2/ast"
	"go.opentelemetry.io/otel/trace"
	"go.yaml.in/yaml/v4"
)

var tracer = gotel.NewTracer("openapitools/graphqlhandler")

// GraphQLHandler implements the ProxyHandler interface for GraphQL proxy.
type GraphQLHandler struct {
	parameters          []*highv3.Parameter
	url                 string
	method              string
	operationName       string
	query               string
	operation           ast.Operation
	variableDefinitions ast.VariableDefinitionList
	// The configuration to transform request headers.
	headers             map[string]jmes.FieldMappingEntryString
	variables           map[string]jmes.FieldMappingEntry
	extensions          map[string]jmes.FieldMappingEntry
	customResponse      *proxyCustomGraphQLResponse
	responseContentType string
}

// NewGraphQLHandler creates a GraphQL request from operation.
func NewGraphQLHandler( //nolint:ireturn,nolintlint
	operation *highv3.Operation,
	rawProxyAction *yaml.Node,
	options *proxyhandler.NewProxyHandlerOptions,
) (proxyhandler.ProxyHandler, error) {
	if rawProxyAction == nil {
		return nil, ErrProxyActionRequired
	}

	var proxyAction ProxyGraphQLActionConfig

	err := rawProxyAction.Decode(&proxyAction)
	if err != nil {
		return nil, err
	}

	if proxyAction.Request == nil {
		return nil, ErrGraphQLQueryEmpty
	}

	handler, err := ValidateGraphQLString(proxyAction.Request.Query)
	if err != nil {
		return nil, err
	}

	handler.url = proxyAction.Request.URL
	handler.method = proxyAction.Request.Method

	if handler.method == "" {
		handler.method = http.MethodPost
	} else if handler.method != http.MethodPost && handler.method != http.MethodGet {
		return nil, ErrInvalidRequestMethod
	}

	responseContentType := oaschema.GetResponseContentTypeFromOperation(operation)
	if responseContentType == "" {
		handler.responseContentType = httpheader.ContentTypeJSON
	} else {
		handler.responseContentType, err = oaschema.ValidateContentType(
			responseContentType,
		)
		if err != nil {
			return nil, err
		}
	}

	getEnvFunc := options.GetEnvFunc()
	handler.parameters = oaschema.MergeParameters(options.Parameters, operation.Parameters)

	handler.headers, err = jmes.EvaluateObjectFieldMappingStringEntries(
		proxyAction.Request.Headers,
		getEnvFunc,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize custom request headers config: %w", err)
	}

	handler.variables, err = jmes.EvaluateObjectFieldMappingEntries(
		proxyAction.Request.Variables,
		getEnvFunc,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize custom request variables config: %w", err)
	}

	handler.extensions, err = jmes.EvaluateObjectFieldMappingEntries(
		proxyAction.Request.Extensions,
		getEnvFunc,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize custom request extensions config: %w", err)
	}

	handler.customResponse, err = newProxyCustomGraphQLResponse(
		proxyAction.Response,
		getEnvFunc,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize response config: %w", err)
	}

	return handler, err
}

// Type returns type of the current handler.
func (*GraphQLHandler) Type() proxyhandler.ProxyActionType {
	return ProxyTypeGraphQL
}

// Handle resolves the HTTP request and proxies that request to the remote server.
func (ge *GraphQLHandler) Handle(
	ctx context.Context,
	request *proxyhandler.Request,
	options *proxyhandler.ProxyHandleOptions,
) (*http.Response, any, error) {
	graphqlPayload := &GraphQLRequestBody{}

	resp, err := ge.handleRequest(ctx, request, graphqlPayload, options)
	if err != nil {
		return nil, nil, err
	}

	if ge.customResponse == nil || ge.customResponse.IsZero() {
		var respBody any

		err := json.NewDecoder(resp.Body).Decode(&respBody)

		goutils.CatchWarnErrorFunc(resp.Body.Close)

		if err != nil {
			return resp, nil, newGraphQLResponseEncodeError(
				oaschema.ErrCodeResponseTransformError,
				err,
			)
		}

		ge.printLog(
			ctx,
			request,
			graphqlPayload,
			resp,
			respBody,
			err,
		)

		return resp, respBody, err
	}

	respBody, err := ge.handleTransformResponse(ctx, resp)

	ge.printLog(
		ctx,
		request,
		graphqlPayload,
		resp,
		respBody,
		err,
	)

	return resp, respBody, err
}

// Stream resolves the HTTP request and proxies that request to the remote server.
// The response is a stream.
func (ge *GraphQLHandler) Stream(
	ctx context.Context,
	request *proxyhandler.Request,
	writer http.ResponseWriter,
	options *proxyhandler.ProxyHandleOptions,
) (*http.Response, error) {
	graphqlPayload := &GraphQLRequestBody{}

	resp, err := ge.handleRequest(ctx, request, graphqlPayload, options)
	if err != nil {
		return nil, err
	}

	options.ForwardResponseHeaders(writer, resp)

	if ge.customResponse == nil {
		if httpheader.IsContentTypeJSON(ge.responseContentType) {
			writer.Header().Set(httpheader.ContentType, ge.responseContentType)

			// No custom response. Write response directly for json content type
			_, err = io.Copy(writer, resp.Body)

			goutils.CatchWarnErrorFunc(resp.Body.Close)

			return resp, err
		}

		var respBody any

		err := json.NewDecoder(resp.Body).Decode(&respBody)

		goutils.CatchWarnErrorFunc(resp.Body.Close)

		if err != nil {
			return resp, err
		}

		_, err = contenttype.Write(writer, ge.responseContentType, respBody)

		return resp, err
	}

	data, err := ge.writeTransformResponse(ctx, resp, writer)

	ge.printLog(
		ctx,
		request,
		graphqlPayload,
		resp,
		data,
		err,
	)

	return resp, err
}

func (ge *GraphQLHandler) printLog(
	ctx context.Context,
	request *proxyhandler.Request,
	graphqlPayload *GraphQLRequestBody,
	response *http.Response,
	respBody any,
	err error,
) {
	logger := gotel.GetLogger(ctx)
	isDebug := logger.Enabled(ctx, slog.LevelDebug)

	if !isDebug && err == nil {
		return
	}

	span := trace.SpanFromContext(ctx)

	requestLogAttrs := make([]slog.Attr, 0, 7)
	requestLogAttrs = append(
		requestLogAttrs,
		slog.String("url", request.URL()),
		slog.String("operation_name", ge.operationName),
		slog.String("operation_type", string(ge.operation)),
		slog.String("query", graphqlPayload.Query),
	)

	requestHeaders := otelutils.ExtractTelemetryHeaders(request.Header(), nil)
	otelutils.SetSpanHeaderMatrixAttributes(span, "http.request.header", requestHeaders)

	requestLogAttrs = append(requestLogAttrs,
		otelutils.NewHeaderMatrixLogGroupAttrs(
			"headers",
			requestHeaders,
		),
	)

	if isDebug {
		requestLogAttrs = append(requestLogAttrs, slog.Any("variables", graphqlPayload.Variables))

		if len(graphqlPayload.Extensions) > 0 {
			requestLogAttrs = append(
				requestLogAttrs,
				slog.Any("extensions", graphqlPayload.Extensions),
			)
		}
	}

	attrs := make([]slog.Attr, 0, 4)
	attrs = append(
		attrs,
		slog.String("type", "proxy-handler"),
		slog.String("handler_type", "graphql"),
		slog.GroupAttrs("request", requestLogAttrs...),
	)

	var message string

	if response != nil {
		message = response.Status
		respHeaders := otelutils.ExtractTelemetryHeaders(response.Header, nil)

		otelutils.SetSpanHeaderMatrixAttributes(span, "http.response.header", respHeaders)

		respLogAttrs := make([]slog.Attr, 0, 3)
		respLogAttrs = append(
			respLogAttrs,
			slog.Int("status", response.StatusCode),
			otelutils.NewHeaderMatrixLogGroupAttrs(
				"headers",
				respHeaders,
			),
		)

		if isDebug {
			respLogAttrs = append(
				respLogAttrs,
				slog.Any("body", respBody),
			)
		}

		attrs = append(attrs, slog.GroupAttrs("response", respLogAttrs...))
	}

	logLevel := slog.LevelDebug

	if err != nil {
		logLevel = slog.LevelError
		message = err.Error()
	}

	logger.LogAttrs(ctx, logLevel, message, attrs...)
}
