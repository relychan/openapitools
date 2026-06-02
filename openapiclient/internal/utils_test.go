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

package internal

import (
	"testing"
)

func TestCutURLPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		wantSegment string
		wantRest    string
	}{
		{
			name:        "empty string",
			input:       "",
			wantSegment: "",
			wantRest:    "",
		},
		{
			name:        "plain segment no delimiter",
			input:       "users",
			wantSegment: "users",
			wantRest:    "",
		},
		{
			name:        "slash separator returns rest",
			input:       "users/123",
			wantSegment: "users",
			wantRest:    "123",
		},
		{
			name:        "slash at start gives empty segment",
			input:       "/users",
			wantSegment: "",
			wantRest:    "users",
		},
		{
			name:        "trailing slash gives empty rest",
			input:       "users/",
			wantSegment: "users",
			wantRest:    "",
		},
		{
			name:        "multiple slashes only first is cut",
			input:       "a/b/c",
			wantSegment: "a",
			wantRest:    "b/c",
		},
		{
			name:        "query string mid-string",
			input:       "search?q=hello&page=2",
			wantSegment: "search",
			wantRest:    "",
		},
		{
			name:        "query string only key no value",
			input:       "items?active",
			wantSegment: "items",
			wantRest:    "",
		},
		{
			name:        "trailing question mark",
			input:       "items?",
			wantSegment: "items",
			wantRest:    "",
		},
		{
			name:        "fragment separator returns rest",
			input:       "section#anchor",
			wantSegment: "section",
			wantRest:    "",
		},
		{
			name:        "fragment at start gives empty segment",
			input:       "#top",
			wantSegment: "",
			wantRest:    "",
		},
		{
			name:        "slash before query",
			input:       "v1/users?role=admin",
			wantSegment: "v1",
			wantRest:    "users?role=admin",
		},
		{
			name:        "invalid query string returns error",
			input:       "items?a=%ZZ",
			wantSegment: "items",
			wantRest:    "",
		},
		{
			name:        "segment with no special chars at max length",
			input:       "abcdef",
			wantSegment: "abcdef",
			wantRest:    "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			segment, rest := cutURLPath(tc.input)
			if segment != tc.wantSegment {
				t.Errorf("cutURLPath(%q) segment = %q, want %q", tc.input, segment, tc.wantSegment)
			}

			if rest != tc.wantRest {
				t.Errorf("cutURLPath(%q) rest = %q, want %q", tc.input, rest, tc.wantRest)
			}
		})
	}
}
