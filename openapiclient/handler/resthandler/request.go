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
	"bytes"
	"errors"
	"io"
	"net/url"
	"strings"

	"github.com/relychan/gohttpc"
	"github.com/relychan/goutils"
	"github.com/relychan/goutils/httperror"
	"github.com/relychan/goutils/httpheader"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/oasvalidator"
	"github.com/relychan/openapitools/oasvalidator/parameter"
	"github.com/relychan/openapitools/openapiclient/handler/proxyhandler"
	"github.com/relychan/openapitools/openapiclient/handler/resthandler/contenttype"
)

func (re *RESTfulHandler) prepareRequest(
	request *proxyhandler.Request,
	options *proxyhandler.ProxyHandleOptions,
) (*gohttpc.RequestWithClient, error) {
	if re.customRequest == nil || re.customRequest.IsZero() {
		req := options.NewRequest(request.Method(), request.URL())

		// Proxies the raw request to the remote service if the body is a reader.
		reader, ok := request.Body().(io.Reader)
		if ok && reader != nil {
			req.SetBody(reader)
		}

		return req, nil
	}

	return re.transformRequest(request, options)
}

func (re *RESTfulHandler) transformRequest( //nolint:gocognit,cyclop,funlen
	request *proxyhandler.Request,
	options *proxyhandler.ProxyHandleOptions,
) (*gohttpc.RequestWithClient, error) {
	requestPath := request.URL()
	method := request.Method()

	if re.customRequest.URL != "" {
		requestPath = re.customRequest.URL
	}

	if re.customRequest.Method != "" {
		method = re.customRequest.Method
	}

	rawRequestData := request.ToMap()
	hasQueryParam := false

	resolvedRequestPath, queryValues, err := re.evaluateRequestPath(
		requestPath,
		request,
		rawRequestData,
	)
	if err != nil {
		return nil, err
	}

	req := options.NewRequest(method, resolvedRequestPath)

	for _, param := range re.customRequest.Parameters {
		switch param.In {
		case oaschema.InHeader:
			rawValue, err := param.Evaluate(rawRequestData)
			if err != nil {
				respErr := httperror.NewBadRequestError(httperror.ValidationError{
					Code:   oasvalidator.ErrCodeRequestTransformError,
					Detail: err.Error(),
					Header: param.Name,
				})
				respErr.Detail = "failed to transform request header"

				return nil, respErr
			}

			if rawValue != nil {
				value := parameter.EncodeHeader(param.BaseParameter, rawValue)
				req.Header().Set(param.Name, value)
			}
		case oaschema.InQuery:
			hasQueryParam = true

			value, err := param.Evaluate(rawRequestData)
			if err != nil {
				respErr := httperror.NewBadRequestError(httperror.ValidationError{
					Code:      oasvalidator.ErrCodeRequestTransformError,
					Detail:    err.Error(),
					Parameter: param.Name,
				})
				respErr.Detail = "failed to transform request query parameter"

				return nil, respErr
			}

			parameter.SetQueryParam(queryValues, param.BaseParameter, value)
		default:
		}
	}

	// Forward all query params if forwardAllQueryParams is true
	// or null and there is no query param in the parameters list.
	rawQueryParams := request.QueryParams()

	if len(rawQueryParams) > 0 &&
		(!hasQueryParam && re.customRequest.ForwardAllQueryParams == nil) ||
		(re.customRequest.ForwardAllQueryParams != nil && *re.customRequest.ForwardAllQueryParams) {
		for key, values := range rawQueryParams {
			escapedKey := url.QueryEscape(key)
			if !queryValues.Has(key) && !queryValues.Has(escapedKey) {
				for _, value := range values {
					escapedValue := url.QueryEscape(value)
					queryValues.Add(escapedKey, escapedValue)
				}
			}
		}
	}

	if len(queryValues) > 0 {
		requestURL, err := goutils.ParsePathOrHTTPURL(resolvedRequestPath)
		if err != nil {
			respErr := httperror.NewBadRequestError(httperror.ValidationError{
				Code:   oasvalidator.ErrCodeInvalidRequestURL,
				Detail: err.Error(),
			})
			respErr.Detail = "failed to parse request URL"

			return nil, respErr
		}

		requestURL.RawQuery = parameter.EncodeQueryValuesUnescape(queryValues)
		req.SetURL(requestURL.String())
	}

	newBody := request.Body()

	if re.customRequest.Body != nil {
		newBody, err = re.customRequest.Body.Transform(rawRequestData)
		if err != nil {
			respErr := httperror.NewBadRequestError(httperror.ValidationError{
				Code:   oasvalidator.ErrCodeRequestTransformError,
				Detail: err.Error(),
			})
			respErr.Detail = "failed to transform request body"

			return nil, respErr
		}
	}

	contentType := re.getDestinedContentType(request)
	req.Header()[httpheader.ContentType] = []string{contentType}

	reader, ok := newBody.(io.Reader)
	if ok && reader != nil {
		req.SetBody(reader)
	} else {
		newBodyBytes, err := contenttype.Encode(contentType, newBody)
		if err != nil {
			errDetail, ok := errors.AsType[*httperror.ValidationError](err)
			if !ok {
				errDetail = &httperror.ValidationError{
					Detail: err.Error(),
					Code:   oasvalidator.ErrCodeRequestTransformError,
				}
			}

			respErr := httperror.NewBadRequestError(*errDetail)
			respErr.Detail = "failed to encode transformed request body"

			return nil, respErr
		}

		req.SetBody(bytes.NewReader(newBodyBytes))
	}

	return req, nil
}

