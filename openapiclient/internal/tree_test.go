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
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"
	"testing"

	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/relychan/openapitools/openapiclient/handler/proxyhandler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTreeNodes(t *testing.T) {
	routes := []struct {
		Path     string
		Pattern  string
		Method   string
		Handlers *highv3.PathItem
		Route    Route
	}{
		{
			Pattern: "/posts",
			Path:    "/posts",
			Method:  http.MethodGet,
			Handlers: &highv3.PathItem{
				Get: &highv3.Operation{},
			},
			Route: Route{
				ParamValues: map[string]any{},
			},
		},
		{
			Path:    "/posts/1",
			Pattern: "/posts/{id}",
			Method:  http.MethodPost,
			Handlers: &highv3.PathItem{
				Post: &highv3.Operation{},
			},
			Route: Route{
				ParamValues: map[string]any{
					"id": "1",
				},
			},
		},
		{
			Path:    "/posts/1/comments/abc",
			Pattern: "/posts/{id}/comments/{commentId:^[a-z]+$}",
			Method:  http.MethodGet,
			Handlers: &highv3.PathItem{
				Get: &highv3.Operation{},
			},
			Route: Route{
				ParamValues: map[string]any{
					"id":        "1",
					"commentId": "abc",
				},
			},
		},
		{
			Path:    "/v1/random/route",
			Pattern: "/v1/*",
			Method:  http.MethodGet,
			Handlers: &highv3.PathItem{
				Get: &highv3.Operation{},
			},
			Route: Route{
				ParamValues: map[string]any{},
			},
		},
		{
			Path:    "/posts/1/authors/abc",
			Pattern: "/posts/{id}/authors/{authorId:^[a-z]+$}?foo=bar",
			Method:  http.MethodGet,
			Handlers: &highv3.PathItem{
				Get: &highv3.Operation{},
			},
			Route: Route{
				ParamValues: map[string]any{
					"id":       "1",
					"authorId": "abc",
				},
			},
		},
	}

	node := new(Node)

	for _, route := range routes {
		_, err := node.InsertRoute(nil, route.Pattern, route.Handlers, &proxyhandler.InsertRouteOptions{})
		assert.NoError(t, err, route.Pattern)
	}

	routeAsText := `
- / []
  - /posts [GET]
    - /{id} []
      - / [POST]
      - /authors []
        - /{authorId:^[a-z]+$} []
          - / [GET]
      - /comments []
        - /{commentId:^[a-z]+$} []
          - / [GET]
  - /v1 []
    - /* [GET]`

	assert.Equal(t, routeAsText, node.printDebug(0))
	for _, route := range routes {
		if !t.Run(route.Path, func(t *testing.T) {
			postNode, err := node.FindRoute(route.Path, route.Method)
			assert.True(t, err == nil)
			assert.True(t, postNode != nil)
			assert.Equal(t, postNode.Pattern, route.Pattern)
			assert.Equal(t, postNode.ParamValues, route.Route.ParamValues)
		}) {
			break
		}
	}

	notFoundNode, err := node.FindRoute("/posts/1/authors", http.MethodGet)
	assert.Equal(t, http.StatusNotFound, err.Status)
	assert.True(t, notFoundNode == nil)
}

// TestRouteInsertionEdgeCases tests edge cases in route insertion
func TestRouteInsertionEdgeCases(t *testing.T) {
	testCases := []struct {
		name        string
		patterns    []string
		expectError bool
		errorType   error
	}{
		{
			name: "overlapping_static_routes",
			patterns: []string{
				"/posts",
				"/posts/new",
				"/posts/123",
			},
			expectError: false,
		},
		{
			name: "param_and_static_mix",
			patterns: []string{
				"/posts/{id}",
				"/posts/new",
			},
			expectError: false,
		},
		{
			name: "multiple_params_same_level",
			patterns: []string{
				"/posts/{id}",
				"/posts/{postId}",
			},
			expectError: false,
		},
		{
			name: "regexp_patterns",
			patterns: []string{
				"/posts/{id:[0-9]+}",
				"/posts/{slug:[a-z-]+}",
			},
			expectError: false,
		},
		{
			name: "nested_params",
			patterns: []string{
				"/users/{userId}/posts/{postId}",
				"/users/{userId}/comments/{commentId}",
			},
			expectError: false,
		},
		{
			name: "catchall_routes",
			patterns: []string{
				"/api/v1/*",
				"/api/v2/*",
			},
			expectError: false,
		},
		{
			name: "root_route",
			patterns: []string{
				"/",
			},
			expectError: false,
		},
		{
			name: "deep_nesting",
			patterns: []string{
				"/a/b/c/d/e/f/g",
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			node := new(Node)

			for _, pattern := range tc.patterns {
				_, err := node.InsertRoute(nil, pattern, &highv3.PathItem{
					Get: &highv3.Operation{},
				}, &proxyhandler.InsertRouteOptions{})

				if tc.expectError {
					assert.True(t, err != nil, "expected error for pattern: %s", pattern)
					if tc.errorType != nil {
						assert.ErrorIs(t, err, tc.errorType)
					}
					return
				}

				assert.NoError(t, err, "failed to insert pattern: %s", pattern)
			}
		})
	}
}

