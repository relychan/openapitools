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
	"io"
	"strings"

	"github.com/pb33f/go-yaml"
	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel"
	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/overlay"
	"github.com/pb33f/libopenapi/utils"
	"github.com/relychan/goutils"
)

// OpenAPIResourceDefinition defines fields of an OpenAPI resource.
type OpenAPIResourceDefinition struct {
	// Settings of the OpenAPI resource.
	Settings *OpenAPIResourceSettings `json:"settings,omitempty" yaml:"settings,omitempty"`
	// Path of URL of the referenced OpenAPI document.
	// Requires at least one of ref or spec.
	// If both fields are configured, the spec will be merged into the reference.
	Ref string `json:"ref,omitempty" yaml:"ref,omitempty"`
	// Specification of the OpenAPI v3 documentation.
	Spec *yaml.Node `json:"spec,omitempty" yaml:"spec,omitempty"`
	// A set of patches, or [overlay actions] to be applied to one or many OpenAPI descriptions.
	//
	// [overlay actions]: https://spec.openapis.org/overlay/v1.1.0.html#action-object
	Patches *yaml.Node `json:"patches,omitempty" yaml:"patches,omitempty"`
}

type rawOpenAPIResourceDefinitionJSON struct {
	Settings *OpenAPIResourceSettings `json:"settings,omitempty"`
	Ref      string                   `json:"ref,omitempty"`
	Spec     json.RawMessage          `json:"spec"`
	Patches  json.RawMessage          `json:"patches"`
}

// MarshalJSON implements json.Marshaler.
func (j OpenAPIResourceDefinition) MarshalJSON() ([]byte, error) {
	result := map[string]any{}

	if j.Ref != "" {
		result["ref"] = j.Ref
	}

	if j.Settings != nil {
		result["settings"] = j.Settings
	}

	if j.Spec != nil {
		var spec any

		err := j.Spec.Load(&spec)
		if err != nil {
			return nil, fmt.Errorf("failed to encode spec: %w", err)
		}

		result["spec"] = spec
	}

	if j.Patches != nil {
		var patches any

		err := j.Patches.Decode(&patches)
		if err != nil {
			return nil, err
		}

		result["patches"] = patches
	}

	return json.Marshal(result)
}

