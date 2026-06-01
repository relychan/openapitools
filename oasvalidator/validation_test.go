package oasvalidator

import (
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/stretchr/testify/assert"
)

func TestValidateBody_InvalidBasicSchema(t *testing.T) {
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
              properties:
                name:
                  type: string
                patties:
                  type: integer
                vegetarian:
                  type: boolean`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()
	pathItem, _ := m.Model.Paths.PathItems.Get("/burgers/createBurger")
	schema := pathItem.Post.RequestBody.Content.First().Value().Schema.Schema()

	value := map[string]any{
		"name":       "Big Mac",
		"patties":    false,
		"vegetarian": 2,
	}

	errors := ValidateValue(schema, value)

	assert.Len(t, errors, 2)
}
