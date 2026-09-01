// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

import (
	"fmt"
	"strings"
)

// EscapeType indicates the context for escaping.
type EscapeType int

const (
	// EscapeNormal is for escaping reserved words (like AND) in terms.
	// This corresponds to Java's Type.NORMAL.
	EscapeNormal EscapeType = iota
	// EscapeString is for escaping syntax.
	// This corresponds to Java's Type.STRING.
	EscapeString
)

// EscapeQuerySyntax defines the escape contract for query syntax elements.
// This is the Go equivalent of Lucene's EscapeQuerySyntax.
type EscapeQuerySyntax interface {
	// Escape escapes special syntax characters in the given text.
	Escape(text string, locale string, escapeType EscapeType) string
}

// EscapeQuerySyntaxImpl is the standard implementation of EscapeQuerySyntax for the standard lucene syntax.
type EscapeQuerySyntaxImpl struct{}

var wildcardChars = []rune{'*', '?'}

var escapableTermExtraFirstChars = []rune{'+', '-', '@'}

var escapableTermChars = []rune{
	'\"', '<', '>', '=', '!', '(', ')', '^', '[', '{', ':', ']', '}', '~', '/',
}

var escapableQuotedChars = []rune{'\"'}

var escapableWhiteChars = []rune{' ', '\t', '\n', '\r', '\f', '\b', '　'}

var escapableWordTokens = []string{
	"AND", "OR", "NOT", "TO", "WITHIN", "SENTENCE", "PARAGRAPH", "INORDER",
}

// NewEscapeQuerySyntaxImpl creates a new EscapeQuerySyntaxImpl.
func NewEscapeQuerySyntaxImpl() *EscapeQuerySyntaxImpl { return &EscapeQuerySyntaxImpl{} }

// Escape escapes special characters in the query string.
func (e *EscapeQuerySyntaxImpl) Escape(text string, locale string, escapeType EscapeType) string {
	if text == "" {
		return text
	}

	// escape wildcards and the escape char (this has to be performed before anything else)
	// since we need to preserve the UnescapedCharSequence and escape the original escape chars
	ucs := NewUnescapedCharSequence(text)
	text = ucs.ToStringEscapedWithChars(wildcardChars)

	if escapeType == EscapeString {
		return e.escapeQuoted(text, locale)
	}
	return e.escapeTerm(text, locale)
}

func (e *EscapeQuerySyntaxImpl) escapeChar(str string, locale string) string {
	if str == "" {
		return str
	}

	buffer := str

	// regular escapable char for terms
	for _, ec := range escapableTermChars {
		buffer = e.escapeIgnoringCase(buffer, string(ec), "\\", locale)
	}

	// first char of a term as more escaping chars
	runes := []rune(buffer)
	if len(runes) > 0 {
		for _, ec := range escapableTermExtraFirstChars {
			if runes[0] == ec {
				buffer = "\\" + buffer
				break
			}
		}
	}

	return buffer
}

func (e *EscapeQuerySyntaxImpl) escapeQuoted(str string, locale string) string {
	if str == "" {
		return str
	}

	buffer := str
	for _, ec := range escapableQuotedChars {
		buffer = e.escapeIgnoringCase(buffer, string(ec), "\\", locale)
	}
	return buffer
}

func (e *EscapeQuerySyntaxImpl) escapeTerm(term string, locale string) string {
	if term == "" {
		return term
	}

	// escape single chars
	term = e.escapeChar(term, locale)
	term = e.escapeWhiteChar(term, locale)

	// escape parser words
	upperTerm := strings.ToUpper(term)
	for _, token := range escapableWordTokens {
		if token == upperTerm {
			return "\\" + term
		}
	}
	return term
}

func (e *EscapeQuerySyntaxImpl) escapeWhiteChar(str string, locale string) string {
	if str == "" {
		return str
	}

	buffer := str
	for _, ec := range escapableWhiteChars {
		buffer = e.escapeIgnoringCase(buffer, string(ec), "\\", locale)
	}
	return buffer
}

