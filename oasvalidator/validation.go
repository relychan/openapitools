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
	"cmp"
	"math"
	"slices"
	"strconv"
	"time"

	"github.com/dlclark/regexp2"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/relychan/goutils"
	"github.com/relychan/openapitools/oaschema"
	"go.yaml.in/yaml/v4"
)

// ValidateContains validates the contains rule against an array value.
func ValidateContains(typeSchema *base.Schema, value []any) ErrorFunc {
	if typeSchema == nil || typeSchema.Contains == nil ||
		((typeSchema.MinContains == nil || *typeSchema.MinContains <= 0) &&
			(typeSchema.MaxContains == nil)) {
		return nil
	}

	schemaContains := typeSchema.Contains.Schema()
	if schemaContains == nil {
		return nil
	}

	var containsCount int64

	for _, item := range value {
		errs := ValidateValue(schemaContains, item)
		if len(errs) == 0 {
			containsCount++
		}
	}

	if typeSchema.MinContains != nil && containsCount < *typeSchema.MinContains {
		return MinContainsErrorFunc(*typeSchema.MinContains, containsCount)
	}

	if typeSchema.MaxContains != nil && containsCount > *typeSchema.MaxContains {
		return MaxContainsErrorFunc(*typeSchema.MinContains, containsCount)
	}

	return nil
}

// ValidateValue validates a value against an OpenAPI schema.
func ValidateValue( //nolint:gocyclo,cyclop,funlen
	typeSchema *base.Schema,
	value any,
) []ErrorFunc {
	switch val := value.(type) {
	case bool:
		return ValidateBoolean(typeSchema, val)
	case *bool:
		return ValidateNullableBoolean(typeSchema, val)
	case []byte:
		if len(typeSchema.Type) > 0 && !slices.Contains(typeSchema.Type, oaschema.String) {
			return []ErrorFunc{InvalidTypeErrorFunc(typeSchema.Type, oaschema.String)}
		}

		if (typeSchema.Nullable == nil || !*typeSchema.Nullable) && val == nil {
			return []ErrorFunc{NotNullError}
		}

		return nil
	case string:
		return ValidateString(typeSchema, val)
	case *string:
		return ValidateNullableString(typeSchema, val)
	case int:
		return ValidateInteger(typeSchema, val)
	case int8:
		return ValidateInteger(typeSchema, val)
	case int16:
		return ValidateInteger(typeSchema, val)
	case int32:
		return ValidateInteger(typeSchema, val)
	case *int:
		return ValidateNullableInteger(typeSchema, val)
	case *int8:
		return ValidateNullableInteger(typeSchema, val)
	case *int16:
		return ValidateNullableInteger(typeSchema, val)
	case *int32:
		return ValidateNullableInteger(typeSchema, val)
	case *uint:
		return ValidateNullableInteger(typeSchema, val)
	case *uint8:
		return ValidateNullableInteger(typeSchema, val)
	case *uint16:
		return ValidateNullableInteger(typeSchema, val)
	case int64:
		return ValidateInteger(typeSchema, val)
	case uint32:
		return ValidateInteger(typeSchema, val)
	case uint64:
		return ValidateInteger(typeSchema, val)
	case *int64:
		return ValidateNullableInteger(typeSchema, val)
	case *uint32:
		return ValidateNullableInteger(typeSchema, val)
	case *uint64:
		return ValidateNullableInteger(typeSchema, val)
	case float32:
		return ValidateNumber(typeSchema, val)
	case *float32:
		return ValidateNullableNumber(typeSchema, val)
	case float64:
		return ValidateNumber(typeSchema, val)
	case *float64:
		return ValidateNullableNumber(typeSchema, val)
	case time.Time:
		return ValidateString(typeSchema, "")
	case *time.Time:
		if val == nil {
			return ValidateNullableString(typeSchema, nil)
		}

		return ValidateNullableString(typeSchema, new(""))
	case []bool:
		return ValidateArray(typeSchema, val, CompareBoolean)
	case []*bool:
		return ValidateArray(typeSchema, val, CompareNullableBoolean)
	case []string:
		return ValidateArray(typeSchema, val, cmp.Compare)
	case []*string:
		return ValidateArray(typeSchema, val, CompareNullable)
	case []int:
		return ValidateArray(typeSchema, val, cmp.Compare)
	case []int8:
		return ValidateArray(typeSchema, val, cmp.Compare)
	case []int16:
		return ValidateArray(typeSchema, val, cmp.Compare)
	case []int32:
		return ValidateArray(typeSchema, val, cmp.Compare)
	case []uint:
		return ValidateArray(typeSchema, val, cmp.Compare)
	case []uint16:
		return ValidateArray(typeSchema, val, cmp.Compare)
	case []*int:
		return ValidateArray(typeSchema, val, CompareNullable)
	case []*int8:
		return ValidateArray(typeSchema, val, CompareNullable)
	case []*int16:
		return ValidateArray(typeSchema, val, CompareNullable)
	case []*int32:
		return ValidateArray(typeSchema, val, CompareNullable)
	case []*uint:
		return ValidateArray(typeSchema, val, CompareNullable)
	case []*uint8:
		return ValidateArray(typeSchema, val, CompareNullable)
	case []*uint16:
		return ValidateArray(typeSchema, val, CompareNullable)
	case []int64:
		return ValidateArray(typeSchema, val, cmp.Compare)
	case []uint32:
		return ValidateArray(typeSchema, val, cmp.Compare)
	case []uint64:
		return ValidateArray(typeSchema, val, cmp.Compare)
	case []*int64:
		return ValidateArray(typeSchema, val, CompareNullable)
	case []*uint32:
		return ValidateArray(typeSchema, val, CompareNullable)
	case []*uint64:
		return ValidateArray(typeSchema, val, CompareNullable)
	case []float32:
		return ValidateArray(typeSchema, val, cmp.Compare)
	case []*float32:
		return ValidateArray(typeSchema, val, CompareNullable)
	case []float64:
		return ValidateArray(typeSchema, val, cmp.Compare)
	case []*float64:
		return ValidateArray(typeSchema, val, CompareNullable)
	case []any:
		return ValidateArray(typeSchema, val, nil)
	case map[string]any:
		return ValidateObject(typeSchema, val)
	default:
		// TODO: reflection
		return nil
	}
}

