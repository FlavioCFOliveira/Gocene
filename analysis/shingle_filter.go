// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"strings"
)

// ShingleFilter combines multiple tokens into shingles (word n-grams).
//
// This is the Go port of Lucene's org.apache.lucene.analysis.shingle.ShingleFilter.
//
// A shingle is a token that combines multiple adjacent tokens. For example,
// with input tokens ["please", "divide", "this", "sentence"], a ShingleFilter
// with maxShingleSize=2 produces:
//
//	["please", "please divide", "divide", "divide this", "this", "this sentence", "sentence"]
//
// The filter supports:
//   - Configurable min/max shingle size
//   - Token separator between shingled tokens
//   - Outputting unigrams (original tokens) alongside shingles
//   - Preserving position increments for proper phrase matching
type ShingleFilter struct {
	*BaseTokenFilter

	// minShingleSize is the minimum number of tokens in a shingle (default: 2).
	minShingleSize int

	// maxShingleSize is the maximum number of tokens in a shingle (default: 2).
	maxShingleSize int

	// tokenSeparator is inserted between tokens when building shingle text (default: " ").
	tokenSeparator string

	// outputUnigrams controls whether original tokens are output alongside shingles.
	outputUnigrams bool

	// inputWindow is the sliding window of buffered tokens.
	// It holds at most maxShingleSize entries, each a snapshot of a single
	// input token's attributes (term, offsets, posIncr).
	inputWindow []windowToken

	// gramSize is the circular sequence controlling how many tokens the
	// current shingle spans. It cycles through:
	//   [1,] minShingleSize, minShingleSize+1, …, maxShingleSize
	// (1 is included only when outputUnigrams == true).
	gramSize circularSeq

	// isOutputHere is true if at least one token has been emitted for the
	// current anchor position (matches Lucene's isOutputHere field).
	isOutputHere bool

	// isFirstToken is true until the very first token has been emitted.
	// The first token in the stream always gets positionIncrement=1;
	// all subsequent tokens get 0, reflecting that shingles share positions
	// with their constituent unigrams.
	isFirstToken bool

	// exhausted is true once input.IncrementToken() has returned false.
	exhausted bool

	// builtGramSize tracks how many tokens of the current shingle text have
	// already been appended to gramBuilder (used when resuming mid-shingle).
	builtGramSize int

	// gramBuilder accumulates shingle text for the token currently being built.
	gramBuilder strings.Builder
}

// windowToken holds a snapshot of a single input token's relevant attributes.
type windowToken struct {
	term        string
	startOffset int
	endOffset   int
}

// circularSeq mimics Lucene's inner CircularSequence class.
// It cycles through { [1,] minShingleSize, …, maxShingleSize } and tracks
// the current and previous values.
type circularSeq struct {
	value         int
	previousValue int
	minValue      int
	minShingle    int
	maxShingle    int
}

func newCircularSeq(outputUnigrams bool, minShingle, maxShingle int) circularSeq {
	cs := circularSeq{
		minShingle: minShingle,
		maxShingle: maxShingle,
	}
	if outputUnigrams {
		cs.minValue = 1
	} else {
		cs.minValue = minShingle
	}
	cs.reset()
	return cs
}

func (cs *circularSeq) reset() {
	cs.previousValue = cs.minValue
	cs.value = cs.minValue
}

func (cs *circularSeq) getValue() int { return cs.value }

func (cs *circularSeq) getPreviousValue() int { return cs.previousValue }

func (cs *circularSeq) atMinValue() bool { return cs.value == cs.minValue }

func (cs *circularSeq) advance() {
	cs.previousValue = cs.value
	switch {
	case cs.value == 1:
		cs.value = cs.minShingle
	case cs.value == cs.maxShingle:
		cs.reset()
	default:
		cs.value++
	}
}

// NewShingleFilter creates a ShingleFilter with default settings:
// minShingleSize=2, maxShingleSize=2, tokenSeparator=" ", outputUnigrams=true.
func NewShingleFilter(input TokenStream) *ShingleFilter {
	f := &ShingleFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		minShingleSize:  2,
		maxShingleSize:  2,
		tokenSeparator:  " ",
		outputUnigrams:  true,
		isFirstToken:    true,
	}
	f.gramSize = newCircularSeq(true, 2, 2)
	return f
}

