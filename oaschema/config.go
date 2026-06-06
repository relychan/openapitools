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

// Package oaschema defines schemas for Open API resources.
package oaschema

import (
	"github.com/hasura/goenvconf"
	"github.com/relychan/gohttpc/httpconfig"
	"github.com/relychan/gohttpc/loadbalancer"
)

// OpenAPIResourceSettings hold settings of the rely proxy.
type OpenAPIResourceSettings struct {
	// Base path of the resource.
	BasePath string `json:"basePath,omitempty" yaml:"basePath,omitempty"`
	// Global settings for the HTTP client.
	HTTP *httpconfig.HTTPClientConfig `json:"http,omitempty" yaml:"http,omitempty"`
	// Headers define custom headers to be injected to the remote server.
	// Merged with the global headers.
	Headers map[string]goenvconf.EnvString `json:"headers,omitempty" yaml:"headers,omitempty"`
	// ForwardHeaders define configurations for headers forwarding
	ForwardHeaders *OpenAPIForwardHeadersConfig `json:"forwardHeaders,omitempty" yaml:"forwardHeaders,omitempty"`
	// HealthCheck define the health check policy for load balancer recovery.
	HealthCheck *HealthCheckConfig `json:"healthCheck,omitempty" yaml:"healthCheck,omitempty"`
}

// OpenAPIForwardHeadersConfig contains configurations for headers forwarding,.
type OpenAPIForwardHeadersConfig struct {
	// Defines header names to be forwarded from the client request.
	Request []string `json:"request,omitempty" yaml:"request,omitempty"`
	// Defines header names to be forwarded from the response.
	Response []string `json:"response,omitempty" yaml:"response,omitempty"`
}

// HealthCheckConfig holds health check configurations for server recovery.
type HealthCheckConfig struct {
	// Configurations for health check through HTTP protocol.
	HTTP *loadbalancer.HTTPHealthCheckConfig `json:"http,omitempty" yaml:"http,omitempty"`
}
