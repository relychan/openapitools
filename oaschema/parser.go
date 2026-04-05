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
	"errors"
	"strings"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
	lowbase "github.com/pb33f/libopenapi/datamodel/low/base"
	v3low "github.com/pb33f/libopenapi/datamodel/low/v3"
	"github.com/pb33f/libopenapi/index"
	"github.com/pb33f/libopenapi/utils"
	"go.yaml.in/yaml/v4"
)

var (
	ErrInvalidOpenAPIv3Spec = errors.New(
		"spec is defined as an OpenAPI 3.x, but is using a swagger (2.0), or unknown version",
	)
	ErrInvalidOpenAPIVersion = errors.New("unable to extract OpenAPI version")
)

// Accepts an OpenAPI/Swagger specification that is a YAML node
// and will return a SpecInfo pointer, which contains details on the version and an un-marshaled
// ensures the document is an OpenAPI document.
// The function is inspired from the original [libopenapi] package. However, the input is a YAML node so the function does not to decode again to improve performance.
//
// [libopenapi]: https://github.com/pb33f/libopenapi/blob/badb17a26aaf89190194e2fbdb1590b08ef25328/datamodel/spec_info.go#L75
func extractOpenAPIv3SpecInfoFromYAML(parsedSpec *yaml.Node) (*datamodel.SpecInfo, error) {
	specInfo := &datamodel.SpecInfo{
		OriginalIndentation: 2,
		RootNode:            parsedSpec,
		SpecType:            datamodel.YAMLFileType,
	}

	_, openAPI3 := utils.FindKeyNode(utils.OpenApi3, parsedSpec.Content)
	if openAPI3 == nil {
		specInfo.Error = ErrInvalidOpenAPIv3Spec

		return specInfo, specInfo.Error
	}

	version, majorVersion, versionError := parseVersionTypeData(openAPI3.Value)
	if versionError != nil {
		return nil, versionError
	}

	specInfo.SpecType = utils.OpenApi3
	specInfo.Version = version
	specInfo.SpecFormat = datamodel.OAS3

	// Extract the prefix version
	prefixVersion := specInfo.Version
	if len(specInfo.Version) >= 3 {
		prefixVersion = specInfo.Version[:3]
	}

	switch prefixVersion {
	case "3.1":
		specInfo.VersionNumeric = 3.1
		specInfo.APISchema = datamodel.OpenAPI31SchemaData
		specInfo.SpecFormat = datamodel.OAS31
		// extract $self field for OpenAPI 3.1+ (might be used as forward-compatible feature)
		_, selfNode := utils.FindKeyNode("$self", parsedSpec.Content)
		if selfNode != nil && selfNode.Value != "" {
			specInfo.Self = selfNode.Value
		}
	case "3.2":
		specInfo.VersionNumeric = 3.2
		specInfo.APISchema = datamodel.OpenAPI32SchemaData
		specInfo.SpecFormat = datamodel.OAS32
		// extract $self field for OpenAPI 3.2+
		_, selfNode := utils.FindKeyNode("$self", parsedSpec.Content)
		if selfNode != nil && selfNode.Value != "" {
			specInfo.Self = selfNode.Value
		}
	default:
		specInfo.VersionNumeric = 3.0
		specInfo.APISchema = datamodel.OpenAPI3SchemaData
	}

	// double check for the right version, people mix this up.
	if majorVersion < 3 {
		specInfo.Error = ErrInvalidOpenAPIv3Spec

		return specInfo, specInfo.Error
	}

	return specInfo, nil
}

func buildV3Model(info *datamodel.SpecInfo) (*libopenapi.DocumentModel[v3high.Document], error) {
	var (
		errs   []error
		lowDoc *v3low.Document
		docErr error
	)

	config := datamodel.NewDocumentConfiguration()

	lowDoc, docErr = v3low.CreateDocumentFromConfig(info, config)
	if docErr != nil {
		errs = append(errs, utils.UnwrapErrors(docErr)...)
	}

	// Do not short-circuit on circular reference errors, so the client
	// has the option of ignoring them.
	for _, err := range utils.UnwrapErrors(docErr) {
		refErr, ok := errors.AsType[*index.ResolvingError](err)
		if ok {
			if refErr.CircularReference == nil {
				return nil, errors.Join(errs...)
			}
		}
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	highDoc := v3high.NewDocument(lowDoc)
	highDoc.Rolodex = lowDoc.Index.GetRolodex()

	highOpenAPI3Model := &libopenapi.DocumentModel[v3high.Document]{
		Model: *highDoc,
		Index: lowDoc.Index,
	}

	lowbase.SchemaQuickHashMap.Clear()

	return highOpenAPI3Model, nil
}

// extract version number from specification.
func parseVersionTypeData(d string) (string, int, error) {
	r := strings.TrimSpace(d)

	if len(r) == 0 {
		return "", 0, ErrInvalidOpenAPIVersion
	}

	return r, int(r[0]) - '0', nil
}
