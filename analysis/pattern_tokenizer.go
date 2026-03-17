// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"bufio"
	"io"
	"regexp"
)

// PatternTokenizer is a tokenizer that uses a regex pattern to find tokens.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.pattern.PatternTokenizer.
//
// The tokenizer works in two modes:
// 1. Split mode (default): The pattern is used as a delimiter to split the input.
//    Tokens are the text between pattern matches.
// 2. Match mode: The pattern is used to find tokens. Only text that matches
//    the pattern is emitted as tokens.
//
// In match mode, capturing groups can be used to extract specific parts of
// the match. The group parameter specifies which capturing group to use (0 = entire match).
//
// Example (split mode with pattern "\\s+"):
//
//	Input: "hello world test"
//	Output tokens: "hello", "world", "test"
//
// Example (match mode with pattern "\\b\\w+\\b"):
//
//	Input: "hello, world! test."
//	Output tokens: "hello", "world", "test"
//
// Example (match mode with pattern "\\b(\\w+)\\b" and group=1):
//
//	Input: "hello world"
//	Output tokens: "hello", "world" (same as group=0 for this pattern)
type PatternTokenizer struct {
	*BaseTokenizer

	// pattern is the compiled regex pattern
	pattern *regexp.Regexp

	// group is the capturing group to extract (0 = entire match)
	group int

	// matchMode determines whether to extract matches (true) or split on pattern (false)
	matchMode bool

	// termAttr holds the CharTermAttribute
	termAttr CharTermAttribute

	// offsetAttr holds the OffsetAttribute
	offsetAttr OffsetAttribute

	// posIncrAttr holds the PositionIncrementAttribute
	posIncrAttr PositionIncrementAttribute

	// typeAttr holds the TypeAttribute
	typeAttr *TypeAttribute

	// inputStr holds the entire input as a string
	inputStr string

	// matches holds the regex matches when in match mode
	matches [][]int

	// currentMatchIndex tracks the current match being processed
	currentMatchIndex int

	// splitStart tracks the start of the next split token
	splitStart int

	// splitEnd tracks the end position for split mode
	splitEnd int
}

// NewPatternTokenizer creates a new PatternTokenizer in split mode.
//
// The pattern is used as a delimiter - tokens are the text between matches.
// This is the default behavior matching Lucene's PatternTokenizer.
//
// Example:
//
//	pattern := regexp.MustCompile(`\s+`)
//	tokenizer := NewPatternTokenizer(pattern)
func NewPatternTokenizer(pattern *regexp.Regexp) *PatternTokenizer {
	return NewPatternTokenizerWithGroup(pattern, -1)
}

// NewPatternTokenizerWithGroup creates a new PatternTokenizer in match mode
// with a specific capturing group.
//
// When group is -1, the tokenizer works in split mode (pattern is delimiter).
// When group is 0 or greater, the tokenizer works in match mode and extracts
// the specified capturing group from each match.
//
// Example (extract entire matches):
//
//	pattern := regexp.MustCompile(`\b\w+\b`)
//	tokenizer := NewPatternTokenizerWithGroup(pattern, 0)
//
// Example (extract first capturing group):
//
//	pattern := regexp.MustCompile(`<(\w+)>`)
//	tokenizer := NewPatternTokenizerWithGroup(pattern, 1)
//	// Input: "<tag1> <tag2>"
//	// Output: "tag1", "tag2"
func NewPatternTokenizerWithGroup(pattern *regexp.Regexp, group int) *PatternTokenizer {
	t := &PatternTokenizer{
		BaseTokenizer: NewBaseTokenizer(),
		pattern:       pattern,
		group:         group,
		matchMode:     group >= 0,
	}

	// Add attributes
	t.termAttr = NewCharTermAttribute()
	t.offsetAttr = NewOffsetAttribute()
	t.posIncrAttr = NewPositionIncrementAttribute()
	t.typeAttr = NewTypeAttribute()

	t.AddAttribute(t.termAttr)
	t.AddAttribute(t.offsetAttr)
	t.AddAttribute(t.posIncrAttr)
	t.AddAttribute(t.typeAttr)

	return t
}

