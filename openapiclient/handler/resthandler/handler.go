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

// Package resthandler evaluates and execute REST requests to the remote server.
package resthandler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/hasura/gotel"
	"github.com/hasura/gotel/otelutils"
	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/relychan/gohttpc"
	"github.com/relychan/goutils"
	"github.com/relychan/goutils/httpheader"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/openapiclient/handler/proxyhandler"
	"github.com/relychan/openapitools/openapiclient/handler/resthandler/contenttype"
	"github.com/relychan/openapitools/openapiclient/handler/resthandler/parameter"
	"go.yaml.in/yaml/v4"
)

// RESTfulHandler implements the ProxyHandler interface for RESTful proxy.
type RESTfulHandler struct {
	operation      *highv3.Operation
	contentType    string
	customRequest  *customRESTRequest
	customResponse *customRESTResponse
	parameters     []*highv3.Parameter
}

// NewRESTfulHandler creates a RESTHandler from operation.
func NewRESTfulHandler(
	operation *highv3.Operation,
	rawProxyAction *yaml.Node,
	options *proxyhandler.NewProxyHandlerOptions,
) (proxyhandler.ProxyHandler, error) {
	handler := &RESTfulHandler{
		operation:  operation,
		parameters: oaschema.MergeParameters(options.Parameters, operation.Parameters),
	}

	if rawProxyAction == nil {
		contentType, err := parseRequestContentType(operation, nil)
		if err != nil {
			return nil, err
		}

		handler.contentType = contentType

		return handler, nil
	}

	var proxyAction ProxyRESTfulActionConfig

	err := rawProxyAction.Decode(&proxyAction)
	if err != nil {
		return nil, err
	}

	contentType, err := parseRequestContentType(operation, proxyAction.Request)
	if err != nil {
		return nil, err
	}

	handler.contentType = contentType

	getEnvFunc := options.GetEnvFunc()

	handler.customRequest, err = newCustomRESTRequestFromConfig(proxyAction.Request, getEnvFunc)
	if err != nil {
		return nil, err
	}

	handler.customResponse, err = newCustomRESTResponse(
		proxyAction.Response,
		getEnvFunc,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize response config: %w", err)
	}

	return handler, nil
}

// Type returns type of the current handler.
func (*RESTfulHandler) Type() proxyhandler.ProxyActionType {
	return ProxyActionTypeREST
}

// Handle resolves the HTTP request and proxies that request to the remote server.
func (re *RESTfulHandler) Handle(
	ctx context.Context,
	request *http.Request,
	options *proxyhandler.ProxyHandleOptions,
) (*http.Response, any, error) {
	req, logAttrs, err := re.constructRequest(request, options)
	if err != nil {
		printDebugLog(
			ctx, request,
			"failed to evaluate request",
			append(
				logAttrs,
				slog.String("error", err.Error()),
			),
		)

		return nil, nil, err
	}

	resp, err := req.Execute(ctx)
	if err != nil {
		printDebugLog(
			ctx, request,
			"failed to execute request",
			append(
				logAttrs,
				slog.String("error", err.Error()),
			),
		)

		return resp, nil, err
	}

	logAttrs = append(logAttrs, slog.Int("response_status", resp.StatusCode))

	if re.customResponse == nil || re.customResponse.IsZero() ||
		(resp.StatusCode < 200 || resp.StatusCode >= 300) ||
		!strings.HasPrefix(resp.Header.Get(httpheader.ContentType), httpheader.ContentTypeJSON) {
		printDebugLog(
			ctx, request,
			resp.Status,
			logAttrs,
		)

		return resp, resp.Body, err
	}

	newResp, respLogAttrs, err := re.transformResponse(resp)
	logAttrs = append(logAttrs, respLogAttrs...)

	if err != nil {
		printDebugLog(
			ctx, request,
			"failed to transform response",
			append(
				logAttrs,
				slog.String("error", err.Error()),
			),
		)

		return resp, nil, err
	}

	printDebugLog(ctx, request, resp.Status, logAttrs)

	return newResp, newResp.Body, err
}

