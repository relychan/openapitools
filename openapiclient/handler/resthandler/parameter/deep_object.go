package parameter

import (
	"slices"

	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/relychan/goutils"
	"github.com/relychan/openapitools/oasvalidator"
)

func decodeQueryDeepObjectFromParameters(
	definitions []*highv3.Parameter,
	queryValues map[string][]string,
	results map[string]any,
) []goutils.ErrorDetail {
	rawNodes, errs := parseDeepObjectNodes(queryValues)
	if len(errs) > 0 {
		return errs
	}

	if len(definitions) == 0 {
		for _, node := range rawNodes {
			node.decodeArbitraryObject(results)
		}

		return nil
	}

	for _, def := range definitions {
		value, decodeErrs := decodeQueryDeepObjectFromParameter(def, rawNodes)
		if len(decodeErrs) > 0 {
			errs = append(errs, decodeErrs...)
		} else {
			results[def.Name] = value
		}
	}

	return errs
}

func decodeQueryDeepObjectFromParameter(
	definition *highv3.Parameter,
	rawNodes ParameterNodes,
) (any, []goutils.ErrorDetail) {
	node := rawNodes.Find(NewKey(definition.Name))
	if node == nil {
		if definition.Required != nil && *definition.Required {
			err := oasvalidator.ParameterRequiredError(definition.Name)
			err.Code = oasvalidator.ErrCodeInvalidQueryParam

			return nil, []goutils.ErrorDetail{*err}
		}

		return nil, nil
	}

	if definition.Schema == nil {
		return node.decodeArbitrary(), nil
	}

	schemaDef := definition.Schema.Schema()
	if schemaDef == nil {
		return node.decodeArbitrary(), nil
	}

	return node.Decode(schemaDef)
}

func parseDeepObjectNodes(queryValues map[string][]string) (ParameterNodes, []goutils.ErrorDetail) {
	var (
		rawNodes = make(ParameterNodes, 0, len(queryValues))
		errs     []goutils.ErrorDetail
	)

	for key, values := range queryValues {
		if key == "" {
			continue
		}

		parsedKeys, ok := parseDeepObjectKey(key)
		if !ok {
			errs = append(errs, goutils.ErrorDetail{
				Code:      oasvalidator.ErrCodeInvalidQueryParam,
				Detail:    "Invalid syntax from query key",
				Parameter: key,
			})

			continue
		}

		err := rawNodes.Insert(parsedKeys, values)
		if err != nil {
			err.Parameter = key

			errs = append(errs, *err)
		}
	}

	if len(errs) > 0 {
		return nil, errs
	}

	// Normalize array elements in the tree.
	for _, node := range rawNodes {
		node.Normalize()
	}

	return slices.Clip(rawNodes), nil
}

func newMixedArrayAndObjectError() *goutils.ErrorDetail {
	return &goutils.ErrorDetail{
		Code:   oasvalidator.ErrCodeInvalidQueryParam,
		Detail: "Query parameters can not contain both array and object",
	}
}