// TestRouteFindingEdgeCases tests edge cases in route finding
func TestRouteFindingEdgeCases(t *testing.T) {
	node := new(Node)

	// Setup routes
	routes := map[string]*highv3.PathItem{
		"/":                                  {Get: &highv3.Operation{}},
		"/posts":                             {Get: &highv3.Operation{}},
		"/posts/new":                         {Get: &highv3.Operation{}},
		"/posts/{id}":                        {Get: &highv3.Operation{}, Post: &highv3.Operation{}},
		"/posts/{id:[0-9]+}":                 {Put: &highv3.Operation{}},
		"/posts/{id}/comments":               {Get: &highv3.Operation{}},
		"/posts/{id}/comments/{commentId}":   {Get: &highv3.Operation{}},
		"/users/{userId}/posts/{postId}":     {Get: &highv3.Operation{}},
		"/api/v1/*":                          {Get: &highv3.Operation{}},
		"/products/{category}/{subcategory}": {Get: &highv3.Operation{}},
		"/products/{category}/{id:[0-9]+}?foo=bar": {Get: &highv3.Operation{}},
	}

	for pattern, handlers := range routes {
		_, err := node.InsertRoute(nil, pattern, handlers, &proxyhandler.InsertRouteOptions{})
		assert.NoError(t, err, "failed to insert route: %s", pattern)
	}

	testCases := []struct {
		name            string
		path            string
		method          string
		shouldFind      bool
		expectedParams  map[string]any
		expectedPattern string
	}{
		{
			name:            "root_path",
			path:            "/",
			method:          http.MethodGet,
			shouldFind:      true,
			expectedParams:  map[string]any{},
			expectedPattern: "/",
		},
		{
			name:            "static_exact_match",
			path:            "/posts",
			method:          http.MethodGet,
			shouldFind:      true,
			expectedParams:  map[string]any{},
			expectedPattern: "/posts",
		},
		{
			name:            "static_priority_over_param",
			path:            "/posts/new",
			method:          http.MethodGet,
			shouldFind:      true,
			expectedParams:  map[string]any{},
			expectedPattern: "/posts/new",
		},
		{
			name:       "regexp_match_numeric",
			path:       "/posts/456",
			method:     http.MethodPut,
			shouldFind: true,
			expectedParams: map[string]any{
				"id": "456",
			},
			expectedPattern: "/posts/{id:[0-9]+}",
		},
		{
			name:       "nested_params",
			path:       "/posts/123/comments/456",
			method:     http.MethodGet,
			shouldFind: true,
			expectedParams: map[string]any{
				"id":        "123",
				"commentId": "456",
			},
			expectedPattern: "/posts/{id}/comments/{commentId}",
		},
		{
			name:       "multiple_params_different_paths",
			path:       "/users/user123/posts/post456",
			method:     http.MethodGet,
			shouldFind: true,
			expectedParams: map[string]any{
				"userId": "user123",
				"postId": "post456",
			},
			expectedPattern: "/users/{userId}/posts/{postId}",
		},
		{
			name:            "catchall_match",
			path:            "/api/v1/anything/goes/here",
			method:          http.MethodGet,
			shouldFind:      true,
			expectedParams:  map[string]any{},
			expectedPattern: "/api/v1/*",
		},
		{
			name:       "multiple_params_same_segment",
			path:       "/products/electronics/smartphones",
			method:     http.MethodGet,
			shouldFind: true,
			expectedParams: map[string]any{
				"category":    "electronics",
				"subcategory": "smartphones",
			},
			expectedPattern: "/products/{category}/{subcategory}",
		},
		{
			name:       "regexp_priority",
			path:       "/products/electronics/12345",
			method:     http.MethodGet,
			shouldFind: true,
			expectedParams: map[string]any{
				"category": "electronics",
				"id":       "12345",
			},
			expectedPattern: "/products/{category}/{id:[0-9]+}?foo=bar",
		},
		{
			name:       "method_not_found",
			path:       "/posts/123",
			method:     http.MethodDelete,
			shouldFind: false,
		},
		{
			name:       "path_not_found",
			path:       "/nonexistent",
			method:     http.MethodGet,
			shouldFind: false,
		},
		{
			name:       "partial_path_not_found",
			path:       "/posts/123/nonexistent",
			method:     http.MethodGet,
			shouldFind: false,
		},
		{
			name:       "double_slash_path",
			path:       "/posts//comments",
			method:     http.MethodGet,
			shouldFind: true, // Router matches this with empty param
			expectedParams: map[string]any{
				"id": "", // Empty param value
			},
			expectedPattern: "/posts/{id}/comments",
		},
		{
			name:            "trailing_slash_mismatch",
			path:            "/posts/",
			method:          http.MethodGet,
			shouldFind:      true,
			expectedPattern: "/posts",
			expectedParams:  map[string]any{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			route, err := node.FindRoute(tc.path, tc.method)

			if tc.shouldFind {
				require.Nil(t, err)
				require.True(t, route != nil, "expected to find route for path: %s", tc.path)
				require.True(t, route.Method != nil)
				assert.Equal(t, tc.expectedPattern, route.Pattern)
				assert.Equal(t, tc.expectedParams, route.ParamValues)
			} else {
				assert.Equal(t, http.StatusNotFound, err.Status)
				assert.True(t, route == nil, "expected not to find route for path: %s", tc.path)
			}
		})
	}
}

