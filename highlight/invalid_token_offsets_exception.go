// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package highlight

// InvalidTokenOffsetsException is returned by the highlighter when a
// token's offset values disagree with the source text (e.g. end < start, or
// offsets beyond the text bounds). Mirrors
// org.apache.lucene.search.highlight.InvalidTokenOffsetsException.
type InvalidTokenOffsetsException struct {
	Message string
}

func (e *InvalidTokenOffsetsException) Error() string { return e.Message }

// NewInvalidTokenOffsetsException builds the error.
func NewInvalidTokenOffsetsException(message string) *InvalidTokenOffsetsException {
	return &InvalidTokenOffsetsException{Message: message}
}
