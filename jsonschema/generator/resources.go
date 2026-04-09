package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"maps"
	"strings"

	"github.com/relychan/goutils"
	"github.com/relychan/jsonschema"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/openapiclient/handler/graphqlhandler"
	"github.com/relychan/openapitools/openapiclient/handler/resthandler"
)

//go:embed openapi-3.json
var openapiDocument []byte

type ProxyActionConfig struct{}

// JSONSchema defines a custom definition for JSON schema.
func (ProxyActionConfig) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		OneOf: []*jsonschema.Schema{
			{
				Description: "Proxy configuration to the remote RESTful service",
				Ref:         "#/$defs/ProxyRESTfulActionConfig",
			},
			{
				Description: "Configurations for proxying request to the remote GraphQL server",
				Ref:         "#/$defs/ProxyGraphQLActionConfig",
			},
		},
	}
}

func genProxyActionSchema(r *jsonschema.Reflector) *jsonschema.Schema {
	reflectSchema := r.Reflect(ProxyActionConfig{})

	for _, externalType := range []any{
		graphqlhandler.ProxyGraphQLActionConfig{},
		resthandler.ProxyRESTfulActionConfig{},
	} {
		externalSchema := r.Reflect(externalType)

		for key, def := range externalSchema.Definitions {
			if _, ok := reflectSchema.Definitions[key]; !ok {
				reflectSchema.Definitions[key] = def
			}
		}
	}

	for key := range reflectSchema.Definitions {
		if strings.HasPrefix(key, "OrderedMap[") {
			delete(reflectSchema.Definitions, key)
		}
	}

	return reflectSchema
}

func genOpenAPIResourceSchema() (*jsonschema.Schema, error) {
	r := new(jsonschema.Reflector)

	err := r.AddGoComments(
		"github.com/relychan/openapitools",
		".",
		jsonschema.WithFullComment(),
	)
	if err != nil {
		return nil, err
	}

	actionSchema := genProxyActionSchema(r)

	openApiSpec, err := loadOpenAPISchema()
	if err != nil {
		return nil, err
	}

	reflectSchema := newOpenAPIResourceSchema()

	maps.Copy(reflectSchema.Definitions, openApiSpec.Definitions)
	openApiSpec.Definitions = nil
	maps.Copy(reflectSchema.Definitions, actionSchema.Definitions)

	settings := r.Reflect(oaschema.OpenAPIResourceSettings{})
	maps.Copy(reflectSchema.Definitions, settings.Definitions)

	remoteSchemas, err := downloadRemoteSchemas()
	if err != nil {
		return nil, err
	}

	for _, rs := range remoteSchemas {
		maps.Copy(reflectSchema.Definitions, rs.Definitions)
	}

	// custom schema types
	reflectSchema.Definitions["Document"] = openApiSpec

	// override graphql http errors response transformation
	httpErrors := &jsonschema.Schema{
		Type:        "object",
		Description: "Evaluation rules to map GraphQL errors to desired HTTP status codes.",
		Properties:  jsonschema.NewProperties(),
	}

	for _, statusCode := range []string{"400", "401", "403", "404", "405", "422", "500", "501"} {
		httpErrors.Properties.Set(statusCode, &jsonschema.Schema{
			Type: "array",
			Items: &jsonschema.Schema{
				Type:      "string",
				MinLength: new(uint64(1)),
			},
			MinItems: new(uint64(1)),
		})
	}

	reflectSchema.Definitions["ProxyCustomGraphQLResponseConfig"].
		Properties.Set("httpErrors", httpErrors)

	return reflectSchema, nil
}

func loadOpenAPISchema() (*jsonschema.Schema, error) {
	jsonSchema := new(jsonschema.Schema)

	err := json.Unmarshal(openapiDocument, jsonSchema)
	if err != nil {
		return nil, fmt.Errorf("failed to decode openapi schema: %w", err)
	}

	return jsonSchema, nil
}

func downloadRemoteSchemas() ([]*jsonschema.Schema, error) {
	fileURLs := []string{
		"https://raw.githubusercontent.com/relychan/gohttpc/refs/heads/main/jsonschema/gohttpc.schema.json",
		"https://raw.githubusercontent.com/relychan/gotransform/refs/heads/main/jsonschema/gotransform.schema.json",
	}

	results := make([]*jsonschema.Schema, 0, len(fileURLs))

	for _, fileURL := range fileURLs {
		jsonSchema, err := goutils.ReadJSONOrYAMLFile[jsonschema.Schema](context.TODO(), fileURL)
		if err != nil {
			return nil, err
		}

		results = append(results, jsonSchema)
	}

	return results, nil
}

