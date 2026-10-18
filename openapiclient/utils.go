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

package openapiclient

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"slices"

	"github.com/hasura/goenvconf"
	"github.com/hasura/gotel"
	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/relychan/goutils/httperror"
	"github.com/relychan/goutils/httpheader"
	"github.com/relychan/openapitools/oasvalidator"
	"github.com/relychan/openapitools/oasvalidator/contentdecoder"
	"github.com/relychan/openapitools/openapiclient/internal"
)

func writeErrorResponse(ctx context.Context, writer http.ResponseWriter, status int, err error) {
	if isResponseWritten(writer) {
		// The response was already (partially) written. Do not write again.
		gotel.GetLogger(ctx).ErrorContext(
			ctx,
			"failed to proxy the request after the response was written",
			slog.String("error", err.Error()),
		)

		return
	}

	if status < 100 || status > 999 {
		status = http.StatusInternalServerError
	}

	body, encodeErr := json.Marshal(err)
	if encodeErr != nil {
		status = http.StatusInternalServerError

		body = []byte(fallbackErrorResponseBody)

		gotel.GetLogger(ctx).ErrorContext(
			ctx,
			"failed to encode the error response",
			slog.String("error", encodeErr.Error()),
		)
	}

	writer.Header()[httpheader.ContentType] = []string{httpheader.ContentTypeJSON}
	writer.WriteHeader(status)

	_, _ = writer.Write(append(body, '\n'))
}

const fallbackErrorResponseBody = `{"title":"Internal Server Error","status":500,"detail":"failed to encode the error response"}`

func isResponseWritten(writer http.ResponseWriter) bool {
	if tw, ok := writer.(headerTrackingResponseWriter); ok && tw.HeaderWritten() {
		return true
	}

	tw, ok := writer.(tracingResponseWriter)

	return ok && tw.BytesWritten() > 0
}

// parse server url from static string or environment variables.
func parseServerURL(server *highv3.Server, getEnv goenvconf.GetEnvFunc) (string, error) {
	serverURL, err := oasvalidator.ReplaceURLTemplate(server.URL, func(s string) (string, error) {
		var variable *highv3.ServerVariable

		envVar := goenvconf.NewEnvStringVariable(s)

		if server.Variables != nil {
			variable, _ = server.Variables.Get(*envVar.Variable)
			if variable != nil {
				envVar.Value = &variable.Default
			}
		}

		part, err := envVar.GetCustom(getEnv)
		if err != nil {
			return "", err
		}

		if variable != nil && len(variable.Enum) > 0 && !slices.Contains(variable.Enum, part) {
			return "", goenvconf.NewParseEnvFailedError(fmt.Sprintf(
				"value of environment variable %s must be in %v, got %q",
				*envVar.Variable,
				variable.Enum,
				part),
				"")
		}

		return part, nil
	})
	if err != nil {
		return "", err
	}

	return serverURL, nil
}

func parseHTTPRequestBody(
	route *internal.Route,
	writer http.ResponseWriter,
	request *http.Request,
) (any, *httperror.HTTPError) {
	if request.Body == nil || request.Body == http.NoBody {
		if !route.IsRequestBodyRequired() {
			return nil, nil
		}

		err := newRequestBodyRequiredError(request.URL.Path)

		writeErrorResponse(request.Context(), writer, err.Status, err)

		return nil, err
	}

	originalContentType := httpheader.GetHeaderValue(request.Header, httpheader.ContentType)
	if canHTTPMethodHaveBody(request.Method) && !httpheader.IsContentTypeJSON(originalContentType) {
		err := newUnsupportedContentTypeError(request.URL.Path, originalContentType)

		writeErrorResponse(request.Context(), writer, err.Status, err)

		return nil, err
	}

	decodedBody, err := contentdecoder.DecodeJSON(request.Body)
	if err != nil {
		respErr := httperror.NewBadRequestError(httperror.ValidationError{
			Detail: err.Error(),
			Code:   oasvalidator.ErrCodeRequestBodyError,
		})
		respErr.Detail = "failed to decode request"

		writeErrorResponse(request.Context(), writer, respErr.Status, respErr)

		return nil, respErr
	}

	return decodedBody, nil
}

func newRequestBodyRequiredError(urlPath string) *httperror.HTTPError {
	err := httperror.NewBadRequestError(httperror.ValidationError{
		Code:     oasvalidator.ErrCodeRequestBodyError,
		Detail:   "request body is required",
		Location: "body",
	})
	err.Detail = "request body is required"
	err.Instance = urlPath

	return err
}

func newUnsupportedContentTypeError(
	urlPath string,
	contentType string,
) *httperror.HTTPError {
	statusCode := http.StatusUnsupportedMediaType
	err := httperror.NewHTTPError(
		statusCode,
		"Unsupported Content-Type "+contentType+". Expected application/json only.",
	)
	err.Code = "415-01"
	err.Instance = urlPath

	return err
}

func canHTTPMethodHaveBody(method string) bool {
	return method != http.MethodGet &&
		method != http.MethodDelete &&
		method != http.MethodConnect &&
		method != http.MethodOptions &&
		method != http.MethodHead
}