// TestComplexRoutingScenarios tests complex real-world routing scenarios
func TestComplexRoutingScenarios(t *testing.T) {
	t.Run("RESTful_API_with_versioning", func(t *testing.T) {
		node := new(Node)

		routes := []string{
			"/api/v1/users",
			"/api/v1/users/{id}",
			"/api/v1/users/{id}/posts",
			"/api/v1/users/{id}/posts/{postId}",
			"/api/v2/users",
			"/api/v2/users/{id}",
		}

		for _, route := range routes {
			_, err := node.InsertRoute(nil, route, &highv3.PathItem{
				Get: &highv3.Operation{},
			}, &proxyhandler.InsertRouteOptions{})
			assert.Nil(t, err)
		}

		// Test v1 routes
		r, err := node.FindRoute("/api/v1/users", http.MethodGet)
		assert.Nil(t, err)
		assert.True(t, r != nil)
		assert.Equal(t, "/api/v1/users", r.Pattern)

		r, err = node.FindRoute("/api/v1/users/123", http.MethodGet)
		assert.Nil(t, err)
		assert.True(t, r != nil)
		assert.Equal(t, "123", r.ParamValues["id"])

		r, err = node.FindRoute("/api/v1/users/123/posts/456", http.MethodGet)
		assert.Nil(t, err)
		assert.True(t, r != nil)
		assert.Equal(t, "123", r.ParamValues["id"])
		assert.Equal(t, "456", r.ParamValues["postId"])

		// Test v2 routes
		r, err = node.FindRoute("/api/v2/users/789", http.MethodGet)
		assert.Nil(t, err)
		assert.True(t, r != nil)
		assert.Equal(t, "/api/v2/users/{id}", r.Pattern)
	})

	t.Run("mixed_static_and_dynamic_routes", func(t *testing.T) {
		node := new(Node)

		routes := map[string]*highv3.PathItem{
			"/posts":           {Get: &highv3.Operation{}},
			"/posts/new":       {Get: &highv3.Operation{}},
			"/posts/popular":   {Get: &highv3.Operation{}},
			"/posts/{id}":      {Get: &highv3.Operation{}},
			"/posts/{id}/edit": {Get: &highv3.Operation{}},
		}

		for pattern, handlers := range routes {
			_, err := node.InsertRoute(nil, pattern, handlers, &proxyhandler.InsertRouteOptions{})
			assert.Nil(t, err)
		}

		// Static routes should take precedence
		r, err := node.FindRoute("/posts/new", http.MethodGet)
		assert.Nil(t, err)
		assert.True(t, r != nil)
		assert.Equal(t, "/posts/new", r.Pattern)

		r, err = node.FindRoute("/posts/popular", http.MethodGet)
		assert.Nil(t, err)
		assert.True(t, r != nil)
		assert.Equal(t, "/posts/popular", r.Pattern)

		// Dynamic route should match other IDs
		r, err = node.FindRoute("/posts/123", http.MethodGet)
		assert.Nil(t, err)
		assert.True(t, r != nil)
		assert.Equal(t, "/posts/{id}", r.Pattern)
		assert.Equal(t, "123", r.ParamValues["id"])

		r, err = node.FindRoute("/posts/456/edit", http.MethodGet)
		assert.Nil(t, err)
		assert.True(t, r != nil)
		assert.Equal(t, "/posts/{id}/edit", r.Pattern)
		assert.Equal(t, "456", r.ParamValues["id"])
	})

	t.Run("regexp_validation_routes", func(t *testing.T) {
		node := new(Node)

		routes := map[string]*highv3.PathItem{
			"/users/{id:[0-9]+}":                       {Get: &highv3.Operation{}},
			"/users/{username:[a-z]+}":                 {Post: &highv3.Operation{}},
			"/posts/{slug:[a-z0-9-]+}":                 {Get: &highv3.Operation{}},
			"/files/{filename:[a-zA-Z0-9._-]+}":        {Get: &highv3.Operation{}},
			"/dates/{date:[0-9]{4}-[0-9]{2}-[0-9]{2}}": {Get: &highv3.Operation{}},
		}

		for pattern, handlers := range routes {
			_, err := node.InsertRoute(nil, pattern, handlers, &proxyhandler.InsertRouteOptions{})
			assert.Nil(t, err)
		}

		// Numeric ID should match
		r, err := node.FindRoute("/users/12345", http.MethodGet)
		assert.Nil(t, err)
		assert.True(t, r != nil)
		assert.Equal(t, "12345", r.ParamValues["id"])

		// Alphabetic username should match
		r, err = node.FindRoute("/users/john", http.MethodPost)
		assert.Nil(t, err)
		assert.True(t, r != nil)
		assert.Equal(t, "john", r.ParamValues["username"])

		// Slug with hyphens should match
		r, err = node.FindRoute("/posts/my-blog-post-123", http.MethodGet)
		assert.Nil(t, err)
		assert.True(t, r != nil)
		assert.Equal(t, "my-blog-post-123", r.ParamValues["slug"])

		// Filename with dots and underscores should match
		r, err = node.FindRoute("/files/my_file.txt", http.MethodGet)
		assert.Nil(t, err)
		assert.True(t, r != nil)
		assert.Equal(t, "my_file.txt", r.ParamValues["filename"])

		// Date format should match
		r, err = node.FindRoute("/dates/2024-01-15", http.MethodGet)
		assert.Nil(t, err)
		assert.True(t, r != nil)
		assert.Equal(t, "2024-01-15", r.ParamValues["date"])

		// Invalid date format should not match
		r, err = node.FindRoute("/dates/2024-1-5", http.MethodGet)
		assert.Equal(t, http.StatusNotFound, err.Status)
		assert.True(t, r == nil)
	})

	t.Run("catchall_with_specific_routes", func(t *testing.T) {
		node := new(Node)

		routes := map[string]*highv3.PathItem{
			"/api/v1/users": {Get: &highv3.Operation{}},
			"/api/v1/posts": {Get: &highv3.Operation{}},
			"/api/v1/*":     {Get: &highv3.Operation{}},
			"/static/css/*": {Get: &highv3.Operation{}},
			"/static/js/*":  {Get: &highv3.Operation{}},
		}

		for pattern, handlers := range routes {
			_, err := node.InsertRoute(nil, pattern, handlers, &proxyhandler.InsertRouteOptions{})
			assert.Nil(t, err)
		}

		// Specific routes should match first
		r, err := node.FindRoute("/api/v1/users", http.MethodGet)
		assert.Nil(t, err)
		assert.True(t, r != nil)
		assert.Equal(t, "/api/v1/users", r.Pattern)

		// Catchall should match unspecified routes
		r, err = node.FindRoute("/api/v1/anything/else", http.MethodGet)
		assert.Nil(t, err)
		assert.True(t, r != nil)
		assert.Equal(t, "/api/v1/*", r.Pattern)

		r, err = node.FindRoute("/static/css/main.css", http.MethodGet)
		assert.Nil(t, err)
		assert.True(t, r != nil)
		assert.Equal(t, "/static/css/*", r.Pattern)
	})

	t.Run("multiple_HTTP_methods", func(t *testing.T) {
		node := new(Node)

		_, insertErr := node.InsertRoute(nil, "/posts/{id}", &highv3.PathItem{
			Get:    &highv3.Operation{},
			Post:   &highv3.Operation{},
			Put:    &highv3.Operation{},
			Patch:  &highv3.Operation{},
			Delete: &highv3.Operation{},
		}, &proxyhandler.InsertRouteOptions{})
		assert.NoError(t, insertErr)

		methods := []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
		}

		for _, method := range methods {
			r, err := node.FindRoute("/posts/123", method)
			assert.Nil(t, err)
			assert.True(t, r != nil, "method %s should be found", method)
			assert.Equal(t, "123", r.ParamValues["id"])
		}

		// Method not defined should not be found
		r, err := node.FindRoute("/posts/123", http.MethodHead)
		assert.Equal(t, http.StatusNotFound, err.Status)
		assert.True(t, r == nil)
	})

	t.Run("deeply_nested_resources", func(t *testing.T) {
		node := new(Node)

		pattern := "/orgs/{orgId}/teams/{teamId}/projects/{projectId}/tasks/{taskId}/comments/{commentId}"
		_, err := node.InsertRoute(nil, pattern, &highv3.PathItem{
			Get: &highv3.Operation{},
		}, &proxyhandler.InsertRouteOptions{})
		assert.Nil(t, err)

		r, err := node.FindRoute("/orgs/org1/teams/team2/projects/proj3/tasks/task4/comments/comment5", http.MethodGet)
		assert.Nil(t, err)
		assert.True(t, r != nil)
		assert.Equal(t, "org1", r.ParamValues["orgId"])
		assert.Equal(t, "team2", r.ParamValues["teamId"])
		assert.Equal(t, "proj3", r.ParamValues["projectId"])
		assert.Equal(t, "task4", r.ParamValues["taskId"])
		assert.Equal(t, "comment5", r.ParamValues["commentId"])
	})

	t.Run("special_characters_in_params", func(t *testing.T) {
		node := new(Node)

		routes := map[string]*highv3.PathItem{
			"/search/{query}":          {Get: &highv3.Operation{}},
			"/users/{email:[-.@\\w]+}": {Get: &highv3.Operation{}},
		}

		for pattern, handlers := range routes {
			_, err := node.InsertRoute(nil, pattern, handlers, &proxyhandler.InsertRouteOptions{})
			assert.Nil(t, err)
		}

		// Query with special characters
		r, err := node.FindRoute("/search/hello-world", http.MethodGet)
		assert.Nil(t, err)
		assert.True(t, r != nil)
		assert.Equal(t, "hello-world", r.ParamValues["query"])

		// Email-like parameter
		r, err = node.FindRoute("/users/user@example.com", http.MethodGet)
		assert.Nil(t, err)
		assert.True(t, r != nil)
		assert.Equal(t, "user@example.com", r.ParamValues["email"])
	})
}

