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

package openapiclient

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/relychan/goutils/httperror"
	"github.com/relychan/openapitools/oaschema"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"
)

func TestExecute_QueryParamMissing(t *testing.T) {
	spec := `openapi: 3.1.0
servers:
  - url: http://localhost:8080
paths:
  /a/fishy/on/a/dishy:
    get:
      parameters:
        - name: fishy
          in: query
          required: true
          schema:
            type: string
      operationId: locateFishy
`

	rawSpec := new(yaml.Node)
	err := yaml.Load([]byte(spec), rawSpec)
	require.NoError(t, err)

	config := &oaschema.OpenAPIResourceDefinition{
		Spec: rawSpec,
	}

	client, err := NewProxyClient(context.TODO(), config)
	require.NoError(t, err)

	_, _, err = client.Execute(context.TODO(), http.MethodGet, "/a/fishy/on/a/dishy", nil, nil)
	require.ErrorContains(t, err, "Required parameter 'fishy' is missing")
}

func TestExecute_QueryParamMaximumLength_violation(t *testing.T) {
	spec := `openapi: 3.1.0
servers:
  - url: http://localhost:8080
paths:
  /a/fishy/on/a/dishy:
    get:
      parameters:
        - name: fishy
          in: query
          required: true
          schema:
            type: string
            maxLength: 1
      operationId: locateFishy
`

	rawSpec := new(yaml.Node)
	err := yaml.Load([]byte(spec), rawSpec)
	require.NoError(t, err)

	config := &oaschema.OpenAPIResourceDefinition{
		Spec: rawSpec,
	}

	client, err := NewProxyClient(context.TODO(), config)
	require.NoError(t, err)

	_, _, err = client.Execute(context.TODO(), http.MethodGet, "/a/fishy/on/a/dishy?fishy=cod", nil, nil)
	require.ErrorContains(t, err, "The length of the string value must be less than or equal 1, but got: 3")
}

func TestExecute_QueryParamMinimumLength_violation(t *testing.T) {
	spec := `openapi: 3.1.0
servers:
  - url: http://localhost:8080
paths:
  /a/fishy/on/a/dishy:
    get:
      parameters:
        - name: fishy
          in: query
          required: true
          schema:
            type: string
            minLength: 4
      operationId: locateFishy
`

	rawSpec := new(yaml.Node)
	err := yaml.Load([]byte(spec), rawSpec)
	require.NoError(t, err)

	config := &oaschema.OpenAPIResourceDefinition{
		Spec: rawSpec,
	}

	client, err := NewProxyClient(context.TODO(), config)
	require.NoError(t, err)

	_, _, err = client.Execute(context.TODO(), http.MethodGet, "/a/fishy/on/a/dishy?fishy=cod", nil, nil)
	require.ErrorContains(t, err, "The length of the string value must be greater than or equal 4, but got: 3")
}

func TestExecute_QueryParamMultiStringField_violation(t *testing.T) {
	spec := `openapi: 3.1.0
servers:
  - url: http://localhost:8080
paths:
  /a/fishy/on/a/dishy:
    get:
      parameters:
        - name: fishy
          in: query
          required: true
          schema:
            type: string
            enum: [cod, halibut]
        - name: dishy
          in: query
          required: true
          schema:
            type: string
      operationId: locateFishy
`

	rawSpec := new(yaml.Node)
	err := yaml.Load([]byte(spec), rawSpec)
	require.NoError(t, err)

	config := &oaschema.OpenAPIResourceDefinition{
		Spec: rawSpec,
	}

	client, err := NewProxyClient(context.TODO(), config)
	require.NoError(t, err)

	_, _, err = client.Execute(context.TODO(), http.MethodGet, "/a/fishy/on/a/dishy?fishy=doc&dishy=halibut", nil, nil)
	require.ErrorContains(t, err, "Value 'doc' does not match any enum values: [cod, halibut]")
}

func TestExecute_QueryParamMultiNumberField_violation(t *testing.T) {
	spec := `openapi: 3.1.0
servers:
  - url: http://localhost:8080
paths:
  /a/fishy/on/a/dishy:
    get:
      parameters:
        - name: fishy
          in: query
          required: true
          schema:
            type: number
            enum: [1, 99]
        - name: dishy
          in: query
          required: true
          schema:
            type: number
      operationId: locateFishy
`

	rawSpec := new(yaml.Node)
	err := yaml.Load([]byte(spec), rawSpec)
	require.NoError(t, err)

	config := &oaschema.OpenAPIResourceDefinition{
		Spec: rawSpec,
	}

	client, err := NewProxyClient(context.TODO(), config)
	require.NoError(t, err)

	_, _, err = client.Execute(context.TODO(), http.MethodGet, "/a/fishy/on/a/dishy?fishy=10&dishy=10", nil, nil)
	require.ErrorContains(t, err, "Value '10' does not match any enum values: [1, 99]")
}

