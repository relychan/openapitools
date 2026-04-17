package oasvalidator

import "errors"

var (
	// ErrInvalidContentType occurs when the content type string is invalid.
	ErrInvalidContentType     = errors.New("invalid content type")
	errUnclosedTemplateString = errors.New("expected a closed curly bracket")
)
