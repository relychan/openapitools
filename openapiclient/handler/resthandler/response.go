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
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/relychan/goutils"
	"github.com/relychan/goutils/httperror"
	"github.com/relychan/goutils/httpheader"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/oasvalidator"
	"github.com/relychan/openapitools/oasvalidator/contentdecoder"
	"github.com/relychan/openapitools/oasvalidator/contentencoder"
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

	var (
		originalBody any
		err          error
	)

	if !isEmptyResponseBody(resp) {
		originalBody, err = contentdecoder.Decode(contentTypeFrom, resp.Body)
	}

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
	if !isDebug && err == nil {
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

	if isEmptyResponseBody(response) {
		span.SetStatus(codes.Ok, "empty response body")

		return nil, nil
	}

	mediaType, shouldValidate, respErr := re.resolveStrictResponseMediaType(
		contentType,
		response.StatusCode,
		options,
	)
	if respErr != nil {
		span.SetStatus(codes.Error, respErr.Detail)
		span.RecordError(respErr)

		return nil, respErr
	}

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

	if shouldValidate {
		validatedErr := validateResponse(mediaType, decodedBody)
		if validatedErr != nil {
			span.SetStatus(codes.Error, validatedErr.Detail)
			span.RecordError(validatedErr)

			return nil, validatedErr
		}
	}

	span.SetStatus(codes.Ok, "")

	return decodedBody, nil
}

// resolveStrictResponseMediaType finds the response media type to validate the response body against in strict mode.
// Returns an error if the operation declares response content but the upstream content type matches none of them.
// Only successful (2xx) responses are validated since the operation only keeps the success response.
func (re *RESTfulHandler) resolveStrictResponseMediaType(
	contentType string,
	statusCode int,
	options *proxyhandler.ProxyHandleOptions,
) (*oaschema.MediaType, bool, *httperror.HTTPError) {
	if options == nil || options.Settings == nil || !options.Settings.Strict ||
		len(re.responses) == 0 || statusCode < 200 || statusCode >= 300 {
		return nil, false, nil
	}

	mediaType := re.findResponseMediaType(contentType)
	if mediaType != nil {
		return mediaType, true, nil
	}

	if contentType == "" {
		// The operation accepts any content type.
		anyMedia, ok := re.responses["*/*"]
		if ok {
			return anyMedia, false, nil
		}
	}

	respErr := httperror.NewServerError(httperror.ValidationError{
		Code:      oasvalidator.ErrCodeRemoteServerError,
		Detail:    "unexpected response content type: " + contentType,
		Parameter: httpheader.ContentType,
		Location:  oaschema.HeaderKey,
	})
	respErr.Detail = "response content type does not match the operation"

	return nil, false, respErr
}

