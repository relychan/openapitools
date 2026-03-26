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
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/hasura/gotel"
	"github.com/hasura/gotel/otelutils"
	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/relychan/gotransform/jmes"
	"github.com/relychan/goutils"
	"github.com/relychan/goutils/httpheader"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/openapiclient/handler/proxyhandler"
	"github.com/vektah/gqlparser/ast"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.yaml.in/yaml/v4"
)

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
func (ge *GraphQLHandler) Handle( //nolint:funlen
	ctx context.Context,
	request *proxyhandler.Request,
	options *proxyhandler.ProxyHandleOptions,
) (*http.Response, any, error) {
	span := trace.SpanFromContext(ctx)

	graphqlPayload := GraphQLRequestBody{
		Query:         ge.query,
		OperationName: ge.operationName,
	}

	span.SetAttributes(
		attribute.String("graphql.operation.name", ge.operationName),
		attribute.String("graphql.operation.type", string(ge.operation)),
		attribute.String("graphql.query", ge.query),
	)

	logAttrs := make([]slog.Attr, 0, 13)

	requestData, err := proxyhandler.NewRequestTemplateData(
		request,
		options.ParamValues,
	)
	if err != nil {
		ge.printLog(
			ctx,
			request, "failed to decode body",
			append(
				logAttrs,
				slog.String("error", err.Error()),
			),
		)

		return nil, nil, err
	}

	rawRequestData := requestData.ToMap()

	variables, err := ge.resolveRequestVariables(requestData, rawRequestData)
	if err != nil {
		ge.printLog(
			ctx,
			request, "failed to evaluate variables",
			append(
				logAttrs,
				slog.String("error", err.Error()),
			),
		)

		return nil, nil, err
	}

	graphqlPayload.Variables = variables

	graphqlPayload.Extensions, err = ge.resolveRequestExtensions(rawRequestData)
	if err != nil {
		ge.printLog(
			ctx,
			request, "failed to evaluate extensions",
			append(
				logAttrs,
				slog.String("error", err.Error()),
			),
		)

		return nil, nil, err
	}

	logAttrs = append(logAttrs,
		slog.GroupAttrs(
			"request_graphql",
			slog.String("query", graphqlPayload.Query),
			slog.Any("variables", graphqlPayload.Variables),
			slog.Any("extensions", graphqlPayload.Extensions),
		),
	)

	req := options.NewRequest(http.MethodPost, ge.url)
	reqHeader := req.Header()

	for key, header := range ge.headers {
		value, err := header.EvaluateString(rawRequestData)
		if err != nil {
			respErr := fmt.Errorf("failed to evaluate custom header %s: %w", key, err)

			ge.printLog(
				ctx, request,
				"invalid header",
				append(
					logAttrs,
					slog.String("error", respErr.Error()),
				),
			)

			return nil, nil, respErr
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
			ctx, request,
			"failed to encode graphql payload",
			append(
				logAttrs,
				slog.String("error", err.Error()),
			),
		)

		return nil, nil, err
	}

	req.SetBody(reader)

	resp, err := req.Execute(ctx)
	if err != nil {
		ge.printLog(
			ctx, request,
			"failed to execute request",
			append(
				logAttrs,
				slog.String("error", err.Error()),
			),
		)

		return resp, nil, err
	}

	logAttrs = append(logAttrs, slog.Int("response_status", resp.StatusCode))

	span.SetAttributes(attribute.Int("http.response.original_status_code", resp.StatusCode))

	if resp.Body == nil {
		ge.printLog(
			ctx, request,
			"invalid response",
			append(
				logAttrs,
				slog.String("error", ErrGraphQLResponseRequired.Error()),
			),
		)

		return resp, nil, err
	}

	newResp, respBody, respLogAttrs, err := ge.transformResponse(resp)
	logAttrs = append(logAttrs, respLogAttrs...)

	if err != nil {
		ge.printLog(
			ctx, request,
			"failed to transform response",
			append(
				logAttrs,
				slog.String("error", err.Error()),
			),
		)

		return resp, nil, err
	}

	ge.printLog(ctx, request, resp.Status, logAttrs)

	return newResp, respBody, err
}

// Stream resolves the HTTP request and proxies that request to the remote server.
// The response is a stream.
func (ge *GraphQLHandler) Stream(
	ctx context.Context,
	request *proxyhandler.Request,
	options *proxyhandler.ProxyHandleOptions,
) (*http.Response, error) {
	resp, data, err := ge.Handle(ctx, request, options)
	if err != nil {
		return resp, err
	}

	buf := new(bytes.Buffer)

	err = json.NewEncoder(buf).Encode(data)
	if err != nil {
		return nil, &goutils.ErrorDetail{
			Detail: err.Error(),
			Code:   oaschema.ErrCodeWriteResponseError,
		}
	}

	resp.Body = io.NopCloser(buf)

	return resp, nil
}