func (n Node) printDebug(indent int) string {
	var sb strings.Builder

	sb.WriteByte('\n')

	if indent > 0 {
		sb.WriteString(strings.Repeat(" ", indent))
	}

	sb.WriteString("- /")
	sb.WriteString(n.String())

	sb.WriteString(fmt.Sprintf(" %v", slices.Collect(maps.Keys(n.handlers))))

	for _, child := range n.children {
		for _, node := range child {
			sb.WriteString(node.printDebug(indent + 2))
		}
	}

	return sb.String()
}

// TestAllHTTPMethods tests all HTTP methods.
func TestAllHTTPMethods(t *testing.T) {
	node := new(Node)

	// Test all standard HTTP methods
	_, err := node.InsertRoute(nil, "/test", &highv3.PathItem{
		Get:     &highv3.Operation{},
		Post:    &highv3.Operation{},
		Put:     &highv3.Operation{},
		Patch:   &highv3.Operation{},
		Delete:  &highv3.Operation{},
		Head:    &highv3.Operation{},
		Options: &highv3.Operation{},
		Trace:   &highv3.Operation{},
	}, &proxyhandler.InsertRouteOptions{})
	assert.Nil(t, err)

	// Test all methods can be found
	methods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
		http.MethodHead,
		http.MethodOptions,
		http.MethodTrace,
	}

	for _, method := range methods {
		r, err := node.FindRoute("/test", method)
		assert.Nil(t, err)
		assert.True(t, r != nil, "method %s should be found", method)
		assert.Equal(t, "/test", r.Pattern)
	}
}

// TestQueryMethod tests the custom QUERY method
func TestQueryMethod(t *testing.T) {
	node := new(Node)

	_, err := node.InsertRoute(nil, "/graphql", &highv3.PathItem{
		Query: &highv3.Operation{},
	}, &proxyhandler.InsertRouteOptions{})
	assert.Nil(t, err)

	r, err := node.FindRoute("/graphql", "QUERY")
	assert.Nil(t, err)
	assert.True(t, r != nil)
	assert.Equal(t, "/graphql", r.Pattern)
}

// TestAdditionalOperations tests custom operations via AdditionalOperations
func TestAdditionalOperations(t *testing.T) {
	node := new(Node)

	additionalOps := orderedmap.New[string, *highv3.Operation]()
	additionalOps.Set("CUSTOM", &highv3.Operation{})
	additionalOps.Set("ANOTHER", &highv3.Operation{})

	_, err := node.InsertRoute(nil, "/custom", &highv3.PathItem{
		AdditionalOperations: additionalOps,
	}, &proxyhandler.InsertRouteOptions{})
	assert.Nil(t, err)

	r, err := node.FindRoute("/custom", "CUSTOM")
	assert.Nil(t, err)
	assert.True(t, r != nil)
	assert.Equal(t, "/custom", r.Pattern)

	r, err = node.FindRoute("/custom", "ANOTHER")
	assert.Nil(t, err)
	assert.True(t, r != nil)
	assert.Equal(t, "/custom", r.Pattern)
}

// TestAdditionalOperationsWithNilValue tests that nil operations in AdditionalOperations are skipped
func TestAdditionalOperationsWithNilValue(t *testing.T) {
	node := new(Node)

	additionalOps := orderedmap.New[string, *highv3.Operation]()
	additionalOps.Set("VALID", &highv3.Operation{})
	additionalOps.Set("NIL", nil) // This should be skipped

	_, insertErr := node.InsertRoute(nil, "/mixed", &highv3.PathItem{
		AdditionalOperations: additionalOps,
	}, &proxyhandler.InsertRouteOptions{})
	assert.NoError(t, insertErr)

	r, err := node.FindRoute("/mixed", "VALID")
	assert.Nil(t, err)
	assert.True(t, r != nil)

	r, err = node.FindRoute("/mixed", "NIL")
	assert.Equal(t, http.StatusNotFound, err.Status)
	assert.True(t, r == nil, "nil operation should not be registered")
}

