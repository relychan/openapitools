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
	"fmt"

	"github.com/hasura/goenvconf"
	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/relychan/gohttpc"
	"github.com/relychan/openapitools/openapiclient/handler/proxyhandler"
	"github.com/relychan/openapitools/openapiclient/internal"
)

// BuildMetadataTree builds the metadata tree from the API document.
func BuildMetadataTree(
	document *highv3.Document,
	clientOptions *gohttpc.ClientOptions,
) (*internal.Node, error) {
	rootNode := new(internal.Node)

	if document.Paths.PathItems == nil {
		return rootNode, nil
	}

	options := &proxyhandler.InsertRouteOptions{
		GetEnv: goenvconf.GetOSEnv,
	}

	if clientOptions != nil && clientOptions.GetEnv != nil {
		options.GetEnv = clientOptions.GetEnv
	}

	for pathItem := document.Paths.PathItems.Oldest(); pathItem != nil; pathItem = pathItem.Next() {
		_, err := rootNode.InsertRoute(pathItem.Key, pathItem.Value, options)
		if err != nil {
			return nil, fmt.Errorf("failed to insert route %s: %w", pathItem.Key, err)
		}
	}

	return rootNode, nil
}
