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

package resthandler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/relychan/goutils"
	"github.com/relychan/goutils/httpheader"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/openapiclient/handler/resthandler/contenttype"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func (re *RESTfulHandler) transformResponse(
	ctx context.Context,
	logger *slog.Logger,
	resp *http.Response,
	writer http.ResponseWriter,
) (any, error) {
	ctx, span := tracer.Start(ctx, "transform_response", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()

	contentTypeFrom := resp.Header.Get(httpheader.ContentType)

	span.SetAttributes(attribute.String("content_type.original", contentTypeFrom))

	originalBody, err := contenttype.Decode(contentTypeFrom, resp.Body)
	if err != nil {
		return nil, re.postTransformedResponse(
			ctx,
			span,
			logger,
			contentTypeFrom,
			nil,
			nil,
			err,
		)
	}

	transformedBody, err := re.customResponse.Body.Transform(originalBody)
	if writer == nil {
		return transformedBody, re.postTransformedResponse(
			ctx,
			span,
			logger,
			contentTypeFrom,
			originalBody,
			transformedBody,
			err,
		)
	}

	// encode the body back to the response stream.
	contentTypeTo := re.responseContentType
	if contentTypeTo == "" {
		contentTypeTo = contentTypeFrom
	}

	_, err = contenttype.Write(writer, contentTypeTo, transformedBody)

	return transformedBody, re.postTransformedResponse(
		ctx,
		span,
		logger,
		contentTypeFrom,
		originalBody,
		transformedBody,
		err,
	)
}

func (*RESTfulHandler) postTransformedResponse(
	ctx context.Context,
	span trace.Span,
	logger *slog.Logger,
	originalContentType string,
	originalBody,
	transformedBody any,
	err error,
) error {
	isDebug := logger.Enabled(ctx, slog.LevelDebug)
	if isDebug && err == nil {
		span.SetStatus(codes.Ok, "")

		return nil
	}

	logAttrs := make([]slog.Attr, 0, 3)
	logAttrs = append(
		logAttrs,
		slog.String("original_content_type", originalContentType),
	)

	if originalBody != nil {
		logAttrs = append(
			logAttrs,
			slog.Any("original_body", originalBody),
		)

		encodedBody, err := json.Marshal(originalBody)
		if err == nil {
			span.SetAttributes(attribute.String("body.original", string(encodedBody)))
		}
	}

	if transformedBody != nil {
		logAttrs = append(logAttrs, slog.Any("body", transformedBody))

		encodedBody, err := json.Marshal(transformedBody)
		if err == nil {
			span.SetAttributes(attribute.String("body", string(encodedBody)))
		}
	}

	if err == nil {
		logger.LogAttrs(ctx, slog.LevelDebug, "transformed successfully", logAttrs...)
		span.SetStatus(codes.Ok, "")

		return nil
	}

	span.SetStatus(codes.Error, err.Error())
	span.RecordError(err)

	logger.LogAttrs(ctx, slog.LevelError, err.Error(), logAttrs...)

	errorDetail, ok := errors.AsType[*goutils.ErrorDetail](err)
	if !ok {
		errorDetail = &goutils.ErrorDetail{
			Detail: err.Error(),
			Code:   oaschema.ErrCodeResponseTransformError,
		}
	}

	respErr := goutils.NewServerError(*errorDetail)
	respErr.Detail = "failed to transform response"

	return respErr
}

func (*RESTfulHandler) resolveRawResponse(
	ctx context.Context,
	response *http.Response,
	writer http.ResponseWriter,
) (any, error) {
	_, span := tracer.Start(ctx, "write_response", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()

	if response.Body == nil {
		span.SetStatus(codes.Ok, "empty response body")

		return nil, nil
	}

	defer goutils.CatchWarnErrorFunc(response.Body.Close)

	if writer != nil {
		writer.WriteHeader(response.StatusCode)

		_, err := io.Copy(writer, response.Body)
		if err != nil {
			respErr := goutils.NewServerError(goutils.ErrorDetail{
				Code:   oaschema.ErrCodeResponseDecodeBodyError,
				Detail: err.Error(),
			})

			respErr.Detail = "failed to write response body"

			span.SetStatus(codes.Error, respErr.Detail)
			span.RecordError(err)

			return nil, err
		}

		span.SetStatus(codes.Ok, "streamed response successfully")

		return nil, nil
	}

	decodedBody, err := contenttype.Decode(
		response.Header.Get(httpheader.ContentType),
		response.Body,
	)
	if err != nil {
		respErr := goutils.NewServerError(goutils.ErrorDetail{
			Code:   oaschema.ErrCodeResponseDecodeBodyError,
			Detail: err.Error(),
		})

		respErr.Detail = "failed to decode response body"

		span.SetStatus(codes.Error, respErr.Detail)
		span.RecordError(err)

		return nil, err
	}

	return decodedBody, nil
}