// ValidateBoolean validates a boolean value against an OpenAPI schema.
func ValidateBoolean(typeSchema *base.Schema, value bool) []ErrorFunc {
	if len(typeSchema.Type) > 0 && !slices.Contains(typeSchema.Type, oaschema.Boolean) {
		return []ErrorFunc{InvalidTypeErrorFunc(typeSchema.Type, oaschema.Boolean)}
	}

	if !ValidateEnum(typeSchema, value) {
		return []ErrorFunc{
			EnumValidationErrorFunc(typeSchema, strconv.FormatBool(value)),
		}
	}

	return nil
}

// ValidateNullableBoolean validates a nullable boolean value against an OpenAPI schema.
func ValidateNullableBoolean(typeSchema *base.Schema, value *bool) []ErrorFunc {
	if !CanNull(typeSchema, value == nil) {
		return []ErrorFunc{NotNullError}
	}

	return ValidateBoolean(typeSchema, *value)
}

// ValidateString validates a string value against an OpenAPI schema.
func ValidateString(typeSchema *base.Schema, value string) []ErrorFunc {
	if len(typeSchema.Type) > 0 && !slices.Contains(typeSchema.Type, oaschema.String) {
		return []ErrorFunc{InvalidTypeErrorFunc(typeSchema.Type, oaschema.String)}
	}

	if !ValidateEnum(typeSchema, value) {
		return []ErrorFunc{
			EnumValidationErrorFunc(typeSchema, value),
		}
	}

	valueLength := int64(len(value))

	var errs []ErrorFunc

	if typeSchema.MaxLength != nil && valueLength > *typeSchema.MaxLength {
		errs = append(errs, MaxLengthValidationErrorFunc(*typeSchema.MaxLength, valueLength))
	} else if typeSchema.MinLength != nil && valueLength < *typeSchema.MinLength {
		errs = append(errs, MinLengthValidationErrorFunc(*typeSchema.MinLength, valueLength))
	}

	if typeSchema.Pattern == "" {
		return errs
	}

	pattern, err := regexp2.Compile(typeSchema.Pattern, regexp2.None)
	// ignore compile error on runtime.
	if err != nil {
		return errs
	}

	matched, err := pattern.MatchString(value)
	if err != nil {
		errs = append(errs, func() *goutils.ErrorDetail {
			return &goutils.ErrorDetail{
				Code:   ErrCodeValidationError,
				Detail: "Failed to validate string value against regular expression: " + err.Error(),
			}
		})
	} else if !matched {
		errs = append(errs, PatternValidationErrorFunc(typeSchema.Pattern))
	}

	return errs
}

// ValidateNullableString validates a nullable string value against an OpenAPI schema.
func ValidateNullableString(typeSchema *base.Schema, value *string) []ErrorFunc {
	if !CanNull(typeSchema, value == nil) {
		return []ErrorFunc{NotNullError}
	}

	return ValidateString(typeSchema, *value)
}

