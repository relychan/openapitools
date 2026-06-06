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
	"encoding/json"
	"fmt"
	"io"
)

// DecodeJSON decodes an arbitrary JSON from a reader stream.
func DecodeJSON(r io.Reader) (any, error) {
	var result any

	decoder := json.NewDecoder(r)

	err := decoder.Decode(&result)
	if err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	return result, nil
}

// UnmarshalJSON unmarshals an arbitrary JSON from bytes.
func UnmarshalJSON(b []byte) (any, error) {
	var result any

	err := json.Unmarshal(b, &result)
	if err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	return result, nil
}
