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
	"slices"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/relychan/goutils/httperror"
	"github.com/samber/lo"
)

// ValidateSchema validates and simplifies the OpenAPI schema type.
func ValidateSchema(schema *base.Schema) *httperror.HTTPError {
	// TODO
	return nil
}

// ValidateAllOf validates allOf in OpenAPI schema.
func ValidateAllOf(schemas []*base.Schema) ([]string, bool, *httperror.ValidationError) {
	if len(schemas) == 0 {
		return nil, false, nil
	}

	var (
		results  []string
		nullable bool
	)

	for _, item := range schemas {
		if item == nil {
			continue
		}

		nullable = nullable || (item.Nullable != nil && *item.Nullable)

		if len(item.Type) == 0 {
			continue
		}

		types := make([]string, 0, len(item.Type))

		for _, t := range item.Type {
			if t == "" {
				continue
			}

			if t == Null {
				nullable = true

				continue
			}

			nt, _ := NormalizeType(t)

			if !slices.Contains(types, nt) {
				types = append(types, nt)
			}
		}

		if len(results) == 0 {
			results = types

			continue
		}

		if len(types) == 0 {
			continue
		}

		// validate if types are intersected.
		intersectedValues := lo.Intersect(results, types)
		if len(intersectedValues) == 0 {
			return nil, false, &httperror.ValidationError{
				Detail: "Mixed types are not allowed in allOf schema",
			}
		}

		results = intersectedValues
	}

	return results, nullable, nil
}
