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

package parameter

import (
	"net/http"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/relychan/goutils/httperror"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/oasvalidator"
	"github.com/stretchr/testify/require"
)

func TestDecodeCookieParameters_NumberValid(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: number`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	pathItem, _ := m.Model.Paths.PathItems.Get("/burgers/beef")
	params, errs := oasvalidator.ValidateParameterDefinitions(pathItem.Get.Parameters)
	require.Len(t, errs, 0)

	inputs := []*http.Cookie{
		{Name: "PattyPreference", Value: "123.45"},
	}

	expected := map[string]any{
		"PattyPreference": float64(123.45),
	}

	results, errs := DecodeCookieParameters(params, inputs)
	require.Len(t, errs, 0)
	require.Equal(t, expected, results)
}

func TestDecodeCookieParameters_CookieParamNumberInvalid(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: number`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	pathItem, _ := m.Model.Paths.PathItems.Get("/burgers/beef")
	params, errs := oasvalidator.ValidateParameterDefinitions(pathItem.Get.Parameters)
	require.Len(t, errs, 0)

	inputs := []*http.Cookie{
		{Name: "PattyPreference", Value: "false"},
	}

	expected := []httperror.ValidationError{
		{
			Detail:    "malformed number",
			Location:  oaschema.CookieKey,
			Parameter: "PattyPreference",
			Code:      oasvalidator.ErrCodeInvalidCookie,
		},
	}

	_, errs = DecodeCookieParameters(params, inputs)
	require.Len(t, errs, 1)
	require.Equal(t, expected, errs)
}

func TestDecodeCookieParameters_CookieParamInteger(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: integer`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	pathItem, _ := m.Model.Paths.PathItems.Get("/burgers/beef")
	params, errs := oasvalidator.ValidateParameterDefinitions(pathItem.Get.Parameters)
	require.Len(t, errs, 0)

	inputs := []*http.Cookie{
		{Name: "PattyPreference", Value: "1"},
	}

	expected := map[string]any{
		"PattyPreference": int64(1),
	}

	result, errs := DecodeCookieParameters(params, inputs)
	require.Len(t, errs, 0)
	require.Equal(t, expected, result)

	invalidInput := []*http.Cookie{
		{Name: "PattyPreference", Value: "false"},
	}

	expectedErrs := []httperror.ValidationError{
		{
			Detail:    "malformed number",
			Location:  oaschema.CookieKey,
			Parameter: "PattyPreference",
			Code:      oasvalidator.ErrCodeInvalidCookie,
		},
	}

	result, errs = DecodeCookieParameters(params, invalidInput)
	require.Len(t, errs, 1)
	require.Equal(t, expectedErrs, errs)
}

func TestDecodeCookieParameters_CookieParamBoolean(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: boolean`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	pathItem, _ := m.Model.Paths.PathItems.Get("/burgers/beef")
	params, errs := oasvalidator.ValidateParameterDefinitions(pathItem.Get.Parameters)
	require.Len(t, errs, 0)

	inputs := []*http.Cookie{
		{Name: "PattyPreference", Value: "true"},
	}

	expected := map[string]any{
		"PattyPreference": true,
	}

	result, errs := DecodeCookieParameters(params, inputs)
	require.Len(t, errs, 0)
	require.Equal(t, expected, result)

	invalidInput := []*http.Cookie{
		{Name: "PattyPreference", Value: "chicken"},
	}

	expectedErrs := []httperror.ValidationError{
		{
			Detail:    "malformed boolean",
			Location:  oaschema.CookieKey,
			Parameter: "PattyPreference",
			Code:      oasvalidator.ErrCodeInvalidCookie,
		},
	}

	result, errs = DecodeCookieParameters(params, invalidInput)
	require.Len(t, errs, 1)
	require.Equal(t, expectedErrs, errs)
}

