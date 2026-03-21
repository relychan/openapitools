// Copyright 2026 RelyChan Pte. Ltd
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package parameter

import (
	"strings"

	"github.com/relychan/openapitools/oaschema"
)

// EncodePath encodes the path parameter from an arbitrary value.
// The value is encoded differently on each style, according to the [OpenAPI specification].
//
// [OpenAPI specification](https://github.com/OAI/OpenAPI-Specification/blob/3.2.0/versions/3.2.0.md#style-examples)
func EncodePath(definition BaseParameter, value any) string {
	style, explode := definition.GetStyleAndExplode()
	items := EvaluateParameterValue(value, ParamKeys{})

	switch style {
	case oaschema.EncodingStyleLabel:
		// /users/.3.4.5
		// /users/.role=admin.firstName=Alex
		if explode {
			return "." + EncodeParamDelimitedStyleNonExplode(items, '.', '=')
		}

		// /users/.3,4,5
		// /users/.role,admin,firstName,Alex
		return "." + EncodeParamDelimitedStyleNonExplode(items, ',', ',')
	case oaschema.EncodingStyleMatrix:
		if len(items) == 0 {
			return ";" + definition.Name + "="
		}

		// /users/;id=3;id=4;id=5
		// /users/;role=admin;firstName=Alex
		if explode {
			return encodeParamMatrixExplode(definition.Name, items)
		}

		// /users/;id=3,4,5
		// /users/;id=role,admin,firstName,Alex
		var sb strings.Builder

		builtParams, count := items.Build("", false)

		sb.Grow(count + len(definition.Name) + 2)
		sb.WriteByte(';')
		sb.WriteString(definition.Name)
		sb.WriteByte('=')
		buildParamDelimitedStyleNonExplode(&sb, builtParams, ',', ',')

		return sb.String()
	default:
		// encode with the simple style
		return encodeParamWithSimpleStyle(items, explode)
	}
}

func encodeParamMatrixExplode(name string, params ParameterItems) string {
	var sb strings.Builder

	sb.Grow((len(name) + 1) * len(params) * 2)

	for _, param := range params {
		if param.IsNested() {
			// The root key is ignored in nested fields.
			// ;R=100;G=200;B=150
			key := param.keys.Format("", false)

			sb.WriteByte(';')
			sb.WriteString(key)
			sb.WriteByte('=')
			sb.WriteString(param.value)

			continue
		}

		// The parameter does not have nested object. Encode with the simple form format.
		// /users?id=3&id=4&id=5
		sb.WriteByte(';')
		sb.WriteString(name)
		sb.WriteByte('=')
		sb.WriteString(param.value)
	}

	return sb.String()
}
