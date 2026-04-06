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

	highoverlay "github.com/pb33f/libopenapi/datamodel/high/overlay"
	"github.com/pb33f/libopenapi/datamodel/low"
	lowoverlay "github.com/pb33f/libopenapi/datamodel/low/overlay"
	"github.com/relychan/goutils"
	"go.yaml.in/yaml/v4"
)

// newOverlayDocumentFromActionNodes creates a new overlay document from actions node.
func newOverlayDocumentFromActionNodes(
	ctx context.Context,
	actionNodes *yaml.Node,
) (*highoverlay.Overlay, error) {
	node := &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  goutils.YAMLMapTag,
		Content: []*yaml.Node{
			newYAMLScalarStringNode("overlay"),
			newYAMLScalarStringNode("1.1.0"),
			newYAMLScalarStringNode("info"),
			{
				Kind: yaml.ScalarNode,
				Tag:  goutils.YAMLNullTag,
			},
			newYAMLScalarStringNode("actions"),
			actionNodes,
		},
	}

	var lowOv lowoverlay.Overlay

	err := low.BuildModel(node, &lowOv)
	if err != nil {
		return nil, err
	}

	err = lowOv.Build(ctx, nil, node, nil)
	if err != nil {
		return nil, err
	}

	return highoverlay.NewOverlay(&lowOv), nil
}

func newYAMLScalarStringNode(value string) *yaml.Node {
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   goutils.YAMLStrTag,
		Value: value,
	}
}
