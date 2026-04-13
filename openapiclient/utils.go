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
	"fmt"
	"html"
	"net/http"
	"slices"
	"strings"

	"github.com/hasura/goenvconf"
	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/relychan/goutils/httpheader"
	"github.com/relychan/openapitools/openapiclient/handler/resthandler/parameter"
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

	return parameter.ReplaceURLTemplate(rawServerURL, func(s string) (string, error) {
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
