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
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/relychan/goutils/httperror"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/oasvalidator"
)

type jsonDecoder struct {
	Media   *highv3.MediaType
	Decoder *json.Decoder
}

// DecodeJSON decodes an arbitrary JSON from a reader stream.
func DecodeJSON(r io.Reader) (any, error) {
	var result any

	decoder := json.NewDecoder(r)

	return result, decoder.Decode(r)
}

// DecodeJSONWithSchema decodes an arbitrary JSON from a reader stream.
func DecodeJSONWithSchema(r io.Reader, oasMedia *highv3.MediaType) (any, error) {
	if oasMedia == nil || (oasMedia.Schema == nil && oasMedia.ItemSchema == nil) {
		return DecodeJSON(r)
	}

	decoder := json.NewDecoder(r)

	decoder.UseNumber()

	jsonDecoder := jsonDecoder{
		Decoder: decoder,
		Media:   oasMedia,
	}

	return jsonDecoder.Decode()
}

func (jd *jsonDecoder) Decode() (any, error) {
	rootToken, err := jd.Decoder.Token()
	if err != nil {
		if err == io.EOF {
			return nil, nil
		}

		return nil, err
	}

	if jd.Media.Schema == nil {
		if rootToken != '[' {
			return nil, &httperror.ValidationError{
				Code:   oasvalidator.ErrCodeMalformedJSON,
				Detail: fmt.Sprintf("Invalid syntax. Expected an array, got %v", rootToken),
			}
		}

		return jd.DecodeArray(jd.Media.ItemSchema.Schema(), "")
	}

	return jd.DecodeToken(rootToken, jd.Media.Schema.Schema(), "")
}

func (jd *jsonDecoder) DecodeToken(
	token json.Token,
	typeSchema *base.Schema,
	pointer string,
) (any, error) {
	switch tok := token.(type) {
	case json.Delim:
		if tok == '[' {
			if typeSchema != nil && len(typeSchema.Type) > 0 &&
				!slices.Contains(typeSchema.Type, oaschema.Array) {
				return nil, &httperror.ValidationError{
					Code: oasvalidator.ErrCodeMalformedJSON,
					Detail: fmt.Sprintf(
						"Invalid syntax. Expected one of %v, got array",
						typeSchema.Type,
					),
					Pointer: pointer,
				}
			}

			results := make([]any, 0)

			var itemSchema *base.Schema

			if typeSchema != nil && typeSchema.Items != nil && typeSchema.Items.A != nil {
				itemSchema = typeSchema.Items.A.Schema()
			}

			for {
				itemPointer := pointer + "/" + strconv.Itoa(len(results))

				nextTok, err := jd.Decoder.Token()
				if err != nil {
					return nil, &httperror.ValidationError{
						Code:    oasvalidator.ErrCodeMalformedJSON,
						Detail:  err.Error(),
						Pointer: itemPointer,
					}
				}

				item, err := jd.DecodeToken(nextTok, itemSchema, itemPointer)
				if err != nil {
					return nil, err
				}

				results = append(results, item)

				if nextTok == ']' {
					break
				}
			}

			return results, nil
		}

		if tok == '{' {
			return jd.DecodeObject(typeSchema, pointer)
		}

		return nil, nil
	case json.Number:
		return jd.DecodeNumber(tok, typeSchema, pointer)
	case string:
		if typeSchema == nil {
			return tok, nil
		}

		return jd.DecodeString(tok, typeSchema, pointer)
	case any:
		if typeSchema == nil {
			return tok, nil
		}

		return tok, nil
	default:
		return nil, nil
	}
}

func (jd *jsonDecoder) DecodeArray(
	itemSchema *base.Schema,
	pointer string,
) (any, error) {
	results := make([]any, 0)

	for {
		itemPointer := pointer + "/" + strconv.Itoa(len(results))

		nextTok, err := jd.Decoder.Token()
		if err != nil {
			return nil, &httperror.ValidationError{
				Code:    oasvalidator.ErrCodeMalformedJSON,
				Detail:  err.Error(),
				Pointer: itemPointer,
			}
		}

		item, err := jd.DecodeToken(nextTok, itemSchema, itemPointer)
		if err != nil {
			return nil, err
		}

		results = append(results, item)

		if nextTok == ']' {
			break
		}
	}

	return results, nil
}

