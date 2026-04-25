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
	"errors"
	"net/http"
	"testing"

	"github.com/relychan/goutils/httperror"
	"github.com/stretchr/testify/assert"
)

// TestErrorTypes tests all error types defined in the package
func TestErrorTypes(t *testing.T) {
	testCases := []struct {
		name  string
		err   error
		check func(t *testing.T, err error)
	}{
		{
			name: "ErrWildcardMustBeLast",
			err:  ErrWildcardMustBeLast,
			check: func(t *testing.T, err error) {
				assert.True(t, errors.Is(err, ErrWildcardMustBeLast))
				assert.ErrorContains(t, err, "wildcard")
				assert.ErrorContains(t, err, "must be the last")
			},
		},
		{
			name: "ErrMissingClosingBracket",
			err:  ErrMissingClosingBracket,
			check: func(t *testing.T, err error) {
				assert.True(t, errors.Is(err, ErrMissingClosingBracket))
				assert.ErrorContains(t, err, "closing delimiter")
			},
		},
		{
			name: "ErrDuplicatedParamKey",
			err:  ErrDuplicatedParamKey,
			check: func(t *testing.T, err error) {
				assert.True(t, errors.Is(err, ErrDuplicatedParamKey))
				assert.ErrorContains(t, err, "duplicate param key")
			},
		},
		{
			name: "ErrInvalidRegexpPatternParamInRoute",
			err:  ErrInvalidRegexpPatternParamInRoute,
			check: func(t *testing.T, err error) {
				assert.True(t, errors.Is(err, ErrInvalidRegexpPatternParamInRoute))
				assert.ErrorContains(t, err, "invalid regexp")
			},
		},
		{
			name: "ErrReplaceMissingChildNode",
			err:  ErrReplaceMissingChildNode,
			check: func(t *testing.T, err error) {
				assert.True(t, errors.Is(err, ErrReplaceMissingChildNode))
				assert.ErrorContains(t, err, "replacing missing child")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.True(t, tc.err != nil)
			tc.check(t, tc.err)
		})
	}
}

// TestNewInvalidOperationMetadataError tests the error constructor
func TestNewInvalidOperationMetadataError(t *testing.T) {
	testCases := []struct {
		name           string
		method         string
		pattern        string
		originalErr    error
		expectedStatus int
		expectedCode   string
	}{
		{
			name:           "GET_request_error",
			method:         http.MethodGet,
			pattern:        "/posts/{id}",
			originalErr:    errors.New("invalid operation"),
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "invalid-operation-metadata",
		},
		{
			name:           "POST_request_error",
			method:         http.MethodPost,
			pattern:        "/users",
			originalErr:    errors.New("missing required field"),
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "invalid-operation-metadata",
		},
		{
			name:           "complex_pattern_error",
			method:         http.MethodPut,
			pattern:        "/users/{userId}/posts/{postId}",
			originalErr:    errors.New("validation failed"),
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "invalid-operation-metadata",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := newInvalidOperationMetadataError(tc.method, tc.pattern, tc.originalErr)

			assert.True(t, err != nil)

			// Check if it's an RFC9457Error
			rfc9457Err, ok := err.(httperror.HTTPError)
			assert.True(t, ok, "error should be of type RFC9457Error")

			// Verify error properties
			assert.Equal(t, tc.expectedStatus, rfc9457Err.Status)
			assert.Equal(t, tc.expectedCode, rfc9457Err.Code)
			assert.Equal(t, "Invalid Operation Metadata", rfc9457Err.Title)
			assert.Equal(t, tc.originalErr.Error(), rfc9457Err.Detail)
			assert.Equal(t, tc.method+" "+tc.pattern, rfc9457Err.Instance)
			assert.Equal(t, "about:blank", rfc9457Err.Type)
		})
	}
}

// TestNodeType tests the nodeTyp enum
func TestNodeType(t *testing.T) {
	testCases := []struct {
		name     string
		nodeType nodeType
		expected uint8
	}{
		{
			name:     "ntStatic",
			nodeType: ntStatic,
			expected: 0,
		},
		{
			name:     "ntRegexp",
			nodeType: ntRegexp,
			expected: 1,
		},
		{
			name:     "ntParam",
			nodeType: ntParam,
			expected: 2,
		},
		{
			name:     "ntCatchAll",
			nodeType: ntCatchAll,
			expected: 3,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, uint8(tc.nodeType))
		})
	}
}
