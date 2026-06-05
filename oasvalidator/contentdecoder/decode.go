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

package contentdecoder

import (
	"bytes"
	"io"

	"github.com/relychan/goutils/httpheader"
)

// Decode decodes the data by content type to arbitrary value.
func Decode(contentType string, rawBody io.Reader) (any, error) {
	if rawBody == nil {
		return nil, nil
	}

	switch {
	case httpheader.IsContentTypeJSON(contentType):
		return DecodeJSON(rawBody)
	case httpheader.IsContentTypeXML(contentType):
		return DecodeXML(rawBody)
	case httpheader.IsContentTypeText(contentType):
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

// Unmarshal decodes data bytes by content type to arbitrary value.
func Unmarshal(contentType string, rawBody []byte) (any, error) {
	if rawBody == nil {
		return nil, nil
	}

	switch {
	case httpheader.IsContentTypeJSON(contentType):
		return UnmarshalJSON(rawBody)
	case httpheader.IsContentTypeXML(contentType):
		return DecodeXML(bytes.NewReader(rawBody))
	case httpheader.IsContentTypeText(contentType):
		return string(rawBody), nil
	default:
		// Decode binary by default.
		return rawBody, nil
	}
}
