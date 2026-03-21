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
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateArbitraryXMLForm(t *testing.T) {
	testCases := []struct {
		Name string
		Body map[string]any

		Expected string
	}{
		{
			Name: "putPetXml",
			Body: map[string]any{
				"id":   "10",
				"name": "doggie",
				"category": map[string]any{
					"id":   "1",
					"name": "Dogs",
				},
				"photoUrls": "string",
				"tags": map[string]any{
					"id":   "0",
					"name": "string",
				},
				"status": "available",
			},
			Expected: "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<xml><category><id>1</id><name>Dogs</name></category><id>10</id><name>doggie</name><photoUrls>string</photoUrls><status>available</status><tags><id>0</id><name>string</name></tags></xml>",
		},
		{
			Name: "putCommentXml",
			Body: map[string]any{
				"user":          "Iggy",
				"comment_count": "6",
				"comment": []any{
					map[string]any{
						"who":       "Iggy",
						"when":      "2021-10-15 13:28:22 UTC",
						"id":        "1",
						"bsrequest": "115",
					},
					map[string]any{
						"who":     "Iggy",
						"when":    "2021-10-15 13:49:39 UTC",
						"id":      "2",
						"project": "home:Admin",
					},
					map[string]any{
						"who":     "Iggy",
						"when":    "2021-10-15 13:54:38 UTC",
						"id":      "3",
						"project": "home:Admin",
						"package": "0ad",
					},
				},
			},
			Expected: `<?xml version="1.0" encoding="UTF-8"?>
<xml><comment><bsrequest>115</bsrequest><id>1</id><when>2021-10-15 13:28:22 UTC</when><who>Iggy</who></comment><comment><id>2</id><project>home:Admin</project><when>2021-10-15 13:49:39 UTC</when><who>Iggy</who></comment><comment><id>3</id><package>0ad</package><project>home:Admin</project><when>2021-10-15 13:54:38 UTC</when><who>Iggy</who></comment><comment_count>6</comment_count><user>Iggy</user></xml>`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			result, err := EncodeXML(tc.Body)
			assert.NoError(t, err)

			parsedResult, err := DecodeXML(bytes.NewBuffer(result))
			assert.NoError(t, err)

			assert.Equal(t, tc.Body, parsedResult)
		})
	}
}

// cpu: Apple M3 Pro
// BenchmarkEncodeXML-11    	  488484	      2456 ns/op	    5439 B/op	      32 allocs/op
func BenchmarkEncodeXML(b *testing.B) {
	input := map[string]any{
		"id":   "10",
		"name": "doggie",
		"category": map[string]any{
			"id":   "1",
			"name": "Dogs",
		},
		"photoUrls": "string",
		"tags": map[string]any{
			"id":   "0",
			"name": "string",
		},
		"status": "available",
	}

	for b.Loop() {
		EncodeXML(input)
	}
}