// TestMultipleMethodsOnSameRoute tests adding multiple methods to the same route
func TestMultipleMethodsOnSameRoute(t *testing.T) {
	node := new(Node)

	// Insert a route with multiple methods at once
	_, err := node.InsertRoute(nil, "/resource", &highv3.PathItem{
		Get:  &highv3.Operation{},
		Post: &highv3.Operation{},
	}, &proxyhandler.InsertRouteOptions{})
	assert.Nil(t, err)

	// Verify both methods work
	r, err := node.FindRoute("/resource", http.MethodGet)
	assert.Nil(t, err)
	assert.True(t, r != nil)

	r, err = node.FindRoute("/resource", http.MethodPost)
	assert.Nil(t, err)
	assert.True(t, r != nil)
}

// TestDuplicateCatchAll tests that duplicate catchall patterns return an error
func TestDuplicateCatchAll(t *testing.T) {
	node := new(Node)

	_, err := node.InsertRoute(nil, "/api/*", &highv3.PathItem{
		Get: &highv3.Operation{},
	}, &proxyhandler.InsertRouteOptions{})
	assert.Nil(t, err)

	// Try to insert another catchall at the same level
	_, err = node.InsertRoute(nil, "/api/*", &highv3.PathItem{
		Post: &highv3.Operation{},
	}, &proxyhandler.InsertRouteOptions{})
	assert.ErrorIs(t, err, ErrDuplicatedRoutingPattern)
}

// TestNodesSortingWithCatchAll tests sorting with catchall nodes
func TestNodesSortingWithCatchAll(t *testing.T) {
	ns := nodes{
		{typ: ntCatchAll, key: "catchall"},
		{typ: ntStatic, key: "static"},
		{typ: ntParam, key: "param"},
	}

	ns.Sort()

	// Verify Less function behavior for catchall
	assert.Equal(t, ns[2].key, "catchall", "catchall should not be less than static")
}

// TestEmptyPatternInsertion tests inserting a route with empty pattern after prefix
func TestEmptyPatternInsertion(t *testing.T) {
	node := new(Node)

	// Insert a route with trailing slash that becomes empty after prefix removal
	_, err := node.InsertRoute(nil, "/", &highv3.PathItem{
		Get: &highv3.Operation{},
	}, &proxyhandler.InsertRouteOptions{})
	assert.Nil(t, err)

	r, err := node.FindRoute("/", http.MethodGet)
	assert.Nil(t, err)
	assert.True(t, r != nil)
	assert.Equal(t, "/", r.Pattern)
}

// TestInsertChildNodeWithEmptySearch tests the empty search path in insertChildNode
func TestInsertChildNodeWithEmptySearch(t *testing.T) {
	node := new(Node)

	// First insert a parent route
	_, err := node.InsertRoute(nil, "/parent", &highv3.PathItem{
		Get: &highv3.Operation{},
	}, &proxyhandler.InsertRouteOptions{})
	assert.Nil(t, err)

	// Now insert a child that will have empty search after parent prefix
	_, err = node.InsertRoute(nil, "/parent/", &highv3.PathItem{
		Post: &highv3.Operation{},
	}, &proxyhandler.InsertRouteOptions{})
	assert.Nil(t, err)

	r, err := node.FindRoute("/parent/", http.MethodPost)
	assert.Nil(t, err)
	assert.True(t, r != nil)
}

// TestPatNextSegmentEdgeCases tests edge cases in pattern parsing
func TestPatNextSegmentEdgeCases(t *testing.T) {
	testCases := []struct {
		name          string
		pattern       string
		expectError   bool
		expectedType  nodeType
		expectedKey   string
		expectedRegex string
	}{
		{
			name:        "missing_closing_bracket",
			pattern:     "{id",
			expectError: true,
		},
		{
			name:          "param_with_colon_regex",
			pattern:       "{id:[0-9]+}",
			expectError:   false,
			expectedType:  ntRegexp,
			expectedKey:   "id",
			expectedRegex: "^[0-9]+$",
		},
		{
			name:         "simple_param",
			pattern:      "{id}",
			expectError:  false,
			expectedType: ntParam,
			expectedKey:  "id",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := patNextSegment(tc.pattern)

			if tc.expectError {
				assert.True(t, err != nil, "expected error for pattern: %s", tc.pattern)
			} else {
				assert.NoError(t, err, "unexpected error for pattern: %s", tc.pattern)
				assert.Equal(t, tc.expectedType, result.NodeType)
				if tc.expectedKey != "" {
					assert.Equal(t, tc.expectedKey, result.ParamName)
				}
				if tc.expectedRegex != "" {
					assert.Equal(t, tc.expectedRegex, result.Regexp)
				}
			}
		})
	}
}

// TestNodeStringMethod tests the String() method for different node types
func TestNodeStringMethod(t *testing.T) {
	node := new(Node)

	// Test static node
	_, err := node.InsertRoute(nil, "/users", &highv3.PathItem{
		Get: &highv3.Operation{},
	}, &proxyhandler.InsertRouteOptions{})
	assert.Nil(t, err)

	// Test param node
	_, err = node.InsertRoute(nil, "/posts/{id}", &highv3.PathItem{
		Get: &highv3.Operation{},
	}, &proxyhandler.InsertRouteOptions{})
	assert.Nil(t, err)

	// Test regexp node
	_, err = node.InsertRoute(nil, "/items/{id:[0-9]+}", &highv3.PathItem{
		Get: &highv3.Operation{},
	}, &proxyhandler.InsertRouteOptions{})
	assert.Nil(t, err)

	// Test catchall node
	_, err = node.InsertRoute(nil, "/api/*", &highv3.PathItem{
		Get: &highv3.Operation{},
	}, &proxyhandler.InsertRouteOptions{})
	assert.Nil(t, err)

	// Verify routes work
	r, err := node.FindRoute("/users", http.MethodGet)
	assert.Nil(t, err)
	assert.True(t, r != nil)

	r, err = node.FindRoute("/posts/123", http.MethodGet)
	assert.Nil(t, err)
	assert.True(t, r != nil)

	r, err = node.FindRoute("/items/456", http.MethodGet)
	assert.Nil(t, err)
	assert.True(t, r != nil)

	r, err = node.FindRoute("/api/anything", http.MethodGet)
	assert.Nil(t, err)
	assert.True(t, r != nil)
}

