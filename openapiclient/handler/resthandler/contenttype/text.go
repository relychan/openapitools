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
	"encoding"
	"encoding/json"
	"io"

	"github.com/relychan/goutils"
	"go.yaml.in/yaml/v4"
)

// EncodeText encodes the arbitrary value to text for text/xxx content type.
func EncodeText(body any) ([]byte, error) {
	buf := new(bytes.Buffer)

	_, err := WriteText(buf, body)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// WriteText encodes the arbitrary value to text for text/xxx content type and write it into the stream.
func WriteText(writer io.Writer, body any) (int, error) {
	scalarValue, ok := goutils.FormatScalar(body)
	if ok {
		return writer.Write([]byte(scalarValue))
	}

	switch value := body.(type) {
	case []byte:
		return writer.Write(value)
	case encoding.TextMarshaler:
		if value == nil {
			return 0, nil
		}

		result, err := value.MarshalText()
		if err != nil {
			return 0, err
		}

		return writer.Write(result)
	case yaml.Marshaler:
		dumper, err := yaml.NewDumper(writer)
		if err != nil {
			return 0, err
		}

		err = dumper.Dump(body)
		if err != nil {
			return 0, err
		}

		return -1, dumper.Close()
	default:
		// Encode value as JSON string
		err := json.NewEncoder(writer).Encode(body)

		return -1, err
	}
}
