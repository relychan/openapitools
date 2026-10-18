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

package oasvalidator

import (
	"testing"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateOperation_BinaryResponses(t *testing.T) {
	binarySchema := base.CreateSchemaProxy(&base.Schema{
		Type:   []string{"string"},
		Format: "binary",
	})

	for _, contentType := range []string{"image/png", "application/pdf", "application/octet-stream", "*/*"} {
		t.Run(contentType, func(t *testing.T) {
			content := orderedmap.New[string, *highv3.MediaType]()
			content.Set(contentType, &highv3.MediaType{Schema: binarySchema})

			codes := orderedmap.New[string, *highv3.Response]()
			codes.Set("200", &highv3.Response{Content: content})

			result, errs := ValidateOperation(nil, &highv3.Operation{
				OperationId: "download",
				Responses:   &highv3.Responses{Codes: codes},
			}, nil)
			require.Empty(t, errs)
			require.NotNil(t, result)

			media, ok := result.Responses[contentType]
			require.True(t, ok)
			assert.Equal(t, contentType, media.ContentType)
			require.NotNil(t, media.Schema)
			assert.Equal(t, "binary", media.Schema.Format)
		})
	}
}

func TestValidateOperation_InvalidResponseContentType(t *testing.T) {
	content := orderedmap.New[string, *highv3.MediaType]()
	content.Set("not/a/valid/type", &highv3.MediaType{})

	codes := orderedmap.New[string, *highv3.Response]()
	codes.Set("200", &highv3.Response{Content: content})

	_, errs := ValidateOperation(nil, &highv3.Operation{
		Responses: &highv3.Responses{Codes: codes},
	}, nil)
	require.Len(t, errs, 1)
	assert.Equal(t, "invalid content type not/a/valid/type", errs[0].Detail)
}

func TestValidateOperation_BinaryRequestBodyStillRejected(t *testing.T) {
	content := orderedmap.New[string, *highv3.MediaType]()
	content.Set("image/png", &highv3.MediaType{})

	_, errs := ValidateOperation(nil, &highv3.Operation{
		RequestBody: &highv3.RequestBody{Content: content},
	}, nil)
	require.NotEmpty(t, errs)
}
