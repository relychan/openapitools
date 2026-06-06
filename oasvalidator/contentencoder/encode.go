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
	"io"

	"github.com/relychan/goutils/httpheader"
)

// Encode encodes the data by content type.
func Encode(contentType string, body any) ([]byte, error) {
	switch {
	case httpheader.IsContentTypeJSON(contentType):
		return json.Marshal(body)
	case httpheader.IsContentTypeXML(contentType):
		return EncodeXML(body)
	case httpheader.IsContentTypeText(contentType):
		return EncodeText(body)
	default:
		// Encode binary by default.
		return EncodeBinary(body)
	}
}

// Write encodes the data by content type and writes it to the stream.
func Write(writer io.Writer, contentType string, body any) (int, error) {
	switch {
	case httpheader.IsContentTypeJSON(contentType):
		return -1, json.NewEncoder(writer).Encode(body)
	case httpheader.IsContentTypeXML(contentType):
		return WriteXML(writer, body)
	case httpheader.IsContentTypeText(contentType):
		return WriteText(writer, body)
	default:
		// Encode binary by default.
		return WriteBinary(writer, body)
	}
}
