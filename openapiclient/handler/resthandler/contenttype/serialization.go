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
	"strings"

	"github.com/relychan/goutils"
	"github.com/relychan/goutils/httpheader"
	"github.com/relychan/openapitools/oaschema"
)

// Serialize encodes the data by content type.
func Serialize(contentType string, body any) (io.Reader, error) {
	var bodyBytes []byte

	var err error

	switch {
	case strings.HasPrefix(contentType, httpheader.ContentTypeJSON):
		bodyBytes, err = json.Marshal(body)
	case IsContentTypeXML(contentType):
		bodyBytes, err = EncodeXML(body)
	case strings.HasPrefix(contentType, "text/"):
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