// NewShingleFilterWithSizes creates a ShingleFilter with custom min/max sizes.
func NewShingleFilterWithSizes(input TokenStream, minShingleSize, maxShingleSize int) *ShingleFilter {
	if minShingleSize < 2 {
		minShingleSize = 2
	}
	if maxShingleSize < minShingleSize {
		maxShingleSize = minShingleSize
	}
	f := &ShingleFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		minShingleSize:  minShingleSize,
		maxShingleSize:  maxShingleSize,
		tokenSeparator:  " ",
		outputUnigrams:  true,
		isFirstToken:    true,
	}
	f.gramSize = newCircularSeq(true, minShingleSize, maxShingleSize)
	return f
}

// SetMaxShingleSize sets the maximum number of tokens in a shingle (must be >= 2).
func (f *ShingleFilter) SetMaxShingleSize(size int) {
	if size < 2 {
		size = 2
	}
	if size < f.minShingleSize {
		size = f.minShingleSize
	}
	f.maxShingleSize = size
	f.gramSize = newCircularSeq(f.outputUnigrams, f.minShingleSize, f.maxShingleSize)
}

// GetMaxShingleSize returns the maximum shingle size.
func (f *ShingleFilter) GetMaxShingleSize() int { return f.maxShingleSize }

// SetMinShingleSize sets the minimum number of tokens in a shingle (must be >= 2).
func (f *ShingleFilter) SetMinShingleSize(size int) {
	if size < 2 {
		size = 2
	}
	if size > f.maxShingleSize {
		size = f.maxShingleSize
	}
	f.minShingleSize = size
	f.gramSize = newCircularSeq(f.outputUnigrams, f.minShingleSize, f.maxShingleSize)
}

// GetMinShingleSize returns the minimum shingle size.
func (f *ShingleFilter) GetMinShingleSize() int { return f.minShingleSize }

// SetTokenSeparator sets the string inserted between tokens when building shingles.
func (f *ShingleFilter) SetTokenSeparator(separator string) { f.tokenSeparator = separator }

// GetTokenSeparator returns the token separator string.
func (f *ShingleFilter) GetTokenSeparator() string { return f.tokenSeparator }

// SetOutputUnigrams sets whether original tokens are output alongside shingles.
func (f *ShingleFilter) SetOutputUnigrams(output bool) {
	f.outputUnigrams = output
	f.gramSize = newCircularSeq(output, f.minShingleSize, f.maxShingleSize)
}

// IsOutputUnigrams returns whether unigrams are being output.
func (f *ShingleFilter) IsOutputUnigrams() bool { return f.outputUnigrams }

// IncrementToken advances to the next output token.
//
// Mirrors Lucene's incrementToken(): it emits one token per call by
// operating on the sliding inputWindow. For each anchor position the
// sequence is: emit gram sizes [1,] minShingle … maxShingle in order.
// When the sequence wraps back to minValue, shiftInputWindow() is called
// first to advance the anchor by one position.
func (f *ShingleFilter) IncrementToken() (bool, error) {
	tokenAvailable := false
	builtGramSize := 0

	if f.gramSize.atMinValue() || len(f.inputWindow) < f.gramSize.getValue() {
		if err := f.shiftInputWindow(); err != nil {
			return false, err
		}
		f.gramBuilder.Reset()
	} else {
		builtGramSize = f.gramSize.getPreviousValue()
	}

	if len(f.inputWindow) >= f.gramSize.getValue() {
		isAllFiller := true // we have no filler concept, always false for real tokens
		_ = isAllFiller

		// Build the gram text, reusing already-appended prefix when resuming.
		endToken := f.inputWindow[0] // will be updated below
		for gramNum := 1; gramNum <= f.gramSize.getValue() && builtGramSize < f.gramSize.getValue(); gramNum++ {
			tok := f.inputWindow[gramNum-1]
			if builtGramSize < gramNum {
				if builtGramSize > 0 {
					f.gramBuilder.WriteString(f.tokenSeparator)
				}
				f.gramBuilder.WriteString(tok.term)
				builtGramSize++
			}
			endToken = tok
		}

		if builtGramSize == f.gramSize.getValue() {
			// Emit this gram: set attributes from the anchor token (inputWindow[0])
			// and override term + end-offset.
			anchor := f.inputWindow[0]
			posIncr := 0
			if f.isFirstToken {
				posIncr = 1
				f.isFirstToken = false
			}

			attrSrc := f.GetAttributeSource()
			if attrSrc != nil {
				if attr := attrSrc.GetAttribute(CharTermAttributeType); attr != nil {
					if ta, ok := attr.(CharTermAttribute); ok {
						ta.SetEmpty()
						ta.AppendString(f.gramBuilder.String())
					}
				}
				if attr := attrSrc.GetAttribute(OffsetAttributeType); attr != nil {
					if oa, ok := attr.(OffsetAttribute); ok {
						oa.SetStartOffset(anchor.startOffset)
						oa.SetEndOffset(endToken.endOffset)
					}
				}
				if attr := attrSrc.GetAttribute(PositionIncrementAttributeType); attr != nil {
					if pa, ok := attr.(PositionIncrementAttribute); ok {
						pa.SetPositionIncrement(posIncr)
					}
				}
			}

			f.isOutputHere = true
			f.gramSize.advance()
			tokenAvailable = true
		}
	}
	return tokenAvailable, nil
}

