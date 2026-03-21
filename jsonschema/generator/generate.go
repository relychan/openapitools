// Package main generates the JSON schema for the relixy metadata.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"

	"github.com/invopop/jsonschema"
	"github.com/relychan/openapitools/oaschema"
)

func main() {
	err := genConfigurationSchema()
	if err != nil {
		panic(err)
	}
}

func genConfigurationSchema() error {
	r := new(jsonschema.Reflector)

	// for _, name := range []string{"/schema/openapi", "/schema/baseschema", "/schema/gqlschema"} {
	// 	err := r.AddGoComments(
	// 		"github.com/relychan/relixy"+name,
	// 		".."+name,
	// 		jsonschema.WithFullComment(),
	// 	)
	// 	if err != nil {
	// 		return err
	// 	}
	// }

	reflectSchema := r.Reflect(oaschema.OpenAPIResourceDefinition{})

	// custom schema types
	openapiSchema, err := genOpenAPIResourceSchema()
	if err != nil {
		return fmt.Errorf("failed to write jsonschema for OpenAPIResourceDefinition: %w", err)
	}

	maps.Copy(reflectSchema.Definitions, openapiSchema.Definitions)

	buffer := new(bytes.Buffer)
	enc := json.NewEncoder(buffer)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", " ")

	err = enc.Encode(reflectSchema)
	if err != nil {
		return err
	}

	return os.WriteFile( //nolint:gosec
		filepath.Join("..", "openapitools.schema.json"),
		buffer.Bytes(), 0o644,
	)
}