func newOverlayActionSchema() *jsonschema.Schema {
	// The schema is copied from https://raw.githubusercontent.com/OAI/Overlay-Specification/refs/heads/main/schemas/v1.1/schema.yaml

	props := jsonschema.NewProperties()
	props.Set("target", &jsonschema.Schema{
		Type:        "string",
		Description: "A RFC9535 JSONPath query expression selecting nodes in the target document.",
		Pattern:     `^\$`,
	})
	props.Set("description", &jsonschema.Schema{
		Type:        "string",
		Description: "A description of the action.",
	})

	updateProps := jsonschema.NewProperties()
	updateProps.Set("update", &jsonschema.Schema{
		Description: "If the target selects object nodes, the value of this field MUST be an object with the properties and values to merge with each selected object. If the target selects array nodes, the value of this field MUST be an array to concatenate with each selected array, or an object or primitive value to append to each selected array. If the target selects primitive nodes, the value of this field MUST be a primitive value to replace each selected node. This field has no impact if the remove field of this action object is true or if the copy field contains a value.",
	})
	updateProps.Set("copy", &jsonschema.Schema{
		Type:        "string",
		Description: "A JSONPath expression selecting a single node to copy into the target nodes. If the target selects object nodes, the value of this field MUST be an object with the properties and values to merge with each selected object. If the target selects array nodes, the value of this field MUST be an array to concatenate with each selected array, or an object or primitive value to append to each selected array. If the target selects primitive nodes, the value of this field MUST be a primitive value to replace each selected node. This field has no impact if the remove field of this action object is true or if the update field contains a value.",
	})

	removeProps := jsonschema.NewProperties()
	removeProps.Set("remove", &jsonschema.Schema{
		Type:        "boolean",
		Description: "A boolean value that indicates that each of the target nodes MUST be removed from the the map or array it is contained in. The default value is false.",
		Const:       true,
	})

	jsonSchema := &jsonschema.Schema{
		Type:        "object",
		Description: "Represents one or more changes to be applied to the target document at the location defined by the target JSONPath expression.",
		Required:    []string{"target"},
		Properties:  props,
		OneOf: []*jsonschema.Schema{
			{
				Type:       "object",
				Title:      "OverlayActionUpdateCopyObject",
				Properties: updateProps,
				OneOf: []*jsonschema.Schema{
					{
						Required: []string{"update"},
					},
					{
						Required: []string{"copy"},
					},
				},
			},
			{
				Type:       "object",
				Title:      "OverlayActionRemoveObject",
				Required:   []string{"remove"},
				Properties: removeProps,
			},
		},
	}

	return jsonSchema
}

func newOpenAPIResourceSchema() *jsonschema.Schema {
	resourceProps := jsonschema.NewProperties()
	resourceProps.Set("settings", &jsonschema.Schema{
		Description: "Settings of the OpenAPI resource.",
		Ref:         "#/$defs/OpenAPIResourceSettings",
	})
	resourceProps.Set("patches", &jsonschema.Schema{
		Description: "A set of patches, or overlay actions to be applied to one or many OpenAPI descriptions. See https://spec.openapis.org/overlay/v1.1.0.html#action-object",
		Type:        "array",
		Items: &jsonschema.Schema{
			Ref: "#/$defs/OverlayActionObject",
		},
	})

	refProps := jsonschema.NewProperties()
	refProps.Set("ref", &jsonschema.Schema{
		Description: "Path of URL of the referenced OpenAPI document.\nRequires at least one of ref or spec.\nIf both fields are configured, the spec will be merged into the reference.",
		Type:        "string",
	})

	specProps := jsonschema.NewProperties()
	specProps.Set("spec", &jsonschema.Schema{
		Description: "Specification of the OpenAPI v3 documentation.",
		Ref:         "#/$defs/Document",
	})

	return &jsonschema.Schema{
		Version: "https://json-schema.org/draft/2020-12/schema",
		ID:      "https://github.com/relychan/openapitools/oaschema/open-api-resource-definition",
		Ref:     "#/$defs/OpenAPIResourceDefinition",
		Definitions: jsonschema.Definitions{
			"OpenAPIResourceDefinition": &jsonschema.Schema{
				Type:        "object",
				Description: "Definition of an OpenAPI resource",
				Properties:  resourceProps,
				OneOf: []*jsonschema.Schema{
					{
						Type:        "object",
						Description: "Definition of an OpenAPI resource",
						Title:       "OpenAPIResourceRef",
						Properties:  refProps,
						Required:    []string{"ref"},
					},
					{
						Type:        "object",
						Description: "Definition of an OpenAPI resource",
						Title:       "OpenAPIResourceSpec",
						Properties:  specProps,
						Required:    []string{"spec"},
					},
				},
			},
			"OverlayActionObject": newOverlayActionSchema(),
		},
	}
}
