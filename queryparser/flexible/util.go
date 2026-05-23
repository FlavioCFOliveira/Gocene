// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// StringUtils provides string conversion helpers used throughout the flexible
// query parser. This is the Go equivalent of Lucene's StringUtils.
type StringUtils struct{}

// ToString converts an arbitrary value to its string representation.
// nil returns an empty string.
func (StringUtils) ToString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	if st, ok := v.(interface{ String() string }); ok {
		return st.String()
	}
	return ""
}

// ToLowerCase converts a rune to lower-case.
func (StringUtils) ToLowerCase(r rune) rune { return unicode.ToLower(r) }

// UnescapedCharSequence is a string wrapper that tracks which characters were
// originally preceded by a backslash escape in the query source.
// This is the Go equivalent of Lucene's UnescapedCharSequence.
type UnescapedCharSequence struct {
	chars      []rune
	wasEscaped []bool
}

// NewUnescapedCharSequence creates an UnescapedCharSequence by scanning s for
// backslash-escaped characters. The escape character itself is consumed.
func NewUnescapedCharSequence(s string) *UnescapedCharSequence {
	runes := []rune(s)
	chars := make([]rune, 0, len(runes))
	wasEscaped := make([]bool, 0, len(runes))

	for i := 0; i < len(runes); i++ {
		if runes[i] == '\\' && i+1 < len(runes) {
			i++
			chars = append(chars, runes[i])
			wasEscaped = append(wasEscaped, true)
		} else {
			chars = append(chars, runes[i])
			wasEscaped = append(wasEscaped, false)
		}
	}
	return &UnescapedCharSequence{chars: chars, wasEscaped: wasEscaped}
}

// Length returns the number of (unescaped) characters.
func (u *UnescapedCharSequence) Length() int { return len(u.chars) }

// CharAt returns the character at index i.
func (u *UnescapedCharSequence) CharAt(i int) rune { return u.chars[i] }

// WasEscaped reports whether the character at index i was backslash-escaped.
func (u *UnescapedCharSequence) WasEscaped(i int) bool { return u.wasEscaped[i] }

// String reconstructs the unescaped string (without backslashes).
func (u *UnescapedCharSequence) String() string { return string(u.chars) }

// ToStringEscaped reconstructs the string, re-inserting backslashes for
// characters that were originally escaped.
func (u *UnescapedCharSequence) ToStringEscaped() string {
	var sb strings.Builder
	sb.Grow(len(u.chars) + len(u.wasEscaped))
	for i, ch := range u.chars {
		if u.wasEscaped[i] {
			sb.WriteRune('\\')
		}
		sb.WriteRune(ch)
	}
	return sb.String()
}

// SubSequence returns a new UnescapedCharSequence for the half-open range [start, end).
func (u *UnescapedCharSequence) SubSequence(start, end int) *UnescapedCharSequence {
	return &UnescapedCharSequence{
		chars:      u.chars[start:end],
		wasEscaped: u.wasEscaped[start:end],
	}
}

// QueryNodeOperation provides utility operations on QueryNode trees.
// This is the Go equivalent of Lucene's QueryNodeOperation.
type QueryNodeOperation struct{}

// UnionQueryNodesContents merges the children of two boolean query nodes into
// a new AndQueryNode. Nil nodes are skipped.
func (QueryNodeOperation) UnionQueryNodesContents(node1, node2 QueryNode) QueryNode {
	var children []QueryNode
	if node1 != nil {
		children = append(children, node1.GetChildren()...)
	}
	if node2 != nil {
		children = append(children, node2.GetChildren()...)
	}
	return NewAndQueryNode(children)
}

// QueryParserHelper orchestrates the full parse → process → build pipeline.
// Callers compose a SyntaxParser, a processor pipeline, and a QueryTreeBuilder,
// then call Parse. This is the Go equivalent of Lucene's QueryParserHelper.
type QueryParserHelper struct {
	config     *QueryConfigHandler
	syntax     syntaxParser
	processors []QueryNodeProcessor
	builder    *QueryTreeBuilder
}

// syntaxParser is a local interface for the parsing step so we can accept
// either *StandardSyntaxParser or any future parser.
type syntaxParser interface {
	Parse(query string) (QueryNode, error)
}

// NewQueryParserHelper creates a QueryParserHelper with the given components.
func NewQueryParserHelper(
	config *QueryConfigHandler,
	syntax syntaxParser,
	processors []QueryNodeProcessor,
	builder *QueryTreeBuilder,
) *QueryParserHelper {
	return &QueryParserHelper{
		config:     config,
		syntax:     syntax,
		processors: processors,
		builder:    builder,
	}
}

// Parse runs the full pipeline: parse → process → build.
func (h *QueryParserHelper) Parse(queryString, defaultField string) (interface{}, error) {
	if defaultField != "" && h.config != nil {
		h.config.Set(NewConfigurationKey("defaultField"), defaultField)
	}

	tree, err := h.syntax.Parse(queryString)
	if err != nil {
		return nil, err
	}

	for _, p := range h.processors {
		tree, err = p.Process(tree)
		if err != nil {
			return nil, err
		}
	}

	if h.builder == nil || tree == nil {
		return tree, nil
	}
	return h.builder.Build(tree)
}

// GetQueryConfigHandler returns the configuration handler.
func (h *QueryParserHelper) GetQueryConfigHandler() *QueryConfigHandler { return h.config }

// isLetter is used internally for unescaped-char utilities.
func isLetter(r rune) bool { return unicode.IsLetter(r) }

// isValidUTF8 reports whether s contains only valid UTF-8.
func isValidUTF8(s string) bool { return utf8.ValidString(s) }