func (jd *jsonDecoder) DecodeObject(
	typeSchema *base.Schema,
	pointer string,
) (any, error) {
	if len(typeSchema.Type) > 0 && !slices.Contains(typeSchema.Type, oaschema.Object) {
		return nil, &httperror.ValidationError{
			Code:    oasvalidator.ErrCodeMalformedJSON,
			Detail:  fmt.Sprintf("Invalid syntax. Expected one of %v, got object", typeSchema.Type),
			Pointer: pointer,
		}
	}

	var (
		hasProperties    bool
		additionalSchema *base.Schema
		results          = make(map[string]any)
	)

	if typeSchema != nil {
		hasProperties = typeSchema.Properties != nil && typeSchema.Properties.Len() > 0

		// We may need to validate additional schema for unknown properties.
		if typeSchema.AdditionalProperties != nil && typeSchema.AdditionalProperties.A != nil {
			additionalSchema = typeSchema.AdditionalProperties.A.Schema()
		}
	}

	for {
		keyTok, err := jd.Decoder.Token()
		if err != nil {
			return nil, &httperror.ValidationError{
				Code:    oasvalidator.ErrCodeMalformedJSON,
				Detail:  err.Error(),
				Pointer: pointer,
			}
		}

		if keyTok == '}' {
			break
		}

		key, ok := keyTok.(string)
		if !ok {
			return nil, &httperror.ValidationError{
				Code: oasvalidator.ErrCodeMalformedJSON,
				Detail: fmt.Sprintf(
					"Invalid object syntax. Expected a key string, got: %v",
					keyTok,
				),
				Pointer: pointer,
			}
		}

		itemPointer := pointer + "/" + key

		valueTok, err := jd.Decoder.Token()
		if err != nil {
			return nil, &httperror.ValidationError{
				Code:    oasvalidator.ErrCodeMalformedJSON,
				Detail:  err.Error(),
				Pointer: itemPointer,
			}
		}

		var valueSchema *base.Schema

		if hasProperties {
			valueProxy, present := typeSchema.Properties.Get(key)
			if present && valueProxy != nil {
				valueSchema = valueProxy.Schema()
			}
		}

		if valueSchema == nil && additionalSchema != nil {
			valueSchema = additionalSchema
		}

		value, err := jd.DecodeToken(valueTok, valueSchema, itemPointer)
		if err != nil {
			return nil, err
		}

		results[key] = value
	}

	return results, nil
}

func (*jsonDecoder) DecodeString(
	token string,
	typeSchema *base.Schema,
	pointer string,
) (any, error) {
	if typeSchema == nil || len(typeSchema.Type) == 0 ||
		slices.ContainsFunc(typeSchema.Type, func(raw string) bool {
			return raw == "string" || raw == "uuid"
		}) {
		return token, nil
	}

	return nil, &httperror.ValidationError{
		Code:    oasvalidator.ErrCodeMalformedJSON,
		Detail:  fmt.Sprintf("Expected one of %s; got number", strings.Join(typeSchema.Type, ", ")),
		Pointer: pointer,
	}
}

func (*jsonDecoder) DecodeNumber(
	token json.Number,
	typeSchema *base.Schema,
	pointer string,
) (any, error) {
	if typeSchema == nil ||
		len(typeSchema.Type) == 0 ||
		slices.ContainsFunc(typeSchema.Type, func(raw string) bool {
			return raw == "number" || raw == "float" || raw == "double"
		}) {
		result, err := token.Float64()
		if err != nil {
			return nil, &httperror.ValidationError{
				Code:    oasvalidator.ErrCodeMalformedJSON,
				Detail:  err.Error(),
				Pointer: pointer,
			}
		}

		return result, nil
	}

	if slices.ContainsFunc(typeSchema.Type, func(raw string) bool {
		return raw == "integer" || raw == "int" || raw == "long" || raw == "int32" || raw == "int64"
	}) {
		result, err := token.Int64()
		if err != nil {
			return nil, &httperror.ValidationError{
				Code:    oasvalidator.ErrCodeMalformedJSON,
				Detail:  err.Error(),
				Pointer: pointer,
			}
		}

		return result, nil
	}

	return nil, &httperror.ValidationError{
		Code:    oasvalidator.ErrCodeMalformedJSON,
		Detail:  fmt.Sprintf("Expected one of %s; got number", strings.Join(typeSchema.Type, ", ")),
		Pointer: pointer,
	}
}
