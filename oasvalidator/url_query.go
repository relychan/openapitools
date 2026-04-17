package oasvalidator

import (
	"net/url"
	"strings"

	"github.com/hasura/goenvconf"
	"github.com/relychan/goutils"
)

// EncodeQueryEscape encodes the values into “URL encoded” form ("bar=baz&foo=quux") sorted by key with escape.
func EncodeQueryEscape(value string, allowReserved bool) string { //nolint:revive,nolintlint
	if allowReserved {
		return QueryEscapeAllowReserved(value)
	}

	return url.QueryEscape(value)
}

// EncodeQueryValuesUnescape encode query values into “URL encoded” form ("bar=baz&foo=quux") sorted by key without escape.
func EncodeQueryValuesUnescape(values url.Values) string {
	if len(values) == 0 {
		return ""
	}

	var buf strings.Builder

	buf.Grow(len(values) * 4)

	keys := goutils.GetSortedKeys(values)

	for _, key := range keys {
		vs := values[key]

		for _, v := range vs {
			if buf.Len() > 0 {
				buf.WriteByte('&')
			}

			buf.WriteString(key)
			buf.WriteByte('=')
			buf.WriteString(v)
		}
	}

	return buf.String()
}

// IsUnreservedCharacter checks if the character is allowed in a URI but do not has a reserved purpose are called unreserved.
//
//	unreserved  = ALPHA / DIGIT / "-" / "." / "_" / "~"
func IsUnreservedCharacter[C byte | rune](c C) bool {
	return goutils.IsMetaCharacter(c) || c == '.' || c == '~'
}

// IsReservedCharacter checks if the character is allowed in a URI and has a reserved purpose.
//
//	reserved    = gen-delims / sub-delims
//	gen-delims  = ":" / "/" / "?" / "#" / "[" / "]" / "@"
//	sub-delims  = "!" / "$" / "&" / "'" / "(" / ")" / "*" / "+" / "," / ";" / "="
func IsReservedCharacter[C byte | rune](c C) bool {
	switch c {
	// gen-delims
	case ':', '/', '?', '#', '[', ']', '@',
		// sub-delims
		'!', '$', '&', '\'', '(', ')', '*', '+', ',', ';', '=', '%':
		return true
	default:
		return false
	}
}

// ReplaceURLTemplate finds and replace variables in the template string.
func ReplaceURLTemplate(input string, get goenvconf.GetEnvFunc) (string, error) {
	if input == "" {
		return "", nil
	}

	var sb strings.Builder

	var inBracket bool

	var i int

	strLength := len(input)
	sb.Grow(strLength)

	for ; i < strLength; i++ {
		char := input[i]
		if char != '{' {
			sb.WriteByte(char)

			continue
		}

		i++

		inBracket = true

		if i == strLength-1 {
			return "", errUnclosedTemplateString
		}

		j := i
		// get and validate environment variable
		for ; j < strLength; j++ {
			nextChar := input[j]
			if nextChar == '}' {
				inBracket = false

				break
			}
		}

		if inBracket {
			return "", errUnclosedTemplateString
		}

		value, err := get(input[i:j])
		if err != nil {
			return "", err
		}

		sb.WriteString(value)

		i = j
	}

	if inBracket {
		return "", errUnclosedTemplateString
	}

	return sb.String(), nil
}

// QueryEscapeAllowReserved escapes the string so it can be safely placed inside a URL query.
// Allow reserved character.
func QueryEscapeAllowReserved(query string) string {
	if strings.ContainsFunc(query, func(r rune) bool {
		return !IsReservedCharacter(r) && !IsUnreservedCharacter(r)
	}) {
		return url.QueryEscape(query)
	}

	return query
}
