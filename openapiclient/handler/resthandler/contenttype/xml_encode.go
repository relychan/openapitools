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
	"encoding/xml"
	"io"
	"reflect"
	"strconv"

	"github.com/relychan/goutils"
	"github.com/relychan/openapitools/oaschema"
)

const xmlRootName = "xml"

var xmlHeaderBytes = []byte(xml.Header)

// xmlEncoder implements a dynamic XML encoder from the HTTP schema.
type xmlEncoder struct {
	encoder *xml.Encoder
}

// EncodeXML encodes the arbitrary body to XML bytes.
func EncodeXML(value any) ([]byte, error) {
	buf := new(bytes.Buffer)

	_, err := WriteXML(buf, value)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// WriteXML encodes the arbitrary body to XML and write to a writer.
func WriteXML(writer io.Writer, value any) (int, error) {
	buf := new(bytes.Buffer)

	enc := xmlEncoder{
		encoder: xml.NewEncoder(buf),
	}

	encodeError := enc.Encode(value)
	if encodeError != nil {
		return 0, encodeError
	}

	err := enc.encoder.Flush()
	if err != nil {
		return 0, newXMLEncodeError(err, "")
	}

	headerCount, err := writer.Write(xmlHeaderBytes)
	if err != nil {
		return 0, newXMLEncodeError(err, "")
	}

	contentCount, err := writer.Write(buf.Bytes())
	if err != nil {
		return 0, newXMLEncodeError(err, "")
	}

	return headerCount + contentCount, nil
}

// Encode writes the XML encoding of v to the stream.
func (enc *xmlEncoder) Encode(value any) error {
	err := enc.encoder.EncodeToken(xml.StartElement{
		Name: xml.Name{Local: xmlRootName},
	})
	if err != nil {
		return newXMLEncodeError(err, "")
	}

	err = enc.encodeField(xml.StartElement{}, value, "")
	if err != nil {
		return err
	}

	err = enc.encoder.EncodeToken(xml.EndElement{
		Name: xml.Name{Local: xmlRootName},
	})
	if err != nil {
		return newXMLEncodeError(err, "")
	}

	return nil
}

func (enc *xmlEncoder) encodeField(
	startElem xml.StartElement,
	value any,
	pointer string,
) error {
	scalarString, ok := goutils.FormatScalar(value)
	if ok {
		return enc.encodeString(startElem, scalarString, pointer)
	}

	switch val := value.(type) {
	case []string:
		start := newXMLStartElement(startElem)

		for i, v := range val {
			err := enc.encodeString(start, v, pointer+"/"+strconv.Itoa(i))
			if err != nil {
				return err
			}
		}
	case []byte:
		err := enc.encodeString(startElem, string(val), pointer)
		if err != nil {
			return err
		}
	case []any:
		start := newXMLStartElement(startElem)

		for i, v := range val {
			err := enc.encodeField(start, v, pointer+"/"+strconv.Itoa(i))
			if err != nil {
				return err
			}
		}
	case map[string]any:
		return enc.encodeStringMap(startElem, val, pointer)
	case map[any]any:
		return enc.encodeAnyMap(startElem, val, pointer)
	default:
		return enc.encodeReflection(startElem, reflect.ValueOf(value), pointer)
	}

	return nil
}

func (enc *xmlEncoder) encodeStringMap(
	startElem xml.StartElement,
	value map[string]any,
	pointer string,
) error {
	if len(value) == 0 {
		return nil
	}

	return enc.writeElement(startElem, pointer, func() error {
		for k, v := range value {
			if k == "" {
				continue
			}

			err := enc.encodeField(xml.StartElement{
				Name: xml.Name{Local: k},
			}, v, pointer+"/"+k)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func (enc *xmlEncoder) encodeAnyMap(
	startElem xml.StartElement,
	value map[any]any,
	pointer string,
) error {
	if len(value) == 0 {
		return nil
	}

	return enc.writeElement(startElem, pointer, func() error {
		for k, v := range value {
			keyStr, ok := goutils.FormatScalar(k)
			if !ok {
				return newXMLInvalidKeyStringError(reflect.TypeOf(k).Kind(), pointer)
			}

			if keyStr == "" || keyStr == goutils.NullStr {
				continue
			}

			err := enc.encodeField(xml.StartElement{
				Name: xml.Name{Local: keyStr},
			}, v, pointer+"/"+keyStr)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func (enc *xmlEncoder) encodeReflection(
	startElem xml.StartElement,
	reflectValue reflect.Value,
	pointer string,
) error {
	reflectValue, ok := goutils.UnwrapPointerFromReflectValue(reflectValue)
	if !ok {
		return nil
	}

	scalarString, ok := goutils.FormatScalarReflection(reflectValue)
	if ok {
		return enc.encodeString(startElem, scalarString, pointer)
	}

	kind := reflectValue.Kind()

	switch kind {
	case reflect.Slice, reflect.Array:
		start := newXMLStartElement(startElem)

		for i := range reflectValue.Len() {
			item := reflectValue.Index(i)

			err := enc.encodeReflection(
				start,
				item,
				pointer+"/"+strconv.Itoa(i),
			)
			if err != nil {
				return err
			}
		}
	case reflect.Map:
		return enc.encodeReflectionMap(startElem, reflectValue, pointer)
	default:
		return enc.encodeReflectionString(startElem, reflectValue, pointer)
	}

	return nil
}

func (enc *xmlEncoder) encodeReflectionMap(
	startElem xml.StartElement,
	valueMap reflect.Value,
	pointer string,
) error {
	return enc.writeElement(startElem, pointer, func() error {
		mapKeys := valueMap.MapKeys()

		for _, mapKey := range mapKeys {
			key, ok := goutils.FormatScalarReflection(mapKey)
			if !ok {
				return newXMLInvalidKeyStringError(mapKey.Kind(), pointer)
			}

			if key == "" {
				continue
			}

			value := valueMap.MapIndex(mapKey)

			err := enc.encodeReflection(xml.StartElement{
				Name: xml.Name{Local: key},
			}, value, pointer+"/"+key)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func (enc *xmlEncoder) encodeReflectionString(
	startElem xml.StartElement,
	reflectValue reflect.Value,
	pointer string,
) error {
	str, ok := goutils.FormatScalarReflection(reflectValue)
	if ok {
		return enc.encodeString(startElem, str, pointer)
	}

	return nil
}

// Encode the string value to the XML tag:
//
//	<{name} attribute="{attribute}">{value}</{name}>
func (enc *xmlEncoder) encodeString(
	startElem xml.StartElement,
	value string,
	pointer string,
) error {
	return enc.writeElement(startElem, pointer, func() error {
		return enc.encoder.EncodeToken(xml.CharData(value))
	})
}

func (enc *xmlEncoder) writeElement(
	startElement xml.StartElement,
	pointer string,
	inner func() error,
) error {
	if startElement.Name.Local != "" {
		err := enc.encoder.EncodeToken(startElement)
		if err != nil {
			return newXMLEncodeError(err, pointer)
		}
	}

	err := inner()
	if err != nil {
		return newXMLEncodeError(err, pointer)
	}

	if startElement.Name.Local != "" {
		err = enc.encoder.EncodeToken(xml.EndElement{
			Name: startElement.Name,
		})
		if err != nil {
			return newXMLEncodeError(err, pointer)
		}
	}

	return nil
}

func newXMLInvalidKeyStringError(kind reflect.Kind, pointer string) *goutils.ErrorDetail {
	return &goutils.ErrorDetail{
		Detail:  "expected the type of key is a scalar, got: " + kind.String(),
		Pointer: pointer,
		Code:    oaschema.ErrCodeEncodeBodyError,
	}
}

func newXMLEncodeError(err error, pointer string) *goutils.ErrorDetail {
	return &goutils.ErrorDetail{
		Detail:  err.Error(),
		Pointer: pointer,
		Code:    oaschema.ErrCodeEncodeBodyError,
	}
}

func newXMLStartElement(parent xml.StartElement) xml.StartElement {
	result := xml.StartElement{
		Name: xml.Name{
			Local: "value",
		},
	}

	if parent.Name.Local != "" {
		result.Name = parent.Name
	}

	return result
}
