// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"errors"
	"io"
	"unicode/utf8"
)

// EdgeNGramTokenizer generates edge n-grams (prefixes) from the input text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.ngram.EdgeNGramTokenizer.
//
// Edge n-grams are n-grams anchored to the beginning of the input. For example,
// with minGram=2 and maxGram=4, the input "hello" would produce:
//   - "he" (2 characters)
//   - "hel" (3 characters)
//   - "hell" (4 characters)
//
// The tokenizer reads the entire input as a single token source, then generates
// edge n-grams of varying lengths. This is useful for:
//   - Prefix matching and autocomplete functionality
//   - Edge n-gram analysis for search suggestions
//   - Building prefix-based indexes
//
// Example:
//
//	Input: "hello"
//	minGram: 2, maxGram: 4
//	Output tokens: "he", "hel", "hell"
//
// Unicode characters are handled correctly - the tokenizer operates on
// Unicode code points (runes), not bytes.
type EdgeNGramTokenizer struct {
	*BaseTokenizer

	// minGram is the minimum n-gram size
	minGram int

	// maxGram is the maximum n-gram size
	maxGram int

	// inputBuffer holds the entire input as runes
	inputBuffer []rune

	// inputLength is the length of input in runes
	inputLength int

	// currentGramSize is the current n-gram size being emitted
	currentGramSize int

	// termAttr holds the CharTermAttribute
	termAttr CharTermAttribute

	// offsetAttr holds the OffsetAttribute
	offsetAttr OffsetAttribute

	// posIncrAttr holds the PositionIncrementAttribute
	posIncrAttr PositionIncrementAttribute
}

// NewEdgeNGramTokenizer creates a new EdgeNGramTokenizer with the specified
// minimum and maximum n-gram sizes.
//
// Parameters:
//   - minGram: the minimum n-gram size (must be >= 1)
//   - maxGram: the maximum n-gram size (must be >= minGram)
//
// Returns an error if minGram or maxGram are invalid.
func NewEdgeNGramTokenizer(minGram, maxGram int) (*EdgeNGramTokenizer, error) {
	if minGram < 1 {
		return nil, errors.New("minGram must be >= 1")
	}
	if maxGram < minGram {
		return nil, errors.New("maxGram must be >= minGram")
	}

	t := &EdgeNGramTokenizer{
		BaseTokenizer: NewBaseTokenizer(),
		minGram:       minGram,
		maxGram:       maxGram,
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

// SetReader sets the input source for this Tokenizer.
func (t *EdgeNGramTokenizer) SetReader(input io.Reader) error {
	t.BaseTokenizer.SetReader(input)

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

	// Convert to runes for proper Unicode handling
	t.inputBuffer = []rune(string(buf))
	t.inputLength = len(t.inputBuffer)
	t.currentGramSize = t.minGram

	return nil
}

// IncrementToken advances to the next token.
// Returns true if a token is available, false if at end of stream.
func (t *EdgeNGramTokenizer) IncrementToken() (bool, error) {
	if t.input == nil {
		return false, nil
	}

	// Check if we've emitted all n-grams
	if t.currentGramSize > t.maxGram || t.currentGramSize > t.inputLength {
		return false, nil
	}

	// Clear attributes for new token
	t.ClearAttributes()

	// Generate the n-gram
	endPos := t.currentGramSize
	if endPos > t.inputLength {
		endPos = t.inputLength
	}

	// Get the n-gram as a string
	ngram := string(t.inputBuffer[:endPos])

	// Set attributes
	t.termAttr.SetValue(ngram)
	t.offsetAttr.SetStartOffset(0)

	// Calculate end offset in bytes for proper highlighting
	endOffset := t.runeOffsetToByteOffset(endPos)
	t.offsetAttr.SetEndOffset(endOffset)

	// First token has position increment 1, subsequent tokens have 0
	// since they are derived from the same input position
	if t.currentGramSize == t.minGram {
		t.posIncrAttr.SetPositionIncrement(1)
	} else {
		t.posIncrAttr.SetPositionIncrement(0)
	}

	// Move to next gram size
	t.currentGramSize++

	return true, nil
}

// runeOffsetToByteOffset converts a rune offset to a byte offset.
func (t *EdgeNGramTokenizer) runeOffsetToByteOffset(runeOffset int) int {
	if runeOffset <= 0 {
		return 0
	}
	if runeOffset >= t.inputLength {
		// Calculate total byte length
		byteLen := 0
		for _, r := range t.inputBuffer {
			byteLen += utf8.RuneLen(r)
		}
		return byteLen
	}

	byteOffset := 0
	for i := 0; i < runeOffset && i < t.inputLength; i++ {
		byteOffset += utf8.RuneLen(t.inputBuffer[i])
	}
	return byteOffset
}

// Reset resets the tokenizer to a clean state.
func (t *EdgeNGramTokenizer) Reset() error {
	t.BaseTokenizer.Reset()
	t.currentGramSize = t.minGram
	return nil
}

// End performs end-of-stream operations.
func (t *EdgeNGramTokenizer) End() error {
	if t.offsetAttr != nil {
		// Set end offset to the byte length of the entire input
		endOffset := t.runeOffsetToByteOffset(t.inputLength)
		t.offsetAttr.SetEndOffset(endOffset)
	}
	return nil
}

// GetMinGram returns the minimum n-gram size.
func (t *EdgeNGramTokenizer) GetMinGram() int {
	return t.minGram
}

// GetMaxGram returns the maximum n-gram size.
func (t *EdgeNGramTokenizer) GetMaxGram() int {
	return t.maxGram
}

// Ensure EdgeNGramTokenizer implements Tokenizer
var _ Tokenizer = (*EdgeNGramTokenizer)(nil)

// EdgeNGramTokenizerFactory creates EdgeNGramTokenizer instances.
type EdgeNGramTokenizerFactory struct {
	minGram int
	maxGram int
}

// NewEdgeNGramTokenizerFactory creates a new EdgeNGramTokenizerFactory.
//
// Parameters:
//   - minGram: the minimum n-gram size (must be >= 1)
//   - maxGram: the maximum n-gram size (must be >= minGram)
func NewEdgeNGramTokenizerFactory(minGram, maxGram int) (*EdgeNGramTokenizerFactory, error) {
	if minGram < 1 {
		return nil, errors.New("minGram must be >= 1")
	}
	if maxGram < minGram {
		return nil, errors.New("maxGram must be >= minGram")
	}

	return &EdgeNGramTokenizerFactory{
		minGram: minGram,
		maxGram: maxGram,
	}, nil
}

// Create creates a new EdgeNGramTokenizer.
func (f *EdgeNGramTokenizerFactory) Create() Tokenizer {
	tokenizer, err := NewEdgeNGramTokenizer(f.minGram, f.maxGram)
	if err != nil {
		// This should not happen since parameters were validated in factory constructor
		panic(err)
	}
	return tokenizer
}

// Ensure EdgeNGramTokenizerFactory implements TokenizerFactory
var _ TokenizerFactory = (*EdgeNGramTokenizerFactory)(nil)