// SetReader sets the input source for this Tokenizer.
func (t *PatternTokenizer) SetReader(input io.Reader) error {
	t.BaseTokenizer.SetReader(input)

	// Read entire input into memory
	// PatternTokenizer needs the full input to perform regex operations
	buf := make([]byte, 0, 1024)
	scanner := bufio.NewScanner(input)
	scanner.Split(bufio.ScanBytes)

	for scanner.Scan() {
		buf = append(buf, scanner.Bytes()...)
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	t.inputStr = string(buf)
	t.currentMatchIndex = 0
	t.splitStart = 0
	t.splitEnd = len(t.inputStr)

	if t.matchMode {
		// Find all matches for match mode
		t.matches = t.pattern.FindAllStringIndex(t.inputStr, -1)
	} else {
		// Find all matches for split mode (these are the delimiters)
		t.matches = t.pattern.FindAllStringIndex(t.inputStr, -1)
	}

	return nil
}

// IncrementToken advances to the next token.
func (t *PatternTokenizer) IncrementToken() (bool, error) {
	if t.input == nil {
		return false, nil
	}

	// Clear attributes for new token
	t.ClearAttributes()

	if t.matchMode {
		return t.incrementTokenMatchMode()
	}
	return t.incrementTokenSplitMode()
}

// incrementTokenMatchMode extracts pattern matches as tokens.
func (t *PatternTokenizer) incrementTokenMatchMode() (bool, error) {
	if t.currentMatchIndex >= len(t.matches) {
		return false, nil
	}

	match := t.matches[t.currentMatchIndex]
	t.currentMatchIndex++

	// Get the matched substring
	matchedText := t.inputStr[match[0]:match[1]]

	// Extract the specified group if needed
	var tokenText string
	var startOffset, endOffset int

	if t.group == 0 {
		// Use entire match
		tokenText = matchedText
		startOffset = match[0]
		endOffset = match[1]
	} else {
		// Find the specific capturing group
		// We need to re-run FindStringSubmatch to get group positions
		submatches := t.pattern.FindStringSubmatchIndex(t.inputStr[match[0]:])
		if submatches == nil || len(submatches) < (t.group+1)*2 {
			// Group doesn't exist in this match, skip it
			return t.incrementTokenMatchMode()
		}

		groupStart := match[0] + submatches[t.group*2]
		groupEnd := match[0] + submatches[t.group*2+1]

		if groupStart == groupEnd {
			// Empty group, skip this match
			return t.incrementTokenMatchMode()
		}

		tokenText = t.inputStr[groupStart:groupEnd]
		startOffset = groupStart
		endOffset = groupEnd
	}

	// Set token attributes
	t.termAttr.SetValue(tokenText)
	t.offsetAttr.SetStartOffset(startOffset)
	t.offsetAttr.SetEndOffset(endOffset)
	t.posIncrAttr.SetPositionIncrement(1)
	t.typeAttr.SetType("word")

	return true, nil
}

// incrementTokenSplitMode splits on pattern (pattern is delimiter).
func (t *PatternTokenizer) incrementTokenSplitMode() (bool, error) {
	// Skip empty tokens at the beginning
	for t.currentMatchIndex < len(t.matches) {
		match := t.matches[t.currentMatchIndex]

		// Check if there's text between splitStart and the delimiter
		if t.splitStart < match[0] {
			// Emit token from splitStart to delimiter start
			tokenText := t.inputStr[t.splitStart:match[0]]
			startOffset := t.splitStart
			endOffset := match[0]

			// Update splitStart for next token
			t.splitStart = match[1]
			t.currentMatchIndex++

			// Set token attributes
			t.termAttr.SetValue(tokenText)
			t.offsetAttr.SetStartOffset(startOffset)
			t.offsetAttr.SetEndOffset(endOffset)
			t.posIncrAttr.SetPositionIncrement(1)
			t.typeAttr.SetType("word")

			return true, nil
		}

		// No text before this delimiter, skip it
		t.splitStart = match[1]
		t.currentMatchIndex++
	}

	// Check for remaining text after last delimiter
	if t.splitStart < t.splitEnd {
		tokenText := t.inputStr[t.splitStart:t.splitEnd]
		startOffset := t.splitStart
		endOffset := t.splitEnd

		// Mark as done
		t.splitStart = t.splitEnd

		// Set token attributes
		t.termAttr.SetValue(tokenText)
		t.offsetAttr.SetStartOffset(startOffset)
		t.offsetAttr.SetEndOffset(endOffset)
		t.posIncrAttr.SetPositionIncrement(1)
		t.typeAttr.SetType("word")

		return true, nil
	}

	return false, nil
}

// Reset resets the tokenizer.
func (t *PatternTokenizer) Reset() error {
	t.BaseTokenizer.Reset()
	t.currentMatchIndex = 0
	t.splitStart = 0
	if t.inputStr != "" {
		t.splitEnd = len(t.inputStr)
	}
	return nil
}

// End performs end-of-stream operations.
func (t *PatternTokenizer) End() error {
	if t.offsetAttr != nil {
		t.offsetAttr.SetEndOffset(len(t.inputStr))
	}
	return nil
}

// Close releases resources.
func (t *PatternTokenizer) Close() error {
	t.inputStr = ""
	t.matches = nil
	t.currentMatchIndex = 0
	t.splitStart = 0
	t.splitEnd = 0
	return t.BaseTokenizer.Close()
}

// GetPattern returns the regex pattern used by this tokenizer.
func (t *PatternTokenizer) GetPattern() *regexp.Regexp {
	return t.pattern
}

// GetGroup returns the capturing group number (0 = entire match, -1 = split mode).
func (t *PatternTokenizer) GetGroup() int {
	if t.matchMode {
		return t.group
	}
	return -1
}

// IsMatchMode returns true if the tokenizer is in match mode.
func (t *PatternTokenizer) IsMatchMode() bool {
	return t.matchMode
}

// Ensure PatternTokenizer implements Tokenizer
var _ Tokenizer = (*PatternTokenizer)(nil)

// PatternTokenizerFactory creates PatternTokenizer instances.
type PatternTokenizerFactory struct {
	// pattern is the compiled regex pattern
	pattern *regexp.Regexp

	// group is the capturing group to extract
	group int
}

// NewPatternTokenizerFactory creates a new PatternTokenizerFactory for split mode.
//
// The pattern string is compiled and used as a delimiter.
// Returns nil if the pattern is invalid.
func NewPatternTokenizerFactory(pattern string) *PatternTokenizerFactory {
	return NewPatternTokenizerFactoryWithGroup(pattern, -1)
}

// NewPatternTokenizerFactoryWithGroup creates a new PatternTokenizerFactory.
//
// When group is -1, the tokenizer works in split mode.
// When group is 0 or greater, the tokenizer works in match mode.
// Returns nil if the pattern is invalid.
func NewPatternTokenizerFactoryWithGroup(pattern string, group int) *PatternTokenizerFactory {
	// Validate the pattern compiles
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}

	return &PatternTokenizerFactory{
		pattern: re,
		group:   group,
	}
}

// Create creates a new PatternTokenizer.
func (f *PatternTokenizerFactory) Create() Tokenizer {
	return NewPatternTokenizerWithGroup(f.pattern, f.group)
}

// Ensure PatternTokenizerFactory implements TokenizerFactory
var _ TokenizerFactory = (*PatternTokenizerFactory)(nil)
