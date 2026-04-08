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
	"log"
	"os"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/relychan/goutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"
)

func TestOpenAPIResourceDefinition_UnmarshalJSON(t *testing.T) {
	testCases := []struct {
		name        string
		jsonData    string
		expectError bool
		checkFunc   func(*testing.T, *OpenAPIResourceDefinition)
	}{
		{
			name: "valid minimal spec",
			jsonData: `{
				"spec": {
					"openapi": "3.0.0",
					"info": {
						"title": "Test API",
						"version": "1.0.0"
					},
					"paths": {}
				}
			}`,
			expectError: false,
			checkFunc: func(t *testing.T, def *OpenAPIResourceDefinition) {
				assert.True(t, len(def.Spec.Content) > 0)
			},
		},
		{
			name: "valid spec with settings",
			jsonData: `{
				"settings": {
					"basePath": "/api/v1"
				},
				"spec": {
					"openapi": "3.0.0",
					"info": {
						"title": "Test API",
						"version": "1.0.0"
					},
					"paths": {}
				}
			}`,
			expectError: false,
			checkFunc: func(t *testing.T, def *OpenAPIResourceDefinition) {
				assert.True(t, len(def.Spec.Content) > 0)
			},
		},
		{
			name:        "invalid json",
			jsonData:    `{"spec": invalid}`,
			expectError: true,
			checkFunc:   nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var def OpenAPIResourceDefinition
			err := json.Unmarshal([]byte(tc.jsonData), &def)
			if tc.expectError {
				assert.True(t, err != nil, "expected error but got nil")
			} else {
				assert.NoError(t, err)
				if tc.checkFunc != nil {
					tc.checkFunc(t, &def)
				}
			}
		})
	}
}

func TestOpenAPIResourceDefinition_UnmarshalYAML(t *testing.T) {
	testCases := []struct {
		name        string
		yamlData    string
		expectError bool
		checkFunc   func(*testing.T, *OpenAPIResourceDefinition)
	}{
		{
			name: "valid minimal spec with servers and paths",
			yamlData: `spec:
  openapi: "3.0.0"
  info:
    title: Test API
    version: "1.0.0"
  servers:
    - url: "{SERVER_URL}"
      variables:
        SERVER_URL: 
          default: https://api.example.com
  paths:
    /users:
      get:
        operationId: getUsers`,
			expectError: false,
			checkFunc: func(t *testing.T, def *OpenAPIResourceDefinition) {
				assert.True(t, len(def.Spec.Content) > 0)
			},
		},
		{
			name: "valid spec with settings",
			yamlData: `settings:
  basePath: /api/v1
spec:
  openapi: "3.0.0"
  info:
    title: Test API
    version: "1.0.0"
  servers:
    - url: https://api.example.com
  paths: {}`,
			expectError: false,
			checkFunc: func(t *testing.T, def *OpenAPIResourceDefinition) {
				assert.True(t, len(def.Spec.Content) > 0)
			},
		},
		{
			name: "invalid spec format",
			yamlData: `spec:
		  invalid: data`,
			expectError: true,
			checkFunc:   nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var def OpenAPIResourceDefinition
			err := yaml.Load([]byte(tc.yamlData), &def)

			if tc.expectError {
				require.True(t, err != nil, "expected error but got nil")
			} else {
				require.NoError(t, err)
				if tc.checkFunc != nil {
					tc.checkFunc(t, &def)
				}
			}
		})
	}
}

