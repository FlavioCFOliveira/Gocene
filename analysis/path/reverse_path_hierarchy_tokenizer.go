// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package path provides tokenizers for path-like and domain-like hierarchies.
package path

import (
	"io"
	"strings"
	"unicode/utf8"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

const (
	// DefaultDelimiter is the default path delimiter ('/' rune).
	DefaultDelimiter = '/'
	// DefaultSkip is the default number of trailing components to skip.
	DefaultSkip = 0

	defaultBufferSize = 1024
)

// ReversePathHierarchyTokenizer tokenizes domain-like hierarchies from right to left.
//
// This is the Go port of org.apache.lucene.analysis.path.ReversePathHierarchyTokenizer
// from Apache Lucene 10.4.0.
//
// Given "www.site.co.uk" (delimiter '.') the tokenizer emits:
//
//	www.site.co.uk
//	site.co.uk
//	co.uk
//	uk
//
// The skip parameter drops the rightmost skip components from the output endpoint,
// so the full input is shorter and the number of tokens is reduced accordingly.
type ReversePathHierarchyTokenizer struct {
	*analysis.BaseTokenizer

	delimiter   rune
	replacement rune
	skip        int

	termAttr    analysis.CharTermAttribute
	offsetAttr  analysis.OffsetAttribute
	posIncrAttr analysis.PositionIncrementAttribute

	// State populated on first IncrementToken call.
	resultToken       strings.Builder
	resultTokenBuffer []rune
	delimPositions    []int // rune-index fences at each delimiter position
	delimitersCount   int
	endPosition       int // rune index up to which tokens are emitted
	finalOffset       int // byte length of full input (for End())
	skipped           int // how many tokens have been emitted so far
	byteOff           []int // maps rune index → byte offset in original input
}

// NewReversePathHierarchyTokenizer creates a tokenizer with default settings
// (delimiter='/', skip=0).
func NewReversePathHierarchyTokenizer() *ReversePathHierarchyTokenizer {
	return NewReversePathHierarchyTokenizerFull(DefaultDelimiter, DefaultDelimiter, DefaultSkip)
}

// NewReversePathHierarchyTokenizerSkip creates a tokenizer with a custom skip value.
func NewReversePathHierarchyTokenizerSkip(skip int) *ReversePathHierarchyTokenizer {
	return NewReversePathHierarchyTokenizerFull(DefaultDelimiter, DefaultDelimiter, skip)
}

// NewReversePathHierarchyTokenizerDelim creates a tokenizer with custom delimiter.
func NewReversePathHierarchyTokenizerDelim(delimiter rune) *ReversePathHierarchyTokenizer {
	return NewReversePathHierarchyTokenizerFull(delimiter, delimiter, DefaultSkip)
}

// NewReversePathHierarchyTokenizerFull creates a tokenizer with all parameters.
func NewReversePathHierarchyTokenizerFull(delimiter, replacement rune, skip int) *ReversePathHierarchyTokenizer {
	if skip < 0 {
		panic("skip cannot be negative")
	}
	t := &ReversePathHierarchyTokenizer{
		BaseTokenizer:     analysis.NewBaseTokenizer(),
		delimiter:         delimiter,
		replacement:       replacement,
		skip:              skip,
		resultTokenBuffer: make([]rune, defaultBufferSize),
		delimPositions:    make([]int, 0, defaultBufferSize/10),
		delimitersCount:   -1,
	}
	t.resultToken.Grow(defaultBufferSize)

	t.termAttr = analysis.NewCharTermAttribute()
	t.offsetAttr = analysis.NewOffsetAttribute()
	t.posIncrAttr = analysis.NewPositionIncrementAttribute()

	t.AddAttribute(t.termAttr)
	t.AddAttribute(t.offsetAttr)
	t.AddAttribute(t.posIncrAttr)

	return t
}

// IncrementToken advances to the next token, returning true when a token was produced.
func (t *ReversePathHierarchyTokenizer) IncrementToken() (bool, error) {
	t.ClearAttributes()

	// First call: read the entire input, build delimiter position table.
	if t.delimitersCount == -1 {
		if err := t.readInput(); err != nil {
			return false, err
		}
	}

	t.posIncrAttr.SetPositionIncrement(1)

	// Java: while (skipped < delimitersCount - skip - 1)
	for t.skipped < t.delimitersCount-t.skip-1 {
		start := t.delimPositions[t.skipped]
		// Term: runes [start, endPosition).
		t.termAttr.SetValue(string(t.resultTokenBuffer[start:t.endPosition]))
		startByte := t.runeIdxToByte(start)
		endByte := t.runeIdxToByte(t.endPosition)
		t.offsetAttr.SetStartOffset(startByte)
		t.offsetAttr.SetEndOffset(endByte)
		t.skipped++
		return true, nil
	}

	return false, nil
}

// readInput reads the full reader content, populates resultTokenBuffer and delimPositions.
func (t *ReversePathHierarchyTokenizer) readInput() error {
	reader := t.GetReader()
	if reader == nil {
		t.delimitersCount = 0
		return nil
	}

	raw, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	// byte length for finalOffset / End().
	t.finalOffset = len(raw)

	// Decode runes, building resultToken (with replacement) and delimPositions.
	t.delimPositions = t.delimPositions[:0]
	t.delimPositions = append(t.delimPositions, 0) // virtual fence at rune-position 0
	t.resultToken.Reset()

	length := 0 // rune count
	for len(raw) > 0 {
		r, size := utf8.DecodeRune(raw)
		raw = raw[size:]
		length++
		if r == t.delimiter {
			t.delimPositions = append(t.delimPositions, length)
			t.resultToken.WriteRune(t.replacement)
		} else {
			t.resultToken.WriteRune(r)
		}
	}

	t.delimitersCount = len(t.delimPositions)
	// If the last rune was NOT a delimiter, there is no trailing fence yet;
	// add one so that [delimPositions[last-1], delimPositions[last]) covers the final segment.
	if t.delimitersCount == 0 || t.delimPositions[t.delimitersCount-1] < length {
		t.delimPositions = append(t.delimPositions, length)
		t.delimitersCount++
	}

	// Materialise rune buffer.
	runes := []rune(t.resultToken.String())
	if len(t.resultTokenBuffer) < len(runes) {
		t.resultTokenBuffer = make([]rune, len(runes))
	}
	copy(t.resultTokenBuffer, runes)
	t.resultToken.Reset()

	// endPosition: Java: int idx = delimitersCount - 1 - skip; endPosition = delimPositions[idx]
	idx := t.delimitersCount - 1 - t.skip
	if idx >= 0 {
		t.endPosition = t.delimPositions[idx]
	}

	// Build rune-index → byte-offset lookup.
	t.byteOff = make([]int, len(runes)+1)
	bytePos := 0
	for i, r := range runes {
		t.byteOff[i] = bytePos
		bytePos += utf8.RuneLen(r)
	}
	t.byteOff[len(runes)] = bytePos

	return nil
}

func (t *ReversePathHierarchyTokenizer) runeIdxToByte(runeIdx int) int {
	if runeIdx < 0 || t.byteOff == nil || runeIdx >= len(t.byteOff) {
		return 0
	}
	return t.byteOff[runeIdx]
}

// End is called after all tokens have been emitted.
func (t *ReversePathHierarchyTokenizer) End() error {
	if err := t.BaseTokenizer.End(); err != nil {
		return err
	}
	t.offsetAttr.SetStartOffset(t.finalOffset)
	t.offsetAttr.SetEndOffset(t.finalOffset)
	return nil
}

// Reset resets the tokenizer state so it can be reused with a new reader.
func (t *ReversePathHierarchyTokenizer) Reset() error {
	if err := t.BaseTokenizer.Reset(); err != nil {
		return err
	}
	t.resultToken.Reset()
	t.finalOffset = 0
	t.endPosition = 0
	t.skipped = 0
	t.delimitersCount = -1
	t.delimPositions = t.delimPositions[:0]
	t.byteOff = nil
	return nil
}

// Ensure interface compliance.
var _ analysis.Tokenizer = (*ReversePathHierarchyTokenizer)(nil)