// TestFindMethodEdgeCases tests edge cases in findMethod
func TestFindMethodEdgeCases(t *testing.T) {
	node := new(Node)

	_, insertErr := node.InsertRoute(nil, "/test", &highv3.PathItem{
		Get:  &highv3.Operation{},
		Post: &highv3.Operation{},
	}, &proxyhandler.InsertRouteOptions{})
	assert.NoError(t, insertErr)

	// Find existing method
	r, err := node.FindRoute("/test", http.MethodGet)
	assert.Nil(t, err)
	assert.True(t, r != nil)

	// Find non-existing method
	r, err = node.FindRoute("/test", http.MethodDelete)
	assert.Equal(t, http.StatusNotFound, err.Status)
	assert.True(t, r == nil)
}

// TestExtractParametersFromOperationV3 tests parameter extraction
func TestExtractParametersFromOperationV3(t *testing.T) {
	node := new(Node)

	// Create operation with parameters
	param1 := &highv3.Parameter{}
	param1.Name = "id"
	param1.In = "path"

	param2 := &highv3.Parameter{}
	param2.Name = "query"
	param2.In = "query"

	_, err := node.InsertRoute(nil, "/users/{id}", &highv3.PathItem{
		Get: &highv3.Operation{
			Parameters: []*highv3.Parameter{param1, param2},
		},
	}, &proxyhandler.InsertRouteOptions{})
	assert.Nil(t, err)

	r, err := node.FindRoute("/users/123", http.MethodGet)
	assert.Nil(t, err)
	assert.True(t, r != nil)
	assert.Equal(t, "123", r.ParamValues["id"])
}

// TestInvalidRegexpPattern tests that invalid regexp patterns return an error
func TestInvalidRegexpPattern(t *testing.T) {
	node := new(Node)

	// Try to insert a route with an invalid regexp pattern
	_, err := node.InsertRoute(nil, "/users/{id:[0-9++}", &highv3.PathItem{
		Get: &highv3.Operation{},
	}, &proxyhandler.InsertRouteOptions{})
	// The error is wrapped, so we check that it's not nil
	assert.True(t, err != nil, "expected error for invalid regexp pattern")
}

// TestEmptyHandlers tests routes with no operations
func TestEmptyHandlers(t *testing.T) {
	node := new(Node)

	// Try to insert a route with no operations
	_, err := node.InsertRoute(nil, "/empty", &highv3.PathItem{}, &proxyhandler.InsertRouteOptions{})
	// Should return nil because no handlers were created
	assert.Nil(t, err)

	// Route should not be found
	r, err := node.FindRoute("/empty", http.MethodGet)
	assert.ErrorContains(t, err, "404")
	assert.True(t, r == nil)
}

// TestWildcardNotLast tests that wildcard must be the last segment
func TestWildcardNotLast(t *testing.T) {
	node := new(Node)

	// Try to insert a route with wildcard not at the end
	_, err := node.InsertRoute(nil, "/api/*/something", &highv3.PathItem{
		Get: &highv3.Operation{},
	}, &proxyhandler.InsertRouteOptions{})
	assert.ErrorIs(t, err, ErrWildcardMustBeLast)
}

// TestDuplicateParamKeys tests that duplicate parameter keys are handled
func TestDuplicateParamKeys(t *testing.T) {
	node := new(Node)

	// Try to insert a route with duplicate parameter keys
	// This actually succeeds because the duplicate check happens at a different level
	_, err := node.InsertRoute(nil, "/users/{id}/posts/{id}", &highv3.PathItem{
		Get: &highv3.Operation{},
	}, &proxyhandler.InsertRouteOptions{})
	// The router allows this
	assert.Nil(t, err)

	r, err := node.FindRoute("/users/user1/posts/post1", http.MethodGet)
	assert.Nil(t, err)
	assert.True(t, r != nil)
	// The first id value is kept
	assert.Equal(t, "user1", r.ParamValues["id"])
}

// TestMissingClosingBracket tests that missing closing bracket returns an error
func TestMissingClosingBracket(t *testing.T) {
	node := new(Node)

	// Try to insert a route with missing closing bracket
	_, err := node.InsertRoute(nil, "/users/{id", &highv3.PathItem{
		Get: &highv3.Operation{},
	}, &proxyhandler.InsertRouteOptions{})
	assert.ErrorIs(t, err, ErrMissingClosingBracket)
}

// TestNodeStringDefault tests the default case in String method
func TestNodeStringDefault(t *testing.T) {
	// Create a node with an invalid type
	n := Node{typ: nodeType(99)}
	result := n.String()
	assert.Equal(t, "", result)
}

// TestComplexNestedRoutes tests complex nested route scenarios
func TestComplexNestedRoutes(t *testing.T) {
	node := new(Node)

	routes := []string{
		"/api/v1/users",
		"/api/v1/users/{id}",
		"/api/v1/users/{id}/profile",
		"/api/v1/users/{id}/settings",
		"/api/v1/posts",
		"/api/v1/posts/{postId}",
		"/api/v1/posts/{postId}/comments",
		"/api/v1/posts/{postId}/comments/{commentId}",
	}

	for _, route := range routes {
		_, err := node.InsertRoute(nil, route, &highv3.PathItem{
			Get: &highv3.Operation{},
		}, &proxyhandler.InsertRouteOptions{})
		assert.NoError(t, err, "failed to insert route: %s", route)
	}

	// Test all routes can be found
	testCases := []struct {
		path           string
		expectedParams map[string]any
	}{
		{"/api/v1/users", map[string]any{}},
		{"/api/v1/users/123", map[string]any{"id": "123"}},
		{"/api/v1/users/456/profile", map[string]any{"id": "456"}},
		{"/api/v1/users/789/settings", map[string]any{"id": "789"}},
		{"/api/v1/posts", map[string]any{}},
		{"/api/v1/posts/post1", map[string]any{"postId": "post1"}},
		{"/api/v1/posts/post2/comments", map[string]any{"postId": "post2"}},
		{"/api/v1/posts/post3/comments/comment1", map[string]any{"postId": "post3", "commentId": "comment1"}},
	}

	for _, tc := range testCases {
		r, err := node.FindRoute(tc.path, http.MethodGet)
		assert.Nil(t, err)
		assert.True(t, r != nil, "route not found: %s", tc.path)
		assert.Equal(t, tc.expectedParams, r.ParamValues)
	}
}

