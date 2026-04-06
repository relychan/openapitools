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

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel"
	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/relychan/goutils"
	"go.yaml.in/yaml/v4"
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

		if node.Content[0].Kind == yaml.SequenceNode &&
			len(node.Content[0].Content) > 0 {
			j.Patches = node.Content[0]
		} else if node.Content[0].Kind != yaml.ScalarNode ||
			node.Content[0].Tag != goutils.NullStr {
			return fmt.Errorf(
				"%w. Expected an array, got %s",
				ErrInvalidOpenAPIResourceDefinitionJSON,
				node.Content[0].Tag,
			)
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
			ErrInvalidOpenAPIResourceDefinitionYAML,
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
				ErrInvalidOpenAPIResourceDefinitionYAML,
				keyNode.Tag,
			)
		}

		if keyNode.Value == "" {
			return fmt.Errorf("%w. Object key is empty", ErrInvalidOpenAPIResourceDefinitionYAML)
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
					ErrInvalidOpenAPIResourceDefinitionYAML,
					valueNode.Tag,
				)
			}
		case "spec":
			j.Spec = &yaml.Node{
				Kind:    yaml.DocumentNode,
				Content: []*yaml.Node{valueNode},
			}
		case "patches":
			j.Patches = valueNode
		default:
		}
	}

	return nil
}

// Build validates and merge the openapi specification with the reference if exist.
func (j *OpenAPIResourceDefinition) Build(ctx context.Context) (*highv3.Document, error) {
	var (
		specBytes []byte
		err       error
	)

	havePatches := j.Patches != nil && len(j.Patches.Content) > 0

	if j.Spec != nil {
		if !havePatches {
			return j.buildSpecNodeWithoutPatch()
		}

		// dump yaml to bytes for applying overlay patches.
		specBytes, err = yaml.Dump(j.Spec)
		if err != nil {
			return nil, err
		}
	} else if j.Ref != "" {
		if !havePatches {
			return j.buildSpecFromRef(ctx)
		}

		if !strings.HasSuffix(j.Ref, ".yaml") {
			return j.buildJSONDocumentWithOverlay(ctx)
		}

		// read document file to apply overlay patches.
		rawSourceReader, _, err := goutils.FileReaderFromPath(ctx, j.Ref)
		if err != nil {
			return nil, err
		}

		specBytes, err = io.ReadAll(rawSourceReader)

		goutils.CatchWarnErrorFunc(rawSourceReader.Close)

		if err != nil {
			return nil, err
		}
	}

	if len(specBytes) == 0 {
		return nil, ErrResourceSpecRequired
	}

	if !havePatches {
		// build openapi model from raw bytes.
		oasConfig := datamodel.NewDocumentConfiguration()
		oasConfig.SkipJSONConversion = true

		doc, err := libopenapi.NewDocumentWithConfiguration(specBytes, oasConfig)
		if err != nil {
			return nil, err
		}

		spec, err := doc.BuildV3Model()
		if err != nil {
			return nil, err
		}

		return &spec.Model, nil
	}

	// apply overlay patches
	ov, err := newOverlayDocumentFromActionNodes(ctx, j.Patches)
	if err != nil {
		return nil, err
	}

	result, err := libopenapi.ApplyOverlayToSpecBytes(specBytes, ov)
	if err != nil {
		return nil, err
	}

	model, err := result.OverlayDocument.BuildV3Model()
	if err != nil {
		return nil, err
	}

	return &model.Model, nil
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

func (j *OpenAPIResourceDefinition) buildJSONDocumentWithOverlay(
	ctx context.Context,
) (*highv3.Document, error) {
	doc, err := j.buildDocumentFromRef(ctx)
	if err != nil {
		return nil, err
	}

	model, err := doc.BuildV3Model()
	if err != nil {
		return nil, err
	}

	rawYaml, err := model.Model.Render()
	if err != nil {
		return nil, err
	}

	// apply overlay patches
	ov, err := newOverlayDocumentFromActionNodes(ctx, j.Patches)
	if err != nil {
		return nil, err
	}

	result, err := libopenapi.ApplyOverlayToSpecBytes(rawYaml, ov)
	if err != nil {
		return nil, err
	}

	model, err = result.OverlayDocument.BuildV3Model()
	if err != nil {
		return nil, err
	}

	return &model.Model, nil
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

	var doc *highv3.Document

	if sourceDoc.GetSpecInfo().SpecFormat == datamodel.OAS2 {
		spec, err := sourceDoc.BuildV2Model()
		if err != nil {
			return nil, err
		}

		doc, err = convertSwaggerToOpenAPIv3Document(&spec.Model)
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
