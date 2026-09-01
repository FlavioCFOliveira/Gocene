// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"strings"
	"unicode"
)

// WhitespaceTokenizer is a tokenizer that splits text on whitespace.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.WhitespaceTokenizer.
type WhitespaceTokenizer struct {
	reader  io.Reader
	buffer  []byte
	pos     int
	current Token
}

func NewWhitespaceTokenizer(reader io.Reader) *WhitespaceTokenizer {
	return &WhitespaceTokenizer{
		reader: reader,
	}
}

func (t *WhitespaceTokenizer) Reset() error {
	t.pos = 0
	t.buffer = nil
	return nil
}

func (t *WhitespaceTokenizer) Next() (Token, bool) {
	if t.buffer == nil {
		data, err := io.ReadAll(t.reader)
		if err != nil {
			return Token{}, false
		}
		t.buffer = data
	}

	for t.pos < len(t.buffer) && unicode.IsSpace(rune(t.buffer[t.pos])) {
		t.pos++
	}
	if t.pos >= len(t.buffer) {
		return Token{}, false
	}

	start := t.pos
	for t.pos < len(t.buffer) && !unicode.IsSpace(rune(t.buffer[t.pos])) {
		t.pos++
	}

	t.current = Token{
		Term:        string(t.buffer[start:t.pos]),
		StartOffset: start,
		EndOffset:   t.pos,
		Position:    0,
		PositionInc: 1,
	}
	return t.current, true
}

func (t *WhitespaceTokenizer) Close() error {
	return nil
}

// LowerCaseFilter is a token filter that converts tokens to lower case.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.LowerCaseFilter.
type LowerCaseFilter struct {
	source TokenStream
}

func NewLowerCaseFilter(source TokenStream) *LowerCaseFilter {
	return &LowerCaseFilter{
		source: source,
	}
}

func (f *LowerCaseFilter) Reset() error {
	return f.source.Reset()
}

func (f *LowerCaseFilter) Next() (Token, bool) {
	token, ok := f.source.Next()
	if !ok {
		return Token{}, false
	}
	token.Term = strings.ToLower(token.Term)
	return token, true
}

func (f *LowerCaseFilter) Close() error {
	return f.source.Close()
}
