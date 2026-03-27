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
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"

	"github.com/relychan/gohttpc"
	"github.com/relychan/goutils"
	"github.com/relychan/goutils/httpheader"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/openapiclient/handler/proxyhandler"
	"github.com/relychan/openapitools/openapiclient/handler/resthandler/contenttype"
	"github.com/relychan/openapitools/openapiclient/handler/resthandler/parameter"
)

func (re *RESTfulHandler) prepareRequest(
	request *proxyhandler.Request,
	options *proxyhandler.ProxyHandleOptions,
) (*gohttpc.RequestWithClient, error) {
	if re.customRequest == nil || re.customRequest.IsZero() {
		req := options.NewRequest(request.Method, options.Path)

		// Proxies the raw request to the remote service if the body is a reader.
		reader, ok := request.Body.(io.Reader)
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
	requestPath := options.Path
	method := request.Method

	if re.customRequest.URL != "" {
		requestPath = re.customRequest.URL
	}

	if re.customRequest.Method != "" {
		method = re.customRequest.Method
	}

	requestData, err := proxyhandler.NewRequestTemplateData(
		request,
		options.ParamValues,
	)
	if err != nil {
		return nil, err
	}

	rawRequestData := requestData.ToMap()
	hasQueryParam := false

	resolvedRequestPath, queryValues, err := re.evaluateRequestPath(
		requestPath,
		requestData,
		rawRequestData,
	)
	if err != nil {
		return nil, err
	}

	req := options.NewRequest(method, "")

	for i, param := range re.customRequest.Parameters {
		switch param.In {
		case oaschema.InHeader:
			rawValue, err := param.Evaluate(rawRequestData)
			if err != nil {
				return nil, &goutils.ErrorDetail{
					Code:    oaschema.ErrCodeRequestTransformError,
					Detail:  "failed to transform request header: " + err.Error(),
					Pointer: "/parameters/" + strconv.Itoa(i),
					Header:  param.Name,
				}
			}

			if rawValue != nil {
				value := parameter.EncodeHeader(param.BaseParameter, rawValue)
				req.Header().Set(param.Name, value)
			}
		case oaschema.InQuery:
			hasQueryParam = true

			value, err := param.Evaluate(rawRequestData)
			if err != nil {
				return nil, fmt.Errorf(
					"failed to transform request header %s: %w",
					param.Name,
					err,
				)
			}

			parameter.SetQueryParam(queryValues, param.BaseParameter, value)
		default:
		}
	}

	// Forward all query params if forwardAllQueryParams is true
	// or null and there is no query param in the parameters list.
	if request.URL.RawQuery != "" &&
		(!hasQueryParam && re.customRequest.ForwardAllQueryParams == nil) ||
		(re.customRequest.ForwardAllQueryParams != nil && *re.customRequest.ForwardAllQueryParams) {
		for key, values := range requestData.QueryParams {
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
			return nil, err
		}

		requestURL.RawQuery = parameter.EncodeQueryValuesUnescape(queryValues)
	} else {
		req.SetURL(resolvedRequestPath)
	}

	newBody := request.Body

	if re.customRequest.Body != nil {
		newBody, err = re.customRequest.Body.Transform(rawRequestData)
		if err != nil {
			return nil, &goutils.ErrorDetail{
				Code:    oaschema.ErrCodeRequestTransformError,
				Detail:  "failed to transform request body: " + err.Error(),
				Pointer: "/body",
			}
		}
	}

	contentType := re.getDestinedContentType(request)
	req.Header().Set(httpheader.ContentType, contentType)

	reader, ok := newBody.(io.Reader)
	if ok && reader != nil {
		req.SetBody(reader)
	} else {
		newBodyBytes, err := contenttype.Encode(contentType, newBody)
		if err != nil {
			return nil, err
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

	contentType := request.Header.Get(httpheader.ContentType)
	if contentType != "" {
		return contentType
	}

	return httpheader.ContentTypeJSON
}

func (re *RESTfulHandler) evaluateRequestPath(
	requestPath string,
	requestData *proxyhandler.RequestTemplateData,
	rawRequestData map[string]any,
) (string, url.Values, error) {
	if requestPath == "" {
		return "", url.Values{}, nil
	}

	newRequestPath, err := parameter.ReplaceURLTemplate(
		requestPath,
		func(key string) (string, error) {
			for _, param := range re.customRequest.Parameters {
				if param.Name != key {
					continue
				}

				value, err := param.Evaluate(rawRequestData)
				if err != nil {
					return "", fmt.Errorf(
						"failed to evaluate variable %s in request path %s: %w",
						key,
						requestPath,
						err,
					)
				}

				return goutils.ToString(value), nil
			}

			// fallback to get the parameter from the original request path.
			value, ok := requestData.Params[key]
			if ok {
				return goutils.ToString(value), nil
			}

			return "", fmt.Errorf(
				"%w: the parameter `%s` can not be resolved",
				errInvalidRequestPath,
				key,
			)
		})
	if err != nil {
		return "", nil, err
	}

	return extractQueryValuesFromPath(newRequestPath)
}

func extractQueryValuesFromPath(newRequestPath string) (string, url.Values, error) {
	u, query, _ := strings.Cut(newRequestPath, "?")
	if query == "" {
		return newRequestPath, url.Values{}, nil
	}

	query, fragment, _ := strings.Cut(query, "#")

	q, err := url.ParseQuery(query)
	if err != nil {
		return "", nil, fmt.Errorf(
			"invalid query params in request path %s: %w",
			newRequestPath,
			err,
		)
	}

	if fragment != "" {
		u += "#" + fragment
	}

	return u, q, nil
}