// ValidateNumber validates a number value against an OpenAPI schema.
func ValidateNumber[T float32 | float64](typeSchema *base.Schema, value T) []ErrorFunc {
	if len(typeSchema.Type) > 0 && !slices.Contains(typeSchema.Type, oaschema.Number) {
		return []ErrorFunc{InvalidTypeErrorFunc(typeSchema.Type, oaschema.Number)}
	}

	val := float64(value)

	if !ValidateEnum(typeSchema, value) {
		return []ErrorFunc{
			EnumValidationErrorFunc(
				typeSchema,
				strconv.FormatFloat(val, 'f', -1, 64),
			),
		}
	}

	return validateNumberRules(typeSchema, val)
}

// ValidateNullableNumber validates a nullable number value against an OpenAPI schema.
func ValidateNullableNumber[T float32 | float64](typeSchema *base.Schema, value *T) []ErrorFunc {
	if !CanNull(typeSchema, value == nil) {
		return []ErrorFunc{NotNullError}
	}

	return ValidateNumber(typeSchema, *value)
}

// ValidateInteger validates a number value against an OpenAPI schema.
func ValidateInteger[T int | int8 | int16 | int32 | int64 | uint | uint8 | uint16 | uint32 | uint64](
	typeSchema *base.Schema,
	value T,
) []ErrorFunc {
	if len(typeSchema.Type) > 0 && !slices.Contains(typeSchema.Type, oaschema.Integer) {
		return []ErrorFunc{InvalidTypeErrorFunc(typeSchema.Type, oaschema.Integer)}
	}

	if !ValidateEnum(typeSchema, value) {
		return []ErrorFunc{
			EnumValidationErrorFunc(
				typeSchema,
				strconv.FormatInt(int64(value), 10),
			),
		}
	}

	return validateNumberRules(typeSchema, float64(value))
}

// ValidateNullableInteger validates a nullable integer value against an OpenAPI schema.
func ValidateNullableInteger[T int | int8 | int16 | int32 | int64 | uint | uint8 | uint16 | uint32 | uint64](
	typeSchema *base.Schema,
	value *T,
) []ErrorFunc {
	if !CanNull(typeSchema, value == nil) {
		return []ErrorFunc{NotNullError}
	}

	return ValidateInteger(typeSchema, *value)
}

// ValidateArray validates an array value against an OpenAPI schema.
func ValidateArray[T any](
	typeSchema *base.Schema,
	value []T,
	compare func(a T, b T) int,
) []ErrorFunc {
	if !CanNull(typeSchema, value == nil) {
		return []ErrorFunc{NotNullError}
	}

	if len(typeSchema.Type) > 0 && !slices.Contains(typeSchema.Type, oaschema.Array) {
		return []ErrorFunc{InvalidTypeErrorFunc(typeSchema.Type, oaschema.Array)}
	}

	valueLength := int64(len(value))

	var errs []ErrorFunc

	// array length validations
	if typeSchema.MaxItems != nil && valueLength > *typeSchema.MaxItems {
		errs = append(errs, ArrayMaxItemsValidationErrorFunc(*typeSchema.MaxItems, valueLength))
	} else if typeSchema.MinItems != nil && valueLength < *typeSchema.MinItems {
		errs = append(errs, ArrayMinItemsValidationErrorFunc(*typeSchema.MinItems, valueLength))
	}

	if compare != nil && typeSchema.UniqueItems != nil && *typeSchema.UniqueItems {
		duplicatedItems := FindDuplicatedItemsFunc(value, compare)
		if len(duplicatedItems) > 0 {
			errs = append(errs, ArrayUniqueItemsValidationErrorFunc(duplicatedItems))
		}
	}

	if valueLength == 0 || typeSchema.Items.A == nil {
		return errs
	}

	itemSchema := typeSchema.Items.A.Schema()
	if oaschema.IsSchemaEmpty(itemSchema) {
		return errs
	}

	for i, item := range value {
		itemErrors := ValidateValue(itemSchema, item)
		if len(itemErrors) > 0 {
			errs = slices.Grow(errs, len(itemErrors))

			for j := range itemErrors {
				itemError := itemErrors[j]

				errs = append(errs, func() *goutils.ErrorDetail {
					result := itemError()
					if result.Pointer == "" {
						result.Pointer = "/" + strconv.Itoa(i)
					} else {
						result.Pointer = "/" + strconv.Itoa(i) + result.Pointer
					}

					return result
				})
			}
		}
	}

	return errs
}

