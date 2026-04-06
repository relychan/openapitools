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
	"testing"

	"github.com/relychan/goutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"
)

func TestNewOverlayDocumentFromActionNodes(t *testing.T) {
	ctx := context.Background()

	t.Run("valid_single_action", func(t *testing.T) {
		actionsYAML := `
- target: "$"
  update:
    info:
      x-overlay-applied: test-overlay
`
		var actionsNode yaml.Node
		require.NoError(t, yaml.Unmarshal([]byte(actionsYAML), &actionsNode))
		// yaml.Unmarshal wraps in a document node; get the sequence node.
		require.Equal(t, yaml.DocumentNode, actionsNode.Kind)
		seqNode := actionsNode.Content[0]

		ov, err := newOverlayDocumentFromActionNodes(ctx, seqNode)
		require.NoError(t, err)
		assert.NotNil(t, ov)
	})

	t.Run("empty_actions_sequence", func(t *testing.T) {
		seqNode := &yaml.Node{
			Kind: yaml.SequenceNode,
			Tag:  goutils.YAMLSeqTag,
		}

		ov, err := newOverlayDocumentFromActionNodes(ctx, seqNode)
		require.NoError(t, err)
		assert.NotNil(t, ov)
	})

	t.Run("multiple_actions", func(t *testing.T) {
		actionsYAML := `
- target: "$.info"
  update:
    x-overlay-applied: overlay-one
- target: "$.paths['/pets']"
  update:
    get:
      summary: "List all pets"
`
		var actionsNode yaml.Node
		require.NoError(t, yaml.Unmarshal([]byte(actionsYAML), &actionsNode))
		seqNode := actionsNode.Content[0]

		ov, err := newOverlayDocumentFromActionNodes(ctx, seqNode)
		require.NoError(t, err)
		assert.NotNil(t, ov)
	})
}