// shiftInputWindow slides the window one position to the right, matching
// Lucene's shiftInputWindow(): remove the first element (if any) and then
// fill up to maxShingleSize slots from the input stream.
func (f *ShingleFilter) shiftInputWindow() error {
	if len(f.inputWindow) > 0 {
		f.inputWindow = f.inputWindow[1:]
	}

	for len(f.inputWindow) < f.maxShingleSize {
		tok, err := f.getNextToken()
		if err != nil {
			return err
		}
		if tok == nil {
			break
		}
		f.inputWindow = append(f.inputWindow, *tok)
	}

	f.gramSize.reset()
	f.isOutputHere = false
	return nil
}

// getNextToken reads the next token from the underlying input stream and
// returns a windowToken snapshot, or nil when the stream is exhausted.
func (f *ShingleFilter) getNextToken() (*windowToken, error) {
	if f.exhausted {
		return nil, nil
	}
	hasToken, err := f.input.IncrementToken()
	if err != nil {
		return nil, err
	}
	if !hasToken {
		f.exhausted = true
		return nil, nil
	}

	wt := &windowToken{}
	attrSrc := f.GetAttributeSource()
	if attrSrc != nil {
		if attr := attrSrc.GetAttribute(CharTermAttributeType); attr != nil {
			if ta, ok := attr.(CharTermAttribute); ok {
				wt.term = ta.String()
			}
		}
		if attr := attrSrc.GetAttribute(OffsetAttributeType); attr != nil {
			if oa, ok := attr.(OffsetAttribute); ok {
				wt.startOffset = oa.StartOffset()
				wt.endOffset = oa.EndOffset()
			}
		}
	}
	return wt, nil
}

// End delegates to the wrapped input and resets the local end-state.
func (f *ShingleFilter) End() error {
	return f.BaseTokenFilter.End()
}

// Reset clears all internal state so the filter can be reused after
// the underlying tokenizer has been reset to a new reader.
func (f *ShingleFilter) Reset() error {
	f.inputWindow = f.inputWindow[:0]
	f.gramSize = newCircularSeq(f.outputUnigrams, f.minShingleSize, f.maxShingleSize)
	f.isOutputHere = false
	f.isFirstToken = true
	f.exhausted = false
	f.builtGramSize = 0
	f.gramBuilder.Reset()

	if f.input != nil {
		if resettable, ok := f.input.(interface{ Reset() error }); ok {
			return resettable.Reset()
		}
	}
	return nil
}

// Ensure ShingleFilter implements TokenFilter.
var _ TokenFilter = (*ShingleFilter)(nil)

// ShingleFilterFactory creates ShingleFilter instances.
type ShingleFilterFactory struct {
	minShingleSize int
	maxShingleSize int
	tokenSeparator string
	outputUnigrams bool
}

// NewShingleFilterFactory creates a ShingleFilterFactory with default settings.
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
func (f *ShingleFilterFactory) SetMaxShingleSize(size int) { f.maxShingleSize = size }

// SetMinShingleSize sets the minimum shingle size.
func (f *ShingleFilterFactory) SetMinShingleSize(size int) { f.minShingleSize = size }

// SetTokenSeparator sets the token separator.
func (f *ShingleFilterFactory) SetTokenSeparator(separator string) { f.tokenSeparator = separator }

// SetOutputUnigrams sets whether to output unigrams.
func (f *ShingleFilterFactory) SetOutputUnigrams(output bool) { f.outputUnigrams = output }

// Create creates a ShingleFilter wrapping the given input.
func (f *ShingleFilterFactory) Create(input TokenStream) TokenFilter {
	filter := NewShingleFilterWithSizes(input, f.minShingleSize, f.maxShingleSize)
	filter.SetTokenSeparator(f.tokenSeparator)
	filter.SetOutputUnigrams(f.outputUnigrams)
	return filter
}

// Ensure ShingleFilterFactory implements TokenFilterFactory.
var _ TokenFilterFactory = (*ShingleFilterFactory)(nil)
