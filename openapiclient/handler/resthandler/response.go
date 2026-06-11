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
	"github.com/relychan/goutils/httperror"
	"github.com/relychan/goutils/httpheader"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/oasvalidator"
	"github.com/relychan/openapitools/oasvalidator/contentdecoder"
	"github.com/relychan/openapitools/openapiclient/handler/proxyhandler"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func (re *RESTfulHandler) transformResponse(
	ctx context.Context,
	logger *slog.Logger,
	resp *http.Response,
	contentTypeFrom string,
) (any, error) {
	ctx, span := tracer.Start(ctx, "transform_response", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()

	span.SetAttributes(attribute.String("content_type.original", contentTypeFrom))

	originalBody, err := contentdecoder.Decode(contentTypeFrom, resp.Body)

	goutils.CloseResponse(resp)

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

	errorDetail, ok := errors.AsType[*httperror.ValidationError](err)
	if !ok {
		errorDetail = &httperror.ValidationError{
			Detail: err.Error(),
			Code:   oasvalidator.ErrCodeResponseTransformError,
		}
	}

	respErr := httperror.NewServerError(*errorDetail)
	respErr.Detail = "failed to transform response"

	return respErr
}

func (re *RESTfulHandler) decodeRawResponse(
	ctx context.Context,
	response *http.Response,
	options *proxyhandler.ProxyHandleOptions,
) (any, error) {
	_, span := tracer.Start(ctx, "decode_raw_response", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()

	defer goutils.CloseResponse(response)

	contentType := httpheader.GetHeaderValue(response.Header, httpheader.ContentType)

	decodedBody, err := contentdecoder.Decode(contentType, response.Body)
	if err != nil {
		respErr := httperror.NewServerError(httperror.ValidationError{
			Code:   oasvalidator.ErrCodeResponseDecodeBodyError,
			Detail: err.Error(),
		})

		respErr.Detail = "failed to decode response body"

		span.SetStatus(codes.Error, respErr.Detail)
		span.RecordError(err)

		return nil, respErr
	}

	if options.Settings != nil && options.Settings.Strict {
		validatedErr := re.validateResponse(contentType, decodedBody)
		if validatedErr != nil {
			span.SetStatus(codes.Error, validatedErr.Detail)
			span.RecordError(validatedErr)

			return nil, validatedErr
		}
	}

	span.SetStatus(codes.Ok, "")

	return decodedBody, nil
}

func (re *RESTfulHandler) validateResponse(
	contentType string,
	decodedBody any,
) *httperror.HTTPError {
	mediaType := re.findResponseMediaType(contentType)
	if mediaType != nil && mediaType.Schema != nil {
		errs := oasvalidator.ValidateValue(mediaType.Schema, decodedBody)
		if len(errs) > 0 {
			respErr := httperror.NewServerError(errs...)
			respErr.Detail = "response body is invalid"

			return respErr
		}
	}

	return nil
}

func (re *RESTfulHandler) findResponseMediaType(contentType string) *oaschema.MediaType {
	if len(re.responses) == 0 {
		return nil
	}

	media, ok := re.responses[contentType]
	if ok {
		return media
	}

	for key, media := range re.responses {
		if httpheader.IsContentType(contentType, key) {
			return media
		}
	}

	return nil
}

// Write raw response if the response schema does not exist.
func (re *RESTfulHandler) writeRawResponse(
	ctx context.Context,
	response *http.Response,
	writer http.ResponseWriter,
	options *proxyhandler.ProxyHandleOptions,
) error {
	_, span := tracer.Start(ctx, "write_raw_response", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()

	options.ForwardResponseHeaders(writer, response)

	if response.Body == nil || response.Body == http.NoBody {
		writer.WriteHeader(response.StatusCode)
		span.SetStatus(codes.Ok, "empty response body")

		return nil
	}

	var (
		err         error
		contentType = httpheader.GetHeaderValue(response.Header, httpheader.ContentType)
		strict      = options.Settings != nil && options.Settings.Strict
	)

	switch {
	case contentType == "":
		err = streamResponseDirectly(response, writer)
	case httpheader.IsContentTypeJSON(contentType):
		if strict {
			err = re.streamValidatedJSONResponse(contentType, response, writer)
		} else {
			err = streamResponseDirectly(response, writer)
		}
	case httpheader.IsContentTypeXML(contentType):
		err = re.decodeAndStreamJSON(
			contentType,
			response,
			writer,
			options,
			contentdecoder.DecodeXML,
		)
	default:
		err = streamResponseDirectly(response, writer)
	}

	if err != nil {
		respErr := httperror.NewServerError(httperror.ValidationError{
			Code:   oasvalidator.ErrCodeWriteResponseError,
			Detail: "failed to write response body: " + err.Error(),
		})

		span.SetStatus(codes.Error, "failed to write response body")
		span.RecordError(err)

		return respErr
	}

	span.SetStatus(codes.Ok, "wrote response successfully")

	return nil
}

// Validate the JSON response before streaming to the client.
func (re *RESTfulHandler) streamValidatedJSONResponse(
	contentType string,
	response *http.Response,
	writer http.ResponseWriter,
) error {
	mediaType := re.findResponseMediaType(contentType)
	if mediaType == nil {
		rawBytes, err := io.ReadAll(response.Body)

		goutils.CloseResponse(response)

		if err != nil {
			return err
		}

		if !json.Valid(rawBytes) {
			return goutils.ErrMalformedJSON
		}

		writer.Header()[httpheader.ContentType] = []string{httpheader.ContentTypeJSON}
		writer.WriteHeader(response.StatusCode)

		_, err = writer.Write(rawBytes)

		return err
	}

	var decodedBody any

	err := json.NewDecoder(response.Body).Decode(&decodedBody)

	goutils.CloseResponse(response)

	if err != nil {
		return err
	}

	validatedErr := re.validateResponse(contentType, decodedBody)
	if validatedErr != nil {
		return validatedErr
	}

	writer.Header()[httpheader.ContentType] = []string{httpheader.ContentTypeJSON}
	writer.WriteHeader(response.StatusCode)

	return json.NewEncoder(writer).Encode(decodedBody)
}

func (re *RESTfulHandler) decodeAndStreamJSON(
	contentType string,
	response *http.Response,
	writer http.ResponseWriter,
	options *proxyhandler.ProxyHandleOptions,
	decode func(io.Reader) (any, error),
) error {
	decoded, decodeErr := decode(response.Body)
	goutils.CloseResponse(response)

	if decodeErr != nil {
		return decodeErr
	}

	if options.Settings != nil && options.Settings.Strict {
		validatedErr := re.validateResponse(contentType, decoded)
		if validatedErr != nil {
			return validatedErr
		}
	}

	writer.Header()[httpheader.ContentType] = []string{httpheader.ContentTypeJSON}
	writer.WriteHeader(response.StatusCode)

	return json.NewEncoder(writer).Encode(decoded)
}

// Stream response directly without validation.
func streamResponseDirectly(
	response *http.Response,
	writer http.ResponseWriter,
) error {
	writer.WriteHeader(response.StatusCode)

	_, err := io.Copy(writer, response.Body)

	goutils.CatchWarnErrorFunc(response.Body.Close)

	return err
}
