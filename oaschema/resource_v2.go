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
	"log"
	"slices"
	"strings"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	v2high "github.com/pb33f/libopenapi/datamodel/high/v2"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/pb33f/libopenapi/utils"
	"go.yaml.in/yaml/v4"
)

type swaggerToOpenAPIv3Converter struct {
	swagger *v2high.Swagger
}

func convertSwaggerToOpenAPIv3Document(src *v2high.Swagger) (*v3high.Document, error) {
	sto := &swaggerToOpenAPIv3Converter{
		swagger: src,
	}

	return sto.Convert()
}

// Convert openapi swagger to OpenAPI v3 document.
func (sto *swaggerToOpenAPIv3Converter) Convert() (*v3high.Document, error) {
	newDoc := &v3high.Document{
		Version:      "3.0.0",
		Info:         sto.swagger.Info,
		Tags:         sto.swagger.Tags,
		ExternalDocs: sto.swagger.ExternalDocs,
		Extensions:   sto.swagger.Extensions,
		Security:     sto.swagger.Security,
		Components:   &v3high.Components{},
	}

	if sto.swagger.Host != "" {
		scheme := "https"

		if len(sto.swagger.Schemes) > 0 && !slices.Contains(sto.swagger.Schemes, scheme) {
			scheme = sto.swagger.Schemes[0]
		}

		serverURL := scheme + "://" + sto.swagger.Host

		if sto.swagger.BasePath != "" && sto.swagger.BasePath != "/" {
			serverURL += sto.swagger.BasePath
		}

		newDoc.Servers = []*v3high.Server{
			{
				URL: serverURL,
			},
		}
	}

	newDoc.Components.SecuritySchemes = sto.convertSecurityDefinitions()

	if sto.swagger.Definitions != nil && sto.swagger.Definitions.Definitions != nil {
		newDoc.Components.Schemas = sto.swagger.Definitions.Definitions
	}

	newDoc.Paths = sto.convertPaths()

	// render and replace the definition references to components/*
	rawYamlBytes, err := newDoc.Render()
	if err != nil {
		return nil, err
	}

	rawYamlString := strings.ReplaceAll(
		string(rawYamlBytes),
		"#/definitions/",
		"#/components/schemas/",
	)

	docV3, err := libopenapi.NewDocument([]byte(rawYamlString))
	if err != nil {
		return nil, err
	}

	result, err := docV3.BuildV3Model()
	if err != nil {
		return nil, err
	}

	return &result.Model, nil
}

func (sto *swaggerToOpenAPIv3Converter) convertPaths() *v3high.Paths {
	paths := sto.swagger.Paths
	if paths == nil {
		return nil
	}

	result := &v3high.Paths{
		Extensions: paths.Extensions,
	}

	if paths.PathItems == nil {
		return result
	}

	result.PathItems = orderedmap.New[string, *v3high.PathItem]()

	for iter := paths.PathItems.Oldest(); iter != nil; iter = iter.Next() {
		if iter.Value == nil {
			continue
		}

		item := sto.convertPathItem(iter.Value)
		result.PathItems.Set(iter.Key, item)
	}

	return result
}

func (sto *swaggerToOpenAPIv3Converter) convertPathItem(
	pathItem *v2high.PathItem,
) *v3high.PathItem {
	result := &v3high.PathItem{
		Extensions: pathItem.Extensions,
		Reference:  pathItem.Ref,
		Parameters: make([]*v3high.Parameter, len(pathItem.Parameters)),
	}

	for i, param := range pathItem.Parameters {
		item := sto.convertParameter(param)
		result.Parameters[i] = item
	}

	result.Get = sto.convertOperation(pathItem.Get)
	result.Post = sto.convertOperation(pathItem.Post)
	result.Put = sto.convertOperation(pathItem.Put)
	result.Delete = sto.convertOperation(pathItem.Delete)
	result.Patch = sto.convertOperation(pathItem.Patch)
	result.Head = sto.convertOperation(pathItem.Head)
	result.Options = sto.convertOperation(pathItem.Options)

	return result
}