// UnmarshalJSON implements json.Unmarshaler.
func (j *OpenAPIResourceDefinition) UnmarshalJSON(b []byte) error {
	var rawValue rawOpenAPIResourceDefinitionJSON

	err := json.Unmarshal(b, &rawValue)
	if err != nil {
		return err
	}

	j.Ref = rawValue.Ref
	j.Settings = rawValue.Settings
	j.Spec = nil
	j.Patches = nil

	if len(rawValue.Spec) > 0 {
		var specNode yaml.Node

		err := yaml.Load(rawValue.Spec, &specNode)
		if err != nil {
			return fmt.Errorf("malformed spec: %w", err)
		}

		j.Spec = &specNode
	}

	if len(rawValue.Patches) > 0 {
		node := new(yaml.Node)

		err := yaml.Load(rawValue.Patches, node)
		if err != nil {
			return fmt.Errorf("malformed patches: %w", err)
		}

		if len(node.Content) == 0 {
			return nil
		}

		err = j.validatePatchesNode(node.Content[0])
		if err != nil {
			return err
		}
	}

	return nil
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (j *OpenAPIResourceDefinition) UnmarshalYAML(value *yaml.Node) error {
	if value == nil {
		return nil
	}

	if value.Kind != yaml.MappingNode {
		return fmt.Errorf(
			"%w. Expected an object, got %s",
			ErrInvalidOpenAPIResourceDefinition,
			value.Tag,
		)
	}

	contentLength := len(value.Content)

	for i := 0; i < contentLength; i++ {
		if i == contentLength-1 {
			break
		}

		keyNode := value.Content[i]
		if keyNode.Kind != yaml.ScalarNode || keyNode.Tag != goutils.YAMLStrTag {
			return fmt.Errorf(
				"%w. Expected a key string, got %s",
				ErrInvalidOpenAPIResourceDefinition,
				keyNode.Tag,
			)
		}

		if keyNode.Value == "" {
			return fmt.Errorf("%w. Object key is empty", ErrInvalidOpenAPIResourceDefinition)
		}

		i++

		valueNode := value.Content[i]

		switch keyNode.Value {
		case "settings":
			err := valueNode.Decode(&j.Settings)
			if err != nil {
				return err
			}
		case "ref":
			switch valueNode.Tag {
			case goutils.YAMLStrTag:
				j.Ref = valueNode.Value
			case goutils.YAMLNullTag:
			default:
				return fmt.Errorf(
					"%w. Expected ref is a string, got %s",
					ErrInvalidOpenAPIResourceDefinition,
					valueNode.Tag,
				)
			}
		case "spec":
			j.Spec = &yaml.Node{
				Kind:    yaml.DocumentNode,
				Content: []*yaml.Node{valueNode},
			}
		case "patches":
			err := j.validatePatchesNode(valueNode)
			if err != nil {
				return err
			}
		default:
		}
	}

	return nil
}

// Build validates and merge the openapi specification with the reference if exist.
func (j *OpenAPIResourceDefinition) Build(
	ctx context.Context,
) (*highv3.Document, []*overlay.Warning, error) {
	var (
		specBytes []byte
		err       error
	)

	havePatches := j.Patches != nil && len(j.Patches.Content) > 0

	if j.Spec != nil {
		if !havePatches {
			doc, err := j.buildSpecNodeWithoutPatch()

			return doc, nil, err
		}

		// dump yaml to bytes for applying overlay patches.
		specBytes, err = yaml.Dump(j.Spec)
		if err != nil {
			return nil, nil, err
		}
	} else if j.Ref != "" {
		if !havePatches {
			doc, err := j.buildSpecFromRef(ctx)

			return doc, nil, err
		}

		return j.buildRefDocumentWithOverlay(ctx)
	}

	if len(specBytes) == 0 {
		return nil, nil, ErrResourceSpecRequired
	}

	return j.applyPatches(ctx, specBytes)
}

// applyPatches applies overlay patches to the OpenAPI v3 spec bytes and builds the v3 model.
func (j *OpenAPIResourceDefinition) applyPatches(
	ctx context.Context,
	specBytes []byte,
) (*highv3.Document, []*overlay.Warning, error) {
	ov, err := newOverlayDocumentFromActionNodes(ctx, j.Patches)
	if err != nil {
		return nil, nil, err
	}

	result, err := libopenapi.ApplyOverlayToSpecBytes(specBytes, ov)
	if err != nil {
		return nil, nil, err
	}

	model, err := result.OverlayDocument.BuildV3Model()
	if err != nil {
		return nil, result.Warnings, err
	}

	return &model.Model, result.Warnings, nil
}

func (j *OpenAPIResourceDefinition) buildSpecNodeWithoutPatch() (*highv3.Document, error) {
	// parse document from the yaml node directly.
	doc, err := extractOpenAPIv3SpecInfoFromYAML(j.Spec)
	if err != nil {
		return nil, err
	}

	spec, err := buildV3Model(doc)
	if err != nil {
		return nil, err
	}

	return &spec.Model, nil
}

// buildRefDocumentWithOverlay builds the referenced document and applies overlay patches.
// Patches always target the OpenAPI v3 document. A Swagger 2.0 document is converted
// to OpenAPI v3 before patches are applied, consistent with inline specs that must be v3.
func (j *OpenAPIResourceDefinition) buildRefDocumentWithOverlay(
	ctx context.Context,
) (*highv3.Document, []*overlay.Warning, error) {
	sourceDoc, err := j.buildDocumentFromRef(ctx)
	if err != nil {
		return nil, nil, err
	}

	specInfo := sourceDoc.GetSpecInfo()

	// apply patches to the original YAML source of OpenAPI v3 documents to keep the original layout.
	if specInfo.SpecType == utils.OpenApi3 && strings.HasSuffix(j.Ref, ".yaml") &&
		specInfo.SpecBytes != nil {
		return j.applyPatches(ctx, *specInfo.SpecBytes)
	}

	doc, err := buildV3DocumentFromSource(sourceDoc)
	if err != nil {
		return nil, nil, err
	}

	rawYaml, err := doc.Render()
	if err != nil {
		return nil, nil, err
	}

	return j.applyPatches(ctx, rawYaml)
}

func (j *OpenAPIResourceDefinition) buildDocumentFromRef(
	ctx context.Context,
) (libopenapi.Document, error) {
	rawSourceReader, _, err := goutils.FileReaderFromPath(ctx, j.Ref)
	if err != nil {
		return nil, err
	}

	sourceBytes, err := io.ReadAll(rawSourceReader)

	goutils.CatchWarnErrorFunc(rawSourceReader.Close)

	if err != nil {
		return nil, err
	}

	return libopenapi.NewDocument(sourceBytes)
}

func (j *OpenAPIResourceDefinition) buildSpecFromRef(
	ctx context.Context,
) (*highv3.Document, error) {
	sourceDoc, err := j.buildDocumentFromRef(ctx)
	if err != nil {
		return nil, err
	}

	return buildV3DocumentFromSource(sourceDoc)
}

// buildV3DocumentFromSource builds the OpenAPI v3 model of the document.
// Swagger 2.0 documents are converted to OpenAPI v3.
func buildV3DocumentFromSource(sourceDoc libopenapi.Document) (*highv3.Document, error) {
	if sourceDoc.GetSpecInfo().SpecFormat == datamodel.OAS2 {
		spec, err := sourceDoc.BuildV2Model()
		if err != nil {
			return nil, err
		}

		doc, err := convertSwaggerToOpenAPIv3Document(&spec.Model)
		if err != nil {
			return nil, fmt.Errorf("failed to convert openapi spec v2 to v3: %w", err)
		}

		return doc, nil
	}

	sourceSpec, err := sourceDoc.BuildV3Model()
	if err != nil {
		return nil, err
	}

	return &sourceSpec.Model, nil
}

func (j *OpenAPIResourceDefinition) validatePatchesNode(node *yaml.Node) error {
	if node.Kind == yaml.SequenceNode {
		if len(node.Content) > 0 {
			j.Patches = node
		}

		return nil
	}

	if node.Kind != yaml.ScalarNode ||
		node.Tag != goutils.YAMLNullTag {
		return fmt.Errorf(
			"%w. Expected patches as an array, got %s",
			ErrInvalidOpenAPIResourceDefinition,
			node.Tag,
		)
	}

	return nil
}
