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
	"errors"
	"io"
	"net/http"

	"github.com/buger/jsonparser"
	"github.com/relychan/goutils"
	"github.com/relychan/goutils/httpheader"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/openapiclient/handler/resthandler/contenttype"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func (ge *GraphQLHandler) handleTransformResponse(
	ctx context.Context,
	resp *http.Response,
) (any, error) {
	_, span := tracer.Start(
		ctx,
		"handle_transform_response",
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	var responseBody any

	if (ge.customResponse.HTTPErrorCode != nil && *ge.customResponse.HTTPErrorCode >= 200) ||
		len(ge.customResponse.HTTPErrors) > 0 {
		status, rawBody, gqlError := ge.evaluateGraphQLError(resp)
		if gqlError != nil {
			span.SetStatus(codes.Error, "failed to evaluate graphql error")
			span.RecordError(gqlError)

			return nil, gqlError
		}

		if status >= http.StatusBadRequest {
			resp.StatusCode = status

			var extensions map[string]any

			err := json.Unmarshal(rawBody, &extensions)
			if err != nil {
				span.SetStatus(codes.Error, "failed to decode graphql error")
				span.RecordError(err)

				respErr := goutils.NewServerError()
				respErr.Detail = err.Error()

				return nil, respErr
			}

			respError := goutils.NewRFC9457ErrorWithExtensions(
				goutils.RFC9457Error{
					Type:   "about:blank",
					Status: status,
					Title:  http.StatusText(status),
					Detail: "Received errors from the remote server",
				},
				extensions,
			)

			return nil, respError
		}

		err := json.Unmarshal(rawBody, &responseBody)
		if err != nil {
			span.SetStatus(codes.Error, "failed to decode response body")
			span.RecordError(err)

			return nil, newGraphQLResponseEncodeError(
				oaschema.ErrCodeResponseTransformError,
				err,
			)
		}
	} else {
		err := json.NewDecoder(resp.Body).Decode(&responseBody)

		goutils.CatchWarnErrorFunc(resp.Body.Close)

		if err != nil {
			span.SetStatus(codes.Error, "failed to decode response body")
			span.RecordError(err)

			return nil, newGraphQLResponseEncodeError(
				oaschema.ErrCodeResponseTransformError,
				err,
			)
		}
	}

	if ge.customResponse.Body == nil || ge.customResponse.Body.IsZero() {
		span.SetStatus(codes.Ok, "")

		return responseBody, nil
	}

	transformedBody, err := ge.customResponse.Body.Transform(responseBody)
	if err != nil {
		span.SetStatus(codes.Error, "failed to transform response body")
		span.RecordError(err)

		return responseBody, newGraphQLResponseEncodeError(
			oaschema.ErrCodeResponseTransformError,
			err,
		)
	}

	span.SetStatus(codes.Ok, "")

	return transformedBody, nil
}

func (ge *GraphQLHandler) writeTransformResponse(
	ctx context.Context,
	resp *http.Response,
	writer http.ResponseWriter,
) (any, error) {
	_, span := tracer.Start(
		ctx,
		"write_transform_response",
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	var responseBody any

	status, rawBody, gqlError := ge.evaluateGraphQLError(resp)
	if gqlError != nil {
		message := "failed to evaluate graphql error"

		span.SetStatus(codes.Error, message)
		span.RecordError(gqlError)

		err := goutils.NewRFC9457Error(status, message)
		err.Errors = []goutils.ErrorDetail{*gqlError}

		return nil, err
	}

	if status >= http.StatusBadRequest {
		resp.StatusCode = status

		writer.Header().Set(httpheader.ContentType, httpheader.ContentTypeJSON)
		writer.WriteHeader(status)

		_, err := writer.Write(rawBody)
		if err != nil {
			span.SetStatus(codes.Error, "failed to write graphql error")
			span.RecordError(err)

			return nil, newGraphQLResponseEncodeError(
				oaschema.ErrCodeResponseTransformError,
				err,
			)
		}

		return nil, nil
	}

	if ge.customResponse.Body == nil || ge.customResponse.Body.IsZero() {
		_, err := writer.Write(rawBody)
		if err != nil {
			span.SetStatus(codes.Error, "failed to write graphql response")
			span.RecordError(gqlError)

			return nil, newGraphQLResponseEncodeError(
				oaschema.ErrCodeResponseTransformError,
				err,
			)
		}

		span.SetStatus(codes.Ok, "")

		return nil, nil
	}

	err := json.Unmarshal(rawBody, &responseBody)
	if err != nil {
		span.SetStatus(codes.Error, "failed to decode response body")
		span.RecordError(err)

		return nil, newGraphQLResponseEncodeError(
			oaschema.ErrCodeResponseTransformError,
			err,
		)
	}

	transformedBody, err := ge.customResponse.Body.Transform(responseBody)
	if err != nil {
		span.SetStatus(codes.Error, "failed to transform response body")
		span.RecordError(err)

		return nil, newGraphQLResponseEncodeError(
			oaschema.ErrCodeResponseTransformError,
			err,
		)
	}

	writer.Header().Set(httpheader.ContentType, ge.responseContentType)

	_, err = contenttype.Write(writer, ge.responseContentType, transformedBody)
	if err != nil {
		span.SetStatus(codes.Error, "failed to write response body")
		span.RecordError(err)

		respError := newGraphQLResponseEncodeError(oaschema.ErrCodeWriteResponseError, err)

		return transformedBody, respError
	}

	span.SetStatus(codes.Ok, "")

	return transformedBody, nil
}

func (ge *GraphQLHandler) evaluateGraphQLError(
	resp *http.Response,
) (int, []byte, *goutils.ErrorDetail) {
	rawBytes, err := io.ReadAll(resp.Body)

	goutils.CatchWarnErrorFunc(resp.Body.Close)

	if err != nil {
		respErr := &goutils.ErrorDetail{
			Detail: err.Error(),
			Code:   oaschema.ErrCodeRemoteServerError,
		}

		return http.StatusInternalServerError, nil, respErr
	}

	rawErrors, fieldType, _, err := jsonparser.Get(rawBytes, "errors")
	if err != nil {
		if err == jsonparser.KeyPathNotFoundError || //nolint:err113,errorlint
			errors.Is(err, jsonparser.KeyPathNotFoundError) {
			return http.StatusOK, rawBytes, nil
		}

		respErr := &goutils.ErrorDetail{
			Detail: err.Error(),
			Code:   oaschema.ErrCodeRemoteServerError,
		}

		return http.StatusInternalServerError, nil, respErr
	}

	if fieldType == jsonparser.Null || bytes.Equal(rawErrors, []byte("[]")) {
		return http.StatusOK, rawBytes, nil
	}

	if fieldType != jsonparser.Array {
		err := &goutils.ErrorDetail{
			Detail: "Invalid errors in GraphQL response. Expected an array, got: " + fieldType.String(),
			Code:   oaschema.ErrCodeRemoteServerError,
		}

		return http.StatusInternalServerError, nil, err
	}

	statusCode := http.StatusOK

	if ge.customResponse.HTTPErrorCode != nil && *ge.customResponse.HTTPErrorCode >= 200 {
		statusCode = *ge.customResponse.HTTPErrorCode
	}

	if len(ge.customResponse.HTTPErrors) == 0 {
		return statusCode, rawBytes, nil
	}

	var gqlErrors any

	err = json.Unmarshal(rawErrors, &gqlErrors)
	if err != nil {
		respErr := &goutils.ErrorDetail{
			Detail: err.Error(),
			Code:   oaschema.ErrCodeRemoteServerError,
		}

		return http.StatusInternalServerError, nil, respErr
	}

	for status, expr := range ge.customResponse.HTTPErrors {
		result, err := expr.Search(gqlErrors)
		if err != nil {
			continue
		}

		if isEvaluatedError(result) {
			statusCode = status

			break
		}
	}

	return statusCode, rawBytes, nil
}