func TestExecute_QueryParamWrongTypeNumber(t *testing.T) {
	spec := `openapi: 3.1.0
servers:
  - url: http://localhost:8080
paths:
  /a/fishy/on/a/dishy:
    get:
      parameters:
        - name: fishy
          in: query
          required: true
          schema:
            type: number
      operationId: locateFishy
`

	rawSpec := new(yaml.Node)
	err := yaml.Load([]byte(spec), rawSpec)
	require.NoError(t, err)

	config := &oaschema.OpenAPIResourceDefinition{
		Spec: rawSpec,
	}

	client, err := NewProxyClient(context.TODO(), config)
	require.NoError(t, err)

	_, _, err = client.Execute(context.TODO(), http.MethodGet, "/a/fishy/on/a/dishy?fishy=cod", nil, nil)
	require.ErrorContains(t, err, "malformed number")
}

func TestExecute_QueryParamMinimumNumber_violation(t *testing.T) {
	spec := `openapi: 3.1.0
servers:
  - url: http://localhost:8080
paths:
  /a/fishy/on/a/dishy:
    get:
      parameters:
        - name: fishy
          in: query
          required: true
          schema:
            type: number
            minimum: 200
      operationId: locateFishy
`

	rawSpec := new(yaml.Node)
	err := yaml.Load([]byte(spec), rawSpec)
	require.NoError(t, err)

	config := &oaschema.OpenAPIResourceDefinition{
		Spec: rawSpec,
	}

	client, err := NewProxyClient(context.TODO(), config)
	require.NoError(t, err)

	_, _, err = client.Execute(context.TODO(), http.MethodGet, "/a/fishy/on/a/dishy?fishy=123.4", nil, nil)
	require.ErrorContains(t, err, "Number value must be greater than or equal 200, but got: 123.4")
}

func TestExecute_QueryParamMaximumNumber_violation(t *testing.T) {
	spec := `openapi: 3.1.0
servers:
  - url: http://localhost:8080
paths:
  /a/fishy/on/a/dishy:
    get:
      parameters:
        - name: fishy
          in: query
          required: true
          schema:
            type: number
            maximum: 200
      operationId: locateFishy
`

	rawSpec := new(yaml.Node)
	err := yaml.Load([]byte(spec), rawSpec)
	require.NoError(t, err)

	config := &oaschema.OpenAPIResourceDefinition{
		Spec: rawSpec,
	}

	client, err := NewProxyClient(context.TODO(), config)
	require.NoError(t, err)

	_, _, err = client.Execute(context.TODO(), http.MethodGet, "/a/fishy/on/a/dishy?fishy=1234.5", nil, nil)
	require.ErrorContains(t, err, "Number value must be less than or equal 200, but got: 1234.5")
}

func TestExecute_QueryParamWrongTypeInteger_FloatValue(t *testing.T) {
	spec := `openapi: 3.1.0
servers:
  - url: http://localhost:8080
paths:
  /a/fishy/on/a/dishy:
    get:
      parameters:
        - name: fishy
          in: query
          required: true
          schema:
            type: integer
      operationId: locateFishy
`

	rawSpec := new(yaml.Node)
	err := yaml.Load([]byte(spec), rawSpec)
	require.NoError(t, err)

	config := &oaschema.OpenAPIResourceDefinition{
		Spec: rawSpec,
	}

	client, err := NewProxyClient(context.TODO(), config)
	require.NoError(t, err)

	_, _, err = client.Execute(context.TODO(), http.MethodGet, "/a/fishy/on/a/dishy?fishy=1234.5", nil, nil)
	require.ErrorContains(t, err, "the value is not a valid integer")
}

