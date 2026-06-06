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
	"net/http"

	"github.com/relychan/gohttpc"
	"github.com/relychan/goutils/httperror"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/oasvalidator"
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

func validateRequest(
	route *internal.Route,
	request *proxyhandler.Request,
	cookies []*http.Cookie,
) *httperror.HTTPError {
	errs := validateRequestParameters(route, request)

	bodyErrs := validateRequestBody(route.Method.Operation.RequestBody, request.Body())
	if len(bodyErrs) > 0 {
		errs = append(errs, bodyErrs...)
	}

	if len(cookies) > 0 {
		// Skip cookies if the request is not streaming.
		cookieParams, cookieErrs := parameter.DecodeCookieParameters(
			route.Method.Operation.Parameters,
			cookies,
		)
		if len(cookieErrs) > 0 {
			errs = append(errs, cookieErrs...)
		} else {
			request.SetCookieParams(cookieParams)
		}
	}

	if len(errs) > 0 {
		err := httperror.NewBadRequestError(errs...)
		err.Instance = request.Path()

		return err
	}

	return nil
}

func validateRequestBody(requestBody *oaschema.RequestBody, body any) []httperror.ValidationError {
	if requestBody == nil {
		return nil
	}

	if requestBody.Content != nil && requestBody.Content.Schema != nil {
		errs := oasvalidator.ValidateValue(
			requestBody.Content.Schema,
			body,
		)
		if len(errs) > 0 {
			for i := range errs {
				errs[i].Code = oasvalidator.ErrCodeRequestBodyError
			}

			return errs
		}
	}

	return nil
}

func validateRequestParameters(
	route *internal.Route,
	request *proxyhandler.Request,
) []httperror.ValidationError {
	request.SetURLParams(route.ParamValues)

	queryParams, validationErrs := parameter.DecodeQueryFromParameters(
		route.Method.Operation.Parameters,
		request.Query(),
	)
	if len(validationErrs) == 0 {
		request.SetQueryParams(queryParams)
	}

	headerParams, errs := parameter.DecodeHeaderParameters(
		route.Method.Operation.Parameters,
		request.Header(),
	)
	if len(errs) == 0 {
		request.SetHeaderParams(headerParams)
	} else {
		validationErrs = append(validationErrs, errs...)
	}

	if len(validationErrs) > 0 {
		return validationErrs
	}

	return nil
}