func (e *EscapeQuerySyntaxImpl) escapeIgnoringCase(stringVal string, sequence1 string, escapeChar string, locale string) string {
	if escapeChar == "" || sequence1 == "" || stringVal == "" {
		return stringVal
	}

	count := len(stringVal)
	sequence1Length := len(sequence1)

	if sequence1Length == 0 {
		var result strings.Builder
		result.Grow(count * (1 + len(escapeChar)))
		for i := 0; i < count; i++ {
			result.WriteString(escapeChar)
			result.WriteByte(stringVal[i])
		}
		return result.String()
	}

	lowercase := strings.ToLower(stringVal)
	var result strings.Builder
	first := sequence1[0]
	start := 0
	copyStart := 0

	for start < count {
		firstIndex := strings.IndexByte(lowercase[start:], first)
		if firstIndex == -1 {
			break
		}
		firstIndex += start

		found := true
		if sequence1Length > 1 {
			if firstIndex+sequence1Length > count {
				break
			}
			for i := 1; i < sequence1Length; i++ {
				if lowercase[firstIndex+i] != sequence1[i] {
					found = false
					break
				}
			}
		}

		if found {
			result.WriteString(stringVal[copyStart:firstIndex])
			result.WriteString(escapeChar)
			result.WriteString(stringVal[firstIndex : firstIndex+sequence1Length])
			copyStart = start = firstIndex + sequence1Length
		} else {
			start = firstIndex + 1
		}
	}

	if result.Len() == 0 && copyStart == 0 {
		return stringVal
	}
	result.WriteString(stringVal[copyStart:])
	return result.String()
}

// DiscardEscapeChar returns a string where the escape char has been removed, or kept only once if there was a double escape.
func DiscardEscapeChar(input string) (*UnescapedCharSequence, error) {
	if input == "" {
		return NewUnescapedCharSequence(""), nil
	}

	runes := []rune(input)
	output := make([]rune, len(runes))
	wasEscaped := make([]bool, len(runes))

	length := 0
	lastCharWasEscapeChar := false
	codePointMultiplier := 0
	codePoint := 0

	for i := 0; i < len(runes); i++ {
		curChar := runes[i]
		if codePointMultiplier > 0 {
			val, err := hexToInt(curChar)
			if err != nil {
				return nil, err
			}
			codePoint += val * codePointMultiplier
			codePointMultiplier >>= 4
			if codePointMultiplier == 0 {
				output[length] = rune(codePoint)
				wasEscaped[length] = false
				length++
				codePoint = 0
			}
		} else if lastCharWasEscapeChar {
			if curChar == 'u' {
				codePointMultiplier = 16 * 16 * 16
			} else {
				output[length] = curChar
				wasEscaped[length] = true
				length++
			}
			lastCharWasEscapeChar = false
		} else {
			if curChar == '\\' {
				lastCharWasEscapeChar = true
			} else {
				output[length] = curChar
				wasEscaped[length] = false
				length++
			}
		}
	}

	if codePointMultiplier > 0 {
		return nil, fmt.Errorf("invalid syntax: escape unicode truncation")
	}

	if lastCharWasEscapeChar {
		return nil, fmt.Errorf("invalid syntax: escape character")
	}

	return NewUnescapedCharSequenceFromParts(output, wasEscaped, 0, length), nil
}

func hexToInt(c rune) (int, error) {
	if '0' <= c && c <= '9' {
		return int(c - '0'), nil
	} else if 'a' <= c && c <= 'f' {
		return int(c - 'a' + 10), nil
	} else if 'A' <= c && c <= 'F' {
		return int(c - 'A' + 10), nil
	}
	return 0, fmt.Errorf("invalid hex character: %c", c)
}

// Ensure compile-time interface satisfaction.
var _ EscapeQuerySyntax = (*EscapeQuerySyntaxImpl)(nil)
