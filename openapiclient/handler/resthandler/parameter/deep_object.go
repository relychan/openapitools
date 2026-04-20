package parameter

import (
	"slices"

	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/relychan/goutils"
	"github.com/relychan/openapitools/oaschema"
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

	queryDefs := make([]*highv3.Parameter, 0, len(definitions))

	for _, def := range definitions {
		if def.In == oaschema.InQuery.String() {
			queryDefs = append(queryDefs, def)
		}
	}

	if len(queryDefs) == 0 {
		decodeArbitraryQueryDeepObjectMap(results, rawNodes)

		return nil
	}

	// TODO
	return nil

	// var (
	// 	results     = make(map[string]any)
	// 	parsedKeys = make([]string, 0, len(queryValues))
	// )

	// for _, def := range queryDefs {

	// }

	// if qpe.Schema.Properties != nil {
	// 	for iter := qpe.Schema.Properties.First(); iter != nil; iter = iter.Next() {
	// 		key := iter.Key()

	// 		rawValues, present := qpe.QueryValues[key]
	// 		if !present {
	// 			if len(qpe.Schema.Required) > 0 && slices.Contains(qpe.Schema.Required, key) {
	// 				err := oasvalidator.ObjectRequiredPropertyError(key)
	// 				err.Parameter = qpe.Name

	// 				errs = append(errs, *err)
	// 			}

	// 			continue
	// 		}

	// 		parsedKeys = append(parsedKeys, key)

	// 		schemaProxy := iter.Value()
	// 		if schemaProxy == nil {
	// 			result[key] = rawValues

	// 			continue
	// 		}

	// 		propSchema := schemaProxy.Schema()
	// 		if propSchema == nil {
	// 			result[key] = rawValues

	// 			continue
	// 		}

	// 		propDecoder := &queryParamDecoder{
	// 			Name:      key,
	// 			Style:     qpe.Style,
	// 			Explode:   qpe.Explode,
	// 			RawValues: rawValues,
	// 			Schema:    propSchema,
	// 		}

	// 		value, decodeErrs := propDecoder.Decode()
	// 		if len(decodeErrs) == 0 {
	// 			result[key] = value

	// 			continue
	// 		}

	// 		errs = addParameterErrors(errs, decodeErrs, key)
	// 	}
	// }

	// if qpe.Schema.AdditionalProperties != nil &&
	// 	(qpe.Schema.AdditionalProperties.B || qpe.Schema.AdditionalProperties.A != nil) {
	// 	var propSchema *base.Schema

	// 	if qpe.Schema.AdditionalProperties.N == 0 && qpe.Schema.AdditionalProperties.A != nil {
	// 		propSchema = qpe.Schema.AdditionalProperties.A.Schema()
	// 	}

	// 	for key, rawValues := range qpe.QueryValues {
	// 		if slices.Contains(parsedKeys, key) {
	// 			continue
	// 		}

	// 		if propSchema == nil {
	// 			result[key] = rawValues

	// 			continue
	// 		}

	// 		propDecoder := &queryParamDecoder{
	// 			Name:      key,
	// 			Style:     qpe.Style,
	// 			Explode:   qpe.Explode,
	// 			RawValues: rawValues,
	// 			Schema:    propSchema,
	// 		}

	// 		value, decodeErrs := propDecoder.Decode()
	// 		if len(decodeErrs) == 0 {
	// 			result[key] = value

	// 			continue
	// 		}

	// 		errs = addParameterErrors(errs, decodeErrs, key)
	// 	}
	// }

	// // TODO: patternProperties

	// return result, errs
}

// func decodeArbitraryQueryDeepObject(rawNodes ParameterNodes) any {
// 	nodeLength := len(rawNodes)
// 	if nodeLength == 0 {
// 		return nil
// 	}

// 	if rawNodes[0].key.index != nil {
// 		slices.SortFunc(rawNodes, compareParameterNodes)

// 		results := make([]any, 0, len(rawNodes))

// 		for i, node := range rawNodes {
// 			switch len(node.values) {
// 			case 0:
// 			case 1:
// 				results[i] = node.values[0]
// 			default:
// 				results[i] = node.values
// 			}
// 		}

// 		return results
// 	}

// 	results := make(map[string]any)

// 	decodeArbitraryQueryDeepObjectMap(results, rawNodes)

// 	return results
// }

func decodeArbitraryQueryDeepObjectMap(results map[string]any, rawNodes ParameterNodes) {
	for _, node := range rawNodes {
		switch len(node.values) {
		case 0:
		case 1:
			results[node.key.String()] = node.values[0]
		default:
			results[node.key.String()] = node.values
		}
	}
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

		err := rawNodes.InsertNode(parsedKeys, values)
		if err != nil {
			err.Parameter = key

			errs = append(errs, *err)
		}
	}

	if len(errs) > 0 {
		return nil, errs
	}

	return slices.Clip(rawNodes), nil
}

// func compareParameterNodes(a, b *ParameterNode) int {
// 	if a.key.index == nil && b.key.index != nil {
// 		return 1
// 	}

// 	if a.key.index != nil && b.key.index == nil {
// 		return -1
// 	}

// 	if a.key.index != nil && b.key.index != nil {
// 		if *a.key.index == -1 {
// 			return 1
// 		}

// 		if *b.key.index == -1 {
// 			return 1
// 		}

// 		return *a.key.index - *b.key.index
// 	}

// 	if a.key.key == nil && b.key.key != nil {
// 		return 1
// 	}

// 	if a.key.key != nil && b.key.key == nil {
// 		return -1
// 	}

// 	if a.key.key != nil && b.key.key != nil {
// 		return strings.Compare(*a.key.key, *b.key.key)
// 	}

// 	return 0
// }

func newMixedArrayAndObjectError() *goutils.ErrorDetail {
	return &goutils.ErrorDetail{
		Code:   oasvalidator.ErrCodeInvalidQueryParam,
		Detail: "Query parameters can not contain both array and object",
	}
}
