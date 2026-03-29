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

// Package openapiclient implements a client to proxy requests to external services.
package openapiclient

import (
	"context"
	"fmt"

	"github.com/hasura/goenvconf"
	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/relychan/gohttpc"
	"github.com/relychan/gohttpc/httpconfig"
	"github.com/relychan/gohttpc/loadbalancer"
	"github.com/relychan/gohttpc/loadbalancer/roundrobin"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/openapiclient/handler/proxyhandler"
	"github.com/relychan/openapitools/openapiclient/internal"
)

// ProxyClient helps manage and execute REST and GraphQL APIs from the API document.
type ProxyClient struct {
	clientOptions

	lbClient       *loadbalancer.LoadBalancerClient
	metadata       *oaschema.OpenAPIResourceDefinition
	node           *internal.Node
	defaultHeaders map[string]string
	authenticators *proxyhandler.OpenAPIAuthenticator
}

// NewProxyClient creates a proxy client from the API document.
func NewProxyClient(
	ctx context.Context,
	metadata *oaschema.OpenAPIResourceDefinition,
	options ...ClientOption,
) (*ProxyClient, error) {
	clientOptions := clientOptions{
		ClientOptions: gohttpc.NewClientOptions(),
	}

	for _, opt := range options {
		if opt == nil {
			continue
		}

		opt(&clientOptions)
	}

	client := &ProxyClient{
		metadata:       metadata,
		clientOptions:  clientOptions,
		defaultHeaders: map[string]string{},
	}

	err := client.init(ctx)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// Metadata returns the metadata of the OpenAPI client.
func (pc *ProxyClient) Metadata() *oaschema.OpenAPIResourceDefinition {
	return pc.metadata
}

// Close method performs cleanup and closure activities on the client instance.
func (pc *ProxyClient) Close() error {
	if pc.HTTPClient != nil {
		pc.HTTPClient.CloseIdleConnections()
	}

	if pc.lbClient != nil {
		return pc.lbClient.Close()
	}

	return nil
}

func (pc *ProxyClient) init(ctx context.Context) error {
	spec, err := pc.metadata.Build(ctx)
	if err != nil {
		return err
	}

	err = pc.initHTTPClient()
	if err != nil {
		return err
	}

	err = pc.initServers(spec)
	if err != nil {
		return err
	}

	err = pc.initDefaultHeaders()
	if err != nil {
		return err
	}

	pc.authenticators, err = proxyhandler.NewOpenAPIv3Authenticator(
		spec,
		pc.GetEnvFunc(),
	)
	if err != nil {
		return err
	}

	node, err := BuildMetadataTree(spec, pc.clientOptions)
	if err != nil {
		return err
	}

	pc.node = node

	return nil
}

func (pc *ProxyClient) initDefaultHeaders() error {
	if pc.metadata.Settings == nil {
		return nil
	}

	getEnv := pc.GetEnvFunc()

	for key, envValue := range pc.metadata.Settings.Headers {
		value, err := envValue.GetCustom(getEnv)
		if err != nil {
			return fmt.Errorf("failed to load header %s: %w", key, err)
		}

		if value != "" {
			pc.defaultHeaders[key] = value
		}
	}

	return nil
}

func (pc *ProxyClient) initServers(spec *highv3.Document) error {
	if len(spec.Servers) == 0 {
		return errServerURLRequired
	}

	var err error

	var healthCheckBuilder *loadbalancer.HTTPHealthCheckPolicyBuilder

	if pc.metadata.Settings != nil &&
		pc.metadata.Settings.HealthCheck != nil &&
		pc.metadata.Settings.HealthCheck.HTTP != nil {
		healthCheckBuilder, err = pc.metadata.Settings.HealthCheck.HTTP.ToPolicyBuilder()
		if err != nil {
			return err
		}
	} else {
		healthCheckBuilder = loadbalancer.NewHTTPHealthCheckPolicyBuilder()
	}

	hosts := make([]*loadbalancer.Host, 0, len(spec.Servers))

	for _, server := range spec.Servers {
		host, err := pc.initServer(server, healthCheckBuilder)
		if err != nil {
			return err
		}

		if host != nil {
			hosts = append(hosts, host)
		}
	}

	if len(hosts) == 0 {
		return ErrNoAvailableServer
	}

	wrr, err := roundrobin.NewWeightedRoundRobin(hosts)
	if err != nil {
		return err
	}

	pc.lbClient = loadbalancer.NewLoadBalancerClientWithOptions(wrr, pc.clientOptions)

	return nil
}

func (pc *ProxyClient) initServer(
	server *highv3.Server,
	healthCheckBuilder *loadbalancer.HTTPHealthCheckPolicyBuilder,
) (*loadbalancer.Host, error) {
	getEnv := pc.GetEnvFunc()

	serverURL, err := parseServerURL(server, getEnv)
	if err != nil {
		return nil, err
	}

	if serverURL == "" {
		return nil, nil
	}

	host, err := loadbalancer.NewHost(
		pc.HTTPClient,
		serverURL,
		loadbalancer.WithHTTPHealthCheckPolicyBuilder(healthCheckBuilder),
	)
	if err != nil {
		return nil, err
	}

	if server.Name != "" {
		host.SetName(server.Name)
	}

	rawWeight, exist := server.Extensions.Get(oaschema.XRelyServerWeight)
	if exist && rawWeight != nil {
		var weight int

		err := rawWeight.Decode(&weight)
		if err != nil {
			return nil, fmt.Errorf("failed to decode weight from server: %w", err)
		}

		if weight > 1 {
			host.SetWeight(weight)
		}
	}

	rawHeaders, exist := server.Extensions.Get(oaschema.XRelyServerHeaders)
	if exist && rawHeaders != nil {
		headerEnvs := map[string]goenvconf.EnvString{}

		err := rawHeaders.Decode(&headerEnvs)
		if err != nil {
			return nil, fmt.Errorf("failed to decode headers from server: %w", err)
		}

		if len(headerEnvs) > 0 {
			headers := make(map[string]string)

			for key, header := range headerEnvs {
				value, err := header.GetCustom(getEnv)
				if err != nil {
					return nil, fmt.Errorf("failed to get header %s: %w", key, err)
				}

				if value != "" {
					headers[key] = value
				}
			}

			host.SetHeaders(headers)
		}
	}

	return host, nil
}

func (pc *ProxyClient) initHTTPClient() error {
	var httpConfig *httpconfig.HTTPClientConfig

	if pc.metadata.Settings != nil && pc.metadata.Settings.HTTP != nil {
		httpConfig = pc.metadata.Settings.HTTP
	} else if pc.HTTPClient == nil {
		httpConfig = new(httpconfig.HTTPClientConfig)
	}

	if httpConfig != nil {
		httpClient, err := httpconfig.NewHTTPClientFromConfig(
			httpConfig,
			pc.ClientOptions,
		)
		if err != nil {
			return err
		}

		pc.HTTPClient = httpClient
	}

	return nil
}