func (sto *swaggerToOpenAPIv3Converter) convertOperation(
	operation *v2high.Operation,
) *v3high.Operation {
	if operation == nil {
		return nil
	}

	result := &v3high.Operation{
		Tags:         operation.Tags,
		Extensions:   operation.Extensions,
		Summary:      operation.Summary,
		Description:  operation.Description,
		ExternalDocs: operation.ExternalDocs,
		OperationId:  operation.OperationId,
		Deprecated:   &operation.Deprecated,
		Security:     operation.Security,
	}
	formDataProps := orderedmap.New[string, *base.SchemaProxy]()

	consumes := sto.swagger.Consumes
	if len(operation.Produces) > 0 {
		consumes = operation.Consumes
	}

	if len(operation.Parameters) > 0 {
		result.Parameters = make([]*v3high.Parameter, 0, len(operation.Parameters))

		for _, param := range operation.Parameters {
			location, _ := ParseParameterLocation(param.In)
			log.Println("location", param.In, location)

			switch location {
			case InFormData:
				if param.Name == "" {
					continue
				}

				schema := sto.schemaFromParameter(param)
				formDataProps.Set(param.Name, schema)
			case InBody:
				result.RequestBody = &v3high.RequestBody{
					Content:     orderedmap.New[string, *v3high.MediaType](),
					Extensions:  param.Extensions,
					Description: param.Description,
					Required:    param.Required,
				}

				content := &v3high.MediaType{
					Schema:     param.Schema,
					Extensions: param.Extensions,
				}

				if param.Schema == nil {
					content.Schema = sto.schemaFromParameter(param)
				}

				for _, mediaType := range consumes {
					_, present := result.RequestBody.Content.Get(mediaType)
					if present {
						continue
					}

					result.RequestBody.Content.Set(mediaType, content)
				}
			default:
				item := sto.convertParameter(param)
				result.Parameters = append(result.Parameters, item)
			}
		}

		if len(result.Parameters) != len(operation.Parameters) {
			result.Parameters = slices.Clip(result.Parameters)
		}
	}

	if formDataProps.Len() > 0 {
		result.RequestBody = &v3high.RequestBody{
			Content:  orderedmap.New[string, *v3high.MediaType](),
			Required: new(true),
		}

		content := &v3high.MediaType{
			Schema: base.CreateSchemaProxy(&base.Schema{
				Type:       []string{"object"},
				Properties: formDataProps,
			}),
		}

		for _, mediaType := range consumes {
			_, present := result.RequestBody.Content.Get(mediaType)
			if present {
				continue
			}

			result.RequestBody.Content.Set(mediaType, content)
		}
	}

	if operation.Responses == nil {
		return result
	}

	produces := sto.swagger.Produces
	if len(operation.Produces) > 0 {
		produces = operation.Produces
	}

	result.Responses = &v3high.Responses{
		Extensions: operation.Responses.Extensions,
		Default:    sto.convertResponse(operation.Responses.Default, produces),
		Codes:      sto.convertResponsesMap(operation.Responses.Codes, produces),
	}

	return result
}

func (sto *swaggerToOpenAPIv3Converter) convertResponsesMap(
	responses *orderedmap.Map[string, *v2high.Response],
	produces []string,
) *orderedmap.Map[string, *v3high.Response] {
	if responses == nil || responses.Len() == 0 {
		return nil
	}

	result := orderedmap.New[string, *v3high.Response]()

	for iter := responses.Oldest(); iter != nil; iter = iter.Next() {
		resp := sto.convertResponse(iter.Value, produces)
		if resp != nil {
			result.Set(iter.Key, resp)
		}
	}

	return result
}

func (sto *swaggerToOpenAPIv3Converter) convertResponse(
	value *v2high.Response,
	produces []string,
) *v3high.Response {
	if value == nil {
		return nil
	}

	result := &v3high.Response{
		Description: value.Description,
		Extensions:  value.Extensions,
		Headers:     sto.convertHeaders(value.Headers),
	}

	if len(produces) == 0 {
		return result
	}

	content := &v3high.MediaType{
		Schema: value.Schema,
	}

	if value.Examples != nil && value.Examples.Values != nil &&
		value.Examples.Values.Len() > 0 {
		content.Examples = orderedmap.New[string, *base.Example]()

		for iter := value.Examples.Values.Oldest(); iter != nil; iter = iter.Next() {
			example := &base.Example{
				Value: iter.Value,
			}

			content.Examples.Set(iter.Key, example)
		}
	}

	result.Content = orderedmap.New[string, *v3high.MediaType]()

	for _, mediaType := range produces {
		_, present := result.Content.Get(mediaType)
		if present {
			continue
		}

		result.Content.Set(mediaType, content)
	}

	return result
}

func (sto *swaggerToOpenAPIv3Converter) convertSecurityDefinitions() *orderedmap.Map[string, *v3high.SecurityScheme] {
	securityDefinitions := sto.swagger.SecurityDefinitions
	if securityDefinitions == nil || securityDefinitions.Definitions == nil ||
		securityDefinitions.Definitions.Len() == 0 {
		return nil
	}

	result := orderedmap.New[string, *v3high.SecurityScheme]()

	for iter := securityDefinitions.Definitions.Oldest(); iter != nil; iter = iter.Next() {
		if iter.Value == nil {
			continue
		}

		security := sto.convertSecurityScheme(iter.Value)
		result.Set(iter.Key, security)
	}

	return result
}

