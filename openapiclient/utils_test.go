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

package openapiclient

import (
	"testing"

	"github.com/hasura/goenvconf"
	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/stretchr/testify/assert"
)

func TestParseServerURL(t *testing.T) {
	serverURL := "http://localhost:8080"
	t.Setenv("GRAPHQL_SERVER_URL", serverURL)
	t.Setenv("PORT", "8080")

	testCases := []struct {
		Server   *highv3.Server
		Expected string
	}{
		{
			Server: &highv3.Server{
				URL: "{GRAPHQL_SERVER_URL}",
			},
			Expected: serverURL,
		},
		{
			Server: &highv3.Server{
				URL: "{GRAPHQL_SERVER_URL}/v1/graphql",
			},
			Expected: serverURL + "/v1/graphql",
		},
		{
			Server: &highv3.Server{
				URL: "http://{FOO}:{PORT}",
				Variables: func() *orderedmap.Map[string, *highv3.ServerVariable] {
					vars := orderedmap.New[string, *highv3.ServerVariable]()
					vars.Set("FOO", &highv3.ServerVariable{
						Default: "bar",
					})
					return vars
				}(),
			},
			Expected: "http://bar:8080",
		},
	}

	for _, tc := range testCases {
		parsedURL, err := parseServerURL(tc.Server, goenvconf.GetOSEnv)
		assert.NoError(t, err)
		assert.Equal(t, parsedURL, tc.Expected)
	}
}