func (ge *GraphQLHandler) transformResponse( //nolint:revive
	resp *http.Response,
) (*http.Response, any, []slog.Attr, error) {
	defer goutils.CatchWarnErrorFunc(resp.Body.Close)

	var responseBody map[string]any

	err := json.NewDecoder(resp.Body).Decode(&responseBody)
	if err != nil {
		return resp, nil, nil, fmt.Errorf("failed to decode graphql response: %w", err)
	}

	if ge.customResponse == nil {
		return resp, responseBody, nil, err
	}

	responseLogAttrs := make([]slog.Attr, 0, 3)
	responseLogAttrs = append(responseLogAttrs, slog.Any("original_body", responseBody))

	if ge.customResponse.HTTPErrorCode != nil {
		errorBody, hasError := responseBody["errors"]
		if hasError && errorBody != nil {
			// overwrite the error code.
			resp.StatusCode = *ge.customResponse.HTTPErrorCode
		}
	}

	responseLogAttrs = append(
		responseLogAttrs,
		slog.Int("status_code_final", resp.StatusCode),
	)

	if ge.customResponse.Body == nil || ge.customResponse.Body.IsZero() {
		return resp, responseBody, responseLogAttrs, nil
	}

	transformedBody, err := ge.customResponse.Body.Transform(responseBody)
	if err != nil {
		return resp, responseBody, responseLogAttrs, err
	}

	responseLogAttrs = append(responseLogAttrs, slog.Any("response_body", transformedBody))

	return resp, transformedBody, responseLogAttrs, err
}

func (ge *GraphQLHandler) resolveRequestVariables(
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
				return nil, fmt.Errorf(
					"failed to select value of variable %s: %w",
					varDef.Variable,
					err,
				)
			}

			if value != nil {
				typedValue, err := convertVariableTypeFromUnknownValue(varDef, value)
				if err != nil {
					return nil, fmt.Errorf(
						"failed to evaluate value of variable %s: %w",
						varDef.Variable,
						err,
					)
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
				return nil, fmt.Errorf(
					"failed to evaluate the type of variable %s: %w",
					varDef.Variable,
					err,
				)
			}

			results[varDef.Variable] = typedParam

			continue
		}

		queryValue := requestData.QueryParams.Get(varDef.Variable)
		if queryValue != "" {
			typedValue, err := convertVariableTypeFromString(varDef, queryValue)
			if err != nil {
				return nil, fmt.Errorf(
					"failed to evaluate the type of variable %s: %w",
					varDef.Variable,
					err,
				)
			}

			results[varDef.Variable] = typedValue
		}
	}

	return results, nil
}

func (ge *GraphQLHandler) resolveRequestExtensions(
	rawRequestData map[string]any,
) (map[string]any, error) {
	results := make(map[string]any)

	for key, extension := range ge.extensions {
		value, err := extension.Evaluate(rawRequestData)
		if err != nil {
			return nil, fmt.Errorf("failed to select value of extension %s: %w", key, err)
		}

		results[key] = value
	}

	return results, nil
}

func (ge *GraphQLHandler) printLog(
	ctx context.Context,
	request *proxyhandler.Request,
	message string,
	attrs []slog.Attr,
) {
	logger := gotel.GetLogger(ctx)

	if !logger.Enabled(ctx, slog.LevelDebug) {
		return
	}

	attrs = append(
		attrs,
		slog.String("type", "proxy-handler"),
		slog.String("handler_type", "graphql"),
		slog.String("operation_name", ge.operationName),
		slog.String("operation_type", string(ge.operation)),
		slog.String("request_url", request.URL.String()),
		slog.Any("variables", ge.variables),
		otelutils.NewHeaderMatrixLogGroupAttrs(
			"request_headers",
			otelutils.ExtractTelemetryHeaders(request.Header),
		),
	)

	logger.LogAttrs(ctx, slog.LevelDebug, message, attrs...)
}

func convertVariableTypeFromString(varDef *ast.VariableDefinition, value string) (any, error) {
	if varDef.Type == nil {
		// unknown type. Returns the original value.
		return value, nil
	}

	switch strings.ToLower(varDef.Type.NamedType) {
	case "bool", "boolean":
		return strconv.ParseBool(value)
	case "int", "int8", "int16", "int32", "int64":
		return strconv.ParseInt(value, 10, 64)
	case "uint", "uint8", "uint16", "uint32", "uint64":
		return strconv.ParseUint(value, 10, 64)
	case "number", "decimal", "float", "float32", "float64", "double":
		return strconv.ParseFloat(value, 64)
	default:
		// unknown type. Returns the original value.
		return value, nil
	}
}

func convertVariableTypeFromUnknownValue(varDef *ast.VariableDefinition, value any) (any, error) {
	if varDef.Type == nil || value == nil {
		// unknown type. Returns the original value.
		return value, nil
	}

	if str, ok := value.(string); ok {
		return convertVariableTypeFromString(varDef, str)
	}

	if strPtr, ok := value.(*string); ok {
		if strPtr == nil {
			return nil, nil
		}

		return convertVariableTypeFromString(varDef, *strPtr)
	}

	switch strings.ToLower(varDef.Type.NamedType) {
	case "bool", "boolean":
		return goutils.DecodeNullableBoolean(value)
	case "int", "int8", "int16", "int32", "int64":
		return goutils.DecodeNullableNumber[int64](value)
	case "uint", "uint8", "uint16", "uint32", "uint64":
		return goutils.DecodeNullableNumber[uint64](value)
	case "number", "decimal", "float", "float32", "float64", "double":
		return goutils.DecodeNullableNumber[float64](value)
	default:
		// unknown type. Returns the original value.
		return value, nil
	}
}
