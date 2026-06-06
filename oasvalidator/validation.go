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
	"log/slog"
	"math"
	"reflect"
	"slices"
	"strconv"
	"time"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/relychan/goutils"
	"github.com/relychan/goutils/httperror"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/oasvalidator/regexps"
	"go.yaml.in/yaml/v4"
)

// ValidateContains validates the contains rule against an array value.
func ValidateContains(typeSchema *base.Schema, value []any) *httperror.ValidationError {
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
		return MinContainsError(*typeSchema.MinContains, containsCount)
	}

	if typeSchema.MaxContains != nil && containsCount > *typeSchema.MaxContains {
		return MaxContainsError(*typeSchema.MinContains, containsCount)
	}

	return nil
}

// ValidateValueWithSchemaProxy validates a value against an OpenAPI schema proxy.
func ValidateValueWithSchemaProxy(
	schemaProxy *base.SchemaProxy,
	value any,
) []httperror.ValidationError {
	if schemaProxy == nil {
		return nil
	}

	schema := schemaProxy.Schema()
	if schema == nil {
		return nil
	}

	return ValidateValue(schema, value)
}

// ValidateValue validates a value against an OpenAPI schema.
func ValidateValue(typeSchema *base.Schema, value any) []httperror.ValidationError {
	validationErrors := validateValueOnly(typeSchema, value)

	for i, ao := range typeSchema.AllOf {
		errs := ValidateValueWithSchemaProxy(ao, value)
		if len(errs) > 0 {
			validationErrors = addErrorHints(validationErrors, errs, "/allOf/"+strconv.Itoa(i))
		}
	}

	if len(typeSchema.AnyOf) > 0 {
		var (
			aoErrors     []httperror.ValidationError
			isSuccessful bool
		)

		for i, ao := range typeSchema.AnyOf {
			errs := ValidateValueWithSchemaProxy(ao, value)
			if len(errs) > 0 {
				aoErrors = addErrorHints(aoErrors, errs, "/anyOf/"+strconv.Itoa(i))
			} else {
				isSuccessful = true
			}
		}

		if !isSuccessful {
			validationErrors = append(validationErrors, aoErrors...)
		}
	}

	if len(typeSchema.OneOf) > 0 {
		var (
			ooErrors     []httperror.ValidationError
			isSuccessful bool
		)

		for i, oo := range typeSchema.OneOf {
			errs := ValidateValueWithSchemaProxy(oo, value)
			if len(errs) > 0 {
				ooErrors = addErrorHints(ooErrors, errs, "/oneOf/"+strconv.Itoa(i))

				continue
			}

			isSuccessful = true

			break
		}

		if !isSuccessful {
			validationErrors = append(validationErrors, ooErrors...)
		}
	}

	return validationErrors
}

// ValidateBoolean validates a boolean value against an OpenAPI schema.
func ValidateBoolean(typeSchema *base.Schema, value bool) []httperror.ValidationError {
	if len(typeSchema.Type) > 0 && !slices.Contains(typeSchema.Type, oaschema.Boolean) {
		return []httperror.ValidationError{*InvalidTypeError(typeSchema.Type, oaschema.Boolean)}
	}

	if !ValidateEnum(typeSchema, value) {
		return []httperror.ValidationError{
			*EnumValidationError(typeSchema, strconv.FormatBool(value)),
		}
	}

	return nil
}

// ValidateNullableBoolean validates a nullable boolean value against an OpenAPI schema.
func ValidateNullableBoolean(typeSchema *base.Schema, value *bool) []httperror.ValidationError {
	if !CanNull(typeSchema, value == nil) {
		return []httperror.ValidationError{*NotNullError()}
	}

	return ValidateBoolean(typeSchema, *value)
}