// TestRootRouteWithTrailingSlash tests root route with trailing slash
func TestRootRouteWithTrailingSlash(t *testing.T) {
	node := new(Node)

	_, err := node.InsertRoute(nil, "/", &highv3.PathItem{
		Get: &highv3.Operation{},
	}, &proxyhandler.InsertRouteOptions{})
	assert.Nil(t, err)

	// Test with and without trailing slash
	r, err := node.FindRoute("/", http.MethodGet)
	assert.Nil(t, err)
	assert.True(t, r != nil)

	r, err = node.FindRoute("", http.MethodGet)
	assert.Nil(t, err)
	assert.True(t, r != nil)
}

// TestStaticRouteWithCommonPrefix tests static routes with common prefixes
func TestStaticRouteWithCommonPrefix(t *testing.T) {
	node := new(Node)

	routes := []string{
		"/user",
		"/users",
		"/users/list",
		"/users/active",
	}

	for _, route := range routes {
		_, err := node.InsertRoute(nil, route, &highv3.PathItem{
			Get: &highv3.Operation{},
		}, &proxyhandler.InsertRouteOptions{})
		assert.Nil(t, err)
	}

	// All routes should be findable
	for _, route := range routes {
		r, err := node.FindRoute(route, http.MethodGet)
		assert.Nil(t, err)
		assert.True(t, r != nil, "route not found: %s", route)
		assert.Equal(t, route, r.Pattern)
	}
}

// TestParamRouteWithCommonPrefix tests param routes with common prefixes
func TestParamRouteWithCommonPrefix(t *testing.T) {
	node := new(Node)

	routes := []string{
		"/users/{id}",
		"/users/{id}/posts",
		"/users/{id}/posts/{postId}",
		"/users/{userId}/comments",
	}

	for _, route := range routes {
		_, err := node.InsertRoute(nil, route, &highv3.PathItem{
			Get: &highv3.Operation{},
		}, &proxyhandler.InsertRouteOptions{})
		assert.Nil(t, err)
	}

	// Test finding routes
	r, err := node.FindRoute("/users/123", http.MethodGet)
	assert.Nil(t, err)
	assert.True(t, r != nil)
	assert.Equal(t, "123", r.ParamValues["id"])

	r, err = node.FindRoute("/users/456/posts", http.MethodGet)
	assert.Nil(t, err)
	assert.True(t, r != nil)
	assert.Equal(t, "456", r.ParamValues["id"])

	r, err = node.FindRoute("/users/789/posts/post1", http.MethodGet)
	assert.Nil(t, err)
	assert.True(t, r != nil)
	assert.Equal(t, "789", r.ParamValues["id"])
	assert.Equal(t, "post1", r.ParamValues["postId"])
}

// TestRegexpRouteWithCommonPrefix tests regexp routes with common prefixes
func TestRegexpRouteWithCommonPrefix(t *testing.T) {
	node := new(Node)

	routes := []string{
		"/items/{id:[0-9]+}",
		"/items/{id:[0-9]+}/details",
		"/items/{slug:[a-z-]+}",
	}

	for _, route := range routes {
		_, err := node.InsertRoute(nil, route, &highv3.PathItem{
			Get: &highv3.Operation{},
		}, &proxyhandler.InsertRouteOptions{})
		assert.Nil(t, err)
	}

	// Test numeric ID
	r, err := node.FindRoute("/items/123", http.MethodGet)
	assert.Nil(t, err)
	assert.True(t, r != nil)
	assert.Equal(t, "123", r.ParamValues["id"])

	// Test slug
	r, err = node.FindRoute("/items/my-item", http.MethodGet)
	assert.Nil(t, err)
	assert.True(t, r != nil)
	assert.Equal(t, "my-item", r.ParamValues["slug"])

	// Test details
	r, err = node.FindRoute("/items/456/details", http.MethodGet)
	assert.Nil(t, err)
	assert.True(t, r != nil)
	assert.Equal(t, "456", r.ParamValues["id"])
}

// TestMultipleMethodsSameRoute tests adding multiple methods to the same route
func TestMultipleMethodsSameRoute(t *testing.T) {
	node := new(Node)

	// Insert first route with GET
	_, err := node.InsertRoute(nil, "/test", &highv3.PathItem{
		Get: &highv3.Operation{},
	}, &proxyhandler.InsertRouteOptions{})
	assert.Nil(t, err)

	// The router allows adding different methods to the same route
	// by checking if handlers already exist
	r, err := node.FindRoute("/test", http.MethodGet)
	assert.Nil(t, err)
	assert.True(t, r != nil)
}

// TestDuplicateCatchAllRoute tests inserting duplicate catchall routes
func TestDuplicateCatchAllRoute(t *testing.T) {
	node := new(Node)

	// Insert first catchall
	_, err := node.InsertRoute(nil, "/api/*", &highv3.PathItem{
		Get: &highv3.Operation{},
	}, &proxyhandler.InsertRouteOptions{})
	assert.Nil(t, err)

	// Try to insert another catchall at the same level
	_, err = node.InsertRoute(nil, "/api/*", &highv3.PathItem{
		Post: &highv3.Operation{},
	}, &proxyhandler.InsertRouteOptions{})
	assert.ErrorIs(t, err, ErrDuplicatedRoutingPattern)
}

