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

package internal

import (
	"net/http"
	"net/url"
	"slices"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/openapiclient/handler"
	"github.com/relychan/openapitools/openapiclient/handler/proxyhandler"
)

func createMethods( //nolint:cyclop,funlen
	document *highv3.Document,
	pattern string,
	operations *highv3.PathItem,
	paramKeys []string,
	options *proxyhandler.InsertRouteOptions,
) (map[string]MethodHandler, error) {
	var (
		err      error
		params   = extractParametersFromOperationV3(operations, paramKeys)
		handlers = map[string]MethodHandler{}
	)

	if operations.Get != nil {
		method := http.MethodGet

		handlers[method], err = createMethod(
			document,
			pattern,
			method,
			operations.Get,
			params,
			options,
		)
		if err != nil {
			return nil, newInvalidOperationMetadataError(method, pattern, err)
		}
	}

	if operations.Post != nil {
		method := http.MethodPost

		handlers[method], err = createMethod(
			document,
			pattern,
			method,
			operations.Post,
			params,
			options,
		)
		if err != nil {
			return nil, newInvalidOperationMetadataError(method, pattern, err)
		}
	}

	if operations.Put != nil {
		method := http.MethodPut

		handlers[method], err = createMethod(
			document,
			pattern,
			method,
			operations.Put,
			params,
			options,
		)
		if err != nil {
			return nil, newInvalidOperationMetadataError(method, pattern, err)
		}
	}

	if operations.Patch != nil {
		method := http.MethodPatch

		handlers[method], err = createMethod(
			document,
			pattern,
			method,
			operations.Patch,
			params,
			options,
		)
		if err != nil {
			return nil, newInvalidOperationMetadataError(method, pattern, err)
		}
	}

	if operations.Delete != nil {
		method := http.MethodDelete

		handlers[method], err = createMethod(
			document,
			pattern,
			method,
			operations.Delete,
			params,
			options,
		)
		if err != nil {
			return nil, newInvalidOperationMetadataError(method, pattern, err)
		}
	}

	if operations.Head != nil {
		method := http.MethodHead

		handlers[method], err = createMethod(
			document,
			pattern,
			method,
			operations.Head,
			params,
			options,
		)
		if err != nil {
			return nil, newInvalidOperationMetadataError(method, pattern, err)
		}
	}

	if operations.Options != nil {
		method := http.MethodOptions

		handlers[method], err = createMethod(
			document,
			pattern,
			method,
			operations.Options,
			params,
			options,
		)
		if err != nil {
			return nil, newInvalidOperationMetadataError(method, pattern, err)
		}
	}

	if operations.Query != nil {
		method := "QUERY"

		handlers[method], err = createMethod(
			document,
			pattern,
			method,
			operations.Query,
			params,
			options,
		)
		if err != nil {
			return nil, newInvalidOperationMetadataError(method, pattern, err)
		}
	}

	if operations.Trace != nil {
		method := http.MethodTrace

		handlers[method], err = createMethod(
			document,
			pattern,
			method,
			operations.Trace,
			params,
			options,
		)
		if err != nil {
			return nil, newInvalidOperationMetadataError(method, pattern, err)
		}
	}

	if operations.AdditionalOperations != nil {
		for iter := operations.AdditionalOperations.Oldest(); iter != nil; iter = iter.Next() {
			method := iter.Key
			op := iter.Value

			if op == nil {
				continue
			}

			handlers[method], err = createMethod(
				document,
				pattern,
				method,
				op,
				params,
				options,
			)
			if err != nil {
				return nil, newInvalidOperationMetadataError(method, pattern, err)
			}
		}
	}

	return handlers, nil
}

func createMethod(
	document *highv3.Document,
	pattern string,
	method string,
	operation *highv3.Operation,
	params []*highv3.Parameter,
	options *proxyhandler.InsertRouteOptions,
) (MethodHandler, error) {
	applyOperationReference(document, operation)

	h, err := handler.NewProxyHandler(operation, &proxyhandler.NewProxyHandlerOptions{
		Method:     method,
		Parameters: params,
		GetEnv:     options.GetEnv,
	})
	if err != nil {
		return MethodHandler{}, newInvalidOperationMetadataError(method, pattern, err)
	}

	return MethodHandler{
		Handler:   h,
		Operation: operation,
	}, nil
}

func applyOperationReference(document *highv3.Document, operation *highv3.Operation) {
	if document == nil {
		return
	}

	applyRequestBodyReference(document, operation)

	if document.Components != nil && document.Components.Responses != nil &&
		document.Components.Responses.Len() > 0 && operation.Responses != nil {
		if operation.Responses.Default != nil {
			operation.Responses.Default = applyResponseReference(
				document,
				operation.Responses.Default,
			)
		}

		if operation.Responses.Codes != nil {
			for iter := operation.Responses.Codes.First(); iter != nil; iter = iter.Next() {
				value := iter.Value()
				if value != nil {
					operation.Responses.Codes.Set(
						iter.Key(),
						applyResponseReference(document, value),
					)
				}
			}
		}
	}
}

