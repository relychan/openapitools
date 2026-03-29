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
	"bytes"
	"net/http"
	"reflect"
	"strconv"
	"time"

	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/relychan/goutils"
	"github.com/relychan/goutils/httpheader"
	"github.com/relychan/openapitools/oaschema"
)

type multipartFormEncoder struct {
	media   *highv3.MediaType
	writer  *MultipartWriter
	headers http.Header
}

// EncodeMultipartForm encodes the arbitrary value to [multipart/form-data] content type.
//
// [multipart/form-data]: https://github.com/OAI/OpenAPI-Specification/blob/3.2.0/versions/3.2.0.md#encoding-multipart-media-types
func EncodeMultipartForm(
	bodyData any,
	headers http.Header,
	media *highv3.MediaType,
) ([]byte, string, error) {
	buffer := new(bytes.Buffer)
	writer := NewMultipartWriter(buffer)

	mfb := multipartFormEncoder{
		media:   media,
		writer:  writer,
		headers: headers,
	}

	err := mfb.Encode(bodyData)
	if err != nil {
		return nil, "", err
	}

	err = writer.Close()
	if err != nil {
		return nil, "", err
	}

	return buffer.Bytes(), writer.FormDataContentType(), nil
}

func (mfe *multipartFormEncoder) Encode(rootValue any) error {
	if rootValue == nil {
		return &goutils.ErrorDetail{
			Detail: "request body is required",
			Code:   oaschema.ErrCodeMultipartFormEncodeError,
		}
	}

	switch rootVal := rootValue.(type) {
	case bool,
		string,
		int,
		int8,
		int16,
		int32,
		int64,
		uint,
		uint8,
		uint16,
		uint32,
		uint64,
		float32,
		float64,
		complex64,
		complex128,
		time.Time,
		time.Duration,
		*bool,
		*string,
		*int,
		*int8,
		*int16,
		*int32,
		*int64,
		*uint,
		*uint8,
		*uint16,
		*uint32,
		*uint64,
		*float32,
		*float64,
		*complex64,
		*complex128,
		*time.Time,
		*time.Duration:
		return &goutils.ErrorDetail{
			Detail: "invalid multipart form body. Expected object, got: " +
				reflect.TypeOf(rootValue).Kind().String(),
		}
	case map[string]any:
		for key, val := range rootVal {
			err := mfe.evalValue(key, val)
			if err != nil {
				return err
			}
		}
	case []any:
		for i, val := range rootVal {
			err := mfe.evalValue(buildIndexKey(i), val)
			if err != nil {
				return err
			}
		}
	default:
		return mfe.evalRootValueReflection(reflect.ValueOf(rootValue))
	}

	return nil
}

func (mfe *multipartFormEncoder) evalRootValueReflection(reflectValue reflect.Value) error {
	value, notNull := goutils.UnwrapPointerFromReflectValue(reflectValue)
	if !notNull {
		return nil
	}

	valueKind := value.Kind()

	switch valueKind {
	case reflect.Slice, reflect.Array:
		valueLength := reflectValue.Len()

		for i := range valueLength {
			elem := reflectValue.Index(i)
			k := buildIndexKey(i)

			err := mfe.evalValueReflectionWithDefaultContentType(k, elem, http.Header{})
			if err != nil {
				return err
			}
		}
	case reflect.Map:
		keys := reflectValue.MapKeys()

		for _, key := range keys {
			keyStr, ok := goutils.FormatScalarReflection(key)
			if !ok {
				return &goutils.ErrorDetail{
					Detail: "invalid multipart form body. Expected the object key as a string, got: " +
						key.Kind().
							String(),
				}
			}

			contentType, headers := mfe.evalEncoding(keyStr)
			mapValue := reflectValue.MapIndex(key)

			if oaschema.IsContentTypeJSON(contentType) {
				return mfe.writer.WriteJSON(keyStr, mapValue.Interface(), headers)
			}

			if oaschema.IsContentTypeXML(contentType) {
				return mfe.writer.WriteXML(keyStr, value.Interface(), headers)
			}

			if contentType == "" ||
				oaschema.IsContentTypeText(contentType) {
				return mfe.evalValueReflectionWithDefaultContentType(keyStr, value, headers)
			}

			err := mfe.evalValueReflection(keyStr, mapValue, contentType, headers)
			if err != nil {
				return err
			}
		}
	default:
	}

	return nil
}

func (mfe *multipartFormEncoder) evalValue(key string, value any) error {
	contentType, headers := mfe.evalEncoding(key)

	if oaschema.IsContentTypeJSON(contentType) {
		return mfe.writer.WriteJSON(key, value, headers)
	}

	if oaschema.IsContentTypeXML(contentType) {
		return mfe.writer.WriteXML(key, value, headers)
	}

	if contentType == "" || oaschema.IsContentTypeText(contentType) {
		return mfe.evalValueWithDefaultContentType(key, value, headers)
	}

	switch val := value.(type) {
	case string:
		err := mfe.writer.WriteDataURI(key, val, contentType, headers)
		if err != nil {
			// fallback to encode arbitrary value.
			return mfe.evalValueWithDefaultContentType(key, value, headers)
		}

		return nil
	case []byte:
		return mfe.writer.WriteBinary(key, val, contentType, headers)
	default:
		return mfe.evalValueReflection(key, reflect.ValueOf(value), contentType, headers)
	}
}