// ValidateString validates a string value against an OpenAPI schema.
func ValidateString(typeSchema *base.Schema, value string) []httperror.ValidationError {
	if len(typeSchema.Type) > 0 && !slices.Contains(typeSchema.Type, oaschema.String) {
		return []httperror.ValidationError{*InvalidTypeError(typeSchema.Type, oaschema.String)}
	}

	formatErr := validateFormat(value, typeSchema.Format)
	if formatErr != nil {
		return []httperror.ValidationError{*formatErr}
	}

	if !ValidateEnum(typeSchema, value) {
		return []httperror.ValidationError{
			*EnumValidationError(typeSchema, value),
		}
	}

	valueLength := int64(len(value))

	var errs []httperror.ValidationError

	if typeSchema.MaxLength != nil && valueLength > *typeSchema.MaxLength {
		errs = append(errs, *MaxLengthValidationError(*typeSchema.MaxLength, valueLength))
	} else if typeSchema.MinLength != nil && valueLength < *typeSchema.MinLength {
		errs = append(errs, *MinLengthValidationError(*typeSchema.MinLength, valueLength))
	}

	alError := validateArrayLength(typeSchema, valueLength)
	if alError != nil {
		errs = append(errs, *alError)
	}

	if typeSchema.Pattern == "" {
		return errs
	}

	pattern, err := regexps.Get(typeSchema.Pattern)
	// ignore compile error on runtime.
	if err != nil {
		return errs
	}

	matched, err := pattern.MatchString(value)
	if err != nil {
		errs = append(errs, httperror.ValidationError{
			Code:   ErrCodeValidationError,
			Detail: "Failed to validate string value against regular expression: " + err.Error(),
		})
	} else if !matched {
		errs = append(errs, *PatternValidationError(typeSchema.Pattern))
	}

	return errs
}

// ValidateNullableString validates a nullable string value against an OpenAPI schema.
func ValidateNullableString(typeSchema *base.Schema, value *string) []httperror.ValidationError {
	if !CanNull(typeSchema, value == nil) {
		return []httperror.ValidationError{*NotNullError()}
	}

	return ValidateString(typeSchema, *value)
}

// ValidateNumber validates a number value against an OpenAPI schema.
func ValidateNumber[T float32 | float64](
	typeSchema *base.Schema,
	value T,
) []httperror.ValidationError {
	if len(typeSchema.Type) > 0 && !slices.Contains(typeSchema.Type, oaschema.Number) {
		return []httperror.ValidationError{*InvalidTypeError(typeSchema.Type, oaschema.Number)}
	}

	val := float64(value)

	if !ValidateEnum(typeSchema, value) {
		return []httperror.ValidationError{
			*EnumValidationError(
				typeSchema,
				strconv.FormatFloat(val, 'f', -1, 64),
			),
		}
	}

	return validateNumberRules(typeSchema, val)
}

// ValidateNullableNumber validates a nullable number value against an OpenAPI schema.
func ValidateNullableNumber[T float32 | float64](
	typeSchema *base.Schema,
	value *T,
) []httperror.ValidationError {
	if !CanNull(typeSchema, value == nil) {
		return []httperror.ValidationError{*NotNullError()}
	}

	return ValidateNumber(typeSchema, *value)
}

