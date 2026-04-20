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
	"strings"
	"time"

	"github.com/pb33f/libopenapi/datamodel/high/base"
)

// NormalizeType normalize a schema type.
// Returns the type name and whether if it is a primitive type.
func NormalizeType(typeName string) (string, bool) {
	lowerTypeName := strings.ToLower(typeName)

	switch lowerTypeName {
	case "bool", "boolean":
		return Boolean, true
	case "string", "uuid", "varchar":
		return String, true
	case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64":
		return Integer, true
	case "number", "decimal", "float", "float32", "float64", "double":
		return Number, true
	default:
		// array, object and unknown type.
		return lowerTypeName, false
	}
}

// DetectSchemaFromValue detects the OpenAPI schema type from a Go value.
func DetectSchemaFromValue(value any) *base.Schema { //nolint:gocyclo,cyclop,funlen
	switch val := value.(type) {
	case bool:
		return newBooleanSchema(false)
	case *bool:
		return newBooleanSchema(true)
	case []byte:
		return newStringSchema(false, Binary)
	case string:
		return newStringSchema(false, "")
	case *string:
		return newStringSchema(true, "")
	case int, int8, int16, int32, uint, uint8, uint16:
		return newIntegerSchema(false, Int32)
	case *int, *int8, *int16, *int32, *uint, *uint8, *uint16:
		return newIntegerSchema(true, Int32)
	case int64, uint32, uint64:
		return newIntegerSchema(false, Int64)
	case *int64, *uint32, *uint64:
		return newIntegerSchema(true, Int32)
	case float32:
		return newNumberSchema(false, Float)
	case *float32:
		return newNumberSchema(true, Float)
	case float64:
		return newNumberSchema(false, Double)
	case *float64:
		return newNumberSchema(true, Double)
	case time.Time:
		return newStringSchema(false, DateTime)
	case *time.Time:
		return newStringSchema(true, DateTime)
	case []bool:
		return newArraySchema(newBooleanSchema(false))
	case []*bool:
		return newArraySchema(newBooleanSchema(true))
	case []string:
		return newArraySchema(newStringSchema(false, ""))
	case []*string:
		return newArraySchema(newStringSchema(false, ""))
	case []int, []int8, []int16, []int32, []uint, []uint16:
		return newArraySchema(newIntegerSchema(false, Int32))
	case []*int, []*int8, []*int16, []*int32, []*uint, []*uint8, []*uint16:
		return newArraySchema(newIntegerSchema(true, Int32))
	case []int64, []uint32, []uint64:
		return newArraySchema(newIntegerSchema(false, Int64))
	case []*int64, []*uint32, []*uint64:
		return newArraySchema(newIntegerSchema(true, Int64))
	case []float32:
		return newArraySchema(newNumberSchema(false, Float))
	case []*float32:
		return newArraySchema(newNumberSchema(true, Float))
	case []float64:
		return newArraySchema(newNumberSchema(false, Double))
	case []*float64:
		return newArraySchema(newNumberSchema(true, Double))
	case []time.Time:
		return newArraySchema(newNumberSchema(false, DateTime))
	case []*time.Time:
		return newArraySchema(newNumberSchema(true, DateTime))
	case []any:
		var itemSchema *base.Schema

		if len(val) > 0 {
			itemSchema = DetectSchemaFromValue(val[0])
		}

		return newArraySchema(itemSchema)
	case map[string]any:
		// TODO: infer properties.
		return &base.Schema{
			Type: []string{Object},
		}
	default:
		// TODO: reflection
		return nil
	}
}

func newBooleanSchema(nullable bool) *base.Schema {
	result := &base.Schema{
		Type: []string{Boolean},
	}

	if nullable {
		result.Nullable = &nullable
	}

	return result
}

func newArraySchema(itemSchema *base.Schema) *base.Schema {
	result := &base.Schema{
		Type: []string{Array},
	}

	if itemSchema != nil {
		items := base.CreateSchemaProxy(itemSchema)

		result.Items = &base.DynamicValue[*base.SchemaProxy, bool]{
			N: 0,
			A: items,
		}
	}

	return result
}

func newStringSchema(nullable bool, format string) *base.Schema {
	result := &base.Schema{
		Type:   []string{String},
		Format: format,
	}

	if nullable {
		result.Nullable = &nullable
	}

	return result
}

func newIntegerSchema(nullable bool, format string) *base.Schema {
	result := &base.Schema{
		Type:   []string{Integer},
		Format: format,
	}

	if nullable {
		result.Nullable = &nullable
	}

	return result
}

func newNumberSchema(nullable bool, format string) *base.Schema {
	result := &base.Schema{
		Type:   []string{Number},
		Format: format,
	}

	if nullable {
		result.Nullable = &nullable
	}

	return result
}