func (*swaggerToOpenAPIv3Converter) convertSecurityScheme(
	value *v2high.SecurityScheme,
) *v3high.SecurityScheme {
	result := &v3high.SecurityScheme{
		Description: value.Description,
		Name:        value.Name,
		Extensions:  value.Extensions,
		In:          value.In,
		Type:        value.Type,
	}

	schemeType, _ := ParseSecuritySchemeType(value.Type)

	switch schemeType {
	case BasicAuthScheme:
		result.Type = HTTPAuthScheme.String()
		result.Scheme = BasicAuthScheme.String()
	case OAuth2Scheme:
		flow := &v3high.OAuthFlow{
			AuthorizationUrl: value.AuthorizationUrl,
			TokenUrl:         value.TokenUrl,
			Scopes:           value.Scopes.Values,
		}

		if value.Scopes != nil && value.Scopes.Values != nil {
			flow.Scopes = value.Scopes.Values
		}

		result.Flows = &v3high.OAuthFlows{}

		switch value.Flow {
		case "accessCode":
			result.Flows.AuthorizationCode = flow
		case "implicit":
			result.Flows.Implicit = flow
		case "password":
			result.Flows.Password = flow
		case "application":
			result.Flows.ClientCredentials = flow
		default:
		}
	default:
	}

	return result
}

func (sto *swaggerToOpenAPIv3Converter) convertParameter(
	parameter *v2high.Parameter,
) *v3high.Parameter {
	param := &v3high.Parameter{
		Name:        parameter.Name,
		In:          parameter.In,
		Description: parameter.Description,
		Required:    parameter.Required,
		Schema:      parameter.Schema,
		Example:     parameter.Default,
		Extensions:  parameter.Extensions,
	}

	if parameter.AllowEmptyValue != nil {
		param.AllowEmptyValue = *parameter.AllowEmptyValue
	}

	location, _ := ParseParameterLocation(parameter.In)
	style, explode := getStyleFromCollectionFormat(location, parameter.CollectionFormat)

	param.Style = style
	param.Explode = &explode

	if param.Schema != nil {
		return param
	}

	param.Schema = sto.schemaFromParameter(parameter)

	return param
}

func (sto *swaggerToOpenAPIv3Converter) schemaFromParameter(
	parameter *v2high.Parameter,
) *base.SchemaProxy {
	baseSchema := &base.Schema{
		Format:      parameter.Format,
		Pattern:     parameter.Pattern,
		Default:     parameter.Default,
		UniqueItems: parameter.UniqueItems,
		Enum:        parameter.Enum,
	}

	if parameter.Type != "" {
		baseSchema.Type = []string{parameter.Type}
	}

	if parameter.Maximum != nil {
		maximum := float64(*parameter.Maximum)

		baseSchema.Maximum = &maximum
	}

	if parameter.Minimum != nil {
		minimum := float64(*parameter.Minimum)

		baseSchema.Minimum = &minimum
	}

	if parameter.MultipleOf != nil {
		multipleOf := float64(*parameter.MultipleOf)

		baseSchema.MultipleOf = &multipleOf
	}

	if parameter.ExclusiveMinimum != nil {
		baseSchema.ExclusiveMinimum = &base.DynamicValue[bool, float64]{
			A: *parameter.ExclusiveMinimum,
		}
	}

	if parameter.ExclusiveMaximum != nil {
		baseSchema.ExclusiveMaximum = &base.DynamicValue[bool, float64]{
			A: *parameter.ExclusiveMaximum,
		}
	}

	if parameter.MaxLength != nil && *parameter.MaxLength > 0 {
		maxLength := int64(*parameter.MaxLength)
		baseSchema.MaxLength = &maxLength
	}

	if parameter.MinLength != nil && *parameter.MinLength > 0 {
		minLength := int64(*parameter.MinLength)
		baseSchema.MaxLength = &minLength
	}

	if parameter.MaxItems != nil && *parameter.MaxItems > 0 {
		maxItems := int64(*parameter.MaxItems)
		baseSchema.MaxItems = &maxItems
	}

	if parameter.MinItems != nil && *parameter.MaxItems > 0 {
		minItems := int64(*parameter.MinItems)
		baseSchema.MinItems = &minItems
	}

	if parameter.Items != nil {
		baseSchema.Items = sto.convertItems(parameter.Items)
	}

	return base.CreateSchemaProxy(baseSchema)
}

func (sto *swaggerToOpenAPIv3Converter) convertHeaders(
	value *orderedmap.Map[string, *v2high.Header],
) *orderedmap.Map[string, *v3high.Header] {
	if value == nil || value.Len() == 0 {
		return nil
	}

	result := orderedmap.New[string, *v3high.Header]()

	for iter := value.Oldest(); iter != nil; iter = iter.Next() {
		if iter.Value == nil {
			continue
		}

		header := sto.convertHeader(iter.Value)
		result.Set(iter.Key, header)
	}

	return result
}