// TestOpenAPIResourceDefinition_MarshalJSON tests JSON marshaling
func TestOpenAPIResourceDefinition_MarshalJSON(t *testing.T) {
	rawSpec, err := yaml.Dump(&highv3.Document{
		Info: &base.Info{
			Title:   "Test API",
			Version: "1.0.0",
		},
	})
	assert.NoError(t, err)

	var spec yaml.Node

	err = yaml.Load(rawSpec, &spec)
	assert.NoError(t, err)

	t.Run("with_spec_only", func(t *testing.T) {
		def := OpenAPIResourceDefinition{
			Spec: &spec,
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
			Ref:  "https://example.com/openapi.yaml",
			Spec: &spec,
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
		rawSpec, err := yaml.Dump(&highv3.Document{
			Version: "3.0.0",
			Info: &base.Info{
				Title:   "Test API",
				Version: "1.0.0",
			},
		})
		assert.NoError(t, err)

		var spec yaml.Node

		err = yaml.Load(rawSpec, &spec)
		assert.NoError(t, err)

		def := OpenAPIResourceDefinition{
			Spec: &spec,
		}

		doc, _, err := def.Build(ctx)
		assert.NoError(t, err)
		assert.True(t, doc != nil)
		assert.Equal(t, "Test API", doc.Info.Title)
	})

	t.Run("no_spec_no_ref_error", func(t *testing.T) {
		def := OpenAPIResourceDefinition{}

		doc, _, err := def.Build(ctx)
		assert.True(t, err != nil)
		assert.Equal(t, ErrResourceSpecRequired, err)
		assert.True(t, doc == nil)
	})

	t.Run("with_invalid_ref", func(t *testing.T) {
		def := OpenAPIResourceDefinition{
			Ref: "nonexistent/file.json",
		}

		doc, _, err := def.Build(ctx)
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

			doc, _, err := def.Build(ctx)
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

			doc, _, err := def.Build(ctx)
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

	t.Run("with_overlay_patch", func(t *testing.T) {
		def, err := goutils.ReadJSONOrYAMLFile[OpenAPIResourceDefinition](
			context.Background(),
			"./testdata/test.yaml",
		)
		require.NoError(t, err)

		doc, _, err := def.Build(context.Background())
		require.NoError(t, err)

		infoNode, ok := doc.Info.Extensions.Get("x-overlay-applied")
		assert.True(t, ok)
		assert.Equal(t, "structured-overlay", infoNode.Value)
		rootPath, ok := doc.Paths.PathItems.Get("/")
		assert.True(t, ok)
		assert.Equal(t, "Retrieve the root resource", rootPath.Get.Summary)
		authBasic, ok := doc.Paths.PathItems.Get("/auth/basic")
		assert.True(t, ok)
		assert.Equal(t, 1, len(authBasic.Get.Security))
	})
}

func TestOpenAPIResourceDefinition_UnmarshalJSON_WithPatches(t *testing.T) {
	testCases := []struct {
		name        string
		jsonData    string
		expectError bool
		checkFunc   func(*testing.T, *OpenAPIResourceDefinition)
	}{
		{
			name: "spec_with_patches",
			jsonData: `{
				"spec": {
					"openapi": "3.0.0",
					"info": {"title": "Test", "version": "1.0.0"},
					"paths": {}
				},
				"patches": [
					{"target": "$", "update": {"info": {"x-overlay-applied": "test"}}}
				]
			}`,
			expectError: false,
			checkFunc: func(t *testing.T, def *OpenAPIResourceDefinition) {
				assert.NotNil(t, def.Patches)
				assert.True(t, len(def.Spec.Content) > 0)
			},
		},
		{
			name: "ref_with_patches",
			jsonData: `{
				"ref": "testdata/petstore3/openapi.json",
				"patches": [
					{"target": "$", "update": {"info": {"x-overlay-applied": "test"}}}
				]
			}`,
			expectError: false,
			checkFunc: func(t *testing.T, def *OpenAPIResourceDefinition) {
				assert.NotNil(t, def.Patches)
				assert.Equal(t, "testdata/petstore3/openapi.json", def.Ref)
			},
		},
		{
			name: "invalid_patches_json",
			jsonData: `{
				"spec": {"openapi": "3.0.0", "info": {"title": "T", "version": "1"}, "paths": {}},
				"patches": invalid
			}`,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var def OpenAPIResourceDefinition
			err := json.Unmarshal([]byte(tc.jsonData), &def)
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				if tc.checkFunc != nil {
					tc.checkFunc(t, &def)
				}
			}
		})
	}
}

func TestOpenAPIResourceDefinition_UnmarshalYAML_WithPatches(t *testing.T) {
	testCases := []struct {
		name        string
		yamlData    string
		expectError bool
		checkFunc   func(*testing.T, *OpenAPIResourceDefinition)
	}{
		{
			name: "spec_with_patches",
			yamlData: `
spec:
  openapi: "3.0.0"
  info:
    title: Test API
    version: "1.0.0"
  paths: {}
patches:
  - target: "$"
    update:
      info:
        x-overlay-applied: test-overlay
`,
			expectError: false,
			checkFunc: func(t *testing.T, def *OpenAPIResourceDefinition) {
				assert.NotNil(t, def.Spec)
				assert.NotNil(t, def.Patches)
			},
		},
		{
			name: "ref_with_patches",
			yamlData: `
ref: testdata/petstore3/openapi.json
patches:
  - target: "$"
    update:
      info:
        x-overlay-applied: test-overlay
`,
			expectError: false,
			checkFunc: func(t *testing.T, def *OpenAPIResourceDefinition) {
				assert.Equal(t, "testdata/petstore3/openapi.json", def.Ref)
				assert.NotNil(t, def.Patches)
			},
		},
		{
			name: "patches_without_spec_or_ref",
			yamlData: `
patches:
  - target: "$"
    update:
      info:
        x-overlay-applied: test
`,
			expectError: false,
			checkFunc: func(t *testing.T, def *OpenAPIResourceDefinition) {
				assert.NotNil(t, def.Patches)
				assert.Nil(t, def.Spec)
				assert.Equal(t, "", def.Ref)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var def OpenAPIResourceDefinition
			err := yaml.Load([]byte(tc.yamlData), &def)
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				if tc.checkFunc != nil {
					tc.checkFunc(t, &def)
				}
			}
		})
	}
}

