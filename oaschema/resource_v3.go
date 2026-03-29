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
	orderedmap "github.com/pb33f/ordered-map/v2"
)

func mergeOpenAPIv3Documents(dest, src *highv3.Document) {
	dest.Extensions = mergeOrderedMaps(dest.Extensions, src.Extensions)
	dest.Webhooks = mergeOrderedMaps(dest.Webhooks, src.Webhooks)
	dest.ExternalDocs = mergeExternalDocs(dest.ExternalDocs, src.ExternalDocs)

	if len(src.Security) > 0 {
		dest.Security = src.Security
	}

	if len(src.Servers) > 0 {
		dest.Servers = src.Servers
	}

	if len(src.Tags) > 0 {
		dest.Tags = src.Tags
	}

	mergeOpenAPIv3Paths(dest, src)

	if src.Components == nil {
		return
	}

	if dest.Components == nil {
		dest.Components = &highv3.Components{}
	}

	dest.Components.Callbacks = mergeOrderedMaps(
		dest.Components.Callbacks,
		src.Components.Callbacks,
	)

	dest.Components.Examples = mergeOrderedMaps(
		dest.Components.Examples,
		src.Components.Examples,
	)

	dest.Components.Extensions = mergeOrderedMaps(
		dest.Components.Extensions,
		src.Components.Extensions,
	)

	dest.Components.Headers = mergeOrderedMaps(
		dest.Components.Headers,
		src.Components.Headers,
	)

	dest.Components.Links = mergeOrderedMaps(
		dest.Components.Links,
		src.Components.Links,
	)

	dest.Components.MediaTypes = mergeOrderedMaps(
		dest.Components.MediaTypes,
		src.Components.MediaTypes,
	)

	dest.Components.Parameters = mergeOrderedMaps(
		dest.Components.Parameters,
		src.Components.Parameters,
	)

	dest.Components.PathItems = mergeOrderedMaps(
		dest.Components.PathItems,
		src.Components.PathItems,
	)

	dest.Components.RequestBodies = mergeOrderedMaps(
		dest.Components.RequestBodies,
		src.Components.RequestBodies,
	)

	dest.Components.Responses = mergeOrderedMaps(
		dest.Components.Responses,
		src.Components.Responses,
	)

	dest.Components.Schemas = mergeOrderedMaps(
		dest.Components.Schemas,
		src.Components.Schemas,
	)

	dest.Components.SecuritySchemes = mergeOrderedMaps(
		dest.Components.SecuritySchemes,
		src.Components.SecuritySchemes,
	)
}

func mergeOpenAPIv3Paths(dest, src *highv3.Document) {
	if src.Paths == nil {
		return
	}

	if dest.Paths == nil {
		dest.Paths = src.Paths

		return
	}

	dest.Paths.Extensions = mergeOrderedMaps(
		dest.Paths.Extensions,
		src.Paths.Extensions,
	)

	if src.Paths.PathItems == nil || src.Paths.PathItems.Len() == 0 {
		return
	}

	for iter := src.Paths.PathItems.Oldest(); iter != nil; iter = iter.Next() {
		if iter.Value == nil {
			continue
		}

		mergeOpenAPIv3PathItem(dest, iter)
	}
}

