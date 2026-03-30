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
	"fmt"
	"net/http"

	"github.com/relychan/gohttpc"
	"github.com/relychan/goutils"
	"github.com/relychan/goutils/httpheader"
	"github.com/relychan/openapitools/openapiclient/handler/proxyhandler"
)

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
				nil,
				err,
			)

			return nil, respErr
		}

		if value != nil && *value != "" {
			reqHeader.Set(key, *value)
		}
	}

	reqHeader.Set(httpheader.ContentType, httpheader.ContentTypeJSON)

	reader := new(bytes.Buffer)

	err = json.NewEncoder(reader).Encode(graphqlPayload)
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
