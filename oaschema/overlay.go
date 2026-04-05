package oaschema

import (
	"context"

	highoverlay "github.com/pb33f/libopenapi/datamodel/high/overlay"
	"github.com/pb33f/libopenapi/datamodel/low"
	lowoverlay "github.com/pb33f/libopenapi/datamodel/low/overlay"
	"github.com/pb33f/libopenapi/overlay"
	"go.yaml.in/yaml/v4"
)

// NewOverlayDocument creates a new overlay document from a YAML node.
func NewOverlayDocument(ctx context.Context, node *yaml.Node) (*highoverlay.Overlay, error) {
	if len(node.Content) == 0 {
		return nil, overlay.ErrInvalidOverlay
	}

	var lowOv lowoverlay.Overlay

	err := low.BuildModel(node.Content[0], &lowOv)
	if err != nil {
		return nil, err
	}

	err = lowOv.Build(ctx, nil, node.Content[0], nil)
	if err != nil {
		return nil, err
	}

	return highoverlay.NewOverlay(&lowOv), nil
}
