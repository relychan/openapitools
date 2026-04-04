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

package graphqlhandler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/relychan/gohttpc"
	"github.com/relychan/goutils"
	"github.com/relychan/goutils/httpheader"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/openapiclient/handler/proxyhandler"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const acceptContentTypes = httpheader.ContentTypeJSON + ", application/graphql-response+json"

func (ge *GraphQLHandler) handleRequest(
	ctx context.Context,
	request *proxyhandler.Request,
	graphqlPayload *GraphQLRequestBody,
	options *proxyhandler.ProxyHandleOptions,
) (*http.Response, error) {
	span := trace.SpanFromContext(ctx)

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
			nil,
			err,
		)

		return nil, err
	}

	resp, err := req.Execute(ctx)
	if err != nil {
		ge.printLog(
			ctx,
			request,
			graphqlPayload,
			resp,
			nil,
			err,
		)

		return resp, err
	}

	span.SetAttributes(attribute.Int("http.response.original_status", resp.StatusCode))

	if resp.StatusCode >= http.StatusBadRequest {
		var detail string

		if resp.Body != nil && resp.Body != http.NoBody {
			msgBytes, err := io.ReadAll(resp.Body)

			goutils.CatchWarnErrorFunc(resp.Body.Close)

			if err != nil {
				detail = err.Error()
			} else {
				detail = string(msgBytes)
			}
		}

		err := goutils.NewRFC9457Error(resp.StatusCode, detail)

		ge.printLog(
			ctx,
			request,
			graphqlPayload,
			resp,
			nil,
			err,
		)

		return resp, err
	}

	if resp.Body == nil || resp.Body == http.NoBody {
		errorDetail := goutils.ErrorDetail{
			Detail: "graphql response must be a valid JSON object",
			Code:   oaschema.ErrCodeGraphQLResponseEmpty,
		}

		ge.printLog(
			ctx,
			request,
			graphqlPayload,
			resp,
			nil,
			&errorDetail,
		)

		respErr := goutils.NewServerError(errorDetail)
		respErr.Detail = "failed to encode graphql response"

		return resp, respErr
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

	variables, err := ge.resolveRequestVariables(requestData, rawRequestData)
	if err != nil {
		ge.printLog(
			ctx,
			request,
			graphqlPayload,
			nil,
			nil,
			err,
		)

		return nil, err
	}

	graphqlPayload.Variables = variables

	graphqlPayload.Extensions, err = ge.resolveRequestExtensions(rawRequestData)
	if err != nil {
		ge.printLog(
			ctx,
			request,
			graphqlPayload,
			nil,
			nil,
			err,
		)

		return nil, err
	}

	req := options.NewRequest(ge.method, ge.url)
	reqHeader := req.Header()

	for key, header := range ge.headers {
		value, err := header.EvaluateString(rawRequestData)
		if err != nil {
			respErr := goutils.NewBadRequestError(goutils.ErrorDetail{
				Detail:  err.Error(),
				Pointer: "/headers/" + key,
			})
			respErr.Detail = "failed to evaluate header"

			ge.printLog(
				ctx,
				request,
				graphqlPayload,
				nil,
				nil,
				err,
			)

			return nil, respErr
		}

		if value != nil && *value != "" {
			reqHeader.Set(key, *value)
		}
	}

	reqHeader.Set(httpheader.Accept, acceptContentTypes)

	if ge.method == http.MethodPost {
		reqHeader.Set(httpheader.ContentType, httpheader.ContentTypeJSON)

		return ge.prepareRequestPOST(ctx, request, req, graphqlPayload)
	}

	return ge.prepareRequestGET(ctx, request, req, graphqlPayload)
}

func (ge *GraphQLHandler) prepareRequestGET(
	ctx context.Context,
	request *proxyhandler.Request,
	req *gohttpc.RequestWithClient,
	graphqlPayload *GraphQLRequestBody,
) (*gohttpc.RequestWithClient, error) {
	reqURL, err := goutils.ParsePathOrHTTPURL(ge.url)
	if err != nil {
		respErr := goutils.NewServerError(goutils.ErrorDetail{
			Detail:  err.Error(),
			Pointer: "/url",
		})
		respErr.Detail = "failed to parse request URL"

		ge.printLog(
			ctx,
			request,
			graphqlPayload,
			nil,
			nil,
			err,
		)

		return nil, respErr
	}

	// encode GraphQL request fields as URL queries for HTTP GET requests
	queryValues := reqURL.Query()
	queryValues.Set("query", ge.query)

	if ge.operationName != "" {
		queryValues.Set("operationName", ge.operationName)
	}

	// Each of the variables and extensions parameters, if used, MUST be encoded as a JSON string.
	if len(graphqlPayload.Variables) > 0 {
		jsonVariables, err := json.Marshal(graphqlPayload.Variables)
		if err != nil {
			respErr := goutils.NewServerError(goutils.ErrorDetail{
				Detail:  err.Error(),
				Pointer: "/variables",
			})
			respErr.Detail = "failed to encode request variables"

			ge.printLog(
				ctx,
				request,
				graphqlPayload,
				nil,
				nil,
				err,
			)

			return nil, respErr
		}

		queryValues.Set("variables", string(jsonVariables))
	}

	if len(graphqlPayload.Extensions) > 0 {
		jsonExtensions, err := json.Marshal(graphqlPayload.Extensions)
		if err != nil {
			respErr := goutils.NewServerError(goutils.ErrorDetail{
				Detail:  err.Error(),
				Pointer: "/extensions",
			})
			respErr.Detail = "failed to encode request extensions"

			ge.printLog(
				ctx,
				request,
				graphqlPayload,
				nil,
				nil,
				err,
			)

			return nil, respErr
		}

		queryValues.Set("extensions", string(jsonExtensions))
	}

	reqURL.RawQuery = queryValues.Encode()

	req.SetURL(reqURL.String())

	return req, nil
}

func (ge *GraphQLHandler) prepareRequestPOST(
	ctx context.Context,
	request *proxyhandler.Request,
	req *gohttpc.RequestWithClient,
	graphqlPayload *GraphQLRequestBody,
) (*gohttpc.RequestWithClient, error) {
	reader := new(bytes.Buffer)

	graphqlPayload.Query = ge.query
	graphqlPayload.OperationName = ge.operationName

	err := json.NewEncoder(reader).Encode(graphqlPayload)
	if err != nil {
		respErr := goutils.NewBadRequestError(goutils.ErrorDetail{
			Detail:  err.Error(),
			Pointer: "/body",
		})
		respErr.Detail = "failed to encode body"

		ge.printLog(
			ctx,
			request,
			graphqlPayload,
			nil,
			nil,
			err,
		)

		return nil, respErr
	}

	req.SetBody(reader)

	return req, nil
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
				respErr := goutils.NewBadRequestError(goutils.ErrorDetail{
					Detail:  err.Error(),
					Pointer: "/variables/" + varDef.Variable,
				})
				respErr.Detail = "failed to select value of variable"

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

				return nil, respErr
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
			respErr := goutils.NewBadRequestError(goutils.ErrorDetail{
				Detail:  err.Error(),
				Pointer: "/extensions/" + key,
			})
			respErr.Detail = "failed to select value of extension"

			return nil, respErr
		}

		results[key] = value
	}

	return results, nil
}
