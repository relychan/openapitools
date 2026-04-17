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

package contenttype

import (
	"strings"

	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/oasvalidator"
	"github.com/relychan/openapitools/openapiclient/handler/resthandler/parameter"
)

// implement the encoder of application/x-www-form-urlencoded content type.
type formURLEncoder struct {
	media   *highv3.MediaType
	builder strings.Builder
}

// EncodeFormURLEncoded encodes the arbitrary value to application/x-www-form-urlencoded content type.
func EncodeFormURLEncoded(value any, media *highv3.MediaType) (string, error) {
	params := parameter.EvaluateParameterValue(value, parameter.ParamKeys{})

	if len(params) == 0 {
		return "", nil
	}

	enc := formURLEncoder{
		media:   media,
		builder: strings.Builder{},
	}

	return enc.Encode(params), nil
}

func (fue *formURLEncoder) Encode(params parameter.ParameterItems) string {
	paramMap := make(map[string]parameter.ParameterItems)

	for _, param := range params {
		if fue.builder.Len() > 0 {
			fue.builder.WriteByte('&')
		}

		keys := param.Keys()
		value := param.Value()

		// If the encoding does not exist, use the default encoding for query:
		//   form style and explode
		if len(keys) == 0 {
			fue.builder.WriteString(value)

			continue
		}

		// Encode array element with index
		if len(keys) == 1 && keys[0].Index() != nil {
			fue.builder.WriteByte('[')
			fue.builder.WriteString(keys[0].String())
			fue.builder.WriteString("]=")
			fue.builder.WriteString(value)

			continue
		}

		rootKey := keys[0].String()

		// Encode simple key=value
		if len(keys) == 1 {
			fue.builder.WriteString(rootKey)
			fue.builder.WriteRune('=')
			fue.builder.WriteString(value)

			continue
		}

		paramMap[rootKey] = append(paramMap[rootKey], parameter.NewParameterItem(keys[1:], value))
	}

	for key, param := range paramMap {
		fue.buildParams(key, param)
	}

	return fue.builder.String()
}

func (fue *formURLEncoder) getEncodingStyle(
	rootKey string,
) (oaschema.ParameterEncodingStyle, bool, bool) { //nolint:revive
	style := oaschema.EncodingStyleForm
	explode := true
	allowReserved := false

	if fue.media.Encoding == nil {
		return style, explode, allowReserved
	}

	encoding, _ := fue.media.Encoding.Get(rootKey)
	if encoding == nil {
		return style, explode, allowReserved
	}

	if encoding.Style != "" {
		encStyle, err := oaschema.ParseParameterEncodingStyle(encoding.Style)
		if err == nil {
			style = encStyle
		}
	}

	explode = encoding.Explode == nil || *encoding.Explode
	allowReserved = encoding.AllowReserved

	return style, explode, allowReserved
}

func (fue *formURLEncoder) buildParams(rootKey string, params parameter.ParameterItems) {
	style, explode, allowReserved := fue.getEncodingStyle(rootKey)

	switch style {
	case oaschema.EncodingStyleForm:
		if explode {
			for _, param := range params {
				fue.setParamFormExplode(rootKey, param, allowReserved)
			}

			return
		}

		fue.setParamDelimitedStyleNonExplode(rootKey, params, oaschema.Comma[0], allowReserved)
	case oaschema.EncodingStyleSpaceDelimited:
		fue.setParamDelimitedStyle(rootKey, params, oaschema.Space[0], explode, allowReserved)
	case oaschema.EncodingStylePipeDelimited:
		fue.setParamDelimitedStyle(rootKey, params, oaschema.Pipe[0], explode, allowReserved)
	case oaschema.EncodingStyleDeepObject:
		// simple non-nested objects are serialized as paramName[prop1]=value1&paramName[prop2]=value2&...
		for _, param := range params {
			if len(param.Keys()) == 0 {
				// The parameter does not have nested object. Encode with the simple form format.
				// /users?id=3&id=4&id=5
				fue.addParam(rootKey, param.Value(), allowReserved)

				return
			}

			// The root key is ignored in nested fields.
			// /users?role=admin&firstName=Alex
			queryKey := param.Keys().Format(rootKey, true)

			fue.addParam(queryKey, param.Value(), allowReserved)
		}
	default:
	}
}

// Set delimited-separated array values for simple params.
// For example: /users?id=3|4|5.
func (fue *formURLEncoder) setParamDelimitedStyle(
	rootKey string,
	params parameter.ParameterItems,
	separator byte,
	explode bool,
	allowReserved bool,
) {
	if explode {
		// the same format with the form style
		for _, param := range params {
			fue.setParamFormExplode(rootKey, param, allowReserved)
		}

		return
	}

	fue.setParamDelimitedStyleNonExplode(rootKey, params, separator, allowReserved)
}

// ampersand-separated values, also known as form-style query expansion with explode=true.
func (fue *formURLEncoder) setParamFormExplode(
	rootKey string,
	param parameter.ParameterItem,
	allowReserved bool,
) {
	// The parameter does not have nested object. Encode with the simple form format.
	// /users?id=3&id=4&id=5
	queryKey := rootKey

	if param.IsNested() {
		// The root key is ignored in nested fields.
		// /users?role=admin&firstName=Alex
		queryKey = param.Keys().Format("", false)
	}

	fue.addParam(queryKey, param.Value(), allowReserved)
}

// Encode and set ampersand-separated values with explode=false.
func (fue *formURLEncoder) setParamDelimitedStyleNonExplode(
	rootKey string,
	params parameter.ParameterItems,
	separator byte,
	allowReserved bool,
) {
	encodedValue := parameter.EncodeParamDelimitedStyleNonExplode(params, separator, separator)
	fue.addParam(rootKey, encodedValue, allowReserved)
}

func (fue *formURLEncoder) addParam(key string, value string, allowReserved bool) {
	if fue.builder.Len() > 0 {
		fue.builder.WriteByte('&')
	}

	encodedKey := oasvalidator.EncodeQueryEscape(key, allowReserved)
	encodedValue := oasvalidator.EncodeQueryEscape(value, allowReserved)

	fue.builder.WriteString(encodedKey)
	fue.builder.WriteByte('=')
	fue.builder.WriteString(encodedValue)
}
