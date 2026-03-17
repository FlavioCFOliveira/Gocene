// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"bufio"
	"io"
)

// NGramTokenizer generates n-grams from input text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.ngram.NGramTokenizer.
//
// N-grams are overlapping subsequences of length n from the input text.
// For example, for input "hello" with minGram=2 and maxGram=3:
//   - 2-grams: "he", "el", "ll", "lo"
//   - 3-grams: "hel", "ell", "llo"
//
// The tokenizer emits all n-grams in order of increasing start position,
// and for each position, in order of increasing gram size.
//
// Example:
//
//	Input: "abc", minGram=2, maxGram=3
//	Output tokens: "ab", "abc", "bc"
//
// The tokenizer handles Unicode characters correctly by operating on runes.
// It preserves token positions and offsets for highlighting support.
type NGramTokenizer struct {
	*BaseTokenizer

	// minGram is the minimum n-gram size
	minGram int

	// maxGram is the maximum n-gram size
	maxGram int

	// scanner reads from the input
	scanner *bufio.Scanner

	// buffer holds the runes read from input
	buffer []rune

	// bufferSize is the number of runes in the buffer
	bufferSize int

	// currentPos is the current starting position for n-gram generation
	currentPos int

	// currentGramSize is the current gram size being generated
	currentGramSize int

	// termAttr holds the CharTermAttribute
	termAttr CharTermAttribute

	// offsetAttr holds the OffsetAttribute
	offsetAttr OffsetAttribute

	// posIncrAttr holds the PositionIncrementAttribute
	posIncrAttr PositionIncrementAttribute

	// firstToken tracks if this is the first token for position increment
	firstToken bool
}

// NewNGramTokenizer creates a new NGramTokenizer with the specified gram sizes.
//
// Parameters:
//   - minGram: the minimum n-gram size (must be >= 1)
//   - maxGram: the maximum n-gram size (must be >= minGram)
//
// Returns nil if parameters are invalid.
func NewNGramTokenizer(minGram, maxGram int) *NGramTokenizer {
	if minGram < 1 || maxGram < minGram {
		return nil
	}

	t := &NGramTokenizer{
		BaseTokenizer: NewBaseTokenizer(),
		minGram:       minGram,
		maxGram:       maxGram,
		firstToken:    true,
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
func (t *NGramTokenizer) SetReader(input io.Reader) error {
	t.BaseTokenizer.SetReader(input)
	t.scanner = bufio.NewScanner(input)
	t.scanner.Split(bufio.ScanRunes)
	t.buffer = nil
	t.bufferSize = 0
	t.currentPos = 0
	t.currentGramSize = t.minGram
	t.firstToken = true
	return nil
}

// IncrementToken advances to the next token.
func (t *NGramTokenizer) IncrementToken() (bool, error) {
	if t.input == nil || t.scanner == nil {
		return false, nil
	}

	// Clear attributes for new token
	t.ClearAttributes()

	// Read all input into buffer on first call
	if t.buffer == nil {
		err := t.readAllInput()
		if err != nil {
			return false, err
		}
	}

	// Generate n-grams
	for t.currentPos < t.bufferSize {
		// Check if we can emit an n-gram of currentGramSize from currentPos
		if t.currentPos+t.currentGramSize <= t.bufferSize {
			// Emit n-gram
			t.emitNGram()

			// Move to next gram size or position
			t.currentGramSize++
			if t.currentGramSize > t.maxGram {
				t.currentPos++
				t.currentGramSize = t.minGram
			}

			return true, nil
		}

		// Current gram size too large for remaining characters,
		// try next position with minGram
		t.currentPos++
		t.currentGramSize = t.minGram
	}

	return false, nil
}

// readAllInput reads all runes from the input into the buffer.
func (t *NGramTokenizer) readAllInput() error {
	t.buffer = make([]rune, 0, 256)

	for t.scanner.Scan() {
		r := []rune(t.scanner.Text())[0]
		t.buffer = append(t.buffer, r)
	}

	t.bufferSize = len(t.buffer)
	return t.scanner.Err()
}

// emitNGram emits the current n-gram.
func (t *NGramTokenizer) emitNGram() {
	// Extract n-gram from buffer
	endPos := t.currentPos + t.currentGramSize
	ngram := string(t.buffer[t.currentPos:endPos])

	// Set term attribute
	t.termAttr.SetValue(ngram)

	// Set offset attributes (in characters)
	t.offsetAttr.SetStartOffset(t.currentPos)
	t.offsetAttr.SetEndOffset(endPos)

	// Set position increment
	if t.firstToken {
		t.posIncrAttr.SetPositionIncrement(1)
		t.firstToken = false
	} else {
		t.posIncrAttr.SetPositionIncrement(1)
	}
}

// Reset resets the tokenizer.
func (t *NGramTokenizer) Reset() error {
	t.BaseTokenizer.Reset()
	t.buffer = nil
	t.bufferSize = 0
	t.currentPos = 0
	t.currentGramSize = t.minGram
	t.firstToken = true
	return nil
}

// End performs end-of-stream operations.
func (t *NGramTokenizer) End() error {
	if t.offsetAttr != nil {
		t.offsetAttr.SetEndOffset(t.bufferSize)
	}
	return nil
}

// GetMinGram returns the minimum n-gram size.
func (t *NGramTokenizer) GetMinGram() int {
	return t.minGram
}

// GetMaxGram returns the maximum n-gram size.
func (t *NGramTokenizer) GetMaxGram() int {
	return t.maxGram
}

// Ensure NGramTokenizer implements Tokenizer
var _ Tokenizer = (*NGramTokenizer)(nil)
