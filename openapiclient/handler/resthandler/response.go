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
	resp *http.Response,
	isDebug bool,
) (any, []slog.Attr, error) {
	_, span := tracer.Start(ctx, "transform_response")
	defer span.End()

	contentType := resp.Header.Get(httpheader.ContentType)

	span.SetAttributes(attribute.String("content_type", contentType))

	responseBody, err := contenttype.Decode(contentType, resp.Body)
	if err != nil {
		return nil, nil, recordResponseTransformError(span, err)
	}

	respLogAttrs := make([]slog.Attr, 0, 3)

	if isDebug {
		respLogAttrs = append(
			respLogAttrs,
			slog.Any("original_body", responseBody),
			slog.String("original_content_type", contentType),
		)

		encodedBody, err := json.Marshal(responseBody)
		if err == nil {
			span.SetAttributes(attribute.String("original_body", string(encodedBody)))
		}
	}

	transformedBody, err := re.customResponse.Body.Transform(responseBody)
	if err != nil {
		return resp, respLogAttrs, recordResponseTransformError(span, err)
	}

	if isDebug {
		respLogAttrs = append(respLogAttrs, slog.Any("body", transformedBody))

		encodedBody, err := json.Marshal(transformedBody)
		if err == nil {
			span.SetAttributes(attribute.String("transformed_body", string(encodedBody)))
		}
	}

	return transformedBody, respLogAttrs, nil
}

func recordResponseTransformError(span trace.Span, err error) error {
	span.SetStatus(codes.Error, err.Error())
	span.RecordError(err)

	return &goutils.ErrorDetail{
		Detail: err.Error(),
		Code:   oaschema.ErrCodeResponseTransformError,
	}
}