func TestOpenAPIResourceDefinition_MarshalJSON_WithPatches(t *testing.T) {
	t.Run("with_patches", func(t *testing.T) {
		patchYAML := `- target: "$"
  update:
    info:
      x-overlay-applied: test`

		var patchNode yaml.Node
		require.NoError(t, yaml.Unmarshal([]byte(patchYAML), &patchNode))

		def := OpenAPIResourceDefinition{
			Patches: patchNode.Content[0],
		}

		data, err := json.Marshal(def)
		require.NoError(t, err)

		var result map[string]any
		require.NoError(t, json.Unmarshal(data, &result))
		assert.NotNil(t, result["patches"])
	})

	t.Run("round_trip_patches_via_yaml", func(t *testing.T) {
		original := `
spec:
  openapi: "3.0.0"
  info:
    title: Round Trip
    version: "1.0.0"
  paths: {}
patches:
  - target: "$"
    update:
      info:
        x-overlay-applied: round-trip
`
		var def OpenAPIResourceDefinition
		require.NoError(t, yaml.Load([]byte(original), &def))
		assert.NotNil(t, def.Patches)

		data, err := json.Marshal(def)
		require.NoError(t, err)

		var result map[string]any
		require.NoError(t, json.Unmarshal(data, &result))
		assert.NotNil(t, result["patches"])
	})
}

