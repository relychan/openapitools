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

// Package contenttype implement encoders and decoders for data by content types.
package contenttype

import (
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
)

// DataURIEncoding represents a encoding num for the data URI.
type DataURIEncoding uint8

const (
	// DataUriASCII represents the ascii encoding for the data URI.
	DataUriASCII DataURIEncoding = iota
	// DataURIBase64 represents the base64 encoding enum for the data URI.
	DataURIBase64
)

var errEmptyDataURIContent = errors.New("invalid data uri: the content is empty")

// DataURI represents the Data URI scheme
//
// [Data URI]: https://en.wikipedia.org/wiki/Data_URI_scheme
type DataURI struct {
	MediaType  string
	Parameters map[string]string
	Data       []byte
}

// DecodeDataURI decodes data URI scheme
// data:[<media type>][;<key>=<value>][;<extension>],<data>
func DecodeDataURI(input string) (*DataURI, error) {
	rawDataURI, ok := strings.CutPrefix(input, "data:")
	if !ok {
		// without data URI, decode base64 by default
		rawDecodedBytes, err := base64.StdEncoding.DecodeString(input)
		if err != nil {
			return nil, err
		}

		return &DataURI{
			Data: rawDecodedBytes,
		}, nil
	}

	mediaType, content, found := strings.Cut(rawDataURI, ",")
	if !found || content == "" {
		return nil, errEmptyDataURIContent
	}

	dataURI, encoding := parseMediaTypeForDataURI(mediaType)

	switch encoding {
	case DataURIBase64:
		rawDecodedBytes, err := base64.StdEncoding.DecodeString(content)
		if err != nil {
			return nil, err
		}

		dataURI.Data = rawDecodedBytes
	default:
		dataURI.Data = []byte(url.PathEscape(content))
	}

	return dataURI, nil
}

func parseMediaTypeForDataURI(input string) (*DataURI, DataURIEncoding) {
	dataURI := &DataURI{
		Parameters: map[string]string{},
	}

	dataEncoding := DataUriASCII
	parts := strings.SplitSeq(input, ";")

	for part := range parts {
		trimmed := strings.TrimSpace(part)

		trimmedEnc := DataUriASCII
		if trimmed == "base64" {
			trimmedEnc = DataURIBase64
		}

		if trimmedEnc == DataURIBase64 || trimmedEnc == DataUriASCII {
			dataEncoding = trimmedEnc

			continue
		}

		key, value, found := strings.Cut(part, "=")
		if !found {
			if strings.IndexByte(part, '/') > 0 {
				dataURI.MediaType = part
			}

			continue
		}

		dataURI.Parameters[strings.TrimSpace(key)] = value
	}

	return dataURI, dataEncoding
}
