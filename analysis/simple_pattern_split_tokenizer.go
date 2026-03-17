// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"errors"
	"io"
	"regexp"
)

// ErrNilPattern is returned when a nil pattern is passed to SimplePatternSplitTokenizer.
var ErrNilPattern = errors.New("pattern cannot be nil")

// SimplePatternSplitTokenizer is a tokenizer that splits text on pattern matches.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.pattern.SimplePatternSplitTokenizer.
//
// This tokenizer uses a regular expression to identify split points in the input text.
// The text between split points is emitted as tokens. The pattern matches themselves
// are NOT emitted as tokens - they act purely as delimiters.
//
// This is a lightweight alternative to PatternTokenizer for simple splitting use cases.
// It reads the entire input into memory and applies the regex to split the text.
//
// Example:
//
//	Pattern: `\s+` (one or more whitespace characters)
//	Input: "Hello World  Test"
//	Output tokens: "Hello", "World", "Test"
//
//	Pattern: `[,.]` (comma or period)
//	Input: "Hello,World.Test"
//	Output tokens: "Hello", "World", "Test"
//
// Note: Empty tokens resulting from consecutive split points are skipped.
type SimplePatternSplitTokenizer struct {
	*BaseTokenizer

	// pattern is the regex pattern used to identify split points
	pattern *regexp.Regexp

	// termAttr holds the CharTermAttribute
	termAttr CharTermAttribute

	// offsetAttr holds the OffsetAttribute
	offsetAttr OffsetAttribute

	// posIncrAttr holds the PositionIncrementAttribute
	posIncrAttr PositionIncrementAttribute

	// inputText holds the entire input text
	inputText string

	// splitIndices contains the start/end indices of tokens after splitting
	splitIndices [][2]int

	// currentIndex tracks the current token index in splitIndices
	currentIndex int
}

// NewSimplePatternSplitTokenizer creates a new SimplePatternSplitTokenizer with the given pattern.
//
// The pattern is used to identify split points in the input text. The text between
// matches is emitted as tokens. The pattern matches themselves are not emitted.
//
// Example patterns:
//
//	`\s+` - Split on one or more whitespace characters
//	`[,.;]` - Split on commas, periods, or semicolons
//	`\W+` - Split on one or more non-word characters
//	`\d+` - Split on one or more digits
//
// Returns an error if the pattern is nil.
func NewSimplePatternSplitTokenizer(pattern *regexp.Regexp) (*SimplePatternSplitTokenizer, error) {
	if pattern == nil {
		return nil, ErrNilPattern
	}

	t := &SimplePatternSplitTokenizer{
		BaseTokenizer: NewBaseTokenizer(),
		pattern:       pattern,
	}

	// Add attributes
	t.termAttr = NewCharTermAttribute()
	t.offsetAttr = NewOffsetAttribute()
	t.posIncrAttr = NewPositionIncrementAttribute()

	t.AddAttribute(t.termAttr)
	t.AddAttribute(t.offsetAttr)
	t.AddAttribute(t.posIncrAttr)

	return t, nil
}

// NewSimplePatternSplitTokenizerWithString creates a new SimplePatternSplitTokenizer
// with a pattern compiled from the given string.
//
// Returns an error if the pattern string is invalid.
func NewSimplePatternSplitTokenizerWithString(patternStr string) (*SimplePatternSplitTokenizer, error) {
	pattern, err := regexp.Compile(patternStr)
	if err != nil {
		return nil, err
	}
	return NewSimplePatternSplitTokenizer(pattern)
}

// SetReader sets the input source for this Tokenizer.
func (t *SimplePatternSplitTokenizer) SetReader(input io.Reader) error {
	t.BaseTokenizer.SetReader(input)

	// Read all input text
	data, err := io.ReadAll(input)
	if err != nil {
		return err
	}
	t.inputText = string(data)

	// Split the text using the pattern
	t.splitIndices = t.computeSplitIndices()
	t.currentIndex = 0

	return nil
}

// computeSplitIndices calculates the start and end indices of tokens after splitting.
// It finds all pattern matches and computes the text between them as tokens.
func (t *SimplePatternSplitTokenizer) computeSplitIndices() [][2]int {
	if t.inputText == "" {
		return nil
	}

	var indices [][2]int
	lastEnd := 0

	// Find all matches
	matches := t.pattern.FindAllStringIndex(t.inputText, -1)

	for _, match := range matches {
		start, end := match[0], match[1]

		// If there's text between lastEnd and start, it's a token
		if start > lastEnd {
			indices = append(indices, [2]int{lastEnd, start})
		}

		lastEnd = end
	}

	// Add the final token if there's text after the last match
	if lastEnd < len(t.inputText) {
		indices = append(indices, [2]int{lastEnd, len(t.inputText)})
	}

	return indices
}

// IncrementToken advances to the next token.
func (t *SimplePatternSplitTokenizer) IncrementToken() (bool, error) {
	// Clear attributes for new token
	t.ClearAttributes()

	// Check if we've exhausted all tokens
	if t.currentIndex >= len(t.splitIndices) {
		return false, nil
	}

	// Get the current token indices
	indices := t.splitIndices[t.currentIndex]
	startOffset := indices[0]
	endOffset := indices[1]

	// Extract the token text
	tokenText := t.inputText[startOffset:endOffset]

	// Set attributes
	t.termAttr.SetValue(tokenText)
	t.offsetAttr.SetStartOffset(startOffset)
	t.offsetAttr.SetEndOffset(endOffset)
	t.posIncrAttr.SetPositionIncrement(1)

	// Move to next token
	t.currentIndex++

	return true, nil
}

// Reset resets the tokenizer to a clean state.
func (t *SimplePatternSplitTokenizer) Reset() error {
	t.BaseTokenizer.Reset()
	t.inputText = ""
	t.splitIndices = nil
	t.currentIndex = 0
	return nil
}

// End performs end-of-stream operations.
func (t *SimplePatternSplitTokenizer) End() error {
	if t.offsetAttr != nil {
		t.offsetAttr.SetEndOffset(len(t.inputText))
	}
	return nil
}

// Close releases resources.
func (t *SimplePatternSplitTokenizer) Close() error {
	t.inputText = ""
	t.splitIndices = nil
	t.currentIndex = 0
	return t.BaseTokenizer.Close()
}

// GetPattern returns the regex pattern used by this tokenizer.
func (t *SimplePatternSplitTokenizer) GetPattern() *regexp.Regexp {
	return t.pattern
}

// Ensure SimplePatternSplitTokenizer implements Tokenizer
var _ Tokenizer = (*SimplePatternSplitTokenizer)(nil)