// ValidateObject validates an object value against an OpenAPI schema.
func ValidateObject[T any](typeSchema *base.Schema, value map[string]T) []ErrorFunc {
	if !CanNull(typeSchema, value == nil) {
		return []ErrorFunc{NotNullError}
	}

	if len(typeSchema.Type) > 0 && !slices.Contains(typeSchema.Type, oaschema.Object) {
		return []ErrorFunc{InvalidTypeErrorFunc(typeSchema.Type, oaschema.Object)}
	}

	propertiesLength := int64(len(value))

	var errs []ErrorFunc

	if typeSchema.MaxProperties != nil && *typeSchema.MaxProperties < propertiesLength {
		errs = append(errs, ObjectMaxPropertiesValidationErrorFunc(*typeSchema.MaxProperties, propertiesLength))
	} else if typeSchema.MinProperties != nil && *typeSchema.MinProperties > propertiesLength {
		errs = append(errs, ObjectMinPropertiesValidationErrorFunc(*typeSchema.MinProperties, propertiesLength))
	}

	for _, requiredKey := range typeSchema.Required {
		_, present := value[requiredKey]
		if !present {
			errs = append(errs, ObjectRequiredPropertyErrorFunc(requiredKey))
		}
	}

	if typeSchema.DependentRequired != nil {
		for iter := typeSchema.DependentRequired.First(); iter != nil; iter = iter.Next() {
			key := iter.Key()
			dependents := iter.Value()

			for _, dependent := range dependents {
				_, present := value[dependent]
				if !present {
					errs = append(errs, ObjectDependentRequiredErrorFunc(key, dependent))
				}
			}
		}
	}

	return errs
}

// CanNull checks if a nullable value is allowed by an OpenAPI schema.
func CanNull(typeSchema *base.Schema, isNull bool) bool {
	if !isNull || (typeSchema.Nullable != nil && *typeSchema.Nullable) {
		return true
	}

	if slices.Contains(typeSchema.Type, oaschema.Null) {
		return true
	}

	if len(typeSchema.Enum) > 0 && slices.ContainsFunc(typeSchema.Enum, func(enum *yaml.Node) bool {
		return enum == nil || enum.Tag == goutils.YAMLNullTag
	}) {
		return true
	}

	return false
}

// ValidateEnum validates a value against a list of enum.
func ValidateEnum[T comparable](typeSchema *base.Schema, value T) bool {
	if typeSchema == nil {
		return true
	}

	enums := typeSchema.Enum

	if typeSchema.Const != nil {
		enums = []*yaml.Node{typeSchema.Const}
	}

	if len(enums) == 0 {
		return true
	}

	str, ok := any(value).(string)
	if ok {
		return slices.ContainsFunc(enums, func(enum *yaml.Node) bool {
			return enum != nil && enum.Value == str
		})
	}

	return slices.ContainsFunc(enums, func(enum *yaml.Node) bool {
		if enum == nil {
			return false
		}

		var comparedValue T

		err := enum.Load(&comparedValue)
		if err != nil {
			return false
		}

		return comparedValue == value
	})
}

func validateNumberRules(typeSchema *base.Schema, value float64) []ErrorFunc { //nolint:cyclop
	var errs []ErrorFunc

	if typeSchema.MultipleOf != nil && *typeSchema.MultipleOf > 0 &&
		value != 0 &&
		math.Mod(value, *typeSchema.MultipleOf) != 0 {
		errs = append(errs, MultipleOfValidationErrorFunc(*typeSchema.MultipleOf, value))
	}

	switch {
	case typeSchema.ExclusiveMaximum != nil &&
		typeSchema.ExclusiveMaximum.N == 1 &&
		value >= typeSchema.ExclusiveMaximum.B:
		errs = append(errs, MaximumValidationErrorFunc(typeSchema.ExclusiveMaximum.B, value, true))
	case typeSchema.ExclusiveMaximum != nil &&
		typeSchema.ExclusiveMaximum.A &&
		typeSchema.Maximum != nil &&
		value >= *typeSchema.Maximum:
		errs = append(errs, MaximumValidationErrorFunc(*typeSchema.Maximum, value, true))
	case typeSchema.Maximum != nil && value > *typeSchema.Maximum:
		errs = append(errs, MaximumValidationErrorFunc(*typeSchema.Maximum, value, false))
	case typeSchema.ExclusiveMinimum != nil &&
		typeSchema.ExclusiveMinimum.N == 1 &&
		value <= typeSchema.ExclusiveMinimum.B:
		errs = append(errs, MinimumValidationErrorFunc(typeSchema.ExclusiveMaximum.B, value, true))
	case typeSchema.ExclusiveMinimum != nil &&
		typeSchema.ExclusiveMinimum.A &&
		typeSchema.Minimum != nil &&
		value <= *typeSchema.Minimum:
		errs = append(errs, MinimumValidationErrorFunc(*typeSchema.Minimum, value, true))
	case typeSchema.Minimum != nil && value < *typeSchema.Minimum:
		errs = append(errs, MinimumValidationErrorFunc(*typeSchema.Minimum, value, false))
	default:
	}

	return errs
}
