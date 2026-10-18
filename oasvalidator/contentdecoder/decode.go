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
	"bufio"
	"bytes"
	"errors"
	"io"
	"net/http"

	"github.com/relychan/goutils/httpheader"
)

// Decode decodes the data by content type to arbitrary value.
// An empty JSON or XML body is decoded to nil without error.
func Decode(contentType string, rawBody io.Reader) (any, error) {
	if rawBody == nil || rawBody == http.NoBody {
		return nil, nil
	}

	switch {
	case httpheader.IsContentTypeJSON(contentType):
		reader, isEmpty, err := peekEmptyReader(rawBody)
		if err != nil || isEmpty {
			return nil, err
		}

		return DecodeJSON(reader)
	case httpheader.IsContentTypeXML(contentType):
		reader, isEmpty, err := peekEmptyReader(rawBody)
		if err != nil || isEmpty {
			return nil, err
		}

		return DecodeXML(reader)
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
// An empty JSON or XML body is decoded to nil without error.
func Unmarshal(contentType string, rawBody []byte) (any, error) {
	if rawBody == nil {
		return nil, nil
	}

	switch {
	case httpheader.IsContentTypeJSON(contentType):
		if len(rawBody) == 0 {
			return nil, nil
		}

		return UnmarshalJSON(rawBody)
	case httpheader.IsContentTypeXML(contentType):
		if len(rawBody) == 0 {
			return nil, nil
		}

		return DecodeXML(bytes.NewReader(rawBody))
	case httpheader.IsContentTypeText(contentType):
		return string(rawBody), nil
	default:
		// Decode binary by default.
		return rawBody, nil
	}
}

// peekEmptyReader checks if the reader is empty without consuming its content.
// It returns a reader that must be used for further reads.
func peekEmptyReader(reader io.Reader) (io.Reader, bool, error) {
	bufReader := bufio.NewReader(reader)

	_, err := bufReader.Peek(1)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, true, nil
		}

		return nil, false, err
	}

	return bufReader, false, nil
}