func (sto *swaggerToOpenAPIv3Converter) convertHeader(value *v2high.Header) *v3high.Header {
	baseSchema := &base.Schema{
		Format:      value.Format,
		Pattern:     value.Pattern,
		UniqueItems: &value.UniqueItems,
	}

	if value.Default != nil {
		baseSchema.Default = utils.CreateYamlNode(value.Default)
	}

	if value.Type != "" {
		baseSchema.Type = []string{value.Type}
	}

	if len(value.Enum) > 0 {
		baseSchema.Enum = make([]*yaml.Node, len(value.Enum))

		for i, enum := range value.Enum {
			baseSchema.Enum[i] = utils.CreateYamlNode(enum)
		}
	}

	maximum := float64(value.Maximum)
	baseSchema.Maximum = &maximum
	minimum := float64(value.Minimum)
	baseSchema.Minimum = &minimum
	multipleOf := float64(value.MultipleOf)
	baseSchema.MultipleOf = &multipleOf
	baseSchema.ExclusiveMinimum = &base.DynamicValue[bool, float64]{
		A: value.ExclusiveMinimum,
	}

	baseSchema.ExclusiveMaximum = &base.DynamicValue[bool, float64]{
		A: value.ExclusiveMaximum,
	}

	if value.MaxLength > 0 {
		maxLength := int64(value.MaxLength)
		baseSchema.MaxLength = &maxLength
	}

	if value.MinLength > 0 {
		minLength := int64(value.MinLength)
		baseSchema.MaxLength = &minLength
	}

	if value.MaxItems > 0 {
		maxItems := int64(value.MaxItems)
		baseSchema.MaxItems = &maxItems
	}

	if value.MinItems > 0 {
		minItems := int64(value.MinItems)
		baseSchema.MinItems = &minItems
	}

	if value.Items != nil {
		baseSchema.Items = sto.convertItems(value.Items)
	}

	result := &v3high.Header{
		Description: value.Description,
		Extensions:  value.Extensions,
		Schema:      base.CreateSchemaProxy(baseSchema),
		Explode:     value.CollectionFormat == "multi",
	}

	return result
}

func (sto *swaggerToOpenAPIv3Converter) convertItems(
	items *v2high.Items,
) *base.DynamicValue[*base.SchemaProxy, bool] {
	baseSchema := &base.Schema{
		Format:      items.Format,
		Pattern:     items.Pattern,
		Default:     items.Default,
		UniqueItems: &items.UniqueItems,
		Enum:        items.Enum,
	}

	if items.Type != "" {
		baseSchema.Type = []string{items.Type}
	}

	maximum := float64(items.Maximum)
	baseSchema.Maximum = &maximum
	minimum := float64(items.Minimum)
	baseSchema.Minimum = &minimum
	multipleOf := float64(items.MultipleOf)
	baseSchema.MultipleOf = &multipleOf
	baseSchema.ExclusiveMinimum = &base.DynamicValue[bool, float64]{
		A: items.ExclusiveMinimum,
	}

	baseSchema.ExclusiveMaximum = &base.DynamicValue[bool, float64]{
		A: items.ExclusiveMaximum,
	}

	if items.MaxLength > 0 {
		maxLength := int64(items.MaxLength)
		baseSchema.MaxLength = &maxLength
	}

	if items.MinLength > 0 {
		minLength := int64(items.MinLength)
		baseSchema.MaxLength = &minLength
	}

	if items.MaxItems > 0 {
		maxItems := int64(items.MaxItems)
		baseSchema.MaxItems = &maxItems
	}

	if items.MinItems > 0 {
		minItems := int64(items.MinItems)
		baseSchema.MinItems = &minItems
	}

	if items.Items != nil {
		baseSchema.Items = sto.convertItems(items.Items)
	}

	return &base.DynamicValue[*base.SchemaProxy, bool]{
		N: 0,
		A: base.CreateSchemaProxy(baseSchema),
	}
}

func getStyleFromCollectionFormat(
	location ParameterLocation,
	collectionFormat string,
) (string, bool) {
	multiFormat := "multi"
	formStyle := "form"

	switch location {
	case InPath, InHeader:
		return "simple", collectionFormat == multiFormat
	case InQuery:
		switch collectionFormat {
		case "ssv":
			return "spaceDelimited", false
		case "tsv", "pipes":
			return "pipeDelimited", false
		case multiFormat:
			return formStyle, true
		default:
			return formStyle, false
		}
	case InCookie:
		return formStyle, collectionFormat == multiFormat
	default:
		return "", false
	}
}
