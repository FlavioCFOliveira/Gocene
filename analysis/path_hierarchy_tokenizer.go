// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// PathHierarchyTokenizer is a tokenizer for path-like strings.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.path.PathHierarchyTokenizer.
//
// This tokenizer splits input text on a specified delimiter (default: '/')
// and produces tokens for each component of the path hierarchy.
//
// For example, with input "/a/b/c" and default delimiter '/':
//   - Forward mode produces: "/a", "/a/b", "/a/b/c"
//   - Reverse mode produces: "/a/b/c", "/b/c", "/c"
//
// The tokenizer supports:
//   - Configurable delimiter character
//   - Configurable replacement character (for the delimiter)
//   - Skip parameter to skip initial components
//   - Reverse mode for bottom-up hierarchy traversal
//
// Example use cases:
//   - File system paths: "/usr/local/bin"
//   - URL paths: "/products/electronics/laptops"
//   - Category hierarchies: "Electronics>Computers>Laptops"
//
// The position increment for each token is 1, and the offset attribute
// reflects the position in the original input text.
type PathHierarchyTokenizer struct {
	*BaseTokenizer

	// delimiter is the character used to split the path (default: '/')
	delimiter byte

	// replacement is the character used in output tokens (default: same as delimiter)
	replacement byte

	// skip is the number of initial components to skip (default: 0)
	skip int

	// reverse indicates whether to generate tokens in reverse order
	reverse bool

	// termAttr holds the CharTermAttribute
	termAttr CharTermAttribute

	// offsetAttr holds the OffsetAttribute
	offsetAttr OffsetAttribute

	// posIncrAttr holds the PositionIncrementAttribute
	posIncrAttr PositionIncrementAttribute

	// inputBuffer stores the entire input
	inputBuffer []byte

	// length is the length of the input buffer
	length int

	// tokenCount is the number of tokens to generate
	tokenCount int

	// currentToken is the index of the current token being generated
	currentToken int

	// delimiterPositions stores the positions of delimiters in the input
	delimiterPositions []int

	// startOffset is the start offset for tokens (0 for forward, varies for reverse)
	startOffset int
}

// PathHierarchyTokenizerOption is a functional option for configuring PathHierarchyTokenizer.
type PathHierarchyTokenizerOption func(*PathHierarchyTokenizer)

// WithDelimiter sets the delimiter character.
func WithDelimiter(delimiter byte) PathHierarchyTokenizerOption {
	return func(t *PathHierarchyTokenizer) {
		t.delimiter = delimiter
	}
}

// WithReplacement sets the replacement character.
func WithReplacement(replacement byte) PathHierarchyTokenizerOption {
	return func(t *PathHierarchyTokenizer) {
		t.replacement = replacement
	}
}

// WithSkip sets the number of initial components to skip.
func WithSkip(skip int) PathHierarchyTokenizerOption {
	return func(t *PathHierarchyTokenizer) {
		t.skip = skip
	}
}

// WithReverse enables reverse mode.
func WithReverse(reverse bool) PathHierarchyTokenizerOption {
	return func(t *PathHierarchyTokenizer) {
		t.reverse = reverse
	}
}

