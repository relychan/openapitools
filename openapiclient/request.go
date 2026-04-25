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
	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/relychan/gohttpc"
	"github.com/relychan/goutils"
	"github.com/relychan/goutils/httperror"
	"github.com/relychan/goutils/httpheader"
	"github.com/relychan/openapitools/openapiclient/handler/proxyhandler"
	"github.com/relychan/openapitools/openapiclient/internal"
)

func (pc *ProxyClient) newRequestFunc(
	request *proxyhandler.Request,
	route *internal.Route,
) proxyhandler.NewRequestFunc {
	return func(method string, url string) *gohttpc.RequestWithClient {
		req := pc.lbClient.R(method, url)
		reqHeader := req.Header()

		authenticator := pc.authenticators.GetAuthenticator(route.Method.Operation.Security)
		if authenticator != nil {
			req.SetAuthenticator(authenticator)
		}

		for key, value := range pc.defaultHeaders {
			reqHeader.Set(key, value)
		}

		headers := request.Header()

		if len(headers) > 0 &&
			pc.settings != nil &&
			pc.settings.ForwardHeaders != nil {
			for _, key := range pc.settings.ForwardHeaders.Request {
				value := headers.Get(key)
				if value != "" {
					reqHeader.Set(key, value)
				}
			}
		}

		return req
	}
}

func validateRequestParameters(
	route *internal.Route,
	request *proxyhandler.Request,
) *httperror.HTTPError {
	request.SetURLParams(route.ParamValues)

	return nil
}

func getRequestBodyContentSchema(
	route *internal.Route,
	contentType string,
) (string, *highv3.MediaType) {
	if contentType != "" {
		contentType = httpheader.ExtractBaseMediaType(contentType)
	}

	if route.Method.Operation == nil ||
		route.Method.Operation.RequestBody == nil {
		return contentType, nil
	}

	contents := route.Method.Operation.RequestBody.Content

	if route.Method.Operation.RequestBody.Content == nil || contents.Len() == 0 {
		return contentType, nil
	}

	// Get the default content type if the input
	if contentType == "" {
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

	// Find the exact match of the input content type
	for content := contents.First(); content != nil; content = content.Next() {
		key := content.Key()
		value := content.Value()

		if goutils.HasStringPrefixFold(key, contentType) {
			return contentType, value
		}
	}

	return "", nil
}
