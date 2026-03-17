// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
)

// ShingleFilter combines multiple tokens into shingles (word n-grams).
//
// This is the Go port of Lucene's org.apache.lucene.analysis.shingle.ShingleFilter.
//
// A shingle is a token that combines multiple adjacent tokens. For example,
// with input tokens ["please", "divide", "this", "sentence"], a shingle filter
// with maxShingleSize=2 produces:
//   ["please", "please divide", "divide", "divide this", "this", "this sentence", "sentence"]
//
// Shingles are useful for phrase-like matching without requiring expensive
// phrase queries. They can help with:
//   - Handling multi-word concepts as single tokens
//   - Improving recall for phrase queries
//   - Creating token n-grams for fuzzy matching
//
// The filter supports:
//   - Configurable min/max shingle size
//   - Token separator between shingled tokens
//   - Outputting unigrams (original tokens) alongside shingles
//   - Preserving position increments for proper phrase matching
//
// Example usage:
//
//	tokenizer := NewWhitespaceTokenizer()
//	tokenizer.SetReader(strings.NewReader("hello world test"))
//	filter := NewShingleFilter(tokenizer)
//	filter.SetMaxShingleSize(2)
//	// Produces: "hello", "hello world", "world", "world test", "test"
type ShingleFilter struct {
	*BaseTokenFilter

	// termAttr holds the CharTermAttribute from the shared attribute source
	termAttr CharTermAttribute

	// posIncrAttr holds the PositionIncrementAttribute from the shared attribute source
	posIncrAttr PositionIncrementAttribute

	// offsetAttr holds the OffsetAttribute from the shared attribute source
	offsetAttr OffsetAttribute

	// minShingleSize is the minimum number of tokens in a shingle (default: 2)
	minShingleSize int

	// maxShingleSize is the maximum number of tokens in a shingle (default: 2)
	maxShingleSize int

	// tokenSeparator is the string inserted between tokens in a shingle (default: " ")
	tokenSeparator string

	// outputUnigrams determines if original tokens should be output (default: true)
	outputUnigrams bool

	// tokenBuffer stores tokens for shingle generation
	tokenBuffer []*tokenData

	// numTokens is the number of tokens currently in the buffer
	numTokens int

	// nextTokenToEmit is the index of the next token position to emit shingles from
	nextTokenToEmit int

	// currentShingleSize is the current shingle size being emitted (1 = unigram)
	currentShingleSize int

	// inputExhausted is true when all input tokens have been consumed
	inputExhausted bool

	// isFirstToken is true for the first token in the stream
	isFirstToken bool
}

// tokenData holds the data for a single token in the buffer.
type tokenData struct {
	term              []byte
	startOffset       int
	endOffset         int
	positionIncrement int
}

// NewShingleFilter creates a new ShingleFilter with default settings.
// Default: minShingleSize=2, maxShingleSize=2, tokenSeparator=" ", outputUnigrams=true
func NewShingleFilter(input TokenStream) *ShingleFilter {
	filter := &ShingleFilter{
		BaseTokenFilter:    NewBaseTokenFilter(input),
		minShingleSize:     2,
		maxShingleSize:     2,
		tokenSeparator:     " ",
		outputUnigrams:     true,
		isFirstToken:       true,
		tokenBuffer:        make([]*tokenData, 0, 16),
		nextTokenToEmit:    0,
		currentShingleSize: 1,
		numTokens:          0,
	}

	// Get attributes from the shared AttributeSource
	filter.initAttributes()

	return filter
}

// NewShingleFilterWithSizes creates a new ShingleFilter with custom min/max sizes.
func NewShingleFilterWithSizes(input TokenStream, minShingleSize, maxShingleSize int) *ShingleFilter {
	filter := NewShingleFilter(input)
	filter.SetMinShingleSize(minShingleSize)
	filter.SetMaxShingleSize(maxShingleSize)
	return filter
}

// initAttributes retrieves attributes from the shared AttributeSource.
func (f *ShingleFilter) initAttributes() {
	attrSource := f.GetAttributeSource()
	if attrSource != nil {
		attr := attrSource.GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
		if attr != nil {
			f.termAttr = attr.(CharTermAttribute)
		}

		attr = attrSource.GetAttributeByType(reflect.TypeOf(&positionIncrementAttribute{}))
		if attr != nil {
			f.posIncrAttr = attr.(PositionIncrementAttribute)
		}

		attr = attrSource.GetAttributeByType(reflect.TypeOf(&offsetAttribute{}))
		if attr != nil {
			offsetAttr := attr.(OffsetAttribute)
			f.offsetAttr = offsetAttr
		}
	}
}

