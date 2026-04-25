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

// Package parameter defines serialization functions for HTTP parameters.
package parameter

// EncodeHeader encodes the header from an arbitrary value.
// The value is encoded differently on each style, according to the [OpenAPI specification].
//
// [OpenAPI specification](https://github.com/OAI/OpenAPI-Specification/blob/3.2.0/versions/3.2.0.md#style-examples)
func EncodeHeader(definition BaseParameter, value any) string {
	_, explode := definition.GetStyleAndExplode()
	items := EvaluateParameterValue(value, ParamKeys{})

	return encodeParamWithSimpleStyle(items, explode)
}

// encodeParamWithSimpleStyle serializes items using the OpenAPI simple style.
// With explode=true object properties use '=' as the key/value separator (key=value,key=value);
// without explode both keys and values are comma-separated (key,value,key,value).
func encodeParamWithSimpleStyle(
	items ParameterItems,
	explode bool,
) string {
	if explode {
		return EncodeParamDelimitedStyleNonExplode(items, ',', '=')
	}

	return EncodeParamDelimitedStyleNonExplode(items, ',', ',')
}