func TestOpenAPIResourceDefinition_Build_WithPatches(t *testing.T) {
	ctx := context.Background()

	t.Run("spec_bytes_with_patches_via_json", func(t *testing.T) {
		jsonData := `{
			"spec": {
				"openapi": "3.0.0",
				"info": {"title": "Patched API", "version": "1.0.0"},
				"paths": {}
			},
			"patches": [
				{"target": "$", "update": {"info": {"description": "json-patch"}}}
			]
		}`

		var def OpenAPIResourceDefinition
		require.NoError(t, json.Unmarshal([]byte(jsonData), &def))

		doc, warnings, err := def.Build(ctx)
		require.NoError(t, err)
		require.NotNil(t, doc)
		require.Equal(t, 0, len(warnings))
		require.Equal(t, "Patched API", doc.Info.Title)
		require.Equal(t, "json-patch", doc.Info.Description)
	})

	t.Run("spec_node_with_patches_via_yaml", func(t *testing.T) {
		yamlData := `
spec:
  openapi: "3.0.0"
  info:
    title: Patched YAML API
    version: "1.0.0"
  paths: {}
patches:
  - target: "$"
    update:
      info:
        x-overlay-applied: yaml-patch
  - target: "$"
    update:
      servers:
        - url: "{SERVER_URL}"
        - url: "{SERVER_URL_2}"
`
		var def OpenAPIResourceDefinition
		require.NoError(t, yaml.Load([]byte(yamlData), &def))

		doc, warnings, err := def.Build(ctx)
		require.NoError(t, err)
		require.NotNil(t, doc)
		log.Println("warnings", warnings)
		require.Equal(t, 0, len(warnings))

		ext, ok := doc.Info.Extensions.Get("x-overlay-applied")
		assert.True(t, ok)
		assert.Equal(t, "yaml-patch", ext.Value)
		assert.Equal(t, 2, len(doc.Servers))
	})

	t.Run("ref_with_patches", func(t *testing.T) {
		yamlData := `
ref: testdata/petstore3/openapi.json
patches:
  - target: "$"
    update:
      info:
        x-overlay-applied: ref-patch
`
		var def OpenAPIResourceDefinition
		require.NoError(t, yaml.Load([]byte(yamlData), &def))

		doc, warnings, err := def.Build(ctx)
		require.NoError(t, err)
		require.NotNil(t, doc)
		require.Equal(t, 0, len(warnings))

		ext, ok := doc.Info.Extensions.Get("x-overlay-applied")
		assert.True(t, ok)
		assert.Equal(t, "ref-patch", ext.Value)
	})

	t.Run("patches_only_no_spec_no_ref_error", func(t *testing.T) {
		patchYAML := `- target: "$"
  update:
    info:
      x-overlay-applied: test`

		var patchNode yaml.Node
		require.NoError(t, yaml.Unmarshal([]byte(patchYAML), &patchNode))

		def := OpenAPIResourceDefinition{
			Patches: patchNode.Content[0],
		}

		_, warnings, err := def.Build(ctx)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrResourceSpecRequired)
		require.Equal(t, 0, len(warnings))
	})
}

// goos: darwin
// goarch: arm64
// pkg: github.com/relychan/openapitools/oaschema
// cpu: Apple M3 Pro
// BenchmarkResourceUnmarshaler/build_from_json-11         	     112	  10476485 ns/op	16303214 B/op	   80132 allocs/op
// BenchmarkResourceUnmarshaler/build_from_yaml-11         	     169	   7085021 ns/op	 5987319 B/op	   85498 allocs/op
func BenchmarkResourceUnmarshaler(b *testing.B) {
	rawPetStoreJson, err := os.ReadFile("./testdata/petstore3/openapi.json")
	if err != nil {
		panic(err)
	}

	petStoreJson := fmt.Appendf([]byte{}, `{"spec": %s}`, rawPetStoreJson)

	var petStoreDoc any

	err = json.Unmarshal(petStoreJson, &petStoreDoc)
	if err != nil {
		panic(err)
	}

	petStoreYaml, err := yaml.Dump(petStoreDoc)
	if err != nil {
		panic(err)
	}

	b.Run("build_from_json", func(b *testing.B) {
		for b.Loop() {
			var value OpenAPIResourceDefinition
			err := json.Unmarshal(petStoreJson, &value)
			if err != nil {
				b.Fatal(err)
			}

			_, _, err = value.Build(context.Background())
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("build_from_yaml", func(b *testing.B) {
		for b.Loop() {
			var value OpenAPIResourceDefinition
			err := yaml.Load(petStoreYaml, &value)
			if err != nil {
				b.Fatal(err)
			}

			_, _, err = value.Build(context.Background())
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