func TestExecute_QueryParamMinimumInteger_violation(t *testing.T) {
	spec := `openapi: 3.1.0
servers:
  - url: http://localhost:8080
paths:
  /a/fishy/on/a/dishy:
    get:
      parameters:
        - name: fishy
          in: query
          required: true
          schema:
            type: integer
            minimum: 200
      operationId: locateFishy
`

	rawSpec := new(yaml.Node)
	err := yaml.Load([]byte(spec), rawSpec)
	require.NoError(t, err)

	config := &oaschema.OpenAPIResourceDefinition{
		Spec: rawSpec,
	}

	client, err := NewProxyClient(context.TODO(), config)
	require.NoError(t, err)

	_, _, err = client.Execute(context.TODO(), http.MethodGet, "/a/fishy/on/a/dishy?fishy=123", nil, nil)
	require.ErrorContains(t, err, "Number value must be greater than or equal 200, but got: 123")
}

func TestExecute_QueryParamMaximumInteger_violation(t *testing.T) {
	spec := `openapi: 3.1.0
servers:
  - url: http://localhost:8080
paths:
  /a/fishy/on/a/dishy:
    get:
      parameters:
        - name: fishy
          in: query
          required: true
          schema:
            type: integer
            maximum: 200
      operationId: locateFishy
`

	rawSpec := new(yaml.Node)
	err := yaml.Load([]byte(spec), rawSpec)
	require.NoError(t, err)

	config := &oaschema.OpenAPIResourceDefinition{
		Spec: rawSpec,
	}

	client, err := NewProxyClient(context.TODO(), config)
	require.NoError(t, err)

	_, _, err = client.Execute(context.TODO(), http.MethodGet, "/a/fishy/on/a/dishy?fishy=1234", nil, nil)
	require.ErrorContains(t, err, "Number value must be less than or equal 200, but got: 1234")
}

func TestExecute_QueryParamWrongTypeBool(t *testing.T) {
	spec := `openapi: 3.1.0
servers:
  - url: http://localhost:8080
paths:
  /a/fishy/on/a/dishy:
    get:
      parameters:
        - name: fishy
          in: query
          required: true
          schema:
            type: boolean
      operationId: locateFishy
`

	rawSpec := new(yaml.Node)
	err := yaml.Load([]byte(spec), rawSpec)
	require.NoError(t, err)

	config := &oaschema.OpenAPIResourceDefinition{
		Spec: rawSpec,
	}

	client, err := NewProxyClient(context.TODO(), config)
	require.NoError(t, err)

	_, _, err = client.Execute(context.TODO(), http.MethodGet, "/a/fishy/on/a/dishy?fishy=1234", nil, nil)
	require.ErrorContains(t, err, "malformed boolean")
}

func TestExecute_QueryParamInvalidDateFormat(t *testing.T) {
	spec := `openapi: 3.1.0
servers:
  - url: http://localhost:8080
paths:
  /a/fishy/on/a/dishy:
    get:
      parameters:
        - name: fishy
          in: query
          required: true
          schema:
            type: string
            format: date`

	rawSpec := new(yaml.Node)
	err := yaml.Load([]byte(spec), rawSpec)
	require.NoError(t, err)

	config := &oaschema.OpenAPIResourceDefinition{
		Spec: rawSpec,
	}

	client, err := NewProxyClient(context.TODO(), config)
	require.NoError(t, err)

	_, _, err = client.Execute(context.TODO(), http.MethodGet, "/a/fishy/on/a/dishy?fishy=12/25/2024", nil, nil)
	require.ErrorContains(t, err, "invalid date format: 12/25/2024")
}

func TestExecute_QueryParamInvalidTypeArrayStringEnum(t *testing.T) {
	spec := `openapi: 3.1.0
servers:
  - url: http://localhost:8080
paths:
  /a/fishy/on/a/dishy:
    get:
      parameters:
        - name: fishy
          in: query
          required: true
          schema:
            type: array
            items:
              type: string
              enum: [cod, halibut]
      operationId: locateFishy`

	rawSpec := new(yaml.Node)
	err := yaml.Load([]byte(spec), rawSpec)
	require.NoError(t, err)

	config := &oaschema.OpenAPIResourceDefinition{
		Spec: rawSpec,
	}

	client, err := NewProxyClient(context.TODO(), config)
	require.NoError(t, err)

	_, _, err = client.Execute(context.TODO(), http.MethodGet, "/a/fishy/on/a/dishy?fishy=cod&fishy=haddock", nil, nil)
	herr, _ := errors.AsType[*httperror.HTTPError](err)

	require.Len(t, herr.Errors, 1)
	require.Equal(t, herr.Errors[0].Detail, "Value 'haddock' does not match any enum values: [cod, halibut]")
	require.Equal(t, herr.Errors[0].Parameter, "fishy")
	require.Equal(t, herr.Errors[0].Pointer, "/1")
}

