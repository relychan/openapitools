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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/stretchr/testify/assert"
)

// TestOpenAPIResourceDefinition_MarshalJSON tests JSON marshaling
func TestOpenAPIResourceDefinition_MarshalJSON(t *testing.T) {
	t.Run("with_spec_only", func(t *testing.T) {
		def := OpenAPIResourceDefinition{
			Spec: &highv3.Document{
				Info: &base.Info{
					Title:   "Test API",
					Version: "1.0.0",
				},
			},
		}

		data, err := json.Marshal(def)
		assert.NoError(t, err)
		assert.True(t, len(data) > 0)

		// Verify it can be unmarshaled back
		var result map[string]any
		err = json.Unmarshal(data, &result)
		assert.NoError(t, err)
		assert.True(t, result["spec"] != nil)
	})

	t.Run("with_ref_only", func(t *testing.T) {
		def := OpenAPIResourceDefinition{
			Ref: "https://example.com/openapi.yaml",
		}

		data, err := json.Marshal(def)
		assert.NoError(t, err)

		var result map[string]any
		err = json.Unmarshal(data, &result)
		assert.NoError(t, err)
		assert.Equal(t, "https://example.com/openapi.yaml", result["ref"])
	})

	t.Run("with_ref_and_spec", func(t *testing.T) {
		def := OpenAPIResourceDefinition{
			Ref: "https://example.com/openapi.yaml",
			Spec: &highv3.Document{
				Info: &base.Info{
					Title:   "Test API",
					Version: "1.0.0",
				},
			},
		}

		data, err := json.Marshal(def)
		assert.NoError(t, err)

		var result map[string]any
		err = json.Unmarshal(data, &result)
		assert.NoError(t, err)
		assert.Equal(t, "https://example.com/openapi.yaml", result["ref"])
		assert.True(t, result["spec"] != nil)
	})
}

// TestOpenAPIResourceDefinition_Build tests the Build method
func TestOpenAPIResourceDefinition_Build(t *testing.T) {
	ctx := context.Background()

	t.Run("spec_only_no_ref", func(t *testing.T) {
		def := OpenAPIResourceDefinition{
			Spec: &highv3.Document{
				Info: &base.Info{
					Title:   "Test API",
					Version: "1.0.0",
				},
			},
		}

		doc, err := def.Build(ctx)
		assert.NoError(t, err)
		assert.True(t, doc != nil)
		assert.Equal(t, "Test API", doc.Info.Title)
	})

	t.Run("no_spec_no_ref_error", func(t *testing.T) {
		def := OpenAPIResourceDefinition{}

		doc, err := def.Build(ctx)
		assert.True(t, err != nil)
		assert.Equal(t, ErrResourceSpecRequired, err)
		assert.True(t, doc == nil)
	})

	t.Run("with_invalid_ref", func(t *testing.T) {
		def := OpenAPIResourceDefinition{
			Ref: "nonexistent/file.json",
		}

		doc, err := def.Build(ctx)
		assert.True(t, err != nil)
		assert.True(t, doc == nil)
	})

	t.Run("with_ref_swagger_v2", func(t *testing.T) {
		testCases := []struct {
			Ref string
		}{
			{
				Ref: "petstore2",
			},
		}

		for _, tc := range testCases {
			def := OpenAPIResourceDefinition{
				Ref: fmt.Sprintf("testdata/%s/swagger.json", tc.Ref),
			}

			doc, err := def.Build(ctx)
			assert.NoError(t, err)
			assert.True(t, doc != nil)
			assert.True(t, doc.Info != nil)

			rawYamlBytes, err := doc.Render()
			assert.NoError(t, err)

			expectedPath := fmt.Sprintf("testdata/%s/expected.yaml", tc.Ref)
			// assert.NoError(t, os.WriteFile(expectedPath, rawYamlBytes, 0664))

			newDoc, err := libopenapi.NewDocument(rawYamlBytes)
			assert.NoError(t, err)

			expectedBytes, err := os.ReadFile(expectedPath)
			assert.NoError(t, err)

			expectedRawDoc, err := libopenapi.NewDocument(expectedBytes)
			assert.NoError(t, err)

			changes, err := libopenapi.CompareDocuments(expectedRawDoc, newDoc)
			assert.NoError(t, err)
			assert.Equal(t, 0, len(changes.GetAllChanges()))
		}
	})

	t.Run("with_ref_openapi_v3", func(t *testing.T) {
		testCases := []struct {
			Ref string
		}{
			{
				Ref: "petstore3",
			},
		}

		for _, tc := range testCases {
			def := OpenAPIResourceDefinition{
				Ref: fmt.Sprintf("testdata/%s/openapi.json", tc.Ref),
			}

			doc, err := def.Build(ctx)
			assert.NoError(t, err)
			assert.True(t, doc != nil)
			assert.True(t, doc.Info != nil)

			rawYamlBytes, err := doc.Render()
			assert.NoError(t, err)

			expectedPath := fmt.Sprintf("testdata/%s/expected.yaml", tc.Ref)
			// assert.NoError(t, os.WriteFile(expectedPath, rawYamlBytes, 0664))

			newDoc, err := libopenapi.NewDocument(rawYamlBytes)
			assert.NoError(t, err)

			expectedBytes, err := os.ReadFile(expectedPath)
			assert.NoError(t, err)

			expectedRawDoc, err := libopenapi.NewDocument(expectedBytes)
			assert.NoError(t, err)

			changes, err := libopenapi.CompareDocuments(expectedRawDoc, newDoc)
			assert.NoError(t, err)
			assert.True(t, len(changes.GetAllChanges()) == 0)
		}
	})
}
