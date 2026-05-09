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

package oaschema

import (
	"github.com/pb33f/libopenapi/datamodel/high/base"
	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"go.yaml.in/yaml/v4"
)

type Operation struct {
	OperationID          string
	Parameters           []*Parameter
	RequestContentType   string
	RequestBodyMediaType *highv3.MediaType
	Responses            *highv3.Responses
	Security             []*base.SecurityRequirement
	Servers              []*highv3.Server
	Extensions           *orderedmap.Map[string, *yaml.Node]
	RequestBodyRequired  bool
}