func TestDecodeCookieParameters_CookieParamEnumString(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: string
            enum:
              - beef
              - chicken
              - pea protein`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	pathItem, _ := m.Model.Paths.PathItems.Get("/burgers/beef")
	params, errs := oasvalidator.ValidateParameterDefinitions(pathItem.Get.Parameters)
	require.Len(t, errs, 0)

	inputs := []*http.Cookie{
		{Name: "PattyPreference", Value: "pea protein"},
	}

	expected := map[string]any{
		"PattyPreference": "pea protein",
	}

	result, errs := DecodeCookieParameters(params, inputs)
	require.Len(t, errs, 0)
	require.Equal(t, expected, result)

	invalidInput := []*http.Cookie{
		{Name: "PattyPreference", Value: "pork"},
	}

	expectedErrs := []httperror.ValidationError{
		{
			Detail:    "Value 'pork' does not match any enum values: [beef, chicken, pea protein]",
			Location:  oaschema.CookieKey,
			Parameter: "PattyPreference",
			Code:      oasvalidator.ErrCodeInvalidCookie,
		},
	}

	result, errs = DecodeCookieParameters(params, invalidInput)
	require.Len(t, errs, 1)
	require.Equal(t, expectedErrs, errs)
}

func TestDecodeCookieParameters_CookieParamObject(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          explode: false
          schema:
            type: object
            properties:
              pink:
                type: boolean
              number:
                type: number
            required: [pink, number]`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	pathItem, _ := m.Model.Paths.PathItems.Get("/burgers/beef")
	params, errs := oasvalidator.ValidateParameterDefinitions(pathItem.Get.Parameters)
	require.Len(t, errs, 0)

	inputs := []*http.Cookie{
		{Name: "PattyPreference", Value: "pink,true,number,2"},
	}

	expected := map[string]any{
		"PattyPreference": map[string]any{
			"pink":   true,
			"number": float64(2),
		},
	}

	result, errs := DecodeCookieParameters(params, inputs)
	require.Len(t, errs, 0)
	require.Equal(t, expected, result)

	invalidInput := []*http.Cookie{
		{Name: "PattyPreference", Value: "pink,2,number,2"},
	}

	expectedErrs := []httperror.ValidationError{
		{
			Detail:    "malformed boolean",
			Location:  oaschema.CookieKey,
			Parameter: "PattyPreference",
			Code:      oasvalidator.ErrCodeInvalidCookie,
			Pointer:   "/pink",
		},
	}

	result, errs = DecodeCookieParameters(params, invalidInput)
	require.Len(t, errs, 1)
	require.Equal(t, expectedErrs, errs)
}

func TestDecodeCookieParameters_CookieParamArrayNumber(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: array
            items:
              type: number`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	pathItem, _ := m.Model.Paths.PathItems.Get("/burgers/beef")
	params, errs := oasvalidator.ValidateParameterDefinitions(pathItem.Get.Parameters)
	require.Len(t, errs, 0)

	inputs := []*http.Cookie{
		{Name: "PattyPreference", Value: "2,3,4"},
	}

	expected := map[string]any{
		"PattyPreference": []any{float64(2), float64(3), float64(4)},
	}

	result, errs := DecodeCookieParameters(params, inputs)
	require.Len(t, errs, 0)
	require.Equal(t, expected, result)

	invalidInput := []*http.Cookie{
		{Name: "PattyPreference", Value: "2,true,4,'hello'"},
	}

	expectedErrs := []httperror.ValidationError{
		{
			Detail:    "malformed number",
			Location:  oaschema.CookieKey,
			Parameter: "PattyPreference",
			Code:      oasvalidator.ErrCodeInvalidCookie,
			Pointer:   "/1",
		},
	}

	result, errs = DecodeCookieParameters(params, invalidInput)
	require.Len(t, errs, 1)
	require.Equal(t, expectedErrs, errs)
}

func TestDecodeCookieParameters_CookieParamArrayInteger(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: array
            items:
              type: integer`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	pathItem, _ := m.Model.Paths.PathItems.Get("/burgers/beef")
	params, errs := oasvalidator.ValidateParameterDefinitions(pathItem.Get.Parameters)
	require.Len(t, errs, 0)

	inputs := []*http.Cookie{
		{Name: "PattyPreference", Value: "2,3,4"},
	}

	expected := map[string]any{
		"PattyPreference": []any{int64(2), int64(3), int64(4)},
	}

	result, errs := DecodeCookieParameters(params, inputs)
	require.Len(t, errs, 0)
	require.Equal(t, expected, result)

	invalidInput := []*http.Cookie{
		{Name: "PattyPreference", Value: "2,true,4,'hello'"},
	}

	expectedErrs := []httperror.ValidationError{
		{
			Detail:    "malformed number",
			Location:  oaschema.CookieKey,
			Parameter: "PattyPreference",
			Code:      oasvalidator.ErrCodeInvalidCookie,
			Pointer:   "/1",
		},
	}

	result, errs = DecodeCookieParameters(params, invalidInput)
	require.Len(t, errs, 1)
	require.Equal(t, expectedErrs, errs)
}

