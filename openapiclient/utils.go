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
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"slices"
	"strings"

	"github.com/hasura/goenvconf"
	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/relychan/goutils"
	"github.com/relychan/goutils/httpheader"
	"github.com/relychan/openapitools/oasvalidator"
	"github.com/relychan/openapitools/openapiclient/handler/resthandler/contenttype"
	"github.com/relychan/openapitools/openapiclient/internal"
)

func writeErrorResponse(writer http.ResponseWriter, status int, err error) {
	tracingWriter, ok := writer.(tracingResponseWriter)
	if ok && tracingWriter.BytesWritten() > 0 {
		// The writer were already written. Do not write again.
		return
	}

	writer.Header()[httpheader.ContentType] = []string{httpheader.ContentTypeJSON}
	writer.WriteHeader(status)

	writeErr := json.NewEncoder(writer).Encode(err)
	if writeErr == nil {
		return
	}

	if ok && tracingWriter.BytesWritten() > 0 {
		// The writer were already written. Do not write again.
		return
	}

	_, _ = fmt.Fprintf(
		writer,
		`{"title":"Internal Server Error","detail":"%s"}`,
		html.EscapeString(writeErr.Error()),
	)
}

// parse server url from static string or environment variables.
func parseServerURL(server *highv3.Server, getEnv goenvconf.GetEnvFunc) (string, error) {
	rawServerURL := strings.TrimSpace(server.URL)

	return oasvalidator.ReplaceURLTemplate(rawServerURL, func(s string) (string, error) {
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
			return "", fmt.Errorf( //nolint:err113
				"value of environment variable %s must be in %v, got `%s`",
				*envVar.Variable,
				variable.Enum,
				part,
			)
		}

		return part, nil
	})
}

func parseHTTPRequestBody(
	route *internal.Route,
	writer http.ResponseWriter,
	request *http.Request,
	contentType string,
) (any, error) {
	if request.Body == nil || request.Body == http.NoBody {
		if !route.IsRequestBodyRequired() {
			return nil, nil
		}

		err := goutils.NewBadRequestError()
		err.Detail = "request body is required"

		writeErrorResponse(writer, err.Status, err)

		return nil, err
	}

	decodedBody, err := contenttype.Decode(contentType, request.Body)
	if err == nil {
		return decodedBody, nil
	}

	errorDetail, ok := errors.AsType[*goutils.ErrorDetail](err)
	if !ok {
		errorDetail = &goutils.ErrorDetail{
			Detail: err.Error(),
			Code:   oasvalidator.ErrCodeRequestDecodeBodyError,
		}
	}

	respErr := goutils.NewBadRequestError(*errorDetail)
	respErr.Detail = "failed to decode request"

	writeErrorResponse(writer, respErr.Status, respErr)

	return nil, err
}

func newUnsupportedContentTypeError(
	route *internal.Route,
	urlPath string,
	contentType string,
) *goutils.RFC9457Error {
	var sb strings.Builder

	sb.WriteString("Unsupported Content-Type ")
	sb.WriteString(contentType)
	sb.WriteString(". Expected one of ")

	contents := route.Method.Operation.RequestBody.Content
	isFirstContent := true

	for content := contents.First(); content != nil; content = content.Next() {
		if isFirstContent {
			isFirstContent = false
		} else {
			sb.WriteString(", ")
		}

		sb.WriteString(content.Key())
	}

	statusCode := http.StatusUnsupportedMediaType
	err := goutils.NewRFC9457Error(statusCode, sb.String())
	err.Code = "415-01"
	err.Instance = urlPath

	return err
}
