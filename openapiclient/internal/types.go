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

	"github.com/relychan/goutils"
)

var (
	ErrWildcardMustBeLast = errors.New(
		"wildcard '*' must be the last value in a route. trim trailing text or use a '{param}' instead",
	)
	ErrMissingClosingBracket            = errors.New("route param closing delimiter '}' is missing")
	ErrParamKeyRequired                 = errors.New("param key must not be empty")
	ErrDuplicatedParamKey               = errors.New("routing pattern contains duplicate param key")
	ErrInvalidParamPattern              = errors.New("invalid param pattern")
	ErrDuplicatedRoutingPattern         = errors.New("routing pattern is duplicated")
	ErrInvalidRegexpPatternParamInRoute = errors.New("invalid regexp pattern in route param")
	ErrReplaceMissingChildNode          = errors.New("replacing missing child node")
)

// Route holds parameter values from the request path.
type Route struct {
	Pattern     string
	Method      *MethodHandler
	ParamValues map[string]any
}

// IsRequestBodyRequired checks if the request body of this route is required.
func (r Route) IsRequestBodyRequired() bool {
	return r.Method != nil &&
		r.Method.Operation != nil &&
		r.Method.Operation.RequestBody != nil &&
		r.Method.Operation.RequestBody.Required != nil &&
		*r.Method.Operation.RequestBody.Required
}

func newInvalidOperationMetadataError(method string, pattern string, err error) error {
	return goutils.RFC9457Error{
		Type:     "about:blank",
		Title:    "Invalid Operation Metadata",
		Detail:   err.Error(),
		Status:   http.StatusBadRequest,
		Code:     "invalid-operation-metadata",
		Instance: method + " " + pattern,
	}
}
