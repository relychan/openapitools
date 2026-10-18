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
	"encoding/json"
	"log/slog"
	"math"
	"reflect"
	"slices"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/dlclark/regexp2"
	"github.com/pb33f/go-yaml"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/relychan/goutils"
	"github.com/relychan/goutils/httperror"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/oasvalidator/regexps"
)

// ValidateContains validates the contains rule against an array value.
func ValidateContains(typeSchema *base.Schema, value []any) *httperror.ValidationError {
	if typeSchema == nil || typeSchema.Contains == nil {
		return nil
	}

	// minContains defaults to 1 when the contains keyword is present.
	minContains := int64(1)
	if typeSchema.MinContains != nil {
		minContains = *typeSchema.MinContains
	}

	if minContains <= 0 && typeSchema.MaxContains == nil {
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

	if containsCount < minContains {
		return MinContainsError(minContains, containsCount)
	}

	if typeSchema.MaxContains != nil && containsCount > *typeSchema.MaxContains {
		return MaxContainsError(*typeSchema.MaxContains, containsCount)
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
		validationErrors = append(validationErrors, validateOneOf(typeSchema.OneOf, value)...)
	}

	return validationErrors
}

// validateOneOf validates that the value matches exactly one of the oneOf schemas.
func validateOneOf(oneOf []*base.SchemaProxy, value any) []httperror.ValidationError {
	var (
		ooErrors       []httperror.ValidationError
		matchedIndexes []int
	)

	for i, oo := range oneOf {
		errs := ValidateValueWithSchemaProxy(oo, value)
		if len(errs) > 0 {
			ooErrors = addErrorHints(ooErrors, errs, "/oneOf/"+strconv.Itoa(i))

			continue
		}

		matchedIndexes = append(matchedIndexes, i)
	}

	switch len(matchedIndexes) {
	case 0:
		return ooErrors
	case 1:
		return nil
	default:
		return []httperror.ValidationError{*OneOfMultipleMatchedError(matchedIndexes)}
	}
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
	if value == nil {
		return validateNull(typeSchema)
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

	valueLength := int64(utf8.RuneCountInString(value))

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
	if value == nil {
		return validateNull(typeSchema)
	}

	return ValidateString(typeSchema, *value)
}

// ValidateNumber validates a number value against an OpenAPI schema.
func ValidateNumber[T float32 | float64](
	typeSchema *base.Schema,
	value T,
) []httperror.ValidationError {
	val := float64(value)

	// JSON decoders produce float64 for every number, so a whole number also satisfies the integer type.
	if len(typeSchema.Type) > 0 && !slices.Contains(typeSchema.Type, oaschema.Number) &&
		(!slices.Contains(typeSchema.Type, oaschema.Integer) || !isWholeNumber(val)) {
		return []httperror.ValidationError{*InvalidTypeError(typeSchema.Type, oaschema.Number)}
	}

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
	if value == nil {
		return validateNull(typeSchema)
	}

	return ValidateNumber(typeSchema, *value)
}

// ValidateInteger validates a number value against an OpenAPI schema.
func ValidateInteger[T int | int8 | int16 | int32 | int64 | uint | uint8 | uint16 | uint32 | uint64](
	typeSchema *base.Schema,
	value T,
) []httperror.ValidationError {
	if len(typeSchema.Type) > 0 && !slices.Contains(typeSchema.Type, oaschema.Integer) {
		// An integer is also a valid number.
		if slices.Contains(typeSchema.Type, oaschema.Number) {
			return ValidateNumber(typeSchema, float64(value))
		}

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
	if value == nil {
		return validateNull(typeSchema)
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
func ValidateObject[T any](
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

			propSchema := getSchemaFromProxy(iter.Value())
			if propSchema == nil {
				continue
			}

			// Absent properties are validated by the required rule only.
			prop, present := value[key]
			if !present {
				continue
			}

			validateProperty(propSchema, prop, key)
		}
	}

	patterns := compilePatternProperties(typeSchema)
	additionalSchema, forbidAdditional := getAdditionalPropertiesRule(typeSchema)

	if len(patterns) == 0 && additionalSchema == nil && !forbidAdditional {
		return errs
	}

	for _, key := range getUndeclaredPropertyKeys(typeSchema, value) {
		prop := value[key]
		matched := false

		for _, pp := range patterns {
			if !pp.match(key) {
				continue
			}

			matched = true

			validateProperty(pp.schema, prop, key)
		}

		switch {
		case matched:
		case forbidAdditional:
			errs = append(errs, *ObjectAdditionalPropertyError(key))
		case additionalSchema != nil:
			validateProperty(additionalSchema, prop, key)
		default:
		}
	}

	return errs
}

// getUndeclaredPropertyKeys returns sorted keys of the object that are not declared in properties.
// Keys are sorted to produce deterministic errors.
func getUndeclaredPropertyKeys[T any](typeSchema *base.Schema, value map[string]T) []string {
	keys := make([]string, 0, len(value))

	for key := range value {
		if typeSchema.Properties != nil {
			_, present := typeSchema.Properties.Get(key)
			if present {
				continue
			}
		}

		keys = append(keys, key)
	}

	slices.Sort(keys)

	return keys
}

type compiledPatternProperty struct {
	rawPattern string
	pattern    *regexp2.Regexp
	schema     *base.Schema
}

func (cpp compiledPatternProperty) match(key string) bool {
	matched, err := cpp.pattern.MatchString(key)
	if err != nil {
		slog.Warn(
			"failed to match pattern property: "+err.Error(),
			slog.String("pattern", cpp.rawPattern),
			slog.String("name", key),
		)

		return false
	}

	return matched
}

// compilePatternProperties compiles the patternProperties of the schema.
// Invalid patterns are ignored at runtime.
func compilePatternProperties(typeSchema *base.Schema) []compiledPatternProperty {
	if typeSchema.PatternProperties == nil || typeSchema.PatternProperties.Len() == 0 {
		return nil
	}

	results := make([]compiledPatternProperty, 0, typeSchema.PatternProperties.Len())

	for iter := typeSchema.PatternProperties.First(); iter != nil; iter = iter.Next() {
		rawPattern := iter.Key()

		propSchema := getSchemaFromProxy(iter.Value())
		if propSchema == nil {
			continue
		}

		pattern, err := regexps.Get(rawPattern)
		if err != nil {
			// ignore compile error on runtime.
			slog.Warn(
				"failed to compile regular expression: "+err.Error(),
				slog.String("pattern", rawPattern),
			)

			continue
		}

		results = append(results, compiledPatternProperty{
			rawPattern: rawPattern,
			pattern:    pattern,
			schema:     propSchema,
		})
	}

	return results
}

// getAdditionalPropertiesRule returns the additionalProperties schema if exists,
// or whether additional properties are forbidden (additionalProperties: false).
func getAdditionalPropertiesRule(typeSchema *base.Schema) (*base.Schema, bool) {
	ap := typeSchema.AdditionalProperties
	if ap == nil {
		return nil, false
	}

	if ap.IsB() {
		return nil, !ap.B
	}

	return getSchemaFromProxy(ap.A), false
}

func getSchemaFromProxy(proxy *base.SchemaProxy) *base.Schema {
	if proxy == nil {
		return nil
	}

	return proxy.Schema()
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

	errs = append(errs, validateDependentRequired(typeSchema, func(key string) bool {
		_, present := value[key]

		return present
	})...)

	return errs
}

// validateDependentRequired checks that, for every trigger key present in the object,
// all of its dependent keys are present too.
func validateDependentRequired(
	typeSchema *base.Schema,
	hasKey func(key string) bool,
) []httperror.ValidationError {
	if typeSchema.DependentRequired == nil {
		return nil
	}

	var errs []httperror.ValidationError

	for iter := typeSchema.DependentRequired.First(); iter != nil; iter = iter.Next() {
		key := iter.Key()
		if !hasKey(key) {
			continue
		}

		for _, dependent := range iter.Value() {
			if !hasKey(dependent) {
				errs = append(errs, *ObjectDependentRequiredError(key, dependent))
			}
		}
	}

	return errs
}

// validateNull validates a null value against the schema. A schema without any type
// or enum constraint accepts null values.
func validateNull(typeSchema *base.Schema) []httperror.ValidationError {
	if len(typeSchema.Type) == 0 && len(typeSchema.Enum) == 0 && typeSchema.Const == nil {
		return nil
	}

	if typeSchema.Const != nil && (typeSchema.Const.Tag == goutils.YAMLNullTag) {
		return nil
	}

	if !CanNull(typeSchema, true) {
		return []httperror.ValidationError{*NotNullError()}
	}

	return nil
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
func validateNumberRules(
	typeSchema *base.Schema,
	value float64,
) []httperror.ValidationError {
	var errs []httperror.ValidationError

	if typeSchema.MultipleOf != nil && *typeSchema.MultipleOf > 0 &&
		!isMultipleOf(value, *typeSchema.MultipleOf) {
		errs = append(errs, *MultipleOfValidationError(*typeSchema.MultipleOf, value))
	}

	maxErr := validateNumberMaximum(typeSchema, value)
	if maxErr != nil {
		errs = append(errs, *maxErr)
	}

	minErr := validateNumberMinimum(typeSchema, value)
	if minErr != nil {
		errs = append(errs, *minErr)
	}

	return errs
}

// validateNumberMaximum checks maximum and exclusiveMaximum constraints.
// exclusiveMaximum can be a number (OAS 3.1) or a boolean modifier of maximum (OAS 3.0).
func validateNumberMaximum(typeSchema *base.Schema, value float64) *httperror.ValidationError {
	exclusiveMaximum := typeSchema.ExclusiveMaximum

	switch {
	case exclusiveMaximum != nil && exclusiveMaximum.IsB() && value >= exclusiveMaximum.B:
		return MaximumValidationError(exclusiveMaximum.B, value, true)
	case exclusiveMaximum != nil && exclusiveMaximum.IsA() && exclusiveMaximum.A &&
		typeSchema.Maximum != nil && value >= *typeSchema.Maximum:
		return MaximumValidationError(*typeSchema.Maximum, value, true)
	case typeSchema.Maximum != nil && value > *typeSchema.Maximum:
		return MaximumValidationError(*typeSchema.Maximum, value, false)
	default:
		return nil
	}
}

// validateNumberMinimum checks minimum and exclusiveMinimum constraints.
// exclusiveMinimum can be a number (OAS 3.1) or a boolean modifier of minimum (OAS 3.0).
func validateNumberMinimum(typeSchema *base.Schema, value float64) *httperror.ValidationError {
	exclusiveMinimum := typeSchema.ExclusiveMinimum

	switch {
	case exclusiveMinimum != nil && exclusiveMinimum.IsB() && value <= exclusiveMinimum.B:
		return MinimumValidationError(exclusiveMinimum.B, value, true)
	case exclusiveMinimum != nil && exclusiveMinimum.IsA() && exclusiveMinimum.A &&
		typeSchema.Minimum != nil && value <= *typeSchema.Minimum:
		return MinimumValidationError(*typeSchema.Minimum, value, true)
	case typeSchema.Minimum != nil && value < *typeSchema.Minimum:
		return MinimumValidationError(*typeSchema.Minimum, value, false)
	default:
		return nil
	}
}

// multipleOfEpsilon is the relative tolerance used to absorb floating-point rounding errors
// when checking the multipleOf rule, e.g. 0.3 / 0.1 = 2.9999999999999996.
const multipleOfEpsilon = 1e-9

// isMultipleOf checks if the value is a multiple of the divisor with a relative tolerance.
func isMultipleOf(value float64, divisor float64) bool {
	if value == 0 {
		return true
	}

	quotient := value / divisor
	if math.IsInf(quotient, 0) || math.IsNaN(quotient) {
		return false
	}

	diff := math.Abs(quotient - math.Round(quotient))

	return diff <= multipleOfEpsilon*math.Max(1, math.Abs(quotient))
}

// isWholeNumber checks if the float value is a finite number without fractional part.
func isWholeNumber(value float64) bool {
	return !math.IsInf(value, 0) && !math.IsNaN(value) && value == math.Trunc(value)
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

	errs = append(errs, validateDependentRequired(typeSchema, func(key string) bool {
		return slices.Contains(keys, key)
	})...)

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
		return validateNull(typeSchema)
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
	case nil:
		return validateNull(typeSchema)
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
		return validateAnyArray(typeSchema, val)
	case map[string]any:
		return ValidateObject(typeSchema, val)
	default:
		return validateValueReflection(typeSchema, reflect.ValueOf(value))
	}
}

// validateAnyArray validates an array of arbitrary values. Unique items are compared
// by their canonical JSON representation, so 1 and 1.0 are equal and object key order is ignored.
func validateAnyArray(typeSchema *base.Schema, value []any) []httperror.ValidationError {
	errs := ValidateArrayAndItems(typeSchema, value, nil)

	if typeSchema.UniqueItems == nil || !*typeSchema.UniqueItems || len(value) <= 1 {
		return errs
	}

	duplicatedItems := findDuplicatedAnyItems(value)
	if len(duplicatedItems) > 0 {
		errs = append(errs, *ArrayUniqueItemsValidationError(duplicatedItems))
	}

	return errs
}

// findDuplicatedAnyItems finds duplicated items by comparing their canonical JSON encoding.
func findDuplicatedAnyItems(values []any) []any {
	counts := make(map[string]int, len(values))

	var results []any

	for _, item := range values {
		encoded, err := json.Marshal(item)
		if err != nil {
			continue
		}

		key := string(encoded)

		counts[key]++
		if counts[key] == 2 {
			results = append(results, item)
		}
	}

	return results
}
