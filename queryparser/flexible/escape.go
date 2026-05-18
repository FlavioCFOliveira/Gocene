// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

import "strings"

// EscapeQuerySyntax defines the escape contract for query syntax elements.
// This is the Go equivalent of Lucene's EscapeQuerySyntax.
type EscapeQuerySyntax interface {
	// Escape escapes special syntax characters in the given text.
	// Type indicates what kind of string is being escaped (Term, String).
	Escape(text string, locale string, escapeType EscapeType) string
}

// EscapeType indicates the context for escaping.
type EscapeType int

const (
	// EscapeTerm indicates a term string that may contain wildcards.
	EscapeTerm EscapeType = iota
	// EscapeString indicates a regular string.
	EscapeString
)

// EscapeQuerySyntaxImpl is the standard implementation of EscapeQuerySyntax.
// This is the Go equivalent of Lucene's EscapeQuerySyntaxImpl.
type EscapeQuerySyntaxImpl struct{}

// specialTermChars are the characters that must be escaped in term context.
const specialTermChars = `\+\-\!\(\)\:\^\[\]\"\{\}\~\*\?\|\/\&\@\#`

// Escape escapes special characters in the query string.
// The locale parameter is accepted but ignored (Gocene is English-only).
func (e *EscapeQuerySyntaxImpl) Escape(text string, _ string, escapeType EscapeType) string {
	if text == "" {
		return text
	}
	var sb strings.Builder
	sb.Grow(len(text) * 2)

	for _, ch := range text {
		if e.isSpecial(ch, escapeType) {
			sb.WriteRune('\\')
		}
		sb.WriteRune(ch)
	}
	return sb.String()
}

// isSpecial reports whether ch requires escaping in the given context.
func (e *EscapeQuerySyntaxImpl) isSpecial(ch rune, escapeType EscapeType) bool {
	switch ch {
	case '\\', '+', '-', '!', '(', ')', ':', '^', '[', ']', '"', '{', '}', '~', '|', '/', '&', '@':
		return true
	case '*', '?':
		// Wildcards are only escaped when not in term context
		return escapeType == EscapeString
	default:
		return false
	}
}

// NewEscapeQuerySyntaxImpl creates a new EscapeQuerySyntaxImpl.
func NewEscapeQuerySyntaxImpl() *EscapeQuerySyntaxImpl { return &EscapeQuerySyntaxImpl{} }

// Ensure compile-time interface satisfaction.
var _ EscapeQuerySyntax = (*EscapeQuerySyntaxImpl)(nil)