// SetMaxShingleSize sets the maximum number of tokens in a shingle.
// Must be >= minShingleSize and >= 2.
func (f *ShingleFilter) SetMaxShingleSize(size int) {
	if size < 2 {
		size = 2
	}
	if size < f.minShingleSize {
		size = f.minShingleSize
	}
	f.maxShingleSize = size
}

// GetMaxShingleSize returns the maximum shingle size.
func (f *ShingleFilter) GetMaxShingleSize() int {
	return f.maxShingleSize
}

// SetMinShingleSize sets the minimum number of tokens in a shingle.
// Must be >= 2 and <= maxShingleSize.
func (f *ShingleFilter) SetMinShingleSize(size int) {
	if size < 2 {
		size = 2
	}
	if size > f.maxShingleSize {
		size = f.maxShingleSize
	}
	f.minShingleSize = size
}

// GetMinShingleSize returns the minimum shingle size.
func (f *ShingleFilter) GetMinShingleSize() int {
	return f.minShingleSize
}

// SetTokenSeparator sets the string inserted between tokens in a shingle.
func (f *ShingleFilter) SetTokenSeparator(separator string) {
	f.tokenSeparator = separator
}

// GetTokenSeparator returns the token separator.
func (f *ShingleFilter) GetTokenSeparator() string {
	return f.tokenSeparator
}

// SetOutputUnigrams sets whether original tokens should be output.
// When true, both unigrams and shingles are output.
// When false, only shingles are output.
func (f *ShingleFilter) SetOutputUnigrams(output bool) {
	f.outputUnigrams = output
}

// IsOutputUnigrams returns whether unigrams are being output.
func (f *ShingleFilter) IsOutputUnigrams() bool {
	return f.outputUnigrams
}

// IncrementToken advances to the next token in the stream.
// This implements the core shingle generation logic.
func (f *ShingleFilter) IncrementToken() (bool, error) {
	for {
		// Check if we can emit more shingles from the current buffer position
		if f.nextTokenToEmit < f.numTokens {
			// Try to emit the next shingle for the current position
			emitted := f.emitNextShingle()
			if emitted {
				return true, nil
			}

			// No more shingles for this position, move to next
			f.nextTokenToEmit++
			f.currentShingleSize = 1
			continue
		}

		// No more tokens to emit from buffer, need more input
		if f.inputExhausted {
			return false, nil
		}

		// Get next token from input
		hasToken, err := f.input.IncrementToken()
		if err != nil {
			return false, err
		}

		if hasToken {
			// Add this token to the buffer
			f.addCurrentTokenToBuffer()
			// Continue to try emitting from this new position
			continue
		}

		// No more input tokens
		f.inputExhausted = true
		return false, nil
	}
}

// emitNextShingle emits the next shingle for the current position.
// Returns true if a shingle was emitted.
func (f *ShingleFilter) emitNextShingle() bool {
	if f.nextTokenToEmit >= f.numTokens {
		return false
	}

	// Find the next valid shingle size
	for f.currentShingleSize <= f.maxShingleSize {
		isUnigram := f.currentShingleSize == 1
		canEmit := false

		if isUnigram && f.outputUnigrams {
			// Can emit unigram
			canEmit = true
		} else if !isUnigram && f.currentShingleSize >= f.minShingleSize {
			// Check if we have enough tokens for this shingle
			if f.nextTokenToEmit+f.currentShingleSize <= f.numTokens {
				canEmit = true
			}
		}

		if canEmit {
			// Emit this shingle
			f.emitShingle(f.nextTokenToEmit, f.currentShingleSize)
			f.currentShingleSize++
			return true
		}

		// Try next size
		f.currentShingleSize++
	}

	// No more valid shingle sizes for this position
	return false
}

// emitShingle emits a single shingle.
func (f *ShingleFilter) emitShingle(startIdx, shingleSize int) {
	// Clear attributes for the new token
	f.ClearAttributes()

	endIdx := startIdx + shingleSize
	if endIdx > f.numTokens {
		endIdx = f.numTokens
	}

	// Build the term
	if f.termAttr != nil {
		f.termAttr.SetEmpty()

		for i := startIdx; i < endIdx; i++ {
			token := f.tokenBuffer[i]
			if token == nil {
				continue
			}

			if i > startIdx && f.tokenSeparator != "" {
				f.termAttr.AppendString(f.tokenSeparator)
			}

			f.termAttr.Append(token.term)
		}
	}

	// Set offsets
	if f.offsetAttr != nil {
		startToken := f.tokenBuffer[startIdx]
		endToken := f.tokenBuffer[endIdx-1]
		if startToken != nil {
			f.offsetAttr.SetStartOffset(startToken.startOffset)
		}
		if endToken != nil {
			f.offsetAttr.SetEndOffset(endToken.endOffset)
		}
	}

	// Set position increment
	if f.posIncrAttr != nil {
		if f.isFirstToken {
			f.posIncrAttr.SetPositionIncrement(1)
			f.isFirstToken = false
		} else {
			f.posIncrAttr.SetPositionIncrement(0)
		}
	}
}

