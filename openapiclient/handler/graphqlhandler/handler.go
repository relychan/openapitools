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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/hasura/gotel"
	"github.com/hasura/gotel/otelutils"
	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/relychan/gohttpc"
	"github.com/relychan/gotransform/jmes"
	"github.com/relychan/goutils"
	"github.com/relychan/goutils/httpheader"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/openapiclient/handler/proxyhandler"
	"github.com/vektah/gqlparser/ast"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.yaml.in/yaml/v4"
)

var tracer = gotel.NewTracer("openapitools/graphqlhandler")

// GraphQLHandler implements the ProxyHandler interface for GraphQL proxy.
type GraphQLHandler struct {
	parameters          []*highv3.Parameter
	url                 string
	operationName       string
	query               string
	operation           ast.Operation
	variableDefinitions ast.VariableDefinitionList
	// The configuration to transform request headers.
	headers        map[string]jmes.FieldMappingEntryString
	variables      map[string]jmes.FieldMappingEntry
	extensions     map[string]jmes.FieldMappingEntry
	customResponse *ProxyCustomGraphQLResponse
}

// NewGraphQLHandler creates a GraphQL request from operation.
func NewGraphQLHandler( //nolint:ireturn,nolintlint
	operation *highv3.Operation,
	rawProxyAction *yaml.Node,
	options *proxyhandler.NewProxyHandlerOptions,
) (proxyhandler.ProxyHandler, error) {
	if rawProxyAction == nil {
		return nil, ErrProxyActionInvalid
	}

	var proxyAction ProxyGraphQLActionConfig

	err := rawProxyAction.Decode(&proxyAction)
	if err != nil {
		return nil, err
	}

	if proxyAction.Request == nil {
		return nil, fmt.Errorf("%w: proxy request config is required", ErrProxyActionInvalid)
	}

	handler, err := ValidateGraphQLString(proxyAction.Request.Query)
	if err != nil {
		return nil, err
	}

	handler.url = proxyAction.Request.URL

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

	handler.customResponse, err = NewProxyCustomGraphQLResponse(
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
	span := trace.SpanFromContext(ctx)

	graphqlPayload := &GraphQLRequestBody{
		Query:         ge.query,
		OperationName: ge.operationName,
	}

	span.SetAttributes(
		attribute.String("graphql.operation.name", ge.operationName),
		attribute.String("graphql.operation.type", string(ge.operation)),
		attribute.String("graphql.query", ge.query),
	)

	req, err := ge.prepareRequest(ctx, request, graphqlPayload, options)
	if err != nil {
		ge.printLog(
			ctx,
			request,
			graphqlPayload,
			nil,
			err,
			nil,
		)

		return nil, nil, err
	}

	resp, err := req.Execute(ctx)
	if err != nil {
		ge.printLog(
			ctx,
			request,
			graphqlPayload,
			resp,
			err,
			nil,
		)

		return resp, nil, err
	}

	span.SetAttributes(attribute.Int("http.response.original_status", resp.StatusCode))

	if resp.Body == nil {
		errorDetail := goutils.ErrorDetail{
			Detail: "graphql response must be a valid JSON object",
			Code:   oaschema.ErrCodeGraphQLResponseEmpty,
		}

		ge.printLog(
			ctx,
			request,
			graphqlPayload,
			resp,
			errorDetail,
			nil,
		)

		respErr := goutils.NewServerError(errorDetail)
		respErr.Detail = "failed to encode graphql response"
		respErr.Instance = request.GetURL().String()

		return resp, nil, respErr
	}

	newResp, respBody, err := ge.transformResponse(ctx, request, resp)

	ge.printLog(
		ctx,
		request,
		graphqlPayload,
		resp,
		respBody,
		err,
	)

	return newResp, respBody, err
}

// Stream resolves the HTTP request and proxies that request to the remote server.
// The response is a stream.
func (ge *GraphQLHandler) Stream(
	ctx context.Context,
	request *proxyhandler.Request,
	writer http.ResponseWriter,
	options *proxyhandler.ProxyHandleOptions,
) (*http.Response, error) {
	resp, data, err := ge.Handle(ctx, request, options)
	if err != nil {
		return resp, err
	}

	err = json.NewEncoder(writer).Encode(data)
	if err != nil {
		return nil, newGraphQLResponseEncodeError(request, oaschema.ErrCodeWriteResponseError, err)
	}

	return resp, nil
}

func (ge *GraphQLHandler) prepareRequest(
	ctx context.Context,
	request *proxyhandler.Request,
	graphqlPayload *GraphQLRequestBody,
	options *proxyhandler.ProxyHandleOptions,
) (*gohttpc.RequestWithClient, error) {
	requestData := proxyhandler.NewRequestTemplateData(
		request,
		options.ParamValues,
	)

	rawRequestData := requestData.ToMap()

	variables, err := ge.resolveRequestVariables(request, requestData, rawRequestData)
	if err != nil {
		ge.printLog(
			ctx,
			request,
			graphqlPayload,
			nil,
			err,
			nil,
		)

		return nil, err
	}

	graphqlPayload.Variables = variables

	graphqlPayload.Extensions, err = ge.resolveRequestExtensions(request, rawRequestData)
	if err != nil {
		ge.printLog(
			ctx,
			request,
			graphqlPayload,
			nil,
			err,
			nil,
		)

		return nil, err
	}

	req := options.NewRequest(http.MethodPost, ge.url)
	reqHeader := req.Header()

	for key, header := range ge.headers {
		value, err := header.EvaluateString(rawRequestData)
		if err != nil {
			respErr := fmt.Errorf("failed to evaluate custom header %s: %w", key, err)

			ge.printLog(
				ctx,
				request,
				graphqlPayload,
				nil,
				err,
				nil,
			)

			return nil, respErr
		}

		if value != nil && *value != "" {
			reqHeader.Set(key, *value)
		}
	}

	reqHeader.Set(httpheader.ContentType, httpheader.ContentTypeJSON)

	reader := new(bytes.Buffer)

	enc := json.NewEncoder(reader)
	enc.SetEscapeHTML(false)

	err = enc.Encode(graphqlPayload)
	if err != nil {
		ge.printLog(
			ctx,
			request,
			graphqlPayload,
			nil,
			err,
			nil,
		)

		return nil, err
	}

	req.SetBody(reader)

	return req, nil
}

func (ge *GraphQLHandler) transformResponse(
	ctx context.Context,
	request *proxyhandler.Request,
	resp *http.Response,
) (*http.Response, any, error) {
	_, span := tracer.Start(ctx, "transform_response", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()

	defer goutils.CatchWarnErrorFunc(resp.Body.Close)

	var responseBody map[string]any

	err := json.NewDecoder(resp.Body).Decode(&responseBody)
	if err != nil {
		span.SetStatus(codes.Error, "failed to decode response body")
		span.RecordError(err)

		return resp, nil, newGraphQLResponseEncodeError(
			request,
			oaschema.ErrCodeResponseTransformError,
			err,
		)
	}

	if ge.customResponse == nil {
		span.SetStatus(codes.Ok, "")

		return resp, responseBody, nil
	}

	if ge.customResponse.HTTPErrorCode != nil {
		errorBody, hasError := responseBody["errors"]
		if hasError && errorBody != nil {
			// overwrite the error code.
			resp.StatusCode = *ge.customResponse.HTTPErrorCode
		}
	}

	if ge.customResponse.Body == nil || ge.customResponse.Body.IsZero() {
		span.SetStatus(codes.Ok, "")

		return resp, responseBody, nil
	}

	transformedBody, err := ge.customResponse.Body.Transform(responseBody)
	if err != nil {
		span.SetStatus(codes.Error, "failed to transform response body")
		span.RecordError(err)

		return resp, responseBody, newGraphQLResponseEncodeError(
			request,
			oaschema.ErrCodeResponseTransformError,
			err,
		)
	}

	span.SetStatus(codes.Ok, "")

	return resp, transformedBody, nil
}

func (ge *GraphQLHandler) resolveRequestVariables(
	request *proxyhandler.Request,
	requestData *proxyhandler.RequestTemplateData,
	rawRequestData map[string]any,
) (map[string]any, error) {
	results := make(map[string]any)

	if len(ge.variableDefinitions) == 0 {
		return results, nil
	}

	for _, varDef := range ge.variableDefinitions {
		// Resolve graphql variables. Variables are resolved in order:
		// - In proxy config.
		// - In request parameters, query and body.
		// - Default value in config.
		variable, ok := ge.variables[varDef.Variable]
		if ok {
			value, err := variable.Evaluate(rawRequestData)
			if err != nil {
				respErr := goutils.NewBadRequestError(goutils.ErrorDetail{
					Detail:  err.Error(),
					Pointer: "/variables/" + varDef.Variable,
				})
				respErr.Detail = "failed to select value of variable"
				respErr.Instance = request.GetURL().Path

				return nil, respErr
			}

			if value != nil {
				typedValue, err := convertVariableTypeFromUnknownValue(varDef, value)
				if err != nil {
					respErr := goutils.NewBadRequestError(goutils.ErrorDetail{
						Detail:  err.Error(),
						Pointer: "/variables/" + varDef.Variable,
					})
					respErr.Detail = "failed to evaluate value of variable"
					respErr.Instance = request.GetURL().Path

					return nil, respErr
				}

				results[varDef.Variable] = typedValue
			} else {
				results[varDef.Variable] = value
			}

			continue
		}

		if varDef.Variable == "body" {
			results[varDef.Variable] = requestData.Body

			continue
		}

		param, ok := requestData.Params[varDef.Variable]
		if ok && param != "" {
			typedParam, err := convertVariableTypeFromString(varDef, param)
			if err != nil {
				respErr := goutils.NewBadRequestError(goutils.ErrorDetail{
					Detail:  err.Error(),
					Pointer: "/variables/" + varDef.Variable,
				})
				respErr.Detail = "failed to evaluate the type of variable"
				respErr.Instance = request.GetURL().Path

				return nil, respErr
			}

			results[varDef.Variable] = typedParam

			continue
		}

		queryValue := requestData.QueryParams.Get(varDef.Variable)
		if queryValue != "" {
			typedValue, err := convertVariableTypeFromString(varDef, queryValue)
			if err != nil {
				respErr := goutils.NewBadRequestError(goutils.ErrorDetail{
					Detail:  err.Error(),
					Pointer: "/variables/" + varDef.Variable,
				})
				respErr.Detail = "failed to evaluate the type of variable"
				respErr.Instance = request.GetURL().Path

				return nil, respErr
			}

			results[varDef.Variable] = typedValue
		}
	}

	return results, nil
}

func (ge *GraphQLHandler) resolveRequestExtensions(
	request *proxyhandler.Request,
	rawRequestData map[string]any,
) (map[string]any, error) {
	results := make(map[string]any)

	for key, extension := range ge.extensions {
		value, err := extension.Evaluate(rawRequestData)
		if err != nil {
			respErr := goutils.NewBadRequestError(goutils.ErrorDetail{
				Detail:  err.Error(),
				Pointer: "/extensions/" + key,
			})
			respErr.Detail = "failed to select value of extension"
			respErr.Instance = request.GetURL().Path

			return nil, respErr
		}

		results[key] = value
	}

	return results, nil
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

	requestHeaders := otelutils.ExtractTelemetryHeaders(request.Header())
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
		respHeaders := otelutils.ExtractTelemetryHeaders(response.Header)

		attrs = append(attrs, slog.GroupAttrs(
			"response",
			slog.Int("status", response.StatusCode),
		))

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
