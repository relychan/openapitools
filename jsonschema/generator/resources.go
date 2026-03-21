package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"strings"

	"github.com/invopop/jsonschema"
	"github.com/relychan/goutils"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/openapiclient/handler/graphqlhandler"
	"github.com/relychan/openapitools/openapiclient/handler/resthandler"
)

//go:embed openapi-3.json
var openapiDocument []byte

type RelixyActionConfig struct{}

// JSONSchema defines a custom definition for JSON schema.
func (RelixyActionConfig) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		OneOf: []*jsonschema.Schema{
			{
				Description: "Proxy configuration to the remote REST service",
				Ref:         "#/$defs/RelixyRESTActionConfig",
			},
			{
				Description: "Configurations for proxying request to the remote GraphQL server",
				Ref:         "#/$defs/RelixyGraphQLActionConfig",
			},
		},
	}
}

func genProxyActionSchema() (*jsonschema.Schema, error) {
	r := new(jsonschema.Reflector)

	// for _, name := range []string{
	// 	"proxyc/handler/graphqlhandler",
	// 	"proxyc/handler/proxyhandler",
	// 	"proxyc/handler/resthandler",
	// } {
	// 	err := r.AddGoComments(
	// 		"github.com/relychan/relixy/"+name,
	// 		"../"+name,
	// 		jsonschema.WithFullComment(),
	// 	)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// }

	reflectSchema := r.Reflect(RelixyActionConfig{})

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

	return reflectSchema, nil
}

func genOpenAPIResourceSchema() (*jsonschema.Schema, error) {
	actionSchema, err := genProxyActionSchema()
	if err != nil {
		return nil, fmt.Errorf("failed to write jsonschema for RelixyAction: %w", err)
	}

	r := new(jsonschema.Reflector)

	// err = r.AddGoComments(
	// 	"github.com/relychan/relixy/schema",
	// 	"../schema",
	// 	jsonschema.WithFullComment(),
	// )
	// if err != nil {
	// 	return nil, err
	// }

	reflectSchema := r.Reflect(oaschema.OpenAPIResourceDefinition{})

	openApiSpec, err := loadOpenAPISchema()
	if err != nil {
		return nil, err
	}

	maps.Copy(reflectSchema.Definitions, openApiSpec.Definitions)
	openApiSpec.Definitions = nil
	maps.Copy(reflectSchema.Definitions, actionSchema.Definitions)

	remoteSchemas, err := downloadRemoteSchemas()
	if err != nil {
		return nil, err
	}

	for _, rs := range remoteSchemas {
		maps.Copy(reflectSchema.Definitions, rs.Definitions)
	}

	// custom schema types
	reflectSchema.Definitions["Duration"] = &jsonschema.Schema{
		Type:        "string",
		Description: "Duration string",
		Pattern:     "^((([0-9]+h)?([0-9]+m)?([0-9]+s))|(([0-9]+h)?([0-9]+m))|([0-9]+h))$",
	}

	reflectSchema.Definitions["Document"] = openApiSpec

	// delete unused definitions
	delete(reflectSchema.Definitions, "Contact")
	delete(reflectSchema.Definitions, "Components")
	delete(reflectSchema.Definitions, "ExternalDoc")
	delete(reflectSchema.Definitions, "Tag")
	delete(reflectSchema.Definitions, "SecurityRequirement")
	delete(reflectSchema.Definitions, "Server")
	delete(reflectSchema.Definitions, "Paths")
	delete(reflectSchema.Definitions, "Info")
	delete(reflectSchema.Definitions, "License")

	for key := range reflectSchema.Definitions {
		if strings.HasPrefix(key, "Map[") {
			delete(reflectSchema.Definitions, key)
		}
	}

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
		rawResp, err := http.Get(fileURL) //nolint:bodyclose,noctx,gosec
		if err != nil {
			return nil, fmt.Errorf("failed to download file %s: %w", fileURL, err)
		}

		if rawResp != nil && rawResp.Body != nil {
			defer goutils.CatchWarnErrorFunc(rawResp.Body.Close) //nolint:revive
		}

		if rawResp.StatusCode != http.StatusOK {
			rawBody, err := io.ReadAll(rawResp.Body)
			if err != nil {
				return nil, fmt.Errorf("failed to download %s schema: %s", fileURL, rawResp.Status) //nolint
			}

			return nil, fmt.Errorf("failed to download %s schema: %s", fileURL, string(rawBody)) //nolint
		}

		jsonSchema := new(jsonschema.Schema)

		err = json.NewDecoder(rawResp.Body).Decode(jsonSchema)
		if err != nil {
			return nil, fmt.Errorf("failed to decode gohttpc schema: %w", err)
		}

		results = append(results, jsonSchema)
	}

	return results, nil
}
