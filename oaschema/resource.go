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
	Spec *highv3.Document `json:"spec,omitempty" yaml:"spec,omitempty"`
}

type rawOpenAPIResourceDefinitionJSON struct {
	Settings *OpenAPIResourceSettings `json:"settings,omitempty"`
	Ref      string                   `json:"ref,omitempty"`
	Spec     json.RawMessage          `json:"spec"`
}

// MarshalJSON implements json.Marshaler.
func (j OpenAPIResourceDefinition) MarshalJSON() ([]byte, error) {
	result := map[string]any{}

	if j.Ref != "" {
		result["ref"] = j.Ref
	}

	if j.Settings != nil {
		result["settings"] = j.Spec.Security
	}

	if j.Spec != nil {
		rawJSONBytes, err := j.Spec.RenderJSON("")
		if err != nil {
			return nil, err
		}

		result["spec"] = json.RawMessage(rawJSONBytes)
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

	if len(rawValue.Spec) == 0 {
		return nil
	}

	doc, err := libopenapi.NewDocument(rawValue.Spec)
	if err != nil {
		return err
	}

	spec, err := doc.BuildV3Model()
	if err != nil {
		return err
	}

	j.Spec = &spec.Model

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
		if keyNode.Kind != yaml.ScalarNode && keyNode.Tag != "!!str" {
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
			// Marshal the YAML node back to bytes for libopenapi
			specBytes, err := yaml.Dump(valueNode)
			if err != nil {
				return err
			}

			doc, err := libopenapi.NewDocument(specBytes)
			if err != nil {
				return err
			}

			spec, err := doc.BuildV3Model()
			if err != nil {
				return err
			}

			j.Spec = &spec.Model
		default:
		}
	}

	return nil
}

// Build validates and merge the openapi specification with the reference if exist.
func (j *OpenAPIResourceDefinition) Build(ctx context.Context) (*highv3.Document, error) {
	if j.Ref == "" {
		if j.Spec == nil {
			return nil, ErrResourceSpecRequired
		}

		return j.Spec, nil
	}

	rawSourceReader, _, err := goutils.FileReaderFromPath(ctx, j.Ref)
	if err != nil {
		return nil, err
	}

	defer goutils.CatchWarnErrorFunc(rawSourceReader.Close)

	sourceBytes, err := io.ReadAll(rawSourceReader)
	if err != nil {
		return nil, err
	}

	sourceDoc, err := libopenapi.NewDocument(sourceBytes)
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
	} else {
		sourceSpec, err := sourceDoc.BuildV3Model()
		if err != nil {
			return nil, err
		}

		doc = &sourceSpec.Model
	}

	if j.Spec != nil {
		mergeOpenAPIv3Documents(doc, j.Spec)
	}

	return doc, nil
}