func mergeOpenAPIv3PathItem(
	dest *highv3.Document,
	iter *orderedmap.Pair[string, *highv3.PathItem],
) {
	pathItem, present := dest.Paths.PathItems.Get(iter.Key)
	if !present || pathItem == nil {
		dest.Paths.PathItems.Set(iter.Key, iter.Value)

		return
	}

	pathItem.AdditionalOperations = mergeOrderedMaps(
		pathItem.AdditionalOperations,
		iter.Value.AdditionalOperations,
	)

	pathItem.Extensions = mergeOrderedMaps(
		pathItem.Extensions,
		iter.Value.Extensions,
	)

	if iter.Value.Description != "" {
		pathItem.Description = iter.Value.Description
	}

	if len(iter.Value.Servers) > 0 {
		pathItem.Servers = iter.Value.Servers
	}

	pathItem.Delete = mergeOpenAPIv3Operation(
		pathItem.Delete,
		iter.Value.Delete,
	)

	pathItem.Get = mergeOpenAPIv3Operation(
		pathItem.Get,
		iter.Value.Get,
	)

	pathItem.Head = mergeOpenAPIv3Operation(
		pathItem.Head,
		iter.Value.Head,
	)

	pathItem.Options = mergeOpenAPIv3Operation(
		pathItem.Options,
		iter.Value.Options,
	)

	pathItem.Patch = mergeOpenAPIv3Operation(
		pathItem.Patch,
		iter.Value.Patch,
	)

	pathItem.Post = mergeOpenAPIv3Operation(
		pathItem.Post,
		iter.Value.Post,
	)

	pathItem.Put = mergeOpenAPIv3Operation(
		pathItem.Put,
		iter.Value.Put,
	)

	pathItem.Query = mergeOpenAPIv3Operation(
		pathItem.Query,
		iter.Value.Query,
	)

	pathItem.Trace = mergeOpenAPIv3Operation(
		pathItem.Trace,
		iter.Value.Trace,
	)

	if len(iter.Value.Parameters) > 0 {
		pathItem.Parameters = iter.Value.Parameters
	}

	if len(iter.Value.Reference) > 0 {
		pathItem.Reference = iter.Value.Reference
	}

	if iter.Value.Summary != "" {
		pathItem.Summary = iter.Value.Summary
	}
}

func mergeOpenAPIv3Operation(dest, src *highv3.Operation) *highv3.Operation {
	if src == nil {
		return dest
	}

	if dest == nil {
		return src
	}

	dest.Callbacks = mergeOrderedMaps(dest.Callbacks, src.Callbacks)
	dest.Extensions = mergeOrderedMaps(dest.Extensions, src.Extensions)
	dest.ExternalDocs = mergeExternalDocs(dest.ExternalDocs, src.ExternalDocs)

	if src.Deprecated != nil {
		dest.Deprecated = src.Deprecated
	}

	if src.Description != "" {
		dest.Description = src.Description
	}

	if src.OperationId != "" {
		dest.OperationId = src.OperationId
	}

	if len(src.Parameters) > 0 {
		dest.Parameters = src.Parameters
	}

	if len(src.Security) > 0 {
		dest.Security = src.Security
	}

	if len(src.Servers) > 0 {
		dest.Servers = src.Servers
	}

	if src.Summary != "" {
		dest.Summary = src.Summary
	}

	if len(src.Tags) > 0 {
		dest.Tags = src.Tags
	}

	dest.RequestBody = mergeOpenAPIv3RequestBody(dest.RequestBody, src.RequestBody)
	dest.Responses = mergeOpenAPIv3Responses(dest.Responses, src.Responses)

	return dest
}

func mergeOpenAPIv3RequestBody(dest, src *highv3.RequestBody) *highv3.RequestBody {
	if src == nil {
		return dest
	}

	if dest == nil {
		return src
	}

	dest.Content = mergeOrderedMaps(dest.Content, src.Content)
	dest.Extensions = mergeOrderedMaps(dest.Extensions, src.Extensions)

	if src.Description != "" {
		dest.Description = src.Description
	}

	if src.Reference != "" {
		dest.Reference = src.Reference
	}

	if src.Required != nil {
		dest.Required = src.Required
	}

	return dest
}

func mergeOpenAPIv3Responses(dest, src *highv3.Responses) *highv3.Responses {
	if src == nil {
		return dest
	}

	if dest == nil {
		return src
	}

	dest.Codes = mergeOrderedMaps(dest.Codes, src.Codes)
	dest.Extensions = mergeOrderedMaps(dest.Extensions, src.Extensions)

	if src.Default != nil {
		dest.Default = src.Default
	}

	return dest
}

func mergeExternalDocs(dest, src *base.ExternalDoc) *base.ExternalDoc {
	if src == nil {
		return dest
	}

	if dest == nil {
		return src
	}

	if src.Description != "" {
		dest.Description = src.Description
	}

	if src.URL != "" {
		dest.URL = src.URL
	}

	dest.Extensions = mergeOrderedMaps(
		dest.Extensions,
		src.Extensions,
	)

	return dest
}
