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

// Package handler defines the global proxy handler with default constructors
package handler

import (
	"errors"
	"fmt"

	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/openapiclient/handler/graphqlhandler"
	"github.com/relychan/openapitools/openapiclient/handler/proxyhandler"
	"github.com/relychan/openapitools/openapiclient/handler/resthandler"
	"go.yaml.in/yaml/v4"
)

// ErrUnsupportedProxyType occurs when the proxy type is unsupported.
var ErrUnsupportedProxyType = errors.New("unsupported proxy type")

var proxyHandlerConstructors = map[proxyhandler.ProxyActionType]proxyhandler.NewProxyHandlerFunc{
	resthandler.ProxyActionTypeREST: resthandler.NewRESTfulHandler,
	graphqlhandler.ProxyTypeGraphQL: graphqlhandler.NewGraphQLHandler,
}

// NewProxyHandler creates a proxy handler by type.
func NewProxyHandler( //nolint:ireturn,nolintlint
	operation *highv3.Operation,
	options *proxyhandler.NewProxyHandlerOptions,
) (proxyhandler.ProxyHandler, error) {
	var proxyAction rawProxyActionConfig

	var rawProxyAction *yaml.Node

	if operation.Extensions != nil {
		var exist bool

		rawProxyAction, exist = operation.Extensions.Get(oaschema.XRelyProxyAction)
		if exist && rawProxyAction != nil {
			err := rawProxyAction.Decode(&proxyAction)
			if err != nil {
				return nil, err
			}
		}
	}

	if proxyAction.Type == "" {
		proxyAction.Type = resthandler.ProxyActionTypeREST
	}

	constructor, ok := proxyHandlerConstructors[proxyAction.Type]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedProxyType, proxyAction.Type)
	}

	return constructor(operation, rawProxyAction, options)
}

// RegisterProxyHandler registers the handler to the global registry.
func RegisterProxyHandler(
	proxyType proxyhandler.ProxyActionType,
	constructor proxyhandler.NewProxyHandlerFunc,
) {
	proxyHandlerConstructors[proxyType] = constructor
}

// rawProxyActionConfig represents a raw proxy action with type only.
type rawProxyActionConfig struct {
	// Type of the proxy action.
	Type proxyhandler.ProxyActionType `json:"type" yaml:"type"`
}
