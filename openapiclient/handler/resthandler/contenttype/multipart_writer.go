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
	"maps"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strconv"

	"github.com/relychan/goutils"
	"github.com/relychan/goutils/httperror"
	"github.com/relychan/goutils/httpheader"
	"github.com/relychan/openapitools/oasvalidator"
)

// MultipartWriter extends multipart.Writer with helpers.
type MultipartWriter struct {
	*multipart.Writer
}

// NewMultipartWriter creates a MultipartWriter instance.
func NewMultipartWriter(w io.Writer) *MultipartWriter {
	return &MultipartWriter{multipart.NewWriter(w)}
}

// WriteDataURI write a file from data URI string.
func (w *MultipartWriter) WriteDataURI(
	name string,
	value any,
	contentType string,
	headers http.Header,
) error {
	b64, err := goutils.DecodeString(value)
	if err != nil {
		return newMultipartWriteError(name, err)
	}

	dataURI, err := DecodeDataURI(b64)
	if err != nil {
		return newMultipartWriteError(name, err)
	}

	if dataURI.MediaType == "" {
		dataURI.MediaType = contentType
	}

	return w.WriteBinary(name, dataURI.Data, dataURI.MediaType, headers)
}

// WriteBinary writes a binary file to the multipart form.
func (w *MultipartWriter) WriteBinary(
	name string,
	value []byte,
	contentType string,
	headers http.Header,
) error {
	h := make(textproto.MIMEHeader)
	maps.Copy(h, headers)

	h[httpheader.ContentDisposition] = []string{
		fmt.Sprintf(`form-data; name=%s; filename=%s`,
			strconv.Quote(name), strconv.Quote(name)),
	}

	if contentType == "" {
		contentType = httpheader.ContentTypeOctetStream
	}

	h[httpheader.ContentType] = []string{contentType}

	p, err := w.CreatePart(h)
	if err != nil {
		return newMultipartWriteError(name, err)
	}

	_, err = p.Write(value)
	if err != nil {
		return newMultipartWriteError(name, err)
	}

	return nil
}

// WriteJSON calls CreateFormField and then writes the given value with json encoding.
func (w *MultipartWriter) WriteJSON(fieldName string, value any, headers http.Header) error {
	bs, err := json.Marshal(value)
	if err != nil {
		return newMultipartWriteError(fieldName, err)
	}

	h := createFieldMIMEHeader(fieldName, headers)
	h[httpheader.ContentType] = []string{httpheader.ContentTypeJSON}

	p, err := w.CreatePart(h)
	if err != nil {
		return newMultipartWriteError(fieldName, err)
	}

	_, err = p.Write(bs)
	if err != nil {
		return newMultipartWriteError(fieldName, err)
	}

	return nil
}

// WriteXML calls CreateFormField and then writes the given value with XML encoding.
func (w *MultipartWriter) WriteXML(fieldName string, value any, headers http.Header) error {
	h := createFieldMIMEHeader(fieldName, headers)
	if len(h[httpheader.ContentType]) == 0 || h[httpheader.ContentType][0] == "" {
		h[httpheader.ContentType] = []string{httpheader.ContentTypeTextXML}
	}

	p, err := w.CreatePart(h)
	if err != nil {
		return newMultipartWriteError(fieldName, err)
	}

	_, err = WriteXML(p, value)
	if err != nil {
		return newMultipartWriteError(fieldName, err)
	}

	return nil
}

// WriteField calls CreateFormField and then writes the given value.
func (w *MultipartWriter) WriteField(fieldName, value string, headers http.Header) error {
	h := createFieldMIMEHeader(fieldName, headers)
	if len(h[httpheader.ContentType]) == 0 || h[httpheader.ContentType][0] == "" {
		h[httpheader.ContentType] = []string{httpheader.ContentTypeTextPlain}
	}

	p, err := w.CreatePart(h)
	if err != nil {
		return newMultipartWriteError(fieldName, err)
	}

	_, err = p.Write([]byte(value))
	if err != nil {
		return newMultipartWriteError(fieldName, err)
	}

	return nil
}

func createFieldMIMEHeader(fieldName string, headers http.Header) textproto.MIMEHeader {
	h := make(textproto.MIMEHeader)

	maps.Copy(h, headers)
	h[httpheader.ContentDisposition] = []string{
		"form-data; name=" + strconv.Quote(fieldName),
	}

	return h
}

func newMultipartWriteError(name string, err error) error {
	return &httperror.ValidationError{
		Detail:  err.Error(),
		Pointer: "/" + name,
		Code:    oasvalidator.ErrCodeMultipartFormEncodeError,
	}
}
