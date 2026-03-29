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

	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/stretchr/testify/assert"
)

func TestExtractCommonParametersOfOperation(t *testing.T) {
	testCases := []struct {
		name               string
		pathParams         []*highv3.Parameter
		operation          *highv3.Operation
		expectedPathParams []*highv3.Parameter
		expectedOpParams   []*highv3.Parameter
	}{
		{
			name:               "nil operation",
			pathParams:         []*highv3.Parameter{{Name: "id", In: InPath.String()}},
			operation:          nil,
			expectedPathParams: []*highv3.Parameter{{Name: "id", In: InPath.String()}},
			expectedOpParams:   nil,
		},
		{
			name:               "operation with no parameters",
			pathParams:         []*highv3.Parameter{{Name: "id", In: InPath.String()}},
			operation:          &highv3.Operation{Parameters: []*highv3.Parameter{}},
			expectedPathParams: []*highv3.Parameter{{Name: "id", In: InPath.String()}},
			expectedOpParams:   []*highv3.Parameter{},
		},
		{
			name:       "operation with duplicate path parameter",
			pathParams: []*highv3.Parameter{{Name: "id", In: InPath.String()}},
			operation: &highv3.Operation{
				Parameters: []*highv3.Parameter{
					{Name: "id", In: InPath.String()},
					{Name: "filter", In: InQuery.String()},
				},
			},
			expectedPathParams: []*highv3.Parameter{{Name: "id", In: InPath.String()}},
			expectedOpParams:   []*highv3.Parameter{{Name: "filter", In: InQuery.String()}},
		},
		{
			name:       "operation with new path parameter",
			pathParams: []*highv3.Parameter{{Name: "id", In: InPath.String()}},
			operation: &highv3.Operation{
				Parameters: []*highv3.Parameter{
					{Name: "commentId", In: InPath.String()},
					{Name: "filter", In: InQuery.String()},
				},
			},
			expectedPathParams: []*highv3.Parameter{
				{Name: "id", In: InPath.String()},
				{Name: "commentId", In: InPath.String()},
			},
			expectedOpParams: []*highv3.Parameter{{Name: "filter", In: InQuery.String()}},
		},
		{
			name:       "operation with query and header parameters",
			pathParams: []*highv3.Parameter{{Name: "id", In: InPath.String()}},
			operation: &highv3.Operation{
				Parameters: []*highv3.Parameter{
					{Name: "filter", In: InQuery.String()},
					{Name: "Authorization", In: InHeader.String()},
				},
			},
			expectedPathParams: []*highv3.Parameter{{Name: "id", In: InPath.String()}},
			expectedOpParams: []*highv3.Parameter{
				{Name: "filter", In: InQuery.String()},
				{Name: "Authorization", In: InHeader.String()},
			},
		},
		{
			name:       "operation with same name but different location",
			pathParams: []*highv3.Parameter{{Name: "id", In: InPath.String()}},
			operation: &highv3.Operation{
				Parameters: []*highv3.Parameter{
					{Name: "id", In: InQuery.String()},
				},
			},
			expectedPathParams: []*highv3.Parameter{{Name: "id", In: InPath.String()}},
			expectedOpParams:   []*highv3.Parameter{{Name: "id", In: InQuery.String()}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Make a copy of pathParams to avoid mutation affecting the test
			pathParamsCopy := make([]*highv3.Parameter, len(tc.pathParams))
			copy(pathParamsCopy, tc.pathParams)

			result := ExtractCommonParametersOfOperation(pathParamsCopy, tc.operation)

			assert.Equal(t, tc.expectedPathParams, result)
			if tc.operation != nil {
				assert.Equal(t, tc.expectedOpParams, tc.operation.Parameters)
			}
		})
	}
}

func TestMergeParameters(t *testing.T) {
	testCases := []struct {
		name     string
		dest     []*highv3.Parameter
		src      []*highv3.Parameter
		expected []*highv3.Parameter
	}{
		{
			name:     "empty dest and src",
			dest:     []*highv3.Parameter{},
			src:      []*highv3.Parameter{},
			expected: []*highv3.Parameter{},
		},
		{
			name:     "empty src",
			dest:     []*highv3.Parameter{{Name: "id", In: InPath.String()}},
			src:      []*highv3.Parameter{},
			expected: []*highv3.Parameter{{Name: "id", In: InPath.String()}},
		},
		{
			name: "empty dest",
			dest: []*highv3.Parameter{},
			src:  []*highv3.Parameter{{Name: "id", In: InPath.String()}},
			expected: []*highv3.Parameter{
				{Name: "id", In: InPath.String()},
			},
		},
		{
			name: "merge without duplicates",
			dest: []*highv3.Parameter{
				{Name: "id", In: InPath.String()},
			},
			src: []*highv3.Parameter{
				{Name: "filter", In: InQuery.String()},
			},
			expected: []*highv3.Parameter{
				{Name: "id", In: InPath.String()},
				{Name: "filter", In: InQuery.String()},
			},
		},
		{
			name: "merge with duplicate - src overrides dest",
			dest: []*highv3.Parameter{
				{Name: "id", In: InPath.String(), Required: new(true)},
			},
			src: []*highv3.Parameter{
				{Name: "id", In: InPath.String(), Required: new(false)},
			},
			expected: []*highv3.Parameter{
				{Name: "id", In: InPath.String(), Required: new(false)},
			},
		},
		{
			name: "merge with same name but different location",
			dest: []*highv3.Parameter{
				{Name: "id", In: InPath.String()},
			},
			src: []*highv3.Parameter{
				{Name: "id", In: InQuery.String()},
			},
			expected: []*highv3.Parameter{
				{Name: "id", In: InPath.String()},
				{Name: "id", In: InQuery.String()},
			},
		},
		{
			name: "merge multiple parameters",
			dest: []*highv3.Parameter{
				{Name: "id", In: InPath.String()},
				{Name: "filter", In: InQuery.String()},
			},
			src: []*highv3.Parameter{
				{Name: "filter", In: InQuery.String(), Required: new(true)},
				{Name: "sort", In: InQuery.String()},
				{Name: "Authorization", In: InHeader.String()},
			},
			expected: []*highv3.Parameter{
				{Name: "id", In: InPath.String()},
				{Name: "filter", In: InQuery.String(), Required: new(true)},
				{Name: "sort", In: InQuery.String()},
				{Name: "Authorization", In: InHeader.String()},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := MergeParameters(tc.dest, tc.src)
			assert.Equal(t, tc.expected, result)
		})
	}
}
