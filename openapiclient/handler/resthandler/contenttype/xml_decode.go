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

package contenttype

import (
	"encoding/xml"
	"io"

	"github.com/relychan/goutils"
	"github.com/relychan/openapitools/oaschema"
)

// DecodeXML decodes an arbitrary XML from a reader stream.
func DecodeXML(r io.Reader) (any, error) {
	decoder := xml.NewDecoder(r)

	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, &goutils.ErrorDetail{
				Detail: err.Error(),
				Code:   oaschema.ErrCodeMalformedXML,
			}
		}

		if token == nil {
			break
		}

		if se, ok := token.(xml.StartElement); ok {
			xmlTree := newXMLBlock(se)

			err := evalXMLTree(decoder, xmlTree)
			if err != nil {
				return nil, &goutils.ErrorDetail{
					Detail: err.Error(),
					Code:   oaschema.ErrCodeMalformedXML,
				}
			}

			result := decodeArbitraryXMLBlock(xmlTree)

			return result, nil
		}
	}

	return nil, nil
}

func decodeArbitraryXMLBlock(block *xmlBlock) any {
	if len(block.Start.Attr) == 0 && len(block.Fields) == 0 {
		return block.Data
	}

	result := make(map[string]any)

	if len(block.Start.Attr) > 0 {
		attributes := make(map[string]string)
		for _, attr := range block.Start.Attr {
			attributes[attr.Name.Local] = attr.Value
		}

		result["attributes"] = attributes
	}

	if len(block.Fields) == 0 {
		result["content"] = block.Data

		return result
	}

	for key, field := range block.Fields {
		switch len(field) {
		case 0:
		case 1:
			// limitation: we can't know if the array is wrapped
			result[key] = decodeArbitraryXMLBlock(&field[0])
		default:
			items := make([]any, len(field))
			for i, f := range field {
				items[i] = decodeArbitraryXMLBlock(&f)
			}

			result[key] = items
		}
	}

	return result
}

type xmlBlock struct {
	Start  xml.StartElement
	Data   string
	Fields map[string][]xmlBlock
}

func newXMLBlock(start xml.StartElement) *xmlBlock {
	return &xmlBlock{
		Start:  start,
		Fields: map[string][]xmlBlock{},
	}
}

func evalXMLTree(decoder *xml.Decoder, block *xmlBlock) error {
L:
	for {
		nextToken, err := decoder.Token()
		if err != nil {
			return err
		}

		if nextToken == nil {
			return nil
		}

		switch tok := nextToken.(type) {
		case xml.StartElement:
			childBlock := newXMLBlock(tok)

			err := evalXMLTree(decoder, childBlock)
			if err != nil {
				return err
			}

			block.Fields[tok.Name.Local] = append(block.Fields[tok.Name.Local], *childBlock)
		case xml.CharData:
			block.Data = string(tok)
		case xml.EndElement:
			break L
		default:
		}
	}

	return nil
}
