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

package oasvalidator

import (
	"mime"
	"strings"

	"github.com/relychan/goutils/httpheader"
)

// EqualContentType checks if both content type are equal with parameters excluded.
func EqualContentType(left, right string) bool {
	leftMediaType, _, _ := strings.Cut(left, ";")
	rightMediaType, _, _ := strings.Cut(right, ";")

	return strings.EqualFold(
		strings.TrimSpace(leftMediaType),
		strings.TrimSpace(rightMediaType),
	)
}

// ValidateContentType validates the content type and prefer the application/json content type
// if the content type string has many content types.
func ValidateContentType(contentType string) (string, error) {
	if contentType == "" {
		return contentType, nil
	}

	var result string

	for item := range strings.SplitSeq(contentType, ",") {
		trimmed := strings.TrimSpace(item)

		parsed, _, err := mime.ParseMediaType(trimmed)
		if err != nil {
			continue
		}

		if parsed == httpheader.ContentTypeJSON {
			return trimmed, nil
		}

		if result == "" {
			result = trimmed
		}
	}

	if result != "" {
		return result, nil
	}

	return "", ErrInvalidContentType
}
