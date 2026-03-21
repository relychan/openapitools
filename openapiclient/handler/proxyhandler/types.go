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

package proxyhandler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/hasura/goenvconf"
	"github.com/relychan/goutils"
	"github.com/relychan/goutils/httpheader"
)

var (
	errOAuth2ClientCredentialsRequired = errors.New("clientId and clientSecret must not be empty")
	errOAuth2TokenURLRequired          = errors.New(
		"tokenUrl is required in the OAuth2 Client Credentials flow",
	)
)

// ProxyActionType represents enums of proxy types.
type ProxyActionType string

// InsertRouteOptions represents options for inserting routes.
type InsertRouteOptions struct {
	GetEnv goenvconf.GetEnvFunc
}

// APIKeyCredentials holds apiKey credentials of the security scheme.
type APIKeyCredentials struct {
	APIKey *goenvconf.EnvString `json:"apiKey,omitempty" yaml:"apiKey,omitempty"`
}

// BasicCredentials holds basic credentials of the security scheme.
type BasicCredentials struct {
	Username *goenvconf.EnvString `json:"username,omitempty" yaml:"username,omitempty"`
	Password *goenvconf.EnvString `json:"password,omitempty" yaml:"password,omitempty"`
}

// OAuth2Credentials holds OAuth2 credentials of the security scheme.
type OAuth2Credentials struct {
	ClientID     *goenvconf.EnvString `json:"clientId,omitempty" yaml:"clientId,omitempty"`
	ClientSecret *goenvconf.EnvString `json:"clientSecret,omitempty" yaml:"clientSecret,omitempty"`
	// Optional query parameters for the token and refresh URLs.
	EndpointParams map[string]goenvconf.EnvString `json:"endpointParams,omitempty" yaml:"endpointParams,omitempty"`
}

// RequestTemplateData represents the request data for template transformation.
type RequestTemplateData struct {
	Params      map[string]string
	QueryParams url.Values
	Headers     map[string]string
	Body        any
}

// NewRequestTemplateData creates a new [RequestTemplateData] from the HTTP request to a map for request transformation.
func NewRequestTemplateData(
	request *http.Request,
	contentType string,
	paramValues map[string]string,
) (*RequestTemplateData, bool, error) {
	requestHeaders := map[string]string{}

	for key, header := range request.Header {
		if len(header) == 0 {
			continue
		}

		requestHeaders[strings.ToLower(key)] = header[0]
	}

	requestData := &RequestTemplateData{
		Params:  paramValues,
		Headers: requestHeaders,
	}

	rawQuery := strings.TrimSpace(request.URL.RawQuery)
	if rawQuery == "" {
		requestData.QueryParams = url.Values{}
	} else {
		requestData.QueryParams, _ = url.ParseQuery(rawQuery)
	}

	if request.Body == nil || request.Body == http.NoBody {
		return requestData, true, nil
	}

	switch {
	case strings.HasPrefix(contentType, httpheader.ContentTypeJSON):
		defer goutils.CatchWarnErrorFunc(request.Body.Close)

		var body any

		err := json.NewDecoder(request.Body).Decode(&body)
		if err != nil {
			return nil, true, err
		}

		requestData.Body = body

		return requestData, true, nil
	default:
		// skip other content types.
	}

	return requestData, false, nil
}

// ToMap converts the struct to map.
func (rtd RequestTemplateData) ToMap() map[string]any {
	result := map[string]any{
		"param":   rtd.Params,
		"query":   rtd.QueryParams,
		"headers": rtd.Headers,
	}

	if rtd.Body != nil {
		result["body"] = rtd.Body
	}

	return result
}
