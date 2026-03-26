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
	"encoding/json"
	"io"

	"github.com/relychan/goutils"
	"github.com/relychan/openapitools/oaschema"
)

// Encode encodes the data by content type.
func Encode(contentType string, body any) (io.Reader, error) {
	var bodyBytes []byte

	var err error

	switch {
	case oaschema.IsContentTypeJSON(contentType):
		bodyBytes, err = json.Marshal(body)
	case oaschema.IsContentTypeXML(contentType):
		bodyBytes, err = EncodeXML(body)
	case oaschema.IsContentTypeText(contentType):
		bodyBytes, err = EncodeText(body)
	default:
		// Encode binary by default.
		bodyBytes, err = EncodeBinary(body)
	}

	if err != nil {
		return nil, &goutils.ErrorDetail{
			Code:    oaschema.ErrCodeEncodeBodyError,
			Detail:  "failed to encode request body: " + err.Error(),
			Pointer: "/body",
		}
	}

	return bytes.NewBuffer(bodyBytes), nil
}

// Decode decodes the data by content type to arbitrary value.
func Decode(contentType string, rawBody io.Reader) (any, error) {
	if rawBody == nil {
		return nil, nil
	}

	closer, ok := rawBody.(io.Closer)
	if ok {
		defer goutils.CatchWarnErrorFunc(closer.Close)
	}

	switch {
	case oaschema.IsContentTypeJSON(contentType):
		var result any

		return result, json.NewDecoder(rawBody).Decode(&result)
	case oaschema.IsContentTypeXML(contentType):
		return DecodeXML(rawBody)
	case oaschema.IsContentTypeText(contentType):
		resultBytes, err := io.ReadAll(rawBody)
		if err != nil {
			return nil, err
		}

		return string(resultBytes), nil
	default:
		// Decode binary by default.
		return io.ReadAll(rawBody)
	}
}