// TestPatNextSegmentWithAnchors tests pattern parsing with regex anchors
func TestPatNextSegmentWithAnchors(t *testing.T) {
	node := new(Node)

	// Insert route with anchors in regex
	_, err := node.InsertRoute(nil, "/users/{id:^[0-9]+$}", &highv3.PathItem{
		Get: &highv3.Operation{},
	}, &proxyhandler.InsertRouteOptions{})
	assert.Nil(t, err)

	r, err := node.FindRoute("/users/123", http.MethodGet)
	assert.Nil(t, err)
	assert.True(t, r != nil)
	assert.Equal(t, "123", r.ParamValues["id"])

	// Should not match non-numeric
	r, err = node.FindRoute("/users/abc", http.MethodGet)
	assert.ErrorContains(t, err, "404")
	assert.True(t, r == nil)
}

// TestFindRouteWithNoMatch tests finding a route that doesn't exist
func TestFindRouteWithNoMatch(t *testing.T) {
	node := new(Node)

	_, err := node.InsertRoute(nil, "/users", &highv3.PathItem{
		Get: &highv3.Operation{},
	}, &proxyhandler.InsertRouteOptions{})
	assert.Nil(t, err)

	// Try to find a non-existent route
	r, err := node.FindRoute("/posts", http.MethodGet)
	assert.ErrorContains(t, err, "404")
	assert.True(t, r == nil)

	// Try to find with wrong method
	r, err = node.FindRoute("/users", http.MethodPost)
	assert.ErrorContains(t, err, "404")
	assert.True(t, r == nil)
}

// TestStaticNodeSplitting tests splitting static nodes
func TestStaticNodeSplitting(t *testing.T) {
	node := new(Node)

	// Insert routes that will cause node splitting
	_, err := node.InsertRoute(nil, "/users/list", &highv3.PathItem{
		Get: &highv3.Operation{},
	}, &proxyhandler.InsertRouteOptions{})
	assert.Nil(t, err)

	_, err = node.InsertRoute(nil, "/users/active", &highv3.PathItem{
		Get: &highv3.Operation{},
	}, &proxyhandler.InsertRouteOptions{})
	assert.Nil(t, err)

	// Both routes should be findable
	r, err := node.FindRoute("/users/list", http.MethodGet)
	assert.Nil(t, err)
	assert.True(t, r != nil)

	r, err = node.FindRoute("/users/active", http.MethodGet)
	assert.Nil(t, err)
	assert.True(t, r != nil)
}

// TestParamNodeWithDifferentKeys tests param nodes with different keys
func TestParamNodeWithDifferentKeys(t *testing.T) {
	node := new(Node)

	// Insert routes with different param keys at the same level
	_, err := node.InsertRoute(nil, "/users/{userId}", &highv3.PathItem{
		Get: &highv3.Operation{},
	}, &proxyhandler.InsertRouteOptions{})
	assert.Nil(t, err)

	_, err = node.InsertRoute(nil, "/users/{id}", &highv3.PathItem{
		Post: &highv3.Operation{},
	}, &proxyhandler.InsertRouteOptions{})
	assert.Nil(t, err)

	// Both should work
	r, err := node.FindRoute("/users/123", http.MethodGet)
	assert.Nil(t, err)
	assert.True(t, r != nil)
	assert.Equal(t, "123", r.ParamValues["userId"])

	r, err = node.FindRoute("/users/456", http.MethodPost)
	assert.Nil(t, err)
	assert.True(t, r != nil)
	assert.Equal(t, "456", r.ParamValues["id"])
}

// TestRegexpNodeWithDifferentPatterns tests regexp nodes with different patterns
func TestRegexpNodeWithDifferentPatterns(t *testing.T) {
	node := new(Node)

	// Insert routes with different regexp patterns
	_, err := node.InsertRoute(nil, "/items/{id:[0-9]+}", &highv3.PathItem{
		Get: &highv3.Operation{},
	}, &proxyhandler.InsertRouteOptions{})
	assert.Nil(t, err)

	_, err = node.InsertRoute(nil, "/items/{slug:[a-z-]+}", &highv3.PathItem{
		Post: &highv3.Operation{},
	}, &proxyhandler.InsertRouteOptions{})
	assert.Nil(t, err)

	// Numeric should match GET
	r, err := node.FindRoute("/items/123", http.MethodGet)
	assert.Nil(t, err)
	assert.True(t, r != nil)

	// Slug should match POST
	r, err = node.FindRoute("/items/my-item", http.MethodPost)
	assert.Nil(t, err)
	assert.True(t, r != nil)
}

// goos: darwin
// goarch: arm64
// pkg: github.com/relychan/openapitools/openapiclient/internal
// cpu: Apple M3 Pro
// BenchmarkTree/insert_routes-11         	  258915	      4612 ns/op	   14736 B/op	     156 allocs/op
// BenchmarkTree/find_route-11            	 3123817	       383.0 ns/op	     408 B/op	       5 allocs/op
func BenchmarkTree(b *testing.B) {
	routes := map[string]*highv3.PathItem{
		"/posts":                   {Get: &highv3.Operation{}},
		"/posts/new":               {Get: &highv3.Operation{}},
		"/posts/popular":           {Get: &highv3.Operation{}},
		"/posts/{id}":              {Get: &highv3.Operation{}},
		"/posts/{id}/edit":         {Get: &highv3.Operation{}},
		"/users/{id:[0-9]+}":       {Get: &highv3.Operation{}},
		"/posts/{slug:[a-z0-9-]+}": {Get: &highv3.Operation{}},
	}

	b.Run("insert_routes", func(b *testing.B) {
		for b.Loop() {
			node := new(Node)

			for pattern, handlers := range routes {
				_, err := node.InsertRoute(nil, pattern, handlers, &proxyhandler.InsertRouteOptions{})
				if err != nil {
					panic(err)
				}
			}
		}
	})

	b.Run("find_route", func(b *testing.B) {
		node := new(Node)

		for pattern, handlers := range routes {
			_, err := node.InsertRoute(nil, pattern, handlers, &proxyhandler.InsertRouteOptions{})
			if err != nil {
				b.Fatal(err)
			}
		}

		for b.Loop() {
			node.FindRoute("/posts/hello", http.MethodGet)
		}
	})
}