func TestExecute_QueryParamInvalidTypeArrayInteger(t *testing.T) {
	spec := `openapi: 3.1.0
servers:
  - url: http://localhost:8080
paths:
  /a/fishy/on/a/dishy:
    get:
      parameters:
        - name: fishy
          in: query
          required: true
          schema:
            type: array
            items:
              type: integer
      operationId: locateFishy`

	rawSpec := new(yaml.Node)
	err := yaml.Load([]byte(spec), rawSpec)
	require.NoError(t, err)

	config := &oaschema.OpenAPIResourceDefinition{
		Spec: rawSpec,
	}

	client, err := NewProxyClient(context.TODO(), config)
	require.NoError(t, err)

	_, _, err = client.Execute(context.TODO(), http.MethodGet, "/a/fishy/on/a/dishy?fishy=cod&fishy=haddock", nil, nil)
	herr, _ := errors.AsType[*httperror.HTTPError](err)

	require.Len(t, herr.Errors, 1)
	require.Equal(t, herr.Errors[0].Detail, "malformed number")
	require.Equal(t, herr.Errors[0].Parameter, "fishy")
	require.Equal(t, herr.Errors[0].Pointer, "/0")
}

func TestExecute_QueryParamInvalidTypeObject(t *testing.T) {
	spec := `openapi: 3.1.0
servers:
  - url: http://localhost:8080
paths:
  /a/fishy/on/a/dishy:
    get:
      parameters:
        - name: fishy
          in: query
          required: true
          schema:
            type: array
            items:
              type: object
              properties:
                vinegar:
                  type: boolean
                chips:
                  type: number
              required:
                - vinegar
                - chips
      operationId: locateFishy`

	rawSpec := new(yaml.Node)
	err := yaml.Load([]byte(spec), rawSpec)
	require.NoError(t, err)

	config := &oaschema.OpenAPIResourceDefinition{
		Spec: rawSpec,
	}

	client, err := NewProxyClient(context.TODO(), config)
	require.NoError(t, err)

	_, _, err = client.Execute(context.TODO(), http.MethodGet, "/a/fishy/on/a/dishy?fishy={\"cod\":\"cakes\"}&fishy={\"crab\":\"legs\"}", nil, nil)
	herr, _ := errors.AsType[*httperror.HTTPError](err)

	require.Len(t, herr.Errors, 1)
	require.Equal(t, herr.Errors[0].Detail, "Unsupported object type or nested fields in parameter")
	require.Equal(t, herr.Errors[0].Parameter, "fishy")
}

func TestExecute_QueryParamInvalidObjectContent(t *testing.T) {
	spec := `openapi: 3.1.0
servers:
  - url: http://localhost:8080
paths:
  /a/fishy/on/a/dishy:
    get:
      parameters:
        - name: fishy
          in: query
          required: true
          content:
            application/json:
              schema:
                type: object
                properties:
                  vinegar:
                    type: boolean
                  chips:
                    type: number
                  required:
                    - vinegar
                    - chips
      operationId: locateFishy`

	rawSpec := new(yaml.Node)
	err := yaml.Load([]byte(spec), rawSpec)
	require.NoError(t, err)

	config := &oaschema.OpenAPIResourceDefinition{
		Spec: rawSpec,
	}

	client, err := NewProxyClient(context.TODO(), config)
	require.NoError(t, err)

	_, _, err = client.Execute(context.TODO(), http.MethodGet, "/a/fishy/on/a/dishy?fishy={\"vinegar\":true,\"chips\":123.223}&fishy={\"vinegar\":\"cakes\",\"chips\":\"hello\"}", nil, nil)
	herr, _ := errors.AsType[*httperror.HTTPError](err)

	require.Len(t, herr.Errors, 1)
	require.Equal(t, herr.Errors[0].Detail, "Invalid type or syntax. Expected the type of value to be one of [object], however the request provided 'array' type")
}