// addCurrentTokenToBuffer adds the current token from input to the buffer.
func (f *ShingleFilter) addCurrentTokenToBuffer() {
	var term []byte
	var startOffset, endOffset int
	var posIncr int

	if f.termAttr != nil {
		term = f.termAttr.Bytes()
	}

	if f.offsetAttr != nil {
		startOffset = f.offsetAttr.StartOffset()
		endOffset = f.offsetAttr.EndOffset()
	}

	if f.posIncrAttr != nil {
		posIncr = f.posIncrAttr.GetPositionIncrement()
	}

	// Make a copy of the term
	termCopy := make([]byte, len(term))
	copy(termCopy, term)

	f.tokenBuffer = append(f.tokenBuffer, &tokenData{
		term:              termCopy,
		startOffset:       startOffset,
		endOffset:         endOffset,
		positionIncrement: posIncr,
	})
	f.numTokens++
}

// End performs end-of-stream operations.
func (f *ShingleFilter) End() error {
	// Set final offset if available
	if f.offsetAttr != nil && f.input != nil {
		// Try to get the end offset from the input
		if hasAttrSrc, ok := f.input.(interface{ GetAttributeSource() *AttributeSource }); ok {
			src := hasAttrSrc.GetAttributeSource()
			if src != nil {
				attr := src.GetAttributeByType(reflect.TypeOf(&offsetAttribute{}))
				if attr != nil {
					inputOffset := attr.(OffsetAttribute)
					f.offsetAttr.SetEndOffset(inputOffset.EndOffset())
				}
			}
		}
	}

	return f.BaseTokenFilter.End()
}

// Reset resets the filter state for reuse.
func (f *ShingleFilter) Reset() error {
	f.tokenBuffer = f.tokenBuffer[:0]
	f.numTokens = 0
	f.nextTokenToEmit = 0
	f.currentShingleSize = 1
	f.inputExhausted = false
	f.isFirstToken = true

	return f.BaseTokenFilter.End()
}

// Ensure ShingleFilter implements TokenFilter
var _ TokenFilter = (*ShingleFilter)(nil)

// ShingleFilterFactory creates ShingleFilter instances.
type ShingleFilterFactory struct {
	minShingleSize int
	maxShingleSize int
	tokenSeparator string
	outputUnigrams bool
}

// NewShingleFilterFactory creates a new ShingleFilterFactory with default settings.
func NewShingleFilterFactory() *ShingleFilterFactory {
	return &ShingleFilterFactory{
		minShingleSize: 2,
		maxShingleSize: 2,
		tokenSeparator: " ",
		outputUnigrams: true,
	}
}

// NewShingleFilterFactoryWithSizes creates a ShingleFilterFactory with custom sizes.
func NewShingleFilterFactoryWithSizes(minShingleSize, maxShingleSize int) *ShingleFilterFactory {
	factory := NewShingleFilterFactory()
	factory.minShingleSize = minShingleSize
	factory.maxShingleSize = maxShingleSize
	return factory
}

// SetMaxShingleSize sets the maximum shingle size.
func (f *ShingleFilterFactory) SetMaxShingleSize(size int) {
	f.maxShingleSize = size
}

// SetMinShingleSize sets the minimum shingle size.
func (f *ShingleFilterFactory) SetMinShingleSize(size int) {
	f.minShingleSize = size
}

// SetTokenSeparator sets the token separator.
func (f *ShingleFilterFactory) SetTokenSeparator(separator string) {
	f.tokenSeparator = separator
}

// SetOutputUnigrams sets whether to output unigrams.
func (f *ShingleFilterFactory) SetOutputUnigrams(output bool) {
	f.outputUnigrams = output
}

// Create creates a ShingleFilter wrapping the given input.
func (f *ShingleFilterFactory) Create(input TokenStream) TokenFilter {
	filter := NewShingleFilterWithSizes(input, f.minShingleSize, f.maxShingleSize)
	filter.SetTokenSeparator(f.tokenSeparator)
	filter.SetOutputUnigrams(f.outputUnigrams)
	return filter
}

// Ensure ShingleFilterFactory implements TokenFilterFactory
var _ TokenFilterFactory = (*ShingleFilterFactory)(nil)