func TestDecodeCookieParameters_CookieRequired(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: number
        - name: BunType
          in: cookie
          required: true
          schema:
            type: string`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	pathItem, _ := m.Model.Paths.PathItems.Get("/burgers/beef")
	params, errs := oasvalidator.ValidateParameterDefinitions(pathItem.Get.Parameters)
	require.Len(t, errs, 0)

	expectedErrs := []httperror.ValidationError{
		{
			Detail:    "Required parameter 'PattyPreference' is missing",
			Location:  oaschema.CookieKey,
			Parameter: "PattyPreference",
			Code:      oasvalidator.ErrCodeRequired,
		},
		{
			Detail:    "Required parameter 'BunType' is missing",
			Location:  oaschema.CookieKey,
			Parameter: "BunType",
			Code:      oasvalidator.ErrCodeRequired,
		},
	}

	_, errs = DecodeCookieParameters(params, nil)
	require.Equal(t, expectedErrs, errs)

	inputs := []*http.Cookie{
		{Name: "PattyPreference", Value: "1.5"},
		{Name: "BunType", Value: "sesame"},
	}

	expected := map[string]any{
		"PattyPreference": float64(1.5),
		"BunType":         "sesame",
	}

	result, errs := DecodeCookieParameters(params, inputs)
	require.Len(t, errs, 0)
	require.Equal(t, expected, result)
}

func TestDecodeCookieParameters_CookieOptionalMissing(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: false
          schema:
            type: number`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	pathItem, _ := m.Model.Paths.PathItems.Get("/burgers/beef")
	params, errs := oasvalidator.ValidateParameterDefinitions(pathItem.Get.Parameters)
	require.Len(t, errs, 0)

	expected := map[string]any{
		"PattyPreference": nil,
	}

	result, errs := DecodeCookieParameters(params, nil)
	require.Len(t, errs, 0)
	require.Equal(t, expected, result)

	input := []*http.Cookie{
		{
			Name: "test",
		},
	}

	result, errs = DecodeCookieParameters(params, input)
	require.Len(t, errs, 0)
	require.Equal(t, expected, result)
}

func TestDecodeCookieParameters_CookieCaseSensitive(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: number`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	pathItem, _ := m.Model.Paths.PathItems.Get("/burgers/beef")
	params, errs := oasvalidator.ValidateParameterDefinitions(pathItem.Get.Parameters)
	require.Len(t, errs, 0)

	inputs := []*http.Cookie{
		{Name: "pattypreference", Value: "1.5"},
	}

	expectedErrs := []httperror.ValidationError{
		{
			Detail:    "Required parameter 'PattyPreference' is missing",
			Location:  oaschema.CookieKey,
			Parameter: "PattyPreference",
			Code:      oasvalidator.ErrCodeRequired,
		},
	}

	_, errs = DecodeCookieParameters(params, inputs)
	require.Equal(t, expectedErrs, errs)
}

func TestDecodeCookieParameters_CookieParamStringPattern(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: SessionID
          in: cookie
          required: true
          schema:
            type: string
            pattern: '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	pathItem, _ := m.Model.Paths.PathItems.Get("/burgers/beef")
	params, errs := oasvalidator.ValidateParameterDefinitions(pathItem.Get.Parameters)
	require.Len(t, errs, 0)

	inputs := []*http.Cookie{
		{Name: "SessionID", Value: "550e8400-e29b-41d4-a716-446655440000"},
	}

	expected := map[string]any{
		"SessionID": "550e8400-e29b-41d4-a716-446655440000",
	}

	result, errs := DecodeCookieParameters(params, inputs)
	require.Len(t, errs, 0)
	require.Equal(t, expected, result)

	invalidInput := []*http.Cookie{
		{Name: "SessionID", Value: "pork"},
	}

	expectedErrs := []httperror.ValidationError{
		{
			Detail:    "The value does not match pattern: ^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$",
			Location:  oaschema.CookieKey,
			Parameter: "SessionID",
			Code:      oasvalidator.ErrCodeInvalidCookie,
		},
	}

	result, errs = DecodeCookieParameters(params, invalidInput)
	require.Len(t, errs, 1)
	require.Equal(t, expectedErrs, errs)
}

func TestDecodeCookieParameters_CookieParamStringValidPatternAndMinMaxLength(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: Code
          in: cookie
          required: true
          schema:
            type: string
            pattern: '^[A-Z]+$'
            minLength: 3
            maxLength: 10`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	pathItem, _ := m.Model.Paths.PathItems.Get("/burgers/beef")
	params, errs := oasvalidator.ValidateParameterDefinitions(pathItem.Get.Parameters)
	require.Len(t, errs, 0)

	inputs := []*http.Cookie{
		{Name: "Code", Value: "ABCDEF"},
	}

	expected := map[string]any{
		"Code": "ABCDEF",
	}

	result, errs := DecodeCookieParameters(params, inputs)
	require.Len(t, errs, 0)
	require.Equal(t, expected, result)

	invalidInput := []*http.Cookie{
		{Name: "Code", Value: "pork"},
	}

	expectedErrs := []httperror.ValidationError{
		{
			Detail:    "The value does not match pattern: ^[A-Z]+$",
			Location:  oaschema.CookieKey,
			Parameter: "Code",
			Code:      oasvalidator.ErrCodeInvalidCookie,
		},
	}

	result, errs = DecodeCookieParameters(params, invalidInput)
	require.Len(t, errs, 1)
	require.Equal(t, expectedErrs, errs)
}