func TestExecute_QueryParamInvalidTypeObjectArrayPropType_Ref(t *testing.T) {
	spec := `openapi: 3.1.0
servers:
  - url: http://localhost:8080
components:
  parameters:
    something:
      name: somethingElse
      in: query
      content:
        application/json:
          schema:
            type: array
            items:
              type: object
              properties:
                vinegar:
                  type: boolean
                chips:
                  type: number
              required:
                - vinegar
                - chips
paths:
  /a/fishy/on/a/dishy:
    get:
      parameters:
        - name: fishy
          in: query
          required: true
          content:
            $ref: "#/components/parameters/something/content"
      operationId: locateFishy`

	rawSpec := new(yaml.Node)
	err := yaml.Load([]byte(spec), rawSpec)
	require.NoError(t, err)

	config := &oaschema.OpenAPIResourceDefinition{
		Spec: rawSpec,
	}

	client, err := NewProxyClient(context.TODO(), config)
	require.NoError(t, err)

	_, _, err = client.Execute(context.TODO(), http.MethodGet, "/a/fishy/on/a/dishy?fishy={\"vinegar\":true,\"chips\":123.223}&fishy={\"vinegar\":\"cakes\",\"chips\":\"hello\"}", nil, nil)
	herr, _ := errors.AsType[*httperror.HTTPError](err)

	require.Len(t, herr.Errors, 2)

	for _, err := range herr.Errors {
		if strings.HasSuffix(err.Pointer, "/vinegar") {
			require.Equal(t, err.Detail, "Invalid type or syntax. Expected the type of value to be one of [boolean], however the request provided 'string' type")
		} else {
			require.Equal(t, err.Detail, "Invalid type or syntax. Expected the type of value to be one of [number], however the request provided 'string' type")
		}
	}
}

func TestExecute_QueryParamValidTypeObjectPropType_JSONInvalid(t *testing.T) {
	spec := `openapi: 3.1.0
servers:
  - url: http://localhost:8080
components:
  parameters:
    fishy:
      name: fishy
      in: query
      content:
        application/json:
          schema:
            type: object
            properties:
              vinegar:
                type: boolean
              chips:
                type: number
            required:
              - vinegar
              - chips
paths:
  /a/fishy/on/a/dishy:
    get:
      parameters:
        - $ref: "#/components/parameters/fishy"
      operationId: locateFishy`

	rawSpec := new(yaml.Node)
	err := yaml.Load([]byte(spec), rawSpec)
	require.NoError(t, err)

	config := &oaschema.OpenAPIResourceDefinition{
		Spec: rawSpec,
	}

	client, err := NewProxyClient(context.TODO(), config)
	require.NoError(t, err)

	_, _, err = client.Execute(context.TODO(), http.MethodGet, "/a/fishy/on/a/dishy?fishy=I am not json", nil, nil)
	herr, _ := errors.AsType[*httperror.HTTPError](err)

	require.Len(t, herr.Errors, 1)
	require.Equal(t, herr.Errors[0].Detail, "invalid JSON: invalid character 'I' looking for beginning of value")
}

func TestExecute_QueryParamInvalidTypeObjectPropType_Ref(t *testing.T) {
	spec := `openapi: 3.1.0
servers:
  - url: http://localhost:8080
components:
  schema_validation:
    chippy:
      type: object
      properties:
        vinegar:
          type: boolean
        chips:
          type: number
      required:
        - vinegar
        - chips
  parameters:
    fishy:
      name: fishy
      in: query
      content:
        application/json:
          schema:
            $ref: "#/components/schema_validation/chippy"
paths:
  /a/fishy/on/a/dishy:
    get:
      parameters:
        - $ref: "#/components/parameters/fishy"
      operationId: locateFishy`

	rawSpec := new(yaml.Node)
	err := yaml.Load([]byte(spec), rawSpec)
	require.NoError(t, err)

	config := &oaschema.OpenAPIResourceDefinition{
		Spec: rawSpec,
	}

	client, err := NewProxyClient(context.TODO(), config)
	require.NoError(t, err)

	_, _, err = client.Execute(context.TODO(), http.MethodGet, "/a/fishy/on/a/dishy?fishy={\"vinegar\":1234,\"chips\":false}", nil, nil)
	herr, _ := errors.AsType[*httperror.HTTPError](err)

	require.Len(t, herr.Errors, 2)

	for _, err := range herr.Errors {
		if strings.HasSuffix(err.Pointer, "/vinegar") {
			require.Equal(t, err.Detail, "Invalid type or syntax. Expected the type of value to be one of [boolean], however the request provided 'number' type")
		} else {
			require.Equal(t, err.Detail, "Invalid type or syntax. Expected the type of value to be one of [number], however the request provided 'boolean' type")
		}
	}
}

