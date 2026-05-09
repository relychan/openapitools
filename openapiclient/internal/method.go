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

	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/relychan/goutils/httperror"
	"github.com/relychan/openapitools/oasvalidator"
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
	op, errs := oasvalidator.ValidateOperation(document, operation, params)
	if len(errs) > 0 {
		err := httperror.NewValidationError(errs...)
		err.Instance = pattern

		return MethodHandler{}, err
	}

	h, err := handler.NewProxyHandler(op, &proxyhandler.NewProxyHandlerOptions{
		Method: method,
		GetEnv: options.GetEnv,
	})
	if err != nil {
		return MethodHandler{}, newInvalidOperationMetadataError(method, pattern, err)
	}

	return MethodHandler{
		Handler:   h,
		Operation: op,
	}, nil
}
