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
	"testing"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/relychan/openapitools/oaschema"
	"github.com/stretchr/testify/assert"
)

func TestEqualContentType(t *testing.T) {
	testCases := []struct {
		name     string
		left     string
		right    string
		expected bool
	}{
		{
			name:     "identical types",
			left:     "application/json",
			right:    "application/json",
			expected: true,
		},
		{
			name:     "case insensitive",
			left:     "Application/JSON",
			right:    "application/json",
			expected: true,
		},
		{
			name:     "left has parameters",
			left:     "application/json; charset=utf-8",
			right:    "application/json",
			expected: true,
		},
		{
			name:     "right has parameters",
			left:     "application/json",
			right:    "application/json; charset=utf-8",
			expected: true,
		},
		{
			name:     "both have parameters",
			left:     "application/json; charset=utf-8",
			right:    "application/json; boundary=something",
			expected: true,
		},
		{
			name:     "different types",
			left:     "application/json",
			right:    "text/plain",
			expected: false,
		},
		{
			name:     "different types with parameters",
			left:     "application/json; charset=utf-8",
			right:    "text/html; charset=utf-8",
			expected: false,
		},
		{
			name:     "left has leading space",
			left:     " application/json",
			right:    "application/json",
			expected: true,
		},
		{
			name:     "empty strings",
			left:     "",
			right:    "",
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := EqualContentType(tc.left, tc.right)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestValidateContentType(t *testing.T) {
	testCases := []struct {
		name          string
		contentType   string
		expected      string
		expectedError error
	}{
		{
			name:          "empty string returns empty without error",
			contentType:   "",
			expected:      "",
			expectedError: nil,
		},
		{
			name:          "single valid type",
			contentType:   "application/json",
			expected:      "application/json",
			expectedError: nil,
		},
		{
			name:          "single valid type with parameters",
			contentType:   "application/json; charset=utf-8",
			expected:      "application/json; charset=utf-8",
			expectedError: nil,
		},
		{
			name:          "multiple types, prefers application/json",
			contentType:   "text/plain, application/json",
			expected:      "application/json",
			expectedError: nil,
		},
		{
			name:          "multiple types, application/json with params preferred",
			contentType:   "text/plain, application/json; charset=utf-8",
			expected:      "application/json; charset=utf-8",
			expectedError: nil,
		},
		{
			name:          "multiple types without application/json returns first valid",
			contentType:   "text/plain, text/html",
			expected:      "text/plain",
			expectedError: nil,
		},
		{
			name:          "all invalid types returns error",
			contentType:   "not/a/valid/type, ///invalid",
			expected:      "",
			expectedError: ErrInvalidContentType,
		},
		{
			name:          "mix of invalid and valid types",
			contentType:   "///invalid, text/plain",
			expected:      "text/plain",
			expectedError: nil,
		},
		{
			name:          "application/json appears after other valid types",
			contentType:   "text/html, text/plain, application/json",
			expected:      "application/json",
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ValidateContentType(tc.contentType)
			assert.Equal(t, tc.expected, result)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestFindDuplicatedItems(t *testing.T) {
	t.Run("integers", func(t *testing.T) {
		testCases := []struct {
			name     string
			values   []int
			expected []int
		}{
			{
				name:     "empty slice",
				values:   []int{},
				expected: []int{},
			},
			{
				name:     "single element",
				values:   []int{1},
				expected: []int{},
			},
			{
				name:     "no duplicates",
				values:   []int{1, 2, 3},
				expected: []int{},
			},
			{
				name:     "one duplicate",
				values:   []int{1, 2, 2, 3},
				expected: []int{2},
			},
			{
				name:     "multiple duplicates",
				values:   []int{1, 2, 2, 3, 3, 4},
				expected: []int{2, 3},
			},
			{
				name:     "triplicate",
				values:   []int{1, 2, 2, 2, 3},
				expected: []int{2},
			},
			{
				name:     "all duplicates",
				values:   []int{5, 5, 5},
				expected: []int{5},
			},
			{
				name:     "unsorted input with duplicate",
				values:   []int{3, 1, 2, 1},
				expected: []int{1},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := FindDuplicatedItems(tc.values)
				assert.Equal(t, tc.expected, result)
			})
		}
	})

	t.Run("strings", func(t *testing.T) {
		testCases := []struct {
			name     string
			values   []string
			expected []string
		}{
			{
				name:     "no duplicates",
				values:   []string{"a", "b", "c"},
				expected: []string{},
			},
			{
				name:     "one duplicate",
				values:   []string{"a", "b", "b"},
				expected: []string{"b"},
			},
			{
				name:     "multiple duplicates",
				values:   []string{"a", "a", "b", "b"},
				expected: []string{"a", "b"},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := FindDuplicatedItems(tc.values)
				assert.Equal(t, tc.expected, result)
			})
		}
	})
}

// BenchmarkXxx/validate-11         	32195074	        37.19 ns/op	      56 B/op	       2 allocs/op
// BenchmarkXxx/validate_2-11       	195932468	         6.126 ns/op	       0 B/op	       0 allocs/op
// BenchmarkXxx/validate_object-11  	31653950	        36.41 ns/op	      56 B/op	       2 allocs/op
// BenchmarkXxx/validate_object_2-11         	 8644915	       138.8 ns/op	     344 B/op	       5 allocs/op
func BenchmarkValidation(b *testing.B) {
	schema := &base.Schema{
		Type: []string{oaschema.String},
	}

	b.Run("validate", func(b *testing.B) {
		for b.Loop() {
			_ = ValidateValue(schema, 65)
		}
	})

	b.Run("validate_2", func(b *testing.B) {
		for b.Loop() {
			_ = ValidateValue(schema, "test")
		}
	})

	b.Run("validate_object", func(b *testing.B) {
		value := make(map[string]any)

		for b.Loop() {
			_ = ValidateObject(schema, value)
		}
	})

	b.Run("validate_object_2", func(b *testing.B) {
		value := make(map[string]any)

		for b.Loop() {
			_ = CollectErrors(ValidateValue(schema, value))
		}
	})
}
