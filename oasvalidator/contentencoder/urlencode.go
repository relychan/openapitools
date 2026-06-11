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
	"errors"
	"fmt"
	"net/url"
	"reflect"

	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/oasvalidator/parameter"
)

var errInvalidFormURLEncodedObject = errors.New(
	"failed to encode application/x-www-form-urlencoded. Expected an object")

// EncodeFormURLEncoded encodes the arbitrary value to application/x-www-form-urlencoded content type.
func EncodeFormURLEncoded(
	value any,
	media *oaschema.MediaType,
) ([]byte, error) {
	if value == nil {
		return nil, nil
	}

	objectValue, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf(
			"%w, got %q",
			errInvalidFormURLEncodedObject,
			reflect.TypeOf(value).String(),
		)
	}

	queryValues := url.Values{}

	for key, value := range objectValue {
		param := &oaschema.Parameter{
			Name: key,
		}

		if media != nil && media.Encoding != nil {
			encoding, present := media.Encoding.Get(key)
			if present {
				param.AllowReserved = encoding.AllowReserved
				param.Explode = encoding.Explode

				if encoding.Style != "" {
					style, err := oaschema.ParseParameterEncodingStyle(encoding.Style)
					if err == nil {
						param.Style = &style
					}
				}
			}
		}

		parameter.SetQueryParam(queryValues, param, value)
	}
	// Keys and values are already escaped. Do not escape them again.
	return []byte(parameter.EncodeQueryValuesUnescape(queryValues)), nil
}
