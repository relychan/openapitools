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

package oasvalidator

import (
	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/relychan/goutils/httperror"
	"github.com/relychan/goutils/httpheader"
	"github.com/relychan/openapitools/oaschema"
)

func ValidateOperation(
	document *highv3.Document,
	operation *highv3.Operation,
	additionalParams []*highv3.Parameter,
) (*oaschema.Operation, []httperror.ValidationError) {
	applyOperationReference(document, operation)

	result := &oaschema.Operation{
		OperationID: operation.OperationId,
		Responses:   operation.Responses,
		Security:    operation.Security,
		Servers:     operation.Servers,
		Extensions:  operation.Extensions,
	}

	var errs []httperror.ValidationError

	if len(operation.Parameters)+len(additionalParams) > 0 {
		result.Parameters, errs = ValidateParameterDefinitions(
			append(operation.Parameters, additionalParams...),
		)
	}

	if operation.RequestBody != nil {
		result.RequestBody = &oaschema.RequestBody{
			Required: operation.RequestBody.Required != nil &&
				*operation.RequestBody.Required,
		}

		requestContentType, requestBodyMediaType := getRequestBodyContentSchema(
			operation,
		)

		if requestContentType != "" {
			contentType, err := ValidateContentType(requestContentType)
			if err != nil {
				errs = append(errs, httperror.ValidationError{
					Detail:  err.Error() + " " + contentType,
					Pointer: "/contentType",
					Code:    ErrCodeOpenAPISchemaError,
				})
			}

			result.RequestContentType = contentType
		}

		if requestBodyMediaType != nil {
			bodySchema, validateErrors := ValidateSchemaProxy(requestBodyMediaType.Schema)
			if len(validateErrors) > 0 {
				errs = append(errs, validateErrors...)
			}

			result.RequestBody.Schema = bodySchema

			itemSchema, validateErrors := ValidateSchemaProxy(requestBodyMediaType.ItemSchema)
			if len(validateErrors) > 0 {
				errs = append(errs, validateErrors...)
			}

			result.RequestBody.ItemSchema = itemSchema
			result.RequestBody.Encoding = requestBodyMediaType.Encoding
			result.RequestBody.ItemEncoding = requestBodyMediaType.ItemEncoding
		}
	}

	if len(errs) > 0 {
		return nil, errs
	}

	return result, nil
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

func getRequestBodyContentSchema(operation *highv3.Operation) (string, *highv3.MediaType) {
	contents := operation.RequestBody.Content

	if operation.RequestBody.Content == nil || contents.Len() == 0 {
		return "", nil
	}

	var (
		defaultContentType   string
		defaultContentSchema *highv3.MediaType
	)

	for content := contents.First(); content != nil; content = content.Next() {
		key := content.Key()

		value := content.Value()
		if value == nil {
			continue
		}

		if defaultContentSchema == nil {
			defaultContentType = key
			defaultContentSchema = value
		}

		if httpheader.IsContentTypeJSON(key) {
			return key, value
		}
	}

	return defaultContentType, defaultContentSchema
}
