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
	"github.com/relychan/gohttpc"
	"github.com/relychan/goutils/httperror"
	"github.com/relychan/openapitools/oasvalidator/parameter"
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

	queryParams, errs := parameter.DecodeQueryFromParameters(
		route.Method.Operation.Parameters,
		request.Query(),
	)
	if len(errs) > 0 {
		return httperror.NewBadRequestError(errs...)
	}

	request.SetQueryParams(queryParams)

	return nil
}