// NewPathHierarchyTokenizer creates a new PathHierarchyTokenizer with the given options.
func NewPathHierarchyTokenizer(options ...PathHierarchyTokenizerOption) *PathHierarchyTokenizer {
	t := &PathHierarchyTokenizer{
		BaseTokenizer: NewBaseTokenizer(),
		delimiter:     '/',
		replacement:   0, // 0 means use delimiter
		skip:          0,
		reverse:       false,
	}

	// Apply options
	for _, option := range options {
		option(t)
	}

	// If replacement not explicitly set, use delimiter
	if t.replacement == 0 {
		t.replacement = t.delimiter
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
func (t *PathHierarchyTokenizer) SetReader(input io.Reader) error {
	if err := t.BaseTokenizer.SetReader(input); err != nil {
		return err
	}

	// Read entire input
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

	t.inputBuffer = buf
	t.length = len(buf)
	t.currentToken = 0

	// Find delimiter positions
	t.delimiterPositions = make([]int, 0)
	for i, b := range buf {
		if b == t.delimiter {
			t.delimiterPositions = append(t.delimiterPositions, i)
		}
	}

	// Calculate token count based on delimiter positions and skip
	t.calculateTokenCount()

	return nil
}

// calculateTokenCount calculates the number of tokens to generate.
func (t *PathHierarchyTokenizer) calculateTokenCount() {
	if t.length == 0 {
		t.tokenCount = 0
		return
	}

	// Count components (delimiters + 1)
	componentCount := len(t.delimiterPositions) + 1

	// If input starts with delimiter, the first component is empty
	// We should skip it in both forward and reverse modes
	if t.length > 0 && t.inputBuffer[0] == t.delimiter {
		componentCount--
	}

	// Adjust for skip
	if t.skip >= componentCount {
		t.tokenCount = 0
		return
	}

	t.tokenCount = componentCount - t.skip
}

// IncrementToken advances to the next token.
func (t *PathHierarchyTokenizer) IncrementToken() (bool, error) {
	if t.currentToken >= t.tokenCount {
		return false, nil
	}

	// Clear attributes for new token
	t.ClearAttributes()

	// Calculate the token boundaries
	startOffset, endOffset := t.calculateTokenBounds()

	// Get the token bytes
	tokenBytes := t.inputBuffer[startOffset:endOffset]

	// Apply replacement if needed
	if t.replacement != t.delimiter {
		tokenBytes = t.applyReplacement(tokenBytes)
	}

	// Set attributes
	t.termAttr.SetValue(string(tokenBytes))
	t.offsetAttr.SetStartOffset(startOffset)
	t.offsetAttr.SetEndOffset(endOffset)
	t.posIncrAttr.SetPositionIncrement(1)

	t.currentToken++
	return true, nil
}

// calculateTokenBounds calculates the start and end offsets for the current token.
func (t *PathHierarchyTokenizer) calculateTokenBounds() (int, int) {
	if t.reverse {
		return t.calculateReverseTokenBounds()
	}
	return t.calculateForwardTokenBounds()
}

// calculateForwardTokenBounds calculates bounds for forward mode.
// In forward mode, tokens are cumulative from the start.
// For "/a/b/c": tokens are "/a", "/a/b", "/a/b/c"
// For "a/b/c": tokens are "a", "a/b", "a/b/c"
func (t *PathHierarchyTokenizer) calculateForwardTokenBounds() (int, int) {
	// Start is always 0 in forward mode
	startOffset := 0

	// Calculate which delimiter marks the end of this token
	// If input starts with delimiter, we need to offset by 1
	delimiterOffset := 0
	if t.length > 0 && t.inputBuffer[0] == t.delimiter {
		delimiterOffset = 1
	}

	// The token ends at the delimiter position after (skip + currentToken + 1) components
	// Token 0 ends after component (skip+1), which is at delimiterPositions[skip + offset]
	// Token 1 ends after component (skip+2), which is at delimiterPositions[skip + 1 + offset]
	// etc.
	targetDelimiterIndex := t.skip + t.currentToken + delimiterOffset

	var endOffset int
	if targetDelimiterIndex < len(t.delimiterPositions) {
		// End at the delimiter position after this component
		endOffset = t.delimiterPositions[targetDelimiterIndex]
	} else {
		// End at the end of input
		endOffset = t.length
	}

	return startOffset, endOffset
}

// calculateReverseTokenBounds calculates bounds for reverse mode.
// In reverse mode, tokens start from different positions and go to the end.
// For "/a/b/c": tokens are "/a/b/c", "/b/c", "/c"
// For "a/b/c": tokens are "a/b/c", "/b/c", "/c"
func (t *PathHierarchyTokenizer) calculateReverseTokenBounds() (int, int) {
	// End is always the end of input in reverse mode
	endOffset := t.length

	// Calculate the start position
	// Token 0 starts at 0 (full path)
	// Token 1 starts at delimiterPositions[0] (at first delimiter) - or delimiterPositions[1] if leading delimiter
	// Token 2 starts at delimiterPositions[1] (at second delimiter) - or delimiterPositions[2] if leading delimiter
	// etc.
	startComponentIndex := t.skip + t.currentToken

	var startOffset int
	if startComponentIndex == 0 {
		startOffset = 0
	} else {
		// If input starts with delimiter, offset the delimiter index
		delimiterOffset := 0
		if t.length > 0 && t.inputBuffer[0] == t.delimiter {
			delimiterOffset = 1
		}
		targetDelimiterIndex := startComponentIndex - 1 + delimiterOffset
		if targetDelimiterIndex < len(t.delimiterPositions) {
			// Start at the delimiter position (include the delimiter in the token)
			startOffset = t.delimiterPositions[targetDelimiterIndex]
		} else {
			startOffset = t.length
		}
	}

	return startOffset, endOffset
}

// applyReplacement replaces delimiters with the replacement character.
func (t *PathHierarchyTokenizer) applyReplacement(tokenBytes []byte) []byte {
	result := make([]byte, len(tokenBytes))
	copy(result, tokenBytes)
	for i := range result {
		if result[i] == t.delimiter {
			result[i] = t.replacement
		}
	}
	return result
}

// Reset resets the tokenizer.
func (t *PathHierarchyTokenizer) Reset() error {
	t.BaseTokenizer.Reset()
	t.currentToken = 0
	return nil
}

// End performs end-of-stream operations.
func (t *PathHierarchyTokenizer) End() error {
	if t.offsetAttr != nil {
		t.offsetAttr.SetEndOffset(t.length)
	}
	return nil
}

// Close releases resources.
func (t *PathHierarchyTokenizer) Close() error {
	t.inputBuffer = nil
	t.delimiterPositions = nil
	return t.BaseTokenizer.Close()
}

// PathHierarchyTokenizerFactory creates PathHierarchyTokenizer instances.
type PathHierarchyTokenizerFactory struct {
	delimiter   byte
	replacement byte
	skip        int
	reverse     bool
}

// PathHierarchyTokenizerFactoryOption is a functional option for the factory.
type PathHierarchyTokenizerFactoryOption func(*PathHierarchyTokenizerFactory)

// WithFactoryDelimiter sets the delimiter for the factory.
func WithFactoryDelimiter(delimiter byte) PathHierarchyTokenizerFactoryOption {
	return func(f *PathHierarchyTokenizerFactory) {
		f.delimiter = delimiter
	}
}

// WithFactoryReplacement sets the replacement for the factory.
func WithFactoryReplacement(replacement byte) PathHierarchyTokenizerFactoryOption {
	return func(f *PathHierarchyTokenizerFactory) {
		f.replacement = replacement
	}
}

// WithFactorySkip sets the skip count for the factory.
func WithFactorySkip(skip int) PathHierarchyTokenizerFactoryOption {
	return func(f *PathHierarchyTokenizerFactory) {
		f.skip = skip
	}
}

// WithFactoryReverse enables reverse mode for the factory.
func WithFactoryReverse(reverse bool) PathHierarchyTokenizerFactoryOption {
	return func(f *PathHierarchyTokenizerFactory) {
		f.reverse = reverse
	}
}

// NewPathHierarchyTokenizerFactory creates a new PathHierarchyTokenizerFactory.
func NewPathHierarchyTokenizerFactory(options ...PathHierarchyTokenizerFactoryOption) *PathHierarchyTokenizerFactory {
	f := &PathHierarchyTokenizerFactory{
		delimiter:   '/',
		replacement: 0,
		skip:        0,
		reverse:     false,
	}

	for _, option := range options {
		option(f)
	}

	return f
}

// Create creates a new PathHierarchyTokenizer.
func (f *PathHierarchyTokenizerFactory) Create() Tokenizer {
	opts := []PathHierarchyTokenizerOption{
		WithDelimiter(f.delimiter),
		WithSkip(f.skip),
		WithReverse(f.reverse),
	}

	if f.replacement != 0 {
		opts = append(opts, WithReplacement(f.replacement))
	}

	return NewPathHierarchyTokenizer(opts...)
}

// Ensure PathHierarchyTokenizer implements Tokenizer
var _ Tokenizer = (*PathHierarchyTokenizer)(nil)

// Ensure PathHierarchyTokenizerFactory implements TokenizerFactory
var _ TokenizerFactory = (*PathHierarchyTokenizerFactory)(nil)
