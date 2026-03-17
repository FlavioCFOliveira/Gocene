// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"bufio"
	"io"
	"regexp"
)

// SimplePatternTokenizer is a tokenizer that uses a regular expression to
// find token boundaries or extract tokens.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.pattern.SimplePatternTokenizer.
//
// SimplePatternTokenizer uses a regular expression to find matches in the input
// text. Each match becomes a token. This is a simpler, more lightweight version
// of PatternTokenizer with fewer configuration options.
//
// The tokenizer is useful for:
// - Extracting tokens based on a simple pattern
// - Tokenizing structured text (e.g., log files, CSV data)
// - Custom tokenization where word boundaries follow a predictable pattern
//
// Example:
//
//	pattern := `\w+`  // Match word characters
//	Input: "Hello, World! 123"
//	Output tokens: "Hello", "World", "123"
//
// Note: The pattern is used to find matches, not to split the text. The entire
// match becomes the token.
type SimplePatternTokenizer struct {
	*BaseTokenizer

	// pattern is the compiled regular expression
	pattern *regexp.Regexp

	// scanner reads from the input
	scanner *bufio.Scanner

	// termAttr holds the CharTermAttribute
	termAttr CharTermAttribute

	// offsetAttr holds the OffsetAttribute
	offsetAttr OffsetAttribute

	// posIncrAttr holds the PositionIncrementAttribute
	posIncrAttr PositionIncrementAttribute

	// currentOffset tracks the current position in input
	currentOffset int

	// inputBuffer holds the entire input for pattern matching
	inputBuffer string

	// matches holds the current set of pattern matches
	matches [][]int

	// matchIndex tracks the current match being processed
	matchIndex int
}

// NewSimplePatternTokenizer creates a new SimplePatternTokenizer with the given pattern.
//
// The pattern should be a valid regular expression that defines what constitutes
// a token. Each match of the pattern in the input text becomes a token.
//
// Example patterns:
//   - `\w+` - Match sequences of word characters
//   - `[a-zA-Z]+` - Match sequences of letters only
//   - `\d+` - Match sequences of digits
//   - `[\w@.]+` - Match email-like tokens
//
// Returns an error if the pattern is invalid.
func NewSimplePatternTokenizer(pattern string) (*SimplePatternTokenizer, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	t := &SimplePatternTokenizer{
		BaseTokenizer: NewBaseTokenizer(),
		pattern:       re,
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

// NewSimplePatternTokenizerWithRegexp creates a new SimplePatternTokenizer with a pre-compiled regexp.
//
// This is useful when you want to reuse a compiled regular expression or need
// to set specific regexp flags.
func NewSimplePatternTokenizerWithRegexp(re *regexp.Regexp) *SimplePatternTokenizer {
	t := &SimplePatternTokenizer{
		BaseTokenizer: NewBaseTokenizer(),
		pattern:       re,
	}

	// Add attributes
	t.termAttr = NewCharTermAttribute()
	t.offsetAttr = NewOffsetAttribute()
	t.posIncrAttr = NewPositionIncrementAttribute()

	t.AddAttribute(t.termAttr)
	t.AddAttribute(t.offsetAttr)
	t.AddAttribute(t.posIncrAttr)

	return t
}

// SetReader sets the input source for this Tokenizer.
func (t *SimplePatternTokenizer) SetReader(input io.Reader) error {
	t.BaseTokenizer.SetReader(input)
	t.currentOffset = 0
	t.matchIndex = 0
	t.matches = nil

	// Read entire input into buffer
	buf := make([]byte, 0, 1024)
	temp := make([]byte, 1024)

	for {
		n, err := input.Read(temp)
		if n > 0 {
			buf = append(buf, temp[:n]...)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	t.inputBuffer = string(buf)

	// Find all matches
	t.matches = t.pattern.FindAllStringIndex(t.inputBuffer, -1)

	return nil
}

// IncrementToken advances to the next token.
func (t *SimplePatternTokenizer) IncrementToken() (bool, error) {
	if t.matches == nil || t.matchIndex >= len(t.matches) {
		return false, nil
	}

	// Clear attributes for new token
	t.ClearAttributes()

	// Get current match
	match := t.matches[t.matchIndex]
	start := match[0]
	end := match[1]

	// Set token text
	tokenText := t.inputBuffer[start:end]
	t.termAttr.SetValue(tokenText)

	// Set offsets
	t.offsetAttr.SetStartOffset(start)
	t.offsetAttr.SetEndOffset(end)

	// Set position increment
	t.posIncrAttr.SetPositionIncrement(1)

	// Move to next match
	t.matchIndex++

	return true, nil
}

// Reset resets the tokenizer.
func (t *SimplePatternTokenizer) Reset() error {
	t.BaseTokenizer.Reset()
	t.currentOffset = 0
	t.matchIndex = 0
	// Note: We don't clear matches here because SetReader will be called
	// with the new input before tokenization begins
	return nil
}

// End performs end-of-stream operations.
func (t *SimplePatternTokenizer) End() error {
	if t.offsetAttr != nil {
		t.offsetAttr.SetEndOffset(len(t.inputBuffer))
	}
	return nil
}

// GetPattern returns the regular expression pattern used by this tokenizer.
func (t *SimplePatternTokenizer) GetPattern() *regexp.Regexp {
	return t.pattern
}

// Ensure SimplePatternTokenizer implements Tokenizer
var _ Tokenizer = (*SimplePatternTokenizer)(nil)

// SimplePatternTokenizerFactory creates SimplePatternTokenizer instances.
type SimplePatternTokenizerFactory struct {
	// pattern is the regular expression pattern string
	pattern string

	// compiledPattern is the compiled regexp (cached)
	compiledPattern *regexp.Regexp
}

// NewSimplePatternTokenizerFactory creates a new SimplePatternTokenizerFactory.
//
// The pattern should be a valid regular expression. Returns an error if the
// pattern is invalid.
func NewSimplePatternTokenizerFactory(pattern string) (*SimplePatternTokenizerFactory, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	return &SimplePatternTokenizerFactory{
		pattern:         pattern,
		compiledPattern: re,
	}, nil
}

// Create creates a new SimplePatternTokenizer.
func (f *SimplePatternTokenizerFactory) Create() Tokenizer {
	return NewSimplePatternTokenizerWithRegexp(f.compiledPattern)
}

// GetPattern returns the pattern string used by this factory.
func (f *SimplePatternTokenizerFactory) GetPattern() string {
	return f.pattern
}

// Ensure SimplePatternTokenizerFactory implements TokenizerFactory
var _ TokenizerFactory = (*SimplePatternTokenizerFactory)(nil)
