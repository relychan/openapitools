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

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/stretchr/testify/assert"
)

func TestValidateAllOf(t *testing.T) {
	testCases := []struct {
		AllOf    []*base.Schema
		Expected []string
		Nullable bool
		Error    string
	}{
		{
			AllOf: []*base.Schema{
				{
					Type: []string{Array, Object},
				},
				{
					Type: []string{Array, "int", "float"},
				},
			},
			Expected: []string{Array},
			Nullable: false,
		},
	}

	for _, tc := range testCases {
		result, nullable, err := ValidateAllOf(tc.AllOf)
		if tc.Error != "" {
			assert.ErrorContains(t, err, tc.Error)

			return
		}

		assert.True(t, err == nil)
		assert.Equal(t, tc.Nullable, nullable)
		assert.Equal(t, tc.Expected, result)
	}
}
