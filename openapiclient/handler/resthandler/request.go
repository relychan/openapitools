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
	"github.com/relychan/openapitools/oasvalidator/contentencoder"
	"github.com/relychan/openapitools/oasvalidator/parameter"
	"github.com/relychan/openapitools/openapiclient/handler/proxyhandler"
)

func (re *RESTfulHandler) prepareRequest(
	request *proxyhandler.Request,
	options *proxyhandler.ProxyHandleOptions,
) (*gohttpc.RequestWithClient, error) {
	if re.customRequest == nil || re.customRequest.IsZero() {
		req := options.NewRequest(request.Method(), request.URL())

		body := request.Body()
		if body == nil {
			return req, nil
		}

		// Proxies the raw request to the remote service if the body is a reader.
		reader, ok := body.(io.Reader)
		if ok && reader != nil {
			req.SetBody(reader)

			return req, nil
		}

		// The body was decoded (e.g. by ServeHTTP or Execute). Encode it back
		// with the operation's request content type.
		err := re.setEncodedRequestBody(req, request, body)
		if err != nil {
			return nil, err
		}

		return req, nil
	}

	return re.transformRequest(request, options)
}

func (re *RESTfulHandler) transformRequest( //nolint:cyclop,funlen
	request *proxyhandler.Request,
	options *proxyhandler.ProxyHandleOptions,
) (*gohttpc.RequestWithClient, error) {
	requestPath := request.Path()
	method := request.Method()

	if re.customRequest.Method != "" {
		method = re.customRequest.Method
	}

	rawRequestData := request.ToMap()
	hasQueryParam := false

	var resolvedRequestURL *url.URL

	if re.customRequest.Path != "" {
		resolvedRequestPath, err := re.evaluateRequestPath(
			re.customRequest.Path,
			request,
			rawRequestData,
		)
		if err != nil {
			return nil, err
		}

		resolvedRequestURL, err = goutils.ParseRelativeURI(resolvedRequestPath)
		if err != nil {
			return nil, err
		}
	} else {
		resolvedRequestURL = &url.URL{
			Path: requestPath,
		}
	}

	req := options.NewRequest(method, resolvedRequestURL.Path)
	queryValues := resolvedRequestURL.Query()

	for _, param := range re.customRequest.Parameters {
		switch param.In {
		case oaschema.InHeader:
			rawValue, err := param.Evaluate(rawRequestData)
			if err != nil {
				respErr := httperror.NewBadRequestError(httperror.ValidationError{
					Code:      oasvalidator.ErrCodeRequestTransformError,
					Detail:    err.Error(),
					Parameter: param.Name,
					Location:  oaschema.HeaderKey,
				})
				respErr.Detail = "failed to transform request header"

				return nil, respErr
			}

			if rawValue != nil {
				value := parameter.EncodeHeader(&param.Parameter, rawValue)
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

			parameter.SetQueryParam(queryValues, &param.Parameter, value)
		default:
		}
	}

	// Forward all query params if forwardAllQueryParams is true
	// or null and there is no query param in the parameters list.
	rawQueryParams := request.Query()

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
		resolvedRequestURL.RawQuery = parameter.EncodeQueryValuesUnescape(queryValues)
		req.SetURL(resolvedRequestURL.String())
	}

	newBody := request.Body()

	var err error

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

	err = re.setEncodedRequestBody(req, request, newBody)
	if err != nil {
		return nil, err
	}

	return req, nil
}

// setEncodedRequestBody sets the content type header and the encoded body to the request.
func (re *RESTfulHandler) setEncodedRequestBody(
	req *gohttpc.RequestWithClient,
	request *proxyhandler.Request,
	body any,
) error {
	contentType := re.getDestinedContentType(request)
	req.Header()[httpheader.ContentType] = []string{contentType}

	switch typedBody := body.(type) {
	case io.Reader:
		req.SetBody(typedBody)

		return nil
	case []byte:
		req.SetBody(bytes.NewReader(typedBody))

		return nil
	default:
	}

	newBodyBytes, finalContentType, err := contentencoder.Encode(
		contentType,
		body,
		re.request,
		request.Header(),
	)
	if err != nil {
		errDetail, ok := errors.AsType[*httperror.ValidationError](err)
		if !ok {
			errDetail = &httperror.ValidationError{
				Detail: err.Error(),
				Code:   oasvalidator.ErrCodeRequestTransformError,
			}
		}

		respErr := httperror.NewBadRequestError(*errDetail)
		respErr.Detail = "failed to encode request body"

		return respErr
	}

	req.Header()[httpheader.ContentType] = []string{finalContentType}
	req.SetBody(bytes.NewReader(newBodyBytes))

	return nil
}

// Get the destined content type, fallback to application/json if it does not exist.
func (re *RESTfulHandler) getDestinedContentType(request *proxyhandler.Request) string {
	if re.request != nil && re.request.ContentType != "" {
		return re.request.ContentType
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
) (string, error) {
	return oasvalidator.ReplaceURLTemplate(
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

				return toPathParamValue(key, value)
			}

			// fallback to get the parameter from the original request path.
			value, ok := request.URLParams()[key]
			if ok {
				return toPathParamValue(key, value)
			}

			respErr := httperror.NewBadRequestError(httperror.ValidationError{
				Detail:    "the parameter can not be resolved",
				Parameter: key,
				Code:      oasvalidator.ErrCodeInvalidURLParam,
			})
			respErr.Detail = "failed to evaluate request path"

			return "", respErr
		})
}

// toPathParamValue converts a decoded path parameter value to string.
// The value is not escaped because the upstream URL builder (goutils.AppendURL) stores the path
// in url.URL.Path, which is escaped once when the request is sent. Values containing '?' or '#'
// are rejected because AppendURL splits the query and fragment from the raw string,
// which would allow injecting query parameters or truncating the upstream URL.
func toPathParamValue(key string, value any) (string, error) {
	result := goutils.ToString(value)

	if strings.ContainsAny(result, "?#") {
		respErr := httperror.NewBadRequestError(httperror.ValidationError{
			Detail:    "the path parameter must not contain '?' or '#' characters",
			Parameter: key,
			Code:      oasvalidator.ErrCodeInvalidURLParam,
		})
		respErr.Detail = "failed to evaluate request path"

		return "", respErr
	}

	return result, nil
}
