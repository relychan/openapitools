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
	"strings"

	"github.com/hasura/goenvconf"
)

// ReplaceURLTemplate finds and replace variables in the template string.
func ReplaceURLTemplate(input string, get goenvconf.GetEnvFunc) (string, error) {
	if input == "" {
		return "", nil
	}

	openBracketIndex := strings.IndexRune(input, '{')
	if openBracketIndex == -1 {
		return input, nil
	}

	var (
		sb        strings.Builder
		inBracket bool
		i         = openBracketIndex
	)

	strLength := len(input)
	sb.Grow(strLength)
	sb.WriteString(input[:openBracketIndex])

	for ; i < strLength; i++ {
		char := input[i]
		if char != '{' {
			sb.WriteByte(char)

			continue
		}

		i++

		inBracket = true

		if i == strLength-1 {
			return "", errUnclosedTemplateString
		}

		j := i
		// get and validate environment variable
		for ; j < strLength; j++ {
			nextChar := input[j]
			if nextChar == '}' {
				inBracket = false

				break
			}
		}

		if inBracket {
			return "", errUnclosedTemplateString
		}

		envName := input[i:j]
		if envName == "" {
			return "", errInvalidURLTemplateSyntax
		}

		value, err := get(input[i:j])
		if err != nil {
			return "", err
		}

		sb.WriteString(value)

		i = j
	}

	if inBracket {
		return "", errUnclosedTemplateString
	}

	result := sb.String()

	return result, nil
}
