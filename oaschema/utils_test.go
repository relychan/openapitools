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
			// operation-level path params must not be hoisted into the shared path-level list.
			expectedPathParams: []*highv3.Parameter{{Name: "id", In: InPath.String()}},
			expectedOpParams: []*highv3.Parameter{
				{Name: "commentId", In: InPath.String()},
				{Name: "filter", In: InQuery.String()},
			},
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

func TestExtractCommonParametersOfOperation_NoHoisting(t *testing.T) {
	pathParams := []*highv3.Parameter{{Name: "id", In: InPath.String()}}
	get := &highv3.Operation{
		Parameters: []*highv3.Parameter{{Name: "commentId", In: InPath.String()}},
	}
	post := &highv3.Operation{}

	params := ExtractCommonParametersOfOperation(pathParams, get)
	params = ExtractCommonParametersOfOperation(params, post)

	assert.Equal(t, []*highv3.Parameter{{Name: "id", In: InPath.String()}}, params)
	assert.Len(t, pathParams, 1)
	assert.Equal(t, []*highv3.Parameter{{Name: "commentId", In: InPath.String()}}, get.Parameters)
}

func TestInheritedPathItemParameters(t *testing.T) {
	pathID := &highv3.Parameter{Name: "id", In: InPath.String(), Description: "path-level"}
	pathLimit := &highv3.Parameter{Name: "limit", In: InQuery.String()}
	opID := &highv3.Parameter{Name: "id", In: InPath.String(), Description: "operation-level"}
	opFilter := &highv3.Parameter{Name: "filter", In: InQuery.String()}

	pathParams := []*highv3.Parameter{pathID, pathLimit}
	operation := &highv3.Operation{Parameters: []*highv3.Parameter{opID, opFilter}}

	t.Run("operation_level_overrides_path_level", func(t *testing.T) {
		inherited := InheritedPathItemParameters(pathParams, operation)
		assert.Equal(t, []*highv3.Parameter{pathLimit}, inherited)

		merged := MergeOperationParameters(pathParams, operation)
		assert.Equal(t, []*highv3.Parameter{opID, opFilter, pathLimit}, merged)
	})

	t.Run("inputs_are_not_mutated", func(t *testing.T) {
		_ = MergeOperationParameters(pathParams, operation)

		assert.Equal(t, []*highv3.Parameter{pathID, pathLimit}, pathParams)
		assert.Equal(t, []*highv3.Parameter{opID, opFilter}, operation.Parameters)
	})

	t.Run("nil_operation", func(t *testing.T) {
		assert.Equal(
			t,
			[]*highv3.Parameter{pathID, pathLimit},
			MergeOperationParameters(pathParams, nil),
		)
		assert.Nil(t, InheritedPathItemParameters(nil, operation))
	})

	t.Run("operation_path_params_do_not_leak", func(t *testing.T) {
		other := &highv3.Operation{}

		_ = MergeOperationParameters(pathParams, &highv3.Operation{
			Parameters: []*highv3.Parameter{{Name: "commentId", In: InPath.String()}},
		})

		assert.Equal(
			t,
			[]*highv3.Parameter{pathID, pathLimit},
			MergeOperationParameters(pathParams, other),
		)
	})
}
