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
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/relychan/gohttpc"
	"github.com/relychan/goutils"
	"github.com/relychan/openapitools/openapiclient/handler/proxyhandler"
	"github.com/relychan/openapitools/openapiclient/internal"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

// Execute routes and proxies the request to the remote server.
func (pc *ProxyClient) Execute(
	ctx context.Context,
	req *http.Request,
) (*http.Response, any, error) {
	ctx, span := tracer.Start(ctx, "Proxy")
	defer span.End()

	span.SetAttributes(
		semconv.HTTPRequestMethodKey.String(req.Method),
		semconv.URLOriginal(req.URL.String()),
	)

	requestPath := req.URL.Path

	if pc.metadata.Settings.Expose != nil && !*pc.metadata.Settings.Expose {
		// This API isn't exposed. Returns HTTP 404
		return nil, nil, goutils.RFC9457Error{
			Status:   http.StatusNotFound,
			Title:    "Resource Not Found",
			Instance: req.URL.Path,
		}
	}

	if pc.metadata.Settings.BasePath != "" && req.URL.Path != "" {
		// The URL path may omit the slash character
		if req.URL.Path[0] == '/' {
			requestPath = strings.TrimPrefix(req.URL.Path, pc.metadata.Settings.BasePath)
		} else {
			requestPath = strings.TrimPrefix(
				req.URL.Path,
				pc.metadata.Settings.BasePath[1:],
			)
		}
	}

	span.SetAttributes(semconv.URLPath(requestPath))

	route := pc.node.FindRoute(requestPath, req.Method)
	if route == nil {
		span.SetStatus(codes.Error, "request path or method does not exist")

		return nil, nil, goutils.RFC9457Error{
			Status:   http.StatusNotFound,
			Title:    "Resource Not Found",
			Instance: req.URL.Path,
		}
	}

	span.SetAttributes(
		attribute.String("http.request.proxy.type", string(route.Method.Handler.Type())),
	)

	options := &proxyhandler.ProxyHandleOptions{
		Settings:    pc.metadata.Settings,
		ParamValues: route.ParamValues,
		NewRequest:  pc.newRequestFunc(route),
		Path:        requestPath,
	}

	response, responseBody, err := route.Method.Handler.Handle(ctx, req, options)
	if err != nil {
		span.SetStatus(codes.Error, "proxy failed")
		span.RecordError(err)

		var rfc9457Error goutils.RFC9457Error

		if errors.As(err, &rfc9457Error) {
			rfc9457Error.Instance = req.URL.Path
		} else {
			rfc9457Error = goutils.RFC9457Error{
				Status:   http.StatusInternalServerError,
				Title:    http.StatusText(http.StatusInternalServerError),
				Detail:   err.Error(),
				Instance: req.URL.Path,
			}
		}

		return nil, nil, rfc9457Error
	}

	span.SetStatus(codes.Ok, "")

	return response, responseBody, nil
}

func (pc *ProxyClient) newRequestFunc(route *internal.Route) proxyhandler.NewRequestFunc {
	return func(method string, url string) *gohttpc.RequestWithClient {
		req := pc.lbClient.R(method, url)
		reqHeader := req.Header()

		authenticator := pc.authenticators.GetAuthenticator(route.Method.Security)
		if authenticator != nil {
			req.SetAuthenticator(authenticator)
		}

		for key, value := range pc.defaultHeaders {
			reqHeader.Set(key, value)
		}

		if pc.metadata.Settings.ForwardHeaders != nil {
			for _, key := range pc.metadata.Settings.ForwardHeaders.Request {
				value := reqHeader.Get(key)
				if value != "" {
					reqHeader.Set(key, value)
				}
			}
		}

		return req
	}
}
