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

// Package oasvalidator defines validation functions for OpenAPI spec.
package oasvalidator

import (
	"github.com/relychan/goutils"
	"github.com/relychan/openapitools/oaschema"
)

// DecodePrimitiveValueFromType decodes a value from a primitive type.
func DecodePrimitiveValueFromType(value any, typeName string) (any, string, error) {
	var (
		result any
		err    error
	)

	switch typeName {
	case "bool", "boolean":
		result, err = goutils.DecodeBoolean(value)
		if err != nil {
			return nil, "", err
		}

		return result, oaschema.Boolean, nil
	case "string", "uuid", "varchar":
		result, err = goutils.DecodeString(value)
		if err != nil {
			return nil, "", err
		}

		return result, oaschema.String, nil
	case "int", "int8", "int16", "int32", "int64":
		result, err = goutils.DecodeNumber[int64](value)
		if err != nil {
			return nil, "", err
		}

		return result, oaschema.Integer, nil
	case "uint", "uint8", "uint16", "uint32", "uint64":
		result, err = goutils.DecodeNumber[uint64](value)
		if err != nil {
			return nil, "", err
		}

		return result, oaschema.Integer, nil
	case "number", "decimal", "float", "float32", "float64", "double":
		result, err = goutils.DecodeNumber[float64](value)
		if err != nil {
			return nil, "", err
		}

		return result, oaschema.Number, nil
	default:
		// unknown type. Returns the original value.
		return value, "", nil
	}
}