// ValidateInteger validates a number value against an OpenAPI schema.
func ValidateInteger[T int | int8 | int16 | int32 | int64 | uint | uint8 | uint16 | uint32 | uint64](
	typeSchema *base.Schema,
	value T,
) []httperror.ValidationError {
	if len(typeSchema.Type) > 0 && !slices.Contains(typeSchema.Type, oaschema.Integer) {
		return []httperror.ValidationError{*InvalidTypeError(typeSchema.Type, oaschema.Integer)}
	}

	if !ValidateEnum(typeSchema, value) {
		return []httperror.ValidationError{
			*EnumValidationError(
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
) []httperror.ValidationError {
	if !CanNull(typeSchema, value == nil) {
		return []httperror.ValidationError{*NotNullError()}
	}

	return ValidateInteger(typeSchema, *value)
}

// ValidateArray validates an array value against an OpenAPI schema.
func ValidateArray[T any](
	typeSchema *base.Schema,
	value []T,
	compare func(a T, b T) int,
) []httperror.ValidationError {
	if !CanNull(typeSchema, value == nil) {
		return []httperror.ValidationError{*NotNullError()}
	}

	valueLength := int64(len(value))

	var errs []httperror.ValidationError

	// array length validations
	if typeSchema.MaxItems != nil && valueLength > *typeSchema.MaxItems {
		errs = append(errs, *ArrayMaxItemsValidationError(*typeSchema.MaxItems, valueLength))
	} else if typeSchema.MinItems != nil && valueLength < *typeSchema.MinItems {
		errs = append(errs, *ArrayMinItemsValidationError(*typeSchema.MinItems, valueLength))
	}

	if compare != nil && typeSchema.UniqueItems != nil && *typeSchema.UniqueItems {
		duplicatedItems := FindDuplicatedItemsFunc(value, compare)
		if len(duplicatedItems) > 0 {
			errs = append(errs, *ArrayUniqueItemsValidationError(duplicatedItems))
		}
	}

	return errs
}

// ValidateArrayAndItems validates an array value and its items against an OpenAPI schema.
func ValidateArrayAndItems[T any](
	typeSchema *base.Schema,
	value []T,
	compare func(a T, b T) int,
) []httperror.ValidationError {
	if len(typeSchema.Type) > 0 && !slices.Contains(typeSchema.Type, oaschema.Array) {
		return []httperror.ValidationError{*InvalidTypeError(typeSchema.Type, oaschema.Array)}
	}

	errs := ValidateArray(typeSchema, value, compare)

	if len(value) == 0 || typeSchema.Items == nil ||
		(typeSchema.Items.IsB() || typeSchema.Items.A == nil) {
		return errs
	}

	itemSchema := typeSchema.Items.A.Schema()
	if oaschema.IsSchemaTypeEmpty(itemSchema) {
		return errs
	}

	for i, item := range value {
		itemErrors := ValidateValue(itemSchema, item)
		if len(itemErrors) > 0 {
			errs = slices.Grow(errs, len(itemErrors))

			for j := range itemErrors {
				itemError := itemErrors[j]
				itemError.PrependPointer("/" + strconv.Itoa(i))

				errs = append(errs, itemError)
			}

			// early stop the loop if there are too many errors.
			if len(errs) >= 10 {
				return errs
			}
		}
	}

	return errs
}

// ValidateObject validates an object value against an OpenAPI schema.
func ValidateObject[T any]( //nolint:gocognit,gocyclo,cyclop,funlen
	typeSchema *base.Schema,
	value map[string]T,
) []httperror.ValidationError {
	errs := ValidateObjectWithoutProperties(typeSchema, value)

	validateProperty := func(schema *base.Schema, prop T, key string) {
		es := ValidateValue(schema, prop)
		if len(es) > 0 {
			errs = slices.Grow(errs, len(es))

			for _, err := range es {
				err.PrependPointer("/" + key)
				errs = append(errs, err)
			}
		}
	}

	if typeSchema.Properties != nil && typeSchema.Properties.Len() > 0 {
		for iter := typeSchema.Properties.First(); iter != nil; iter = iter.Next() {
			key := iter.Key()

			schemaProxy := iter.Value()
			if schemaProxy == nil {
				continue
			}

			propSchema := schemaProxy.Schema()
			if propSchema == nil {
				continue
			}

			validateProperty(propSchema, value[key], key)
		}
	}

	if (typeSchema.AdditionalProperties == nil ||
		(typeSchema.AdditionalProperties.IsB() && !typeSchema.AdditionalProperties.B)) &&
		(typeSchema.PatternProperties == nil || typeSchema.PatternProperties.Len() == 0) {
		return errs
	}

	if typeSchema.AdditionalProperties != nil && typeSchema.AdditionalProperties.IsA() &&
		typeSchema.AdditionalProperties.A != nil {
		propSchema := typeSchema.AdditionalProperties.A.Schema()
		if propSchema != nil {
			for key, prop := range value {
				if typeSchema.Properties != nil {
					_, present := typeSchema.Properties.Get(key)
					if present {
						continue
					}
				}

				validateProperty(propSchema, prop, key)
			}
		}
	}

	if typeSchema.PatternProperties != nil && typeSchema.PatternProperties.Len() > 0 {
		for iter := typeSchema.PatternProperties.First(); iter != nil; iter = iter.Next() {
			rawPattern := iter.Key()

			pattern, err := regexps.Get(rawPattern)
			if err != nil {
				// ignore compile error on runtime.
				slog.Warn(
					"failed to compile regular expression: "+err.Error(),
					slog.String("pattern", rawPattern),
				)

				continue
			}

			schemaProxy := iter.Value()
			if schemaProxy == nil {
				continue
			}

			propSchema := schemaProxy.Schema()
			if propSchema == nil {
				continue
			}

			for key, prop := range value {
				if typeSchema.Properties != nil {
					_, present := typeSchema.Properties.Get(key)
					if present {
						continue
					}
				}

				matched, err := pattern.MatchString(key)
				if err != nil {
					slog.Warn(
						"failed to compile pattern property: "+err.Error(),
						slog.String("pattern", key),
						slog.String("name", key),
					)

					continue
				}

				if !matched {
					continue
				}

				validateProperty(propSchema, prop, key)
			}
		}
	}

	return errs
}

// ValidateObjectWithoutProperties validates an object value against an OpenAPI schema.
// This function only validates type, properties length and required properties.
func ValidateObjectWithoutProperties[T any](
	typeSchema *base.Schema,
	value map[string]T,
) []httperror.ValidationError {
	if !CanNull(typeSchema, value == nil) {
		return []httperror.ValidationError{*NotNullError()}
	}

	if len(typeSchema.Type) > 0 && !slices.Contains(typeSchema.Type, oaschema.Object) {
		return []httperror.ValidationError{*InvalidTypeError(typeSchema.Type, oaschema.Object)}
	}

	propertiesLength := int64(len(value))

	var errs []httperror.ValidationError

	propLenErr := validateObjectPropertiesLength(typeSchema, propertiesLength)
	if propLenErr != nil {
		errs = []httperror.ValidationError{*propLenErr}
	}

	for _, requiredKey := range typeSchema.Required {
		_, present := value[requiredKey]
		if !present {
			errs = append(errs, *ObjectRequiredPropertyError(requiredKey))
		}
	}

	if typeSchema.DependentRequired != nil {
		for iter := typeSchema.DependentRequired.First(); iter != nil; iter = iter.Next() {
			key := iter.Key()
			dependents := iter.Value()

			for _, dependent := range dependents {
				_, present := value[dependent]
				if !present {
					errs = append(errs, *ObjectDependentRequiredError(key, dependent))
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

// validateNumberRules checks multipleOf, maximum, and minimum constraints against a float64 value.
func validateNumberRules( //nolint:cyclop
	typeSchema *base.Schema,
	value float64,
) []httperror.ValidationError {
	var errs []httperror.ValidationError

	if typeSchema.MultipleOf != nil && *typeSchema.MultipleOf > 0 &&
		value != 0 &&
		math.Mod(value, *typeSchema.MultipleOf) != 0 {
		errs = append(errs, *MultipleOfValidationError(*typeSchema.MultipleOf, value))
	}

	switch {
	case typeSchema.ExclusiveMaximum != nil &&
		typeSchema.ExclusiveMaximum.N == 1 &&
		value >= typeSchema.ExclusiveMaximum.B:
		errs = append(errs, *MaximumValidationError(typeSchema.ExclusiveMaximum.B, value, true))
	case typeSchema.ExclusiveMaximum != nil &&
		typeSchema.ExclusiveMaximum.A &&
		typeSchema.Maximum != nil &&
		value >= *typeSchema.Maximum:
		errs = append(errs, *MaximumValidationError(*typeSchema.Maximum, value, true))
	case typeSchema.Maximum != nil && value > *typeSchema.Maximum:
		errs = append(errs, *MaximumValidationError(*typeSchema.Maximum, value, false))
	case typeSchema.ExclusiveMinimum != nil &&
		typeSchema.ExclusiveMinimum.N == 1 &&
		value <= typeSchema.ExclusiveMinimum.B:
		errs = append(errs, *MinimumValidationError(typeSchema.ExclusiveMaximum.B, value, true))
	case typeSchema.ExclusiveMinimum != nil &&
		typeSchema.ExclusiveMinimum.A &&
		typeSchema.Minimum != nil &&
		value <= *typeSchema.Minimum:
		errs = append(errs, *MinimumValidationError(*typeSchema.Minimum, value, true))
	case typeSchema.Minimum != nil && value < *typeSchema.Minimum:
		errs = append(errs, *MinimumValidationError(*typeSchema.Minimum, value, false))
	default:
	}

	return errs
}

// validateArrayLength checks maxItems and minItems constraints against the given length.
func validateArrayLength(typeSchema *base.Schema, length int64) *httperror.ValidationError {
	// array length validations
	if typeSchema.MaxItems != nil && length > *typeSchema.MaxItems {
		return ArrayMaxItemsValidationError(*typeSchema.MaxItems, length)
	}

	if typeSchema.MinItems != nil && length < *typeSchema.MinItems {
		return ArrayMinItemsValidationError(*typeSchema.MinItems, length)
	}

	return nil
}

// validateObjectReflection validates a reflect.Map value against an OpenAPI object schema,
// checking type, property count, required keys, and dependent required keys.
func validateObjectReflection(
	typeSchema *base.Schema,
	reflectValue reflect.Value,
) []httperror.ValidationError {
	if len(typeSchema.Type) > 0 && !slices.Contains(typeSchema.Type, oaschema.Object) {
		return []httperror.ValidationError{*InvalidTypeError(typeSchema.Type, oaschema.Object)}
	}

	var (
		errs []httperror.ValidationError
		keys []string
	)

	fieldCount := reflectValue.Len()

	propLenErr := validateObjectPropertiesLength(typeSchema, int64(fieldCount))
	if propLenErr != nil {
		errs = []httperror.ValidationError{*propLenErr}
	}

	if fieldCount > 0 {
		keys = make([]string, 0, fieldCount)

		for _, reflectKey := range reflectValue.MapKeys() {
			kind := reflectKey.Kind()

			if kind != reflect.String {
				errs = append(errs, *ObjectPropertyKeyTypeError(kind.String()))

				return errs
			}

			keys = append(keys, reflectKey.String())
		}
	}

	for _, requiredKey := range typeSchema.Required {
		if !slices.Contains(keys, requiredKey) {
			errs = append(errs, *ObjectRequiredPropertyError(requiredKey))
		}
	}

	if typeSchema.DependentRequired != nil {
		for iter := typeSchema.DependentRequired.First(); iter != nil; iter = iter.Next() {
			key := iter.Key()
			dependents := iter.Value()

			for _, dependent := range dependents {
				if !slices.Contains(keys, dependent) {
					errs = append(errs, *ObjectDependentRequiredError(key, dependent))
				}
			}
		}
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

// validateObjectPropertiesLength checks maxProperties and minProperties constraints.
func validateObjectPropertiesLength(
	typeSchema *base.Schema,
	propertiesLength int64,
) *httperror.ValidationError {
	if typeSchema.MaxProperties != nil && *typeSchema.MaxProperties < propertiesLength {
		return ObjectMaxPropertiesValidationError(*typeSchema.MaxProperties, propertiesLength)
	}

	if typeSchema.MinProperties != nil && *typeSchema.MinProperties > propertiesLength {
		return ObjectMinPropertiesValidationError(*typeSchema.MinProperties, propertiesLength)
	}

	return nil
}

// validateValueReflection validates an arbitrary reflect.Value against an OpenAPI schema by
// unwrapping pointers and dispatching to the appropriate typed validator.
func validateValueReflection(
	typeSchema *base.Schema,
	value reflect.Value,
) []httperror.ValidationError {
	reflectValue, notNull := goutils.UnwrapPointerFromReflectValue(value)
	if !notNull {
		return nil
	}

	valueKind := reflectValue.Kind()

	switch valueKind {
	case reflect.Bool:
		return ValidateBoolean(typeSchema, reflectValue.Bool())
	case reflect.String:
		return ValidateString(typeSchema, reflectValue.String())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return ValidateInteger(typeSchema, reflectValue.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return ValidateInteger(typeSchema, reflectValue.Uint())
	case reflect.Float32, reflect.Float64:
		return ValidateNumber(typeSchema, reflectValue.Float())
	case reflect.Slice, reflect.Array:
		valueLength := reflectValue.Len()

		alError := validateArrayLength(typeSchema, int64(valueLength))
		if alError != nil {
			return []httperror.ValidationError{*alError}
		}

		return nil
	case reflect.Map:
		return validateObjectReflection(typeSchema, reflectValue)
	default:
		return nil
	}
}

func validateValueOnly( //nolint:gocyclo,cyclop,funlen
	typeSchema *base.Schema,
	value any,
) []httperror.ValidationError {
	switch val := value.(type) {
	case bool:
		return ValidateBoolean(typeSchema, val)
	case *bool:
		return ValidateNullableBoolean(typeSchema, val)
	case []byte:
		if len(typeSchema.Type) > 0 && !slices.Contains(typeSchema.Type, oaschema.String) {
			return []httperror.ValidationError{*InvalidTypeError(typeSchema.Type, oaschema.String)}
		}

		if (typeSchema.Nullable == nil || !*typeSchema.Nullable) && val == nil {
			return []httperror.ValidationError{*NotNullError()}
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
		return ValidateArrayAndItems(typeSchema, val, CompareBoolean)
	case []*bool:
		return ValidateArrayAndItems(typeSchema, val, CompareNullableBoolean)
	case []string:
		return ValidateArrayAndItems(typeSchema, val, cmp.Compare)
	case []*string:
		return ValidateArrayAndItems(typeSchema, val, CompareNullable)
	case []int:
		return ValidateArrayAndItems(typeSchema, val, cmp.Compare)
	case []int8:
		return ValidateArrayAndItems(typeSchema, val, cmp.Compare)
	case []int16:
		return ValidateArrayAndItems(typeSchema, val, cmp.Compare)
	case []int32:
		return ValidateArrayAndItems(typeSchema, val, cmp.Compare)
	case []uint:
		return ValidateArrayAndItems(typeSchema, val, cmp.Compare)
	case []uint16:
		return ValidateArrayAndItems(typeSchema, val, cmp.Compare)
	case []*int:
		return ValidateArrayAndItems(typeSchema, val, CompareNullable)
	case []*int8:
		return ValidateArrayAndItems(typeSchema, val, CompareNullable)
	case []*int16:
		return ValidateArrayAndItems(typeSchema, val, CompareNullable)
	case []*int32:
		return ValidateArrayAndItems(typeSchema, val, CompareNullable)
	case []*uint:
		return ValidateArrayAndItems(typeSchema, val, CompareNullable)
	case []*uint8:
		return ValidateArrayAndItems(typeSchema, val, CompareNullable)
	case []*uint16:
		return ValidateArrayAndItems(typeSchema, val, CompareNullable)
	case []int64:
		return ValidateArrayAndItems(typeSchema, val, cmp.Compare)
	case []uint32:
		return ValidateArrayAndItems(typeSchema, val, cmp.Compare)
	case []uint64:
		return ValidateArrayAndItems(typeSchema, val, cmp.Compare)
	case []*int64:
		return ValidateArrayAndItems(typeSchema, val, CompareNullable)
	case []*uint32:
		return ValidateArrayAndItems(typeSchema, val, CompareNullable)
	case []*uint64:
		return ValidateArrayAndItems(typeSchema, val, CompareNullable)
	case []float32:
		return ValidateArrayAndItems(typeSchema, val, cmp.Compare)
	case []*float32:
		return ValidateArrayAndItems(typeSchema, val, CompareNullable)
	case []float64:
		return ValidateArrayAndItems(typeSchema, val, cmp.Compare)
	case []*float64:
		return ValidateArrayAndItems(typeSchema, val, CompareNullable)
	case []any:
		return ValidateArrayAndItems(typeSchema, val, nil)
	case map[string]any:
		return ValidateObject(typeSchema, val)
	default:
		return validateValueReflection(typeSchema, reflect.ValueOf(value))
	}
}
