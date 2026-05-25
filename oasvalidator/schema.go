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

package oasvalidator

import (
	"slices"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/relychan/goutils/httperror"
	"github.com/relychan/openapitools/oaschema"
	"github.com/samber/lo"
)

func ValidateSchemaProxy(proxy *base.SchemaProxy) (*base.Schema, []httperror.ValidationError) {
	if proxy == nil {
		return nil, nil
	}

	schema := proxy.Schema()

	return schema, ValidateSchema(schema)
}

func ValidateSchema(schema *base.Schema) []httperror.ValidationError {
	if schema == nil {
		return nil
	}

	types, nullable := oaschema.GetSchemaTypes(schema)

	schema.Type = types
	schema.Nullable = &nullable

	evaluateSchemaMaximum(schema)
	evaluateSchemaMinimum(schema)

	errs := validateAllOfSchema(schema)
	if len(errs) > 0 {
		return errs
	}

	if len(schema.AnyOf) > 0 {
		unionTypes, errs := validateAllOfOrAnyOf(schema.AnyOf)
		if errs != nil {
			return errs
		}

		if len(schema.Type) > 0 {
			_, err := validateIntersectedTypes(schema.Type, unionTypes, "anyOf")
			if err != nil {
				return []httperror.ValidationError{*err}
			}
		}
	}

	if len(schema.OneOf) > 0 {
		unionTypes, errs := validateAllOfOrAnyOf(schema.OneOf)
		if errs != nil {
			return errs
		}

		if len(schema.Type) > 0 {
			_, err := validateIntersectedTypes(schema.Type, unionTypes, "oneOf")
			if err != nil {
				return []httperror.ValidationError{*err}
			}
		}
	}

	return nil
}