func (re *RESTfulHandler) constructRequest(
	request *http.Request,
	options *proxyhandler.ProxyHandleOptions,
) (*gohttpc.RequestWithClient, []slog.Attr, error) {
	if re.customRequest == nil || re.customRequest.IsZero() {
		req := options.NewRequest(request.Method, options.Path)

		// Proxies the raw request to the remote service when there is no request.
		if request.Body != nil && request.Body != http.NoBody {
			req.SetBody(request.Body)
		}

		return req, nil, nil
	}

	return re.transformRequest(request, options)
}

func (re *RESTfulHandler) transformRequest( //nolint:gocognit,cyclop,funlen
	request *http.Request,
	options *proxyhandler.ProxyHandleOptions,
) (*gohttpc.RequestWithClient, []slog.Attr, error) {
	requestPath := options.Path
	method := request.Method

	if re.customRequest.Path != "" {
		requestPath = re.customRequest.Path
	}

	if re.customRequest.Method != "" {
		method = re.customRequest.Method
	}

	logAttrs := make([]slog.Attr, 0, 9)
	logAttrs = append(
		logAttrs,
		slog.String("path", requestPath),
		slog.String("method", method),
	)

	requestData, alreadyRead, err := proxyhandler.NewRequestTemplateData(
		request,
		request.Header.Get(httpheader.ContentType),
		options.ParamValues,
	)
	if err != nil {
		return nil, logAttrs, err
	}

	rawRequestData := requestData.ToMap()
	hasQueryParam := false

	resolvedRequestPath, queryValues, err := re.evaluateRequestPath(
		requestPath,
		requestData,
		rawRequestData,
	)
	if err != nil {
		return nil, logAttrs, err
	}

	req := options.NewRequest(method, "")

	for i, param := range re.customRequest.Parameters {
		switch param.In {
		case oaschema.InHeader:
			rawValue, err := param.Evaluate(rawRequestData)
			if err != nil {
				return nil, logAttrs, &goutils.ErrorDetail{
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
				return nil, logAttrs, fmt.Errorf(
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
			return nil, nil, err
		}

		requestURL.RawQuery = parameter.EncodeQueryValuesUnescape(queryValues)
	} else {
		req.SetURL(resolvedRequestPath)
	}

	if !alreadyRead || re.customRequest.Body == nil || re.customRequest.Body.IsZero() {
		if !alreadyRead {
			// unsupported content types will be ignored,
			// the client proxies the raw request to the remote service.
			req.SetBody(request.Body)
		}

		return req, logAttrs, nil
	}

	newBody, err := re.customRequest.Body.Transform(rawRequestData)
	if err != nil {
		return nil, logAttrs, &goutils.ErrorDetail{
			Code:    oaschema.ErrCodeRequestTransformError,
			Detail:  "failed to transform request body: " + err.Error(),
			Pointer: "/body",
		}
	}

	contentType := re.getDestinedContentType(request)

	newBodyBytes, err := contenttype.Serialize(contentType, newBody)
	if err != nil {
		return nil, logAttrs, err
	}

	req.Header().Set(httpheader.ContentType, contentType)
	req.SetBody(newBodyBytes)

	return req, logAttrs, nil
}

// Get the destined content type, fallback to application/json if it does not exist.
func (re *RESTfulHandler) getDestinedContentType(request *http.Request) string {
	if re.contentType != "" {
		return re.contentType
	}

	contentType := request.Header.Get(httpheader.ContentType)
	if contentType != "" {
		return contentType
	}

	return httpheader.ContentTypeJSON
}

func (re *RESTfulHandler) transformResponse(
	resp *http.Response,
) (*http.Response, []slog.Attr, error) {
	var responseBody any

	if resp.Body != nil {
		err := json.NewDecoder(resp.Body).Decode(&responseBody)
		goutils.CatchWarnErrorFunc(resp.Body.Close)

		if err != nil {
			return resp, nil, fmt.Errorf("failed to decode http response: %w", err)
		}
	}

	responseLogAttrs := make([]slog.Attr, 0, 2)
	responseLogAttrs = append(responseLogAttrs, slog.Any("original_body", responseBody))

	transformedBody, err := re.customResponse.Body.Transform(responseBody)
	if err != nil {
		return nil, nil, err
	}

	responseLogAttrs = append(responseLogAttrs, slog.Any("response_body", transformedBody))

	buf := new(bytes.Buffer)

	err = json.NewEncoder(buf).Encode(transformedBody)
	if err != nil {
		return resp, nil, fmt.Errorf("failed to decode transformed response: %w", err)
	}

	resp.Body = io.NopCloser(buf)

	return resp, responseLogAttrs, err
}

func (re *RESTfulHandler) evaluateRequestPath( //nolint:funlen
	requestPath string,
	requestData *proxyhandler.RequestTemplateData,
	rawRequestData map[string]any,
) (string, url.Values, error) {
	if requestPath == "" {
		return "", url.Values{}, nil
	}

	var sb strings.Builder

	var i int

	var hasQueryParams, hasFragment bool

	urlLength := len(requestPath)
	sb.Grow(urlLength)

	for ; i < urlLength; i++ {
		char := requestPath[i]

		switch char {
		case '?':
			hasQueryParams = true
		case '#':
			hasFragment = true
		default:
		}

		if char != '{' {
			sb.WriteByte(char)

			continue
		}

		i++

		if i == urlLength-1 {
			return "", nil, fmt.Errorf(
				"%w: closed curly bracket is missing in %s",
				errInvalidRequestPath,
				requestPath,
			)
		}

		j := i
		// get and validate environment variable
		for ; j < urlLength; j++ {
			nextChar := requestPath[j]
			if nextChar == '}' {
				break
			}
		}

		if j == i {
			return "", nil, fmt.Errorf(
				"%w: closed curly bracket is missing in %s",
				errInvalidRequestPath,
				requestPath,
			)
		}

		key := requestPath[i:j]
		paramExist := false

		for _, param := range re.customRequest.Parameters {
			if param.Name != key {
				continue
			}

			paramExist = true

			value, err := param.Evaluate(rawRequestData)
			if err != nil {
				return "", nil, fmt.Errorf(
					"failed to evaluate variable %s in request path %s: %w",
					key,
					requestPath,
					err,
				)
			}

			sb.WriteString(goutils.ToString(value))

			break
		}

		if paramExist {
			i = j

			continue
		}

		// fallback to get the parameter from the original request path.
		value, ok := requestData.Params[key]
		if !ok {
			sb.WriteString(goutils.ToString(value))

			i = j

			continue
		}

		return "", nil, fmt.Errorf(
			"%w: the parameter `%s` can not be resolved",
			errInvalidRequestPath,
			key,
		)
	}

	newRequestPath := sb.String()
	if !hasQueryParams {
		return newRequestPath, url.Values{}, nil
	}

	// Extract fragments and queries from the new request path if exists.
	var fragment string

	if hasFragment {
		newRequestPath, fragment, _ = strings.Cut(newRequestPath, "#")
	}

	extractPath, queries, err := extractQueryValuesFromPath(newRequestPath)
	if err != nil {
		return "", nil, err
	}

	if fragment != "" {
		extractPath += "#" + fragment
	}

	return extractPath, queries, nil
}

func extractQueryValuesFromPath(newRequestPath string) (string, url.Values, error) {
	u, query, _ := strings.Cut(newRequestPath, "?")
	if query == "" {
		return newRequestPath, url.Values{}, nil
	}

	q, err := url.ParseQuery(query)
	if err != nil {
		return "", nil, fmt.Errorf(
			"invalid query params in request path %s: %w",
			newRequestPath,
			err,
		)
	}

	return u, q, nil
}

func printDebugLog(
	ctx context.Context,
	request *http.Request,
	message string,
	attrs []slog.Attr,
) {
	logger := gotel.GetLogger(ctx)

	if !logger.Enabled(ctx, slog.LevelDebug) {
		return
	}

	attrs = append(
		attrs,
		slog.String("type", "proxy-handler"),
		slog.String("handler_type", "rest"),
		slog.String("request_url", request.URL.String()),
		otelutils.NewHeaderLogGroupAttrs(
			"request_headers",
			otelutils.NewTelemetryHeaders(request.Header),
		),
	)

	logger.LogAttrs(ctx, slog.LevelDebug, message, attrs...)
}