func validateResponse(
	mediaType *oaschema.MediaType,
	decodedBody any,
) *httperror.HTTPError {
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
	if len(re.responses) == 0 || contentType == "" {
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

	// Structured syntax suffix types, e.g. application/problem+json, match the application/json response.
	if httpheader.IsContentTypeJSON(contentType) {
		media, ok := re.responses[httpheader.ContentTypeJSON]
		if ok {
			return media
		}
	}

	// Fall back to media type ranges, e.g. image/* or */*. Prefer the more specific range.
	var anyMedia *oaschema.MediaType

	for key, media := range re.responses {
		mediaRange := httpheader.ExtractBaseMediaType(key)

		if mediaRange == "*/*" {
			anyMedia = media

			continue
		}

		prefix, isRange := strings.CutSuffix(mediaRange, "*")
		if isRange && strings.HasSuffix(prefix, "/") &&
			goutils.HasStringPrefixFold(contentType, prefix) {
			return media
		}
	}

	return anyMedia
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

	if isEmptyResponseBody(response) {
		goutils.CloseResponse(response)
		writer.WriteHeader(response.StatusCode)
		span.SetStatus(codes.Ok, "empty response body")

		return nil
	}

	contentType := httpheader.GetHeaderValue(response.Header, httpheader.ContentType)

	mediaType, shouldValidate, respErr := re.resolveStrictResponseMediaType(
		contentType,
		response.StatusCode,
		options,
	)
	if respErr != nil {
		goutils.CloseResponse(response)
		span.SetStatus(codes.Error, respErr.Detail)
		span.RecordError(respErr)

		return respErr
	}

	var err error

	switch {
	case contentType == "":
		err = streamResponseDirectly(contentType, response, writer)
	case httpheader.IsContentTypeJSON(contentType):
		if options.Settings != nil && options.Settings.Strict {
			err = streamValidatedJSONResponse(mediaType, response, writer)
		} else {
			err = streamResponseDirectly(contentType, response, writer)
		}
	case httpheader.IsContentTypeXML(contentType):
		err = decodeAndStreamJSON(
			mediaType,
			shouldValidate,
			response,
			writer,
			contentdecoder.DecodeXML,
		)
	default:
		err = streamResponseDirectly(contentType, response, writer)
	}

	if err != nil {
		span.SetStatus(codes.Error, "failed to write response body")
		span.RecordError(err)

		httpErr, ok := errors.AsType[*httperror.HTTPError](err)
		if ok {
			return httpErr
		}

		return httperror.NewServerError(httperror.ValidationError{
			Code:   oasvalidator.ErrCodeWriteResponseError,
			Detail: "failed to write response body: " + err.Error(),
		})
	}

	span.SetStatus(codes.Ok, "wrote response successfully")

	return nil
}

// Validate the JSON response before streaming to the client.
// The body is fully validated before the response header is written.
func streamValidatedJSONResponse(
	mediaType *oaschema.MediaType,
	response *http.Response,
	writer http.ResponseWriter,
) error {
	rawBytes, err := io.ReadAll(response.Body)

	goutils.CloseResponse(response)

	if err != nil {
		return err
	}

	if mediaType == nil || mediaType.Schema == nil {
		if !json.Valid(rawBytes) {
			return goutils.ErrMalformedJSON
		}

		return writeResponseBytes(writer, response.StatusCode, httpheader.ContentTypeJSON, rawBytes)
	}

	decodedBody, err := contentdecoder.UnmarshalJSON(rawBytes)
	if err != nil {
		return err
	}

	validatedErr := validateResponse(mediaType, decodedBody)
	if validatedErr != nil {
		return validatedErr
	}

	return writeResponseBytes(writer, response.StatusCode, httpheader.ContentTypeJSON, rawBytes)
}

func decodeAndStreamJSON(
	mediaType *oaschema.MediaType,
	shouldValidate bool,
	response *http.Response,
	writer http.ResponseWriter,
	decode func(io.Reader) (any, error),
) error {
	decoded, decodeErr := decode(response.Body)
	goutils.CloseResponse(response)

	if decodeErr != nil {
		return decodeErr
	}

	if shouldValidate {
		validatedErr := validateResponse(mediaType, decoded)
		if validatedErr != nil {
			return validatedErr
		}
	}

	return writeEncodedResponse(writer, response.StatusCode, httpheader.ContentTypeJSON, decoded)
}

// writeEncodedResponse encodes the body before writing the response header,
// so encoding errors can still be returned as a proper error response.
func writeEncodedResponse(
	writer http.ResponseWriter,
	statusCode int,
	contentType string,
	body any,
) error {
	rawBytes, encodedContentType, err := contentencoder.Encode(contentType, body, nil, nil)
	if err != nil {
		errDetail, ok := errors.AsType[*httperror.ValidationError](err)
		if !ok {
			errDetail = &httperror.ValidationError{
				Detail: err.Error(),
				Code:   oasvalidator.ErrCodeResponseEncodeBodyError,
			}
		}

		respErr := httperror.NewServerError(*errDetail)
		respErr.Detail = "failed to encode response body"

		return respErr
	}

	return writeResponseBytes(writer, statusCode, encodedContentType, rawBytes)
}

// writeResponseBytes writes the response header and body.
// Errors returned after the header is written can't be sent to the client.
func writeResponseBytes(
	writer http.ResponseWriter,
	statusCode int,
	contentType string,
	body []byte,
) error {
	writer.Header()[httpheader.ContentType] = []string{contentType}
	writer.WriteHeader(statusCode)

	_, err := writer.Write(body)

	return err
}

// isEmptyResponseBody checks if the response body is empty.
// If the body is not empty, it is wrapped by a buffered reader to peek the first byte.
func isEmptyResponseBody(response *http.Response) bool {
	if response.Body == nil || response.Body == http.NoBody ||
		response.StatusCode == http.StatusNoContent ||
		response.StatusCode == http.StatusNotModified {
		return true
	}

	reader := bufio.NewReader(response.Body)

	_, err := reader.Peek(1)
	if errors.Is(err, io.EOF) {
		return true
	}

	response.Body = &bufferedReadCloser{
		Reader: reader,
		Closer: response.Body,
	}

	return false
}

type bufferedReadCloser struct {
	io.Reader
	io.Closer
}

// Stream response directly without validation.
func streamResponseDirectly(
	contentType string,
	response *http.Response,
	writer http.ResponseWriter,
) error {
	if contentType != "" {
		writer.Header()[httpheader.ContentType] = []string{contentType}
	}

	writer.WriteHeader(response.StatusCode)

	_, err := io.Copy(writer, response.Body)

	goutils.CatchWarnErrorFunc(response.Body.Close)

	return err
}