func validateAllOfSchema( //nolint:gocognit,gocyclo,cyclop,funlen,maintidx
	schema *base.Schema,
) []httperror.ValidationError {
	for _, aop := range schema.AllOf {
		allOf, errs := ValidateSchemaProxy(aop)
		if len(errs) > 0 {
			return errs
		}

		if allOf == nil {
			continue
		}

		intersectedTypes, err := validateIntersectedTypes(schema.Type, allOf.Type, "allOf")
		if err != nil {
			return []httperror.ValidationError{*err}
		}

		schema.Type = intersectedTypes

		if schema.Nullable == nil || !*schema.Nullable {
			schema.Nullable = allOf.Nullable
		}

		if allOf.AdditionalProperties != nil {
			schema.AdditionalProperties = mergeDynamicValue(
				schema.AdditionalProperties,
				allOf.AdditionalProperties,
			)
		}

		if allOf.Anchor != "" && schema.Anchor == "" {
			schema.Anchor = allOf.Anchor
		}

		if len(allOf.AnyOf) > 0 {
			schema.AnyOf = append(schema.AnyOf, allOf.AnyOf...)
		}

		if len(allOf.OneOf) > 0 {
			schema.OneOf = append(schema.OneOf, allOf.OneOf...)
		}

		if allOf.Const != nil && schema.Const == nil {
			schema.Const = allOf.Const
		}

		if allOf.Contains != nil {
			schema.Contains = allOf.Contains
		}

		if allOf.ContentEncoding != "" && schema.ContentEncoding == "" {
			schema.ContentEncoding = allOf.ContentEncoding
		}

		if allOf.ContentMediaType != "" && schema.ContentMediaType == "" {
			schema.ContentMediaType = allOf.ContentMediaType
		}

		if allOf.Comment != "" && schema.Comment == "" {
			schema.Comment = allOf.Comment
		}

		if allOf.Description != "" && schema.Description == "" {
			schema.Description = allOf.Description
		}

		if allOf.DynamicAnchor != "" && schema.DynamicAnchor == "" {
			schema.DynamicAnchor = allOf.DynamicAnchor
		}

		if allOf.DynamicRef != "" && schema.DynamicRef == "" {
			schema.DynamicRef = allOf.DynamicRef
		}

		if allOf.Format != "" && schema.Format == "" {
			schema.Format = allOf.Format
		}

		if allOf.Pattern != "" && schema.Pattern == "" {
			schema.Pattern = allOf.Pattern
		}

		if allOf.Title != "" && schema.Title == "" {
			schema.Title = allOf.Title
		}

		if allOf.Default != nil && schema.Default == nil {
			schema.Default = allOf.Default
		}

		if allOf.DependentRequired != nil && schema.DependentRequired == nil {
			oaschema.MergeOrderedMap(schema.DependentRequired, allOf.DependentRequired)
		}

		if allOf.DependentSchemas != nil && schema.DependentSchemas == nil {
			oaschema.MergeOrderedMap(schema.DependentSchemas, allOf.DependentSchemas)
		}

		if allOf.Discriminator != nil && schema.Discriminator == nil {
			schema.Discriminator = allOf.Discriminator
		}

		if allOf.If != nil && schema.If == nil {
			schema.If = allOf.If
			schema.Then = allOf.Then
			schema.Else = allOf.Else
		}

		if len(allOf.Enum) > 0 {
			if len(schema.Enum) == 0 {
				schema.Enum = allOf.Enum
			} else {
				schema.Enum = append(schema.Enum, allOf.Enum...)
			}
		}

		schema.Maximum = mergeSchemaMaximum(schema.Maximum, allOf.Maximum)
		schema.Minimum = mergeSchemaMinimum(schema.Minimum, allOf.Minimum)
		schema.Items = mergeDynamicValue(schema.Items, allOf.Items)

		mergeSchemaExclusiveMaximum(schema, allOf)
		mergeSchemaExclusiveMinimum(schema, allOf)

		if allOf.Example != nil && schema.Example == nil {
			schema.Example = allOf.Example
		}

		if len(allOf.Examples) > 0 {
			schema.Examples = append(schema.Examples, allOf.Examples...)
		}

		if allOf.Extensions != nil && allOf.Extensions.Len() > 0 {
			oaschema.MergeOrderedMap(schema.Extensions, allOf.Extensions)
		}

		if allOf.ExternalDocs != nil && schema.ExternalDocs == nil {
			schema.ExternalDocs = allOf.ExternalDocs
		}

		schema.MaxContains = mergeSchemaMaximum(schema.MaxContains, allOf.MaxContains)
		schema.MaxItems = mergeSchemaMaximum(schema.MaxItems, allOf.MaxItems)
		schema.MaxLength = mergeSchemaMaximum(schema.MaxLength, allOf.MaxLength)
		schema.MaxProperties = mergeSchemaMaximum(schema.MaxProperties, allOf.MaxProperties)
		schema.MinContains = mergeSchemaMinimum(schema.MinContains, allOf.MinContains)
		schema.MinItems = mergeSchemaMinimum(schema.MinItems, allOf.MinItems)
		schema.MinLength = mergeSchemaMinimum(schema.MinLength, allOf.MinLength)
		schema.MinProperties = mergeSchemaMinimum(schema.MinProperties, allOf.MinProperties)

		if allOf.MultipleOf != nil && schema.MultipleOf == nil {
			schema.MultipleOf = allOf.MultipleOf
		}

		if allOf.Not != nil && schema.Not == nil {
			schema.Not = allOf.Not
		}

		if allOf.Pattern != "" && schema.Pattern == "" {
			schema.Pattern = allOf.Pattern
		}

		oaschema.MergeOrderedMap(schema.Properties, allOf.Properties)
		oaschema.MergeOrderedMap(schema.PatternProperties, allOf.PatternProperties)

		if len(allOf.PrefixItems) > 0 && len(schema.PrefixItems) == 0 {
			schema.PrefixItems = allOf.PrefixItems
		}

		if allOf.PropertyNames != nil && schema.PropertyNames == nil {
			schema.PropertyNames = allOf.PropertyNames
		}

		if allOf.ReadOnly != nil && *allOf.ReadOnly {
			schema.ReadOnly = allOf.ReadOnly
		}

		if len(allOf.Required) > 0 {
			schema.Required = append(schema.Required, allOf.Required...)
		}

		if allOf.UnevaluatedItems != nil && schema.UnevaluatedItems == nil {
			schema.UnevaluatedItems = allOf.UnevaluatedItems
		}

		schema.UnevaluatedProperties = mergeDynamicValue(
			schema.UnevaluatedProperties,
			allOf.UnevaluatedProperties,
		)

		if allOf.UniqueItems != nil && *allOf.UniqueItems {
			schema.UniqueItems = allOf.UniqueItems
		}

		oaschema.MergeOrderedMap(schema.Vocabulary, allOf.Vocabulary)

		if allOf.WriteOnly != nil && *allOf.WriteOnly {
			schema.WriteOnly = allOf.WriteOnly
		}

		if allOf.XML != nil && schema.XML == nil {
			schema.XML = allOf.XML
		}
	}

	schema.AllOf = nil

	return nil
}

func mergeDynamicValue(
	dest, src *base.DynamicValue[*base.SchemaProxy, bool],
) *base.DynamicValue[*base.SchemaProxy, bool] {
	if src == nil {
		return dest
	}

	if dest == nil {
		return src
	}

	if src.IsA() {
		if src.A != nil && (dest.IsB() || dest.A == nil) {
			return src
		}

		return dest
	}

	if src.B && ((dest.IsB() && !dest.B) || dest.A == nil) {
		return src
	}

	return dest
}

func evaluateSchemaMaximum(schema *base.Schema) {
	if schema.ExclusiveMaximum != nil && schema.ExclusiveMaximum.IsA() &&
		!schema.ExclusiveMaximum.A {
		schema.ExclusiveMaximum = nil
	}

	if schema.Maximum == nil {
		if schema.ExclusiveMaximum != nil && schema.ExclusiveMaximum.IsA() {
			schema.ExclusiveMaximum = nil
		}

		return
	}

	if schema.ExclusiveMaximum == nil || schema.ExclusiveMaximum.IsA() {
		return
	}

	if *schema.Maximum < schema.ExclusiveMaximum.B {
		schema.ExclusiveMaximum = nil
	} else {
		schema.Maximum = nil
	}
}