func TestExecute_QueryParamValidateStyle_DeepObjectMultiValuesFailedMultipleSchemas(t *testing.T) {
	spec := `openapi: 3.1.0
servers:
  - url: http://localhost:8080
paths:
  /a/fishy/on/a/dishy:
    get:
      parameters:
        - name: fishy
          in: query
          required: true
          style: deepObject
          schema:
            type: object
            properties:
              ocean:
                type: string
              salt:
                type: boolean
            required:
              - ocean
              - salt
        - name: dishy
          in: query
          required: true
          style: deepObject
          schema:
            type: object
            properties:
              size:
                type: string
              numCracks:
                type: number
            required:
              - size
              - numCracks
        - name: cake
          in: query
          required: true
          style: deepObject
          schema:
            type: object
            properties:
              message:
                type: string
              numCandles:
                type: number
            required:
              - message
              - numCandles
      operationId: locateFishy`

	rawSpec := new(yaml.Node)
	err := yaml.Load([]byte(spec), rawSpec)
	require.NoError(t, err)

	config := &oaschema.OpenAPIResourceDefinition{
		Spec: rawSpec,
	}

	client, err := NewProxyClient(context.TODO(), config)
	require.NoError(t, err)

	_, _, err = client.Execute(context.TODO(), http.MethodGet, "https://things.com/a/fishy/on/a/dishy?fishy[ocean]=atlantic&fishy[salt]=12"+
		"&dishy[size]=big&dishy[numCracks]=false"+
		"&cake[message]=happy%20birthday&cake[numCandles]=false", nil, nil)
	herr, _ := errors.AsType[*httperror.HTTPError](err)

	require.Len(t, herr.Errors, 3)
	require.Equal(t, herr.Errors[0].Parameter, "fishy")
	require.Equal(t, herr.Errors[0].Pointer, "/salt")
	require.Equal(t, herr.Errors[0].Detail, "malformed boolean")
	require.Equal(t, herr.Errors[1].Parameter, "dishy")
	require.Equal(t, herr.Errors[1].Pointer, "/numCracks")
	require.Equal(t, herr.Errors[1].Detail, "malformed number")
	require.Equal(t, herr.Errors[2].Parameter, "cake")
	require.Equal(t, herr.Errors[2].Pointer, "/numCandles")
	require.Equal(t, herr.Errors[2].Detail, "malformed number")
}

func TestExecute_SimpleArrayEncodedPath_InvalidInteger(t *testing.T) {
	spec := `openapi: 3.1.0
servers:
  - url: http://localhost:8080
paths:
  /burgers/{burgerIds*}/locate:
    parameters:
      - name: burgerIds
        in: path
        schema:
          type: array
          items:
            type: integer
    get:
      operationId: locateBurgers`

	rawSpec := new(yaml.Node)
	err := yaml.Load([]byte(spec), rawSpec)
	require.NoError(t, err)

	config := &oaschema.OpenAPIResourceDefinition{
		Spec: rawSpec,
	}

	client, err := NewProxyClient(context.TODO(), config)
	require.NoError(t, err)

	_, _, err = client.Execute(context.TODO(), http.MethodGet, "/burgers/1,pizza,3,4,false/locate", nil, nil)
	herr, _ := errors.AsType[*httperror.HTTPError](err)

	require.Len(t, herr.Errors, 1)
	require.Equal(t, herr.Errors[0].Detail, "malformed number")
	require.Equal(t, herr.Errors[0].Parameter, "burgerIds")
}

func TestExecute_HeaderParamMissing(t *testing.T) {
	spec := `openapi: 3.1.0
servers:
  - url: http://localhost:8080
paths:
  /bish/bosh:
    get:
      parameters:
        - name: bash
          in: header
          required: true
          schema:
            type: string`

	rawSpec := new(yaml.Node)
	err := yaml.Load([]byte(spec), rawSpec)
	require.NoError(t, err)

	config := &oaschema.OpenAPIResourceDefinition{
		Spec: rawSpec,
	}

	client, err := NewProxyClient(context.TODO(), config)
	require.NoError(t, err)

	_, _, err = client.Execute(context.TODO(), http.MethodGet, "/bish/bosh", nil, nil)
	herr, _ := errors.AsType[*httperror.HTTPError](err)

	require.Len(t, herr.Errors, 1)
	require.Equal(t, herr.Errors[0].Detail, "Required parameter 'Bash' is missing")
	require.Equal(t, herr.Errors[0].Parameter, "Bash")
}
