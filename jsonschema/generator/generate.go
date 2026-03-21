// Package main generates the JSON schema for the relixy metadata.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	err := genJSONSchema()
	if err != nil {
		panic(err)
	}
}

func genJSONSchema() error {
	openapiSchema, err := genOpenAPIResourceSchema()
	if err != nil {
		return fmt.Errorf("failed to write jsonschema for OpenAPIResourceDefinition: %w", err)
	}

	buffer := new(bytes.Buffer)
	enc := json.NewEncoder(buffer)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", " ")

	err = enc.Encode(openapiSchema)
	if err != nil {
		return err
	}

	return os.WriteFile( //nolint:gosec
		filepath.Join("jsonschema", "openapitools.schema.json"),
		buffer.Bytes(), 0o644,
	)
}
