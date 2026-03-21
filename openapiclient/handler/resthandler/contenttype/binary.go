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
	"encoding"
	"encoding/json"

	"go.yaml.in/yaml/v4"
)

// EncodeBinary encodes the arbitrary value to bytes for binary content type.
func EncodeBinary(body any) ([]byte, error) {
	switch value := body.(type) {
	case []byte:
		return value, nil
	case encoding.BinaryMarshaler:
		if value == nil {
			return []byte{}, nil
		}

		return value.MarshalBinary()
	case yaml.Marshaler:
		return yaml.Dump(body)
	default:
		// Encode value as JSON string
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}

		return bodyBytes, nil
	}
}