func mergeSchemaMaximum[T int64 | float64](dest, src *T) *T {
	if src == nil {
		return dest
	}

	if dest == nil || *dest > *src {
		return src
	}

	return dest
}

func mergeSchemaExclusiveMaximum(dest, src *base.Schema) { //nolint:dupl
	if src.ExclusiveMaximum == nil {
		return
	}

	if dest.ExclusiveMaximum == nil {
		if src.ExclusiveMaximum.IsA() {
			if dest.Maximum != nil {
				dest.ExclusiveMaximum = src.ExclusiveMaximum
			}

			return
		}

		if dest.Maximum == nil {
			dest.ExclusiveMaximum = src.ExclusiveMaximum

			return
		}

		if *dest.Maximum < src.ExclusiveMaximum.B {
			return
		}

		dest.Maximum = nil
		dest.ExclusiveMaximum = src.ExclusiveMaximum

		return
	}

	if src.ExclusiveMaximum.IsA() {
		if dest.ExclusiveMaximum.IsA() {
			dest.ExclusiveMaximum.A = dest.ExclusiveMaximum.A || src.ExclusiveMaximum.A
		}

		return
	}

	if dest.ExclusiveMaximum.IsA() {
		if dest.Maximum == nil || *dest.Maximum >= src.ExclusiveMaximum.B {
			dest.Maximum = nil
			dest.ExclusiveMaximum = src.ExclusiveMaximum
		}

		return
	}

	if src.ExclusiveMaximum.B < dest.ExclusiveMaximum.B {
		dest.ExclusiveMaximum = src.ExclusiveMaximum
	}
}

func evaluateSchemaMinimum(schema *base.Schema) {
	if schema.ExclusiveMinimum != nil && schema.ExclusiveMinimum.IsA() &&
		!schema.ExclusiveMinimum.A {
		schema.ExclusiveMinimum = nil
	}

	if schema.Minimum == nil {
		if schema.ExclusiveMinimum != nil && schema.ExclusiveMinimum.IsA() {
			schema.ExclusiveMinimum = nil
		}

		return
	}

	if schema.ExclusiveMinimum == nil || schema.ExclusiveMinimum.IsA() {
		return
	}

	if *schema.Minimum > schema.ExclusiveMinimum.B {
		schema.ExclusiveMinimum = nil
	} else {
		schema.Minimum = nil
	}
}

func mergeSchemaMinimum[T int64 | float64](dest, src *T) *T {
	if src == nil {
		return dest
	}

	if dest == nil || *dest < *src {
		return src
	}

	return dest
}

func mergeSchemaExclusiveMinimum(dest, src *base.Schema) { //nolint:dupl
	if src.ExclusiveMinimum == nil {
		return
	}

	if dest.ExclusiveMinimum == nil {
		if src.ExclusiveMinimum.IsA() {
			if dest.Minimum != nil {
				dest.ExclusiveMinimum = src.ExclusiveMinimum
			}

			return
		}

		if dest.Minimum == nil {
			dest.ExclusiveMinimum = src.ExclusiveMinimum

			return
		}

		if *dest.Minimum > src.ExclusiveMinimum.B {
			return
		}

		dest.Minimum = nil
		dest.ExclusiveMinimum = src.ExclusiveMinimum

		return
	}

	if src.ExclusiveMinimum.IsA() {
		if dest.ExclusiveMinimum.IsA() {
			dest.ExclusiveMinimum.A = dest.ExclusiveMinimum.A || src.ExclusiveMinimum.A
		}

		return
	}

	if dest.ExclusiveMinimum.IsA() {
		if dest.Minimum == nil || *dest.Minimum <= src.ExclusiveMinimum.B {
			dest.Minimum = nil
			dest.ExclusiveMinimum = src.ExclusiveMinimum
		}

		return
	}

	if src.ExclusiveMinimum.B > dest.ExclusiveMinimum.B {
		dest.ExclusiveMinimum = src.ExclusiveMinimum
	}
}

func validateAllOfOrAnyOf(proxies []*base.SchemaProxy) ([]string, []httperror.ValidationError) {
	var unionTypes []string

	for _, aop := range proxies {
		item, errs := ValidateSchemaProxy(aop)
		if len(errs) > 0 {
			return nil, errs
		}

		if item == nil {
			continue
		}

		if len(item.Type) > 0 {
			if len(unionTypes) == 0 {
				unionTypes = item.Type
			} else {
				for _, t := range item.Type {
					if !slices.Contains(unionTypes, t) {
						unionTypes = append(unionTypes, t)
					}
				}
			}
		}
	}

	return unionTypes, nil
}

func validateIntersectedTypes(
	types, srcTypes []string,
	errorType string,
) ([]string, *httperror.ValidationError) {
	if len(srcTypes) == 0 {
		return types, nil
	}

	if len(types) == 0 {
		return srcTypes, nil
	}

	results := lo.Intersect(srcTypes, types)
	if len(results) == 0 {
		return nil, UnionTypeMismatchedError(errorType, types, srcTypes)
	}

	return results, nil
}