func applyRequestBodyReference(document *highv3.Document, operation *highv3.Operation) {
	if operation.RequestBody == nil || operation.RequestBody.Reference == "" ||
		document.Components == nil || document.Components.RequestBodies == nil ||
		document.Components.RequestBodies.Len() == 0 {
		return
	}

	refBody, present := document.Components.RequestBodies.Get(operation.RequestBody.Reference)
	if !present || refBody == nil {
		return
	}

	if operation.RequestBody.Description == "" &&
		(operation.RequestBody.Content == nil || operation.RequestBody.Content.Len() == 0) &&
		(operation.RequestBody.Extensions == nil || operation.RequestBody.Extensions.Len() == 0) &&
		operation.RequestBody.Required == nil {
		operation.RequestBody = refBody

		return
	}

	operation.RequestBody.Reference = ""

	if refBody.Description != "" && operation.RequestBody.Description == "" {
		operation.RequestBody.Description = refBody.Description
	}

	if refBody.Required != nil && operation.RequestBody.Required == nil {
		operation.RequestBody.Required = refBody.Required
	}

	operation.RequestBody.Content = oaschema.MergeOrderedMap(
		operation.RequestBody.Content,
		refBody.Content,
	)

	operation.RequestBody.Extensions = oaschema.MergeOrderedMap(
		operation.RequestBody.Extensions,
		refBody.Extensions,
	)
}

func applyResponseReference(document *highv3.Document, response *highv3.Response) *highv3.Response {
	if response.Reference == "" {
		return response
	}

	refResponse, present := document.Components.Responses.Get(response.Reference)
	if !present || refResponse == nil {
		return response
	}

	if response.Description == "" && response.Summary == "" &&
		(response.Content == nil || response.Content.Len() == 0) &&
		(response.Extensions == nil || response.Extensions.Len() == 0) &&
		(response.Headers == nil || response.Headers.Len() == 0) &&
		(response.Links == nil || response.Links.Len() == 0) {
		return refResponse
	}

	response.Reference = ""

	if refResponse.Description != "" && response.Description == "" {
		response.Description = refResponse.Description
	}

	if refResponse.Summary != "" && response.Summary == "" {
		response.Summary = refResponse.Summary
	}

	response.Headers = oaschema.MergeOrderedMap(
		response.Headers,
		refResponse.Headers,
	)

	response.Links = oaschema.MergeOrderedMap(
		response.Links,
		refResponse.Links,
	)

	response.Content = oaschema.MergeOrderedMap(
		response.Content,
		refResponse.Content,
	)

	response.Extensions = oaschema.MergeOrderedMap(
		response.Extensions,
		refResponse.Extensions,
	)

	return response
}

func extractParametersFromOperationV3(
	operations *highv3.PathItem,
	paramKeys []string,
) []*highv3.Parameter {
	params := operations.Parameters
	params = oaschema.ExtractCommonParametersOfOperation(params, operations.Get)
	params = oaschema.ExtractCommonParametersOfOperation(params, operations.Post)
	params = oaschema.ExtractCommonParametersOfOperation(params, operations.Put)
	params = oaschema.ExtractCommonParametersOfOperation(params, operations.Patch)
	params = oaschema.ExtractCommonParametersOfOperation(params, operations.Delete)
	params = oaschema.ExtractCommonParametersOfOperation(params, operations.Head)
	params = oaschema.ExtractCommonParametersOfOperation(params, operations.Options)
	params = oaschema.ExtractCommonParametersOfOperation(params, operations.Query)
	params = oaschema.ExtractCommonParametersOfOperation(params, operations.Trace)

	if operations.AdditionalOperations != nil {
		for iter := operations.AdditionalOperations.Oldest(); iter != nil; iter = iter.Next() {
			if iter.Value == nil {
				continue
			}

			params = oaschema.ExtractCommonParametersOfOperation(params, iter.Value)
		}
	}

	// validates and add unknown parameters from the request pattern
	for _, key := range paramKeys {
		if slices.ContainsFunc(params, func(param *highv3.Parameter) bool {
			return param.In == oaschema.InPath.String() && param.Name == key
		}) {
			continue
		}

		params = append(params, &highv3.Parameter{
			Name:     key,
			In:       oaschema.InPath.String(),
			Required: new(true),
			Schema: base.CreateSchemaProxy(&base.Schema{
				Type: []string{"string"},
			}),
		})
	}

	return params
}

// cut the first path of the url and parse the query param if exists. Ignore fragments.
func cutURLPath(search string) (string, string, url.Values, error) { //nolint:revive
	if search == "" {
		return search, "", nil, nil
	}

	var endPathIndex int

	maxLength := len(search)

L:
	for ; endPathIndex < maxLength; endPathIndex++ {
		c := search[endPathIndex]

		switch c {
		case '/', '#':
			break L
		case '?':
			if endPathIndex == maxLength-1 {
				return search[:endPathIndex], "", nil, nil
			}

			queryParams, err := url.ParseQuery(search[endPathIndex+1:])
			if err != nil {
				return "", "", nil, err
			}

			return search[:endPathIndex], "", queryParams, nil
		default:
		}
	}

	if endPathIndex == maxLength {
		return search, "", nil, nil
	}

	return search[0:endPathIndex], search[endPathIndex+1:], nil, nil
}
