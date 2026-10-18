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

package oaschema

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetParameterStyleAndExplode_Defaults(t *testing.T) {
	style := func(s ParameterEncodingStyle) *ParameterEncodingStyle { return &s }

	testCases := []struct {
		name            string
		location        ParameterLocation
		style           *ParameterEncodingStyle
		explode         *bool
		expectedStyle   ParameterEncodingStyle
		expectedExplode bool
	}{
		{"query_default", InQuery, nil, nil, EncodingStyleForm, true},
		{"query_form", InQuery, style(EncodingStyleForm), nil, EncodingStyleForm, true},
		{
			"query_space_delimited",
			InQuery,
			style(EncodingStyleSpaceDelimited),
			nil,
			EncodingStyleSpaceDelimited,
			false,
		},
		{
			"query_pipe_delimited",
			InQuery,
			style(EncodingStylePipeDelimited),
			nil,
			EncodingStylePipeDelimited,
			false,
		},
		{
			"query_deep_object",
			InQuery,
			style(EncodingStyleDeepObject),
			nil,
			EncodingStyleDeepObject,
			false,
		},
		{
			"query_explicit_explode",
			InQuery,
			style(EncodingStyleDeepObject),
			new(true),
			EncodingStyleDeepObject,
			true,
		},
		{"query_form_explicit_no_explode", InQuery, nil, new(false), EncodingStyleForm, false},
		{"cookie_default", InCookie, nil, nil, EncodingStyleForm, true},
		{"cookie_style", InCookie, style(EncodingStyleCookie), nil, EncodingStyleCookie, true},
		{"path_default", InPath, nil, nil, EncodingStyleSimple, false},
		{"header_default", InHeader, nil, nil, EncodingStyleSimple, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s, explode := GetParameterStyleAndExplode(tc.location, tc.style, tc.explode)
			assert.Equal(t, tc.expectedStyle, s)
			assert.Equal(t, tc.expectedExplode, explode)
		})
	}
}
