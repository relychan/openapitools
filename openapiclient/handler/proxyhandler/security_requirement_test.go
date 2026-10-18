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

package proxyhandler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/hasura/goenvconf"
	"github.com/pb33f/libopenapi"
	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildTestDocument(t *testing.T, spec string) *highv3.Document {
	t.Helper()

	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)

	model, err := doc.BuildV3Model()
	require.NoError(t, err)

	return &model.Model
}

func getOperation(t *testing.T, doc *highv3.Document, path string) *highv3.Operation {
	t.Helper()

	item, ok := doc.Paths.PathItems.Get(path)
	require.True(t, ok)
	require.NotNil(t, item.Get)

	return item.Get
}

const securityRequirementSpec = `openapi: 3.1.0
info:
  title: test
  version: "1"
security:
  - keyA: []
components:
  securitySchemes:
    keyA:
      type: apiKey
      in: header
      name: X-Key-A
      x-rely-security-credentials:
        apiKey:
          value: a
    keyB:
      type: apiKey
      in: header
      name: X-Key-B
      x-rely-security-credentials:
        apiKey:
          value: b
    unconfigured:
      type: apiKey
      in: header
      name: X-Unconfigured
paths:
  /inherit:
    get: {}
  /public:
    get:
      security: []
  /optional:
    get:
      security:
        - {}
  /ordered:
    get:
      security:
        - unconfigured: []
        - keyB: []
        - keyA: []
  /and:
    get:
      security:
        - keyA: []
          keyB: []
  /and-unsatisfiable:
    get:
      security:
        - keyA: []
          unconfigured: []
  /unknown:
    get:
      security:
        - unconfigured: []
`

func authenticateHeaders(t *testing.T, auth *OpenAPIAuthenticator, doc *highv3.Document, path string) http.Header {
	t.Helper()

	au := auth.GetAuthenticator(getOperation(t, doc, path).Security)
	if au == nil {
		return nil
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	require.NoError(t, au.Authenticate(req))

	return req.Header
}

func TestGetAuthenticator_SecurityRequirementSemantics(t *testing.T) {
	doc := buildTestDocument(t, securityRequirementSpec)

	auth, err := NewOpenAPIv3Authenticator(doc, goenvconf.GetOSEnv)
	require.NoError(t, err)

	t.Run("absent_security_inherits_document", func(t *testing.T) {
		op := getOperation(t, doc, "/inherit")
		assert.Nil(t, op.Security)

		headers := authenticateHeaders(t, auth, doc, "/inherit")
		assert.Equal(t, "a", headers.Get("X-Key-A"))
		assert.False(t, auth.IsSecurityDisabled(op.Security))
	})

	t.Run("explicit_empty_security_disables_auth", func(t *testing.T) {
		op := getOperation(t, doc, "/public")
		assert.NotNil(t, op.Security)
		assert.Empty(t, op.Security)

		assert.Nil(t, auth.GetAuthenticator(op.Security))
		assert.True(t, auth.IsSecurityDisabled(op.Security))
	})

	t.Run("optional_security", func(t *testing.T) {
		assert.Nil(t, authenticateHeaders(t, auth, doc, "/optional"))
	})

	t.Run("first_satisfiable_requirement_wins", func(t *testing.T) {
		// run many times to make sure the result is deterministic.
		for range 50 {
			headers := authenticateHeaders(t, auth, doc, "/ordered")
			assert.Equal(t, "b", headers.Get("X-Key-B"))
			assert.Empty(t, headers.Get("X-Key-A"))
		}
	})

	t.Run("and_requirement_applies_all_schemes", func(t *testing.T) {
		headers := authenticateHeaders(t, auth, doc, "/and")
		assert.Equal(t, "a", headers.Get("X-Key-A"))
		assert.Equal(t, "b", headers.Get("X-Key-B"))
	})

	t.Run("and_requirement_unsatisfiable", func(t *testing.T) {
		assert.Nil(t, authenticateHeaders(t, auth, doc, "/and-unsatisfiable"))
	})

	t.Run("unknown_scheme_does_not_fallback", func(t *testing.T) {
		assert.Nil(t, authenticateHeaders(t, auth, doc, "/unknown"))
	})
}

func TestGetAuthenticator_NoDocumentSecurity(t *testing.T) {
	doc := buildTestDocument(t, `openapi: 3.1.0
info:
  title: test
  version: "1"
components:
  securitySchemes:
    keyA:
      type: apiKey
      in: header
      name: X-Key-A
      x-rely-security-credentials:
        apiKey:
          value: a
paths:
  /pets:
    get: {}
`)

	auth, err := NewOpenAPIv3Authenticator(doc, goenvconf.GetOSEnv)
	require.NoError(t, err)

	// No security requirement anywhere: credentials must not be attached.
	assert.Nil(t, authenticateHeaders(t, auth, doc, "/pets"))
}

func TestOAuth2Authenticator_TokenLocationAndOperationScopes(t *testing.T) {
	var (
		mu     sync.Mutex
		scopes []string
	)

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()

		mu.Lock()
		scopes = strings.Fields(r.Form.Get("scope"))
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "test-token",
			"token_type":   "bearer",
			"expires_in":   3600,
		})
	}))
	defer tokenServer.Close()

	doc := buildTestDocument(t, `openapi: 3.1.0
info:
  title: test
  version: "1"
security:
  - oauth2: [read]
components:
  securitySchemes:
    oauth2:
      type: oauth2
      in: header
      name: X-Custom-Token
      scheme: custom
      flows:
        clientCredentials:
          tokenUrl: `+tokenServer.URL+`
          scopes:
            read: read
            write: write
          x-rely-security-credentials:
            clientId:
              value: id
            clientSecret:
              value: secret
paths:
  /pets:
    get:
      security:
        - oauth2: [write, read]
`)

	auth, err := NewOpenAPIv3Authenticator(doc, goenvconf.GetOSEnv)
	require.NoError(t, err)

	headers := authenticateHeaders(t, auth, doc, "/pets")
	require.NotNil(t, headers)
	assert.Equal(t, "custom test-token", headers.Get("X-Custom-Token"))
	assert.Empty(t, headers.Get("Authorization"))

	mu.Lock()
	defer mu.Unlock()

	assert.ElementsMatch(t, []string{"read", "write"}, scopes)
}
