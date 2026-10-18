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

package contentencoder

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/relychan/goutils/httpheader"
	"github.com/relychan/openapitools/oaschema"
)

// ErrMultipartWriteUnsupported occurs when [Write] is called with the multipart/form-data content type.
// The multipart boundary is generated during encoding, so the final Content-Type header
// cannot be known by the caller in advance. Use [Encode] instead.
var ErrMultipartWriteUnsupported = errors.New(
	"multipart/form-data is not supported by the stream writer, use Encode instead",
)

// Encode encodes the data by content type.
// It returns the encoded bytes and the Content-Type value that must be sent with the body.
// The returned content type is the input content type, except for multipart/form-data
// whose content type includes the generated boundary parameter.
// The headers argument is the original request headers used to resolve per-part
// encoding headers of multipart/form-data bodies. It can be nil.
func Encode(
	contentType string,
	body any,
	media *oaschema.MediaType,
	headers http.Header,
) ([]byte, string, error) {
	var (
		result []byte
		err    error
	)

	switch {
	case httpheader.IsContentTypeJSON(contentType):
		result, err = json.Marshal(body)
	case httpheader.IsContentTypeXML(contentType):
		result, err = EncodeXML(body)
	case httpheader.IsContentTypeText(contentType):
		result, err = EncodeText(body)
	case httpheader.IsContentType(contentType, httpheader.ContentTypeFormURLEncoded):
		result, err = EncodeFormURLEncoded(body, media)
	case httpheader.IsContentType(contentType, httpheader.ContentTypeMultipartFormData):
		highMedia := &highv3.MediaType{}

		if media != nil {
			highMedia.Encoding = media.Encoding
		}

		return EncodeMultipartForm(body, headers, highMedia)
	default:
		// Encode binary by default.
		result, err = EncodeBinary(body)
	}

	if err != nil {
		return nil, "", err
	}

	return result, contentType, nil
}

// Write encodes the data by content type and writes it to the stream.
// The multipart/form-data content type is not supported because the boundary is generated
// while encoding. Use [Encode] instead.
func Write(writer io.Writer, contentType string, body any, media *oaschema.MediaType) (int, error) {
	switch {
	case httpheader.IsContentType(contentType, httpheader.ContentTypeMultipartFormData):
		return 0, ErrMultipartWriteUnsupported
	case httpheader.IsContentTypeJSON(contentType):
		return -1, json.NewEncoder(writer).Encode(body)
	case httpheader.IsContentTypeXML(contentType):
		return WriteXML(writer, body)
	case httpheader.IsContentTypeText(contentType):
		return WriteText(writer, body)
	case httpheader.IsContentType(contentType, httpheader.ContentTypeFormURLEncoded):
		result, err := EncodeFormURLEncoded(body, media)
		if err != nil {
			return 0, err
		}

		return writer.Write(result)
	default:
		// Encode binary by default.
		return WriteBinary(writer, body)
	}
}