// By default, the Content-Type of individual request parts is set automatically according to the type of the schema properties that describe the request parts:
//
//   - text/plain: Primitive or array of primitives.
//   - application/json: Complex value or array of complex values.
//   - application/octet-stream: String in the binary or base64 format.
func (mfe *multipartFormEncoder) evalValueWithDefaultContentType(
	key string,
	value any,
	headers http.Header,
) error {
	scalarValue, ok := goutils.FormatScalar(value)
	if ok {
		if scalarValue == goutils.NullStr {
			scalarValue = ""
		}

		return mfe.writer.WriteField(key, scalarValue, headers)
	}

	switch val := value.(type) {
	case []float64:
		for i, item := range val {
			k := buildArrayIndexKey(key, i)
			v := strconv.FormatFloat(item, 'f', -1, 64)

			err := mfe.writer.WriteField(k, v, headers)
			if err != nil {
				return err
			}
		}
	case []string:
		for i, item := range val {
			err := mfe.writer.WriteField(buildArrayIndexKey(key, i), item, headers)
			if err != nil {
				return err
			}
		}
	case []any:
		for i, item := range val {
			k := buildArrayIndexKey(key, i)

			err := mfe.evalChildValue(k, item, headers)
			if err != nil {
				return err
			}
		}
	case map[string]any, map[any]any:
		// Encode application/json content type for complex value or array of complex values.
		return mfe.writer.WriteJSON(key, value, headers)
	default:
		return mfe.evalValueReflectionWithDefaultContentType(key, reflect.ValueOf(value), headers)
	}

	return nil
}

func (mfe *multipartFormEncoder) evalValueReflection(
	key string,
	reflectValue reflect.Value,
	contentType string,
	headers http.Header,
) error {
	value, notNull := goutils.UnwrapPointerFromReflectValue(reflectValue)
	if !notNull {
		return nil
	}

	valueKind := value.Kind()

	switch valueKind {
	case reflect.String:
		err := mfe.writer.WriteDataURI(key, value.String(), contentType, headers)
		if err != nil {
			// fallback to encode arbitrary value.
			return mfe.evalValueReflectionWithDefaultContentType(key, value, headers)
		}

		return nil
	case reflect.Slice, reflect.Array:
		if value.Elem().Kind() == reflect.Uint8 {
			return mfe.writer.WriteBinary(key, reflectValue.Bytes(), contentType, headers)
		}

		// fallback to encode arbitrary value.
		return mfe.evalValueReflectionWithDefaultContentType(key, value, headers)
	case reflect.Map:
		// Encode application/json content type for complex value or array of complex values.
		return mfe.writer.WriteJSON(key, value, headers)
	default:
		return mfe.evalValueReflectionWithDefaultContentType(key, value, headers)
	}
}

func (mfe *multipartFormEncoder) evalValueReflectionWithDefaultContentType(
	key string,
	reflectValue reflect.Value,
	headers http.Header,
) error {
	value, notNull := goutils.UnwrapPointerFromReflectValue(reflectValue)
	if !notNull {
		return nil
	}

	scalarValue, ok := goutils.FormatScalarReflection(value)
	if ok {
		return mfe.writer.WriteField(key, scalarValue, headers)
	}

	valueKind := value.Kind()

	switch valueKind {
	case reflect.Slice, reflect.Array:
		valueLength := reflectValue.Len()

		for i := range valueLength {
			elem := reflectValue.Index(i)
			k := buildArrayIndexKey(key, i)

			err := mfe.evalChildValueReflection(k, elem, headers)
			if err != nil {
				return err
			}
		}
	case reflect.Map:
		// Encode application/json content type for complex value or array of complex values.
		return mfe.writer.WriteJSON(key, value, headers)
	default:
	}

	return nil
}

func (mfe *multipartFormEncoder) evalChildValueReflection(
	key string,
	reflectValue reflect.Value,
	headers http.Header,
) error {
	value, notNull := goutils.UnwrapPointerFromReflectValue(reflectValue)
	if !notNull {
		return nil
	}

	scalarValue, ok := goutils.FormatScalarReflection(value)
	if ok {
		return mfe.writer.WriteField(key, scalarValue, headers)
	}

	valueKind := value.Kind()

	switch valueKind {
	case reflect.Slice, reflect.Array, reflect.Map:
		return mfe.writer.WriteJSON(key, value.Interface(), headers)
	default:
		return nil
	}
}

func (mfe *multipartFormEncoder) evalChildValue(key string, value any, headers http.Header) error {
	scalarValue, ok := goutils.FormatScalar(value)
	if ok {
		if scalarValue == goutils.NullStr {
			scalarValue = ""
		}

		return mfe.writer.WriteField(key, scalarValue, headers)
	}

	return mfe.writer.WriteJSON(key, value, headers)
}

func (mfe *multipartFormEncoder) evalEncoding(key string) (string, http.Header) {
	var encoding *highv3.Encoding

	if mfe.media.Encoding != nil {
		encoding, _ = mfe.media.Encoding.Get(key)
	}

	contentType := ""

	if encoding == nil {
		return contentType, make(http.Header)
	}

	headers := getHeadersFromSchema(mfe.headers, encoding.Headers)

	if encoding.ContentType != "" {
		headers.Set(httpheader.ContentType, encoding.ContentType)
		contentType = encoding.ContentType
	}

	return contentType, headers
}

func buildIndexKey(index int) string {
	return "[" + strconv.Itoa(index) + "]"
}

func buildArrayIndexKey(key string, index int) string {
	return key + buildIndexKey(index)
}