// Get the destined content type, fallback to application/json if it does not exist.
func (re *RESTfulHandler) getDestinedContentType(request *proxyhandler.Request) string {
	if re.requestContentType != "" {
		return re.requestContentType
	}

	contentType := httpheader.GetHeaderValue(request.Header(), httpheader.ContentType)
	if contentType != "" {
		return contentType
	}

	return httpheader.ContentTypeJSON
}

func (re *RESTfulHandler) evaluateRequestPath(
	requestPath string,
	request *proxyhandler.Request,
	rawRequestData map[string]any,
) (string, url.Values, error) {
	if requestPath == "" {
		return "", url.Values{}, nil
	}

	newRequestPath, err := oasvalidator.ReplaceURLTemplate(
		requestPath,
		func(key string) (string, error) {
			for _, param := range re.customRequest.Parameters {
				if param.Name != key {
					continue
				}

				value, err := param.Evaluate(rawRequestData)
				if err != nil {
					respErr := httperror.NewBadRequestError(httperror.ValidationError{
						Detail:    err.Error(),
						Pointer:   "/" + param.Name,
						Parameter: key,
						Code:      oasvalidator.ErrCodeInvalidRequestURL,
					})
					respErr.Detail = "failed to evaluate variable"

					return "", respErr
				}

				return goutils.ToString(value), nil
			}

			// fallback to get the parameter from the original request path.
			value, ok := request.URLParams()[key]
			if ok {
				return goutils.ToString(value), nil
			}

			respErr := httperror.NewBadRequestError(httperror.ValidationError{
				Detail:    "the parameter can not be resolved",
				Parameter: key,
				Code:      oasvalidator.ErrCodeInvalidURLParam,
			})
			respErr.Detail = "failed to evaluate request path"

			return "", respErr
		})
	if err != nil {
		return "", nil, err
	}

	return extractQueryValuesFromPath(newRequestPath)
}

func extractQueryValuesFromPath(
	newRequestPath string,
) (string, url.Values, error) {
	u, query, _ := strings.Cut(newRequestPath, "?")
	if query == "" {
		return newRequestPath, url.Values{}, nil
	}

	query, fragment, _ := strings.Cut(query, "#")

	q, err := url.ParseQuery(query)
	if err != nil {
		respErr := httperror.NewBadRequestError(httperror.ValidationError{
			Detail: err.Error(),
			Code:   oasvalidator.ErrCodeInvalidRequestURL,
		})
		respErr.Detail = "invalid query params"

		return "", nil, respErr
	}

	if fragment != "" {
		u += "#" + fragment
	}

	return u, q, nil
}
