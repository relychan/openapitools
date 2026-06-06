package oasvalidator

import (
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/relychan/goutils/httperror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateValue_BasicSchema(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      requestBody:
        required: false
        content:
          "*/json":
            schema:
              type: object
              required: [name, patties, vegetarian]
              properties:
                name:
                  type: string
                patties:
                  type: integer
                vegetarian:
                  type: boolean`

	doc, err := libopenapi.NewDocument([]byte(spec))
	assert.NoError(t, err)

	m, err := doc.BuildV3Model()
	assert.NoError(t, err)

	pathItem, _ := m.Model.Paths.PathItems.Get("/burgers/createBurger")
	schema := pathItem.Post.RequestBody.Content.First().Value().Schema

	value := map[string]any{
		"name":       "Big Mac",
		"patties":    false,
		"vegetarian": 2,
	}

	errors := ValidateValueWithSchemaProxy(schema, value)
	expectedErrors := []httperror.ValidationError{
		{
			Detail:  "Invalid type or syntax. Expected the type of value to be one of [integer], however the request provided 'boolean' type",
			Pointer: "/patties",
			Code:    "validation_error",
		}, {
			Detail:  "Invalid type or syntax. Expected the type of value to be one of [boolean], however the request provided 'integer' type",
			Pointer: "/vegetarian",
			Code:    "validation_error",
		},
	}
	require.Equal(t, expectedErrors, errors)

	validValue := map[string]interface{}{
		"name":       "Big Mac",
		"patties":    2,
		"vegetarian": true,
	}
	errors = ValidateValueWithSchemaProxy(schema, validValue)
	require.Len(t, errors, 0)
}

func TestValidateValue_AllOf(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schema_validation/TestBody'
components:
  schema_validation:
    Nutrients:
      type: object
      required: [fat, salt, meat]
      properties:
        fat:
          type: number
        salt:
          type: number
        meat:
          type: string
          enum:
            - beef
            - pork
            - lamb
            - vegetables
    TestBody:
      type: object
      allOf:
        - $ref: '#/components/schema_validation/Nutrients'
      properties:
        name:
          type: string
        patties:
          type: integer
        vegetarian:
          type: boolean
      required: [name, patties, vegetarian]    `

	doc, err := libopenapi.NewDocument([]byte(spec))
	assert.NoError(t, err)

	m, err := doc.BuildV3Model()
	assert.NoError(t, err)

	pathItem, _ := m.Model.Paths.PathItems.Get("/burgers/createBurger")
	schema := pathItem.Post.RequestBody.Content.First().Value().Schema

	value := map[string]any{
		"name":       "Big Mac",
		"patties":    2,
		"vegetarian": true,
		"fat":        10.0,
		"salt":       0.5,
		"meat":       "beef",
	}

	errors := ValidateValueWithSchemaProxy(schema, value)
	require.Len(t, errors, 0)

	invalidValue := map[string]interface{}{
		"name":       "Big Mac",
		"patties":    2,
		"vegetarian": true,
		"fat":        10.0,
		"salt":       false,    // invalid
		"meat":       "turkey", // invalid
	}

	errors = ValidateValueWithSchemaProxy(schema, invalidValue)
	expectedErrors := []httperror.ValidationError{
		{
			Detail:  "Invalid type or syntax. Expected the type of value to be one of [number], however the request provided 'boolean' type",
			Pointer: "/salt",
			Code:    "validation_error",
			Hint:    "/allOf/0",
		},
		{
			Detail:  "Value 'turkey' does not match any enum values: [beef, pork, lamb, vegetables]",
			Pointer: "/meat",
			Code:    "validation_error",
			Hint:    "/allOf/0",
		},
	}
	require.Equal(t, expectedErrors, errors)
}

func TestValidateValue_AllOfAnyOf(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schema_validation/TestBody'
components:
  schema_validation:
    Uncooked:
      type: object
      required: [uncookedWeight, uncookedHeight]
      properties:
        uncookedWeight:
          type: number
        uncookedHeight:
          type: number
    Cooked:
      type: object
      required: [usedOil, usedAnimalFat]
      properties:
        usedOil:
          type: boolean
        usedAnimalFat:
          type: boolean
    Nutrients:
      type: object
      required: [fat, salt, meat]
      properties:
        fat:
          type: number
        salt:
          type: number
        meat:
          type: string
          enum:
            - beef
            - pork
            - lamb
            - vegetables
    TestBody:
      type: object
      oneOf:
        - $ref: '#/components/schema_validation/Uncooked'
        - $ref: '#/components/schema_validation/Cooked'
      allOf:
        - $ref: '#/components/schema_validation/Nutrients'
      properties:
        name:
          type: string
        patties:
          type: integer
        vegetarian:
          type: boolean
      required: [name, patties, vegetarian]    `

	doc, err := libopenapi.NewDocument([]byte(spec))
	assert.NoError(t, err)

	m, err := doc.BuildV3Model()
	assert.NoError(t, err)

	pathItem, _ := m.Model.Paths.PathItems.Get("/burgers/createBurger")
	schema := pathItem.Post.RequestBody.Content.First().Value().Schema

	value := map[string]interface{}{
		"name":          "Big Mac",
		"patties":       2,
		"vegetarian":    true,
		"fat":           10.0,
		"salt":          0.5,
		"meat":          "beef",
		"usedOil":       true,
		"usedAnimalFat": false,
	}

	errors := ValidateValueWithSchemaProxy(schema, value)
	require.Len(t, errors, 0)
}

func TestValidateValue_OneOf(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schema_validation/TestBody'
components:
  schema_validation:
    Uncooked:
      type: object
      required: [uncookedWeight, uncookedHeight]
      properties:
        uncookedWeight:
          type: number
        uncookedHeight:
          type: number
    Cooked:
      type: object
      required: [usedOil, usedAnimalFat]
      properties:
        usedOil:
          type: boolean
        usedAnimalFat:
          type: boolean
    Nutrients:
      type: object
      required: [fat, salt, meat]
      properties:
        fat:
          type: number
        salt:
          type: number
        meat:
          type: string
          enum:
            - beef
            - pork
            - lamb
            - vegetables
    TestBody:
      type: object
      oneOf:
        - $ref: '#/components/schema_validation/Uncooked'
        - $ref: '#/components/schema_validation/Cooked'
      allOf:
        - $ref: '#/components/schema_validation/Nutrients'
      properties:
        name:
          type: string
        patties:
          type: integer
        vegetarian:
          type: boolean
      required: [name, patties, vegetarian]    `

	doc, err := libopenapi.NewDocument([]byte(spec))
	assert.NoError(t, err)

	m, err := doc.BuildV3Model()
	assert.NoError(t, err)

	pathItem, _ := m.Model.Paths.PathItems.Get("/burgers/createBurger")
	schema := pathItem.Post.RequestBody.Content.First().Value().Schema

	value := map[string]interface{}{
		"name":       "Big Mac",
		"patties":    2,
		"vegetarian": true,
		"fat":        10.0,
		"salt":       0.5,
		"meat":       "beef",
	}

	errors := ValidateValueWithSchemaProxy(schema, value)

	expectedErrors := []httperror.ValidationError{
		{
			Detail:  "Required property 'uncookedWeight' is missing in the object",
			Pointer: "/uncookedWeight",
			Code:    "validation_error",
			Hint:    "/oneOf/0",
		},
		{
			Detail:  "Required property 'uncookedHeight' is missing in the object",
			Pointer: "/uncookedHeight",
			Code:    "validation_error",
			Hint:    "/oneOf/0",
		},
		{
			Detail:  "Required property 'usedOil' is missing in the object",
			Pointer: "/usedOil",
			Code:    "validation_error",
			Hint:    "/oneOf/1",
		},
		{
			Detail:  "Required property 'usedAnimalFat' is missing in the object",
			Pointer: "/usedAnimalFat",
			Code:    "validation_error",
			Hint:    "/oneOf/1",
		},
	}
	require.Equal(t, expectedErrors, errors)
}

func TestValidateValue_InvalidMinMax(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schema_validation/TestBody'
components:
  schema_validation:
    TestBody:
      type: object
      properties:
        name:
          type: string
        patties:
          type: integer
          maximum: 3
          minimum: 1
        vegetarian:
          type: boolean
      required: [name, patties, vegetarian]    `

	doc, err := libopenapi.NewDocument([]byte(spec))
	assert.NoError(t, err)

	m, err := doc.BuildV3Model()
	assert.NoError(t, err)

	pathItem, _ := m.Model.Paths.PathItems.Get("/burgers/createBurger")
	schema := pathItem.Post.RequestBody.Content.First().Value().Schema

	value := map[string]interface{}{
		"name":       "Big Mac",
		"patties":    5,
		"vegetarian": true,
		"fat":        10.0,
		"salt":       0.5,
		"meat":       "beef",
	}

	errors := ValidateValueWithSchemaProxy(schema, value)

	expectedErrors := []httperror.ValidationError{
		{
			Detail:  "Number value must be less than or equal 3, but got: 5",
			Pointer: "/patties",
			Code:    "validation_error",
		},
	}
	require.Equal(t, expectedErrors, errors)
}

func TestValidateValue_InvalidMaxItems(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schema_validation/TestBody'
components:
  schema_validation:
    TestBody:
      type: array
      maxItems: 2
      items:
        type: object
        properties:
          name:
            type: string
          patties:
            type: integer
            maximum: 3
            minimum: 1
          vegetarian:
            type: boolean
        required: [name, patties, vegetarian]    `

	doc, err := libopenapi.NewDocument([]byte(spec))
	assert.NoError(t, err)

	m, err := doc.BuildV3Model()
	assert.NoError(t, err)

	pathItem, _ := m.Model.Paths.PathItems.Get("/burgers/createBurger")
	schema := pathItem.Post.RequestBody.Content.First().Value().Schema

	value := map[string]interface{}{
		"name":       "Big Mac",
		"patties":    2,
		"vegetarian": true,
		"fat":        10.0,
		"salt":       0.5,
		"meat":       "beef",
	}
	valueArray := []interface{}{value, value, value, value} // two too many!

	errors := ValidateValueWithSchemaProxy(schema, valueArray)

	expectedErrors := []httperror.ValidationError{
		{
			Detail: "Array must have a maximum items length of 2, but got 4 items",
			Code:   "validation_error",
		},
	}
	require.Equal(t, expectedErrors, errors)
}

func TestValidateValue_InvalidEmail(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schema_validation/V1_UserRequest'
components:
  schema_validation:
    V1_UserRequest:
            type: object
            properties:
                email:
                    type: string
                    format: email
                    minLength: 1
                    maxLength: 320`

	doc, err := libopenapi.NewDocument([]byte(spec))
	assert.NoError(t, err)

	m, err := doc.BuildV3Model()
	assert.NoError(t, err)

	pathItem, _ := m.Model.Paths.PathItems.Get("/burgers/createBurger")
	schema := pathItem.Post.RequestBody.Content.First().Value().Schema

	value := map[string]interface{}{
		"email": "test",
	}
	errors := ValidateValueWithSchemaProxy(schema, value)

	expectedErrors := []httperror.ValidationError{
		{
			Detail:  "Invalid email; @ character must exist",
			Pointer: "/email",
		},
	}
	require.Equal(t, expectedErrors, errors)
}
