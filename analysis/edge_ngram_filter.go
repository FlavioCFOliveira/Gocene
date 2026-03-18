// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"unicode/utf8"
)

// EdgeNGramFilter generates edge n-grams (prefix n-grams) from tokens.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.EdgeNGramTokenFilter.
//
// Edge n-grams are prefix substrings of a token. For example, given the token
// "hello" with minGram=2 and maxGram=4, the filter produces:
//   - "he" (2-gram)
//   - "hel" (3-gram)
//   - "hell" (4-gram)
//
// This filter is useful for autocomplete/search-as-you-type functionality.
//
// Position increments are handled as follows:
//   - The first n-gram retains the original position increment
//   - Subsequent n-grams have position increment 0 (same position as first)
//
// Offsets are adjusted for each n-gram to reflect the substring boundaries.
type EdgeNGramFilter struct {
	*BaseTokenFilter

	// minGram is the minimum n-gram size
	minGram int

	// maxGram is the maximum n-gram size
	maxGram int

	// preserveOriginal indicates whether to preserve the original token
	preserveOriginal bool

	// termAttr holds the CharTermAttribute from the shared attribute source
	termAttr CharTermAttribute

	// posIncrAttr holds the PositionIncrementAttribute from the shared attribute source
	posIncrAttr PositionIncrementAttribute

	// offsetAttr holds the OffsetAttribute from the shared attribute source
	offsetAttr OffsetAttribute

	// currentToken holds the current token being processed
	currentToken string

	// currentStartOffset holds the start offset of the current token
	currentStartOffset int

	// currentEndOffset holds the end offset of the current token
	currentEndOffset int

	// currentPosIncr holds the position increment of the current token
	currentPosIncr int

	// gramSize is the current n-gram size being emitted
	gramSize int

	// tokenLength is the length of the current token in runes
	tokenLength int

	// state tracks the filter state
	state filterState
}

// filterState represents the current state of the filter
type filterState int

const (
	// stateReading indicates we're reading the next input token
	stateReading filterState = iota

	// stateEmitting indicates we're emitting n-grams for the current token
	stateEmitting

	// stateEmitOriginal indicates we need to emit the original token
	stateEmitOriginal
)

// NewEdgeNGramFilter creates a new EdgeNGramFilter with the given min and max gram sizes.
//
// Parameters:
//   - input: the input TokenStream
//   - minGram: the minimum n-gram size (must be >= 1)
//   - maxGram: the maximum n-gram size (must be >= minGram)
//
// The filter will not preserve the original token - only n-grams are emitted.
// Use NewEdgeNGramFilterWithOptions for more control.
func NewEdgeNGramFilter(input TokenStream, minGram, maxGram int) *EdgeNGramFilter {
	return NewEdgeNGramFilterWithOptions(input, minGram, maxGram, false)
}

// NewEdgeNGramFilterWithOptions creates a new EdgeNGramFilter with full options.
//
// Parameters:
//   - input: the input TokenStream
//   - minGram: the minimum n-gram size (must be >= 1)
//   - maxGram: the maximum n-gram size (must be >= minGram)
//   - preserveOriginal: if true, the original token is also emitted (after n-grams)
func NewEdgeNGramFilterWithOptions(input TokenStream, minGram, maxGram int, preserveOriginal bool) *EdgeNGramFilter {
	// Validate parameters
	if minGram < 1 {
		minGram = 1
	}
	if maxGram < minGram {
		maxGram = minGram
	}

	filter := &EdgeNGramFilter{
		BaseTokenFilter:  NewBaseTokenFilter(input),
		minGram:          minGram,
		maxGram:          maxGram,
		preserveOriginal: preserveOriginal,
		state:            stateReading,
		gramSize:         minGram,
		currentPosIncr:   1,
	}

	// Get attributes from the shared AttributeSource
	attrSource := filter.GetAttributeSource()
	if attrSource != nil {
		if attr := attrSource.GetAttributeByType(reflect.TypeOf(&charTermAttribute{})); attr != nil {
			filter.termAttr = attr.(CharTermAttribute)
		}
		if attr := attrSource.GetAttributeByType(reflect.TypeOf(&positionIncrementAttribute{})); attr != nil {
			filter.posIncrAttr = attr.(PositionIncrementAttribute)
		}
		if attr := attrSource.GetAttributeByType(reflect.TypeOf(&offsetAttribute{})); attr != nil {
			filter.offsetAttr = attr.(OffsetAttribute)
		}
	}

	return filter
}

// IncrementToken advances to the next token, emitting edge n-grams.
func (f *EdgeNGramFilter) IncrementToken() (bool, error) {
	for {
		switch f.state {
		case stateReading:
			// Read the next token from input
			hasToken, err := f.input.IncrementToken()
			if err != nil {
				return false, err
			}
			if !hasToken {
				return false, nil
			}

			// Get the current token text
			if f.termAttr != nil {
				f.currentToken = f.termAttr.String()
			} else {
				f.currentToken = ""
			}

			// Get the current offsets
			if f.offsetAttr != nil {
				f.currentStartOffset = f.offsetAttr.StartOffset()
				f.currentEndOffset = f.offsetAttr.EndOffset()
			} else {
				f.currentStartOffset = 0
				f.currentEndOffset = len(f.currentToken)
			}

			// Get the current position increment
			if f.posIncrAttr != nil {
				f.currentPosIncr = f.posIncrAttr.GetPositionIncrement()
			} else {
				f.currentPosIncr = 1
			}

			// Calculate token length in runes
			f.tokenLength = utf8.RuneCountInString(f.currentToken)

			// If token is shorter than minGram, skip it (unless preserving original)
			if f.tokenLength < f.minGram {
				if f.preserveOriginal {
					// Emit the original token as-is
					if f.posIncrAttr != nil {
						f.posIncrAttr.SetPositionIncrement(f.currentPosIncr)
					}
					return true, nil
				}
				// Skip this token and continue to next
				continue
			}

			// Start emitting n-grams
			f.gramSize = f.minGram
			f.state = stateEmitting
			// Fall through to emit the first n-gram

		case stateEmitting:
			// Calculate the maximum gram size for this token
			maxGramForToken := f.maxGram
			if f.tokenLength < maxGramForToken {
				maxGramForToken = f.tokenLength
			}

			// Check if we've emitted all n-grams
			if f.gramSize > maxGramForToken {
				if f.preserveOriginal && f.tokenLength >= f.minGram {
					// Transition to emitting the original token
					f.state = stateEmitOriginal
					continue
				}
				// Done with this token, read next
				f.state = stateReading
				continue
			}

			// Emit the n-gram
			f.emitNGram(f.gramSize)
			f.gramSize++
			return true, nil

		case stateEmitOriginal:
			// Emit the original token
			f.state = stateReading
			if f.termAttr != nil {
				f.termAttr.SetValue(f.currentToken)
			}
			if f.offsetAttr != nil {
				f.offsetAttr.SetStartOffset(f.currentStartOffset)
				f.offsetAttr.SetEndOffset(f.currentEndOffset)
			}
			if f.posIncrAttr != nil {
				// Original token gets position increment 0 (same position as last n-gram)
				f.posIncrAttr.SetPositionIncrement(0)
			}
			return true, nil
		}
	}
}

// emitNGram emits an n-gram of the specified size.
func (f *EdgeNGramFilter) emitNGram(size int) {
	if f.termAttr != nil {
		// Extract the prefix of the specified size (in runes)
		gram := f.prefixString(f.currentToken, size)
		f.termAttr.SetValue(gram)
	}

	if f.offsetAttr != nil {
		// Calculate the end offset for this n-gram
		// We need to find the byte offset of the size-th rune
		endOffset := f.currentStartOffset + f.runeOffset(f.currentToken, size)
		f.offsetAttr.SetStartOffset(f.currentStartOffset)
		f.offsetAttr.SetEndOffset(endOffset)
	}

	if f.posIncrAttr != nil {
		// First n-gram gets the original position increment
		// Subsequent n-grams get position increment 0
		if f.gramSize == f.minGram {
			f.posIncrAttr.SetPositionIncrement(f.currentPosIncr)
		} else {
			f.posIncrAttr.SetPositionIncrement(0)
		}
	}
}

// prefixString returns the first n runes of a string.
func (f *EdgeNGramFilter) prefixString(s string, n int) string {
	if n <= 0 {
		return ""
	}

	// Count runes and find the byte offset
	count := 0
	byteOffset := 0
	for _, r := range s {
		if count >= n {
			break
		}
		count++
		byteOffset += utf8.RuneLen(r)
	}

	return s[:byteOffset]
}

// runeOffset returns the byte offset of the nth rune in a string.
func (f *EdgeNGramFilter) runeOffset(s string, n int) int {
	if n <= 0 {
		return 0
	}

	count := 0
	byteOffset := 0
	for _, r := range s {
		if count >= n {
			break
		}
		count++
		byteOffset += utf8.RuneLen(r)
	}

	return byteOffset
}

// Reset resets the filter state for a new token stream.
func (f *EdgeNGramFilter) Reset() error {
	f.state = stateReading
	f.gramSize = f.minGram
	f.currentToken = ""
	f.currentPosIncr = 1
	return nil
}

// GetMinGram returns the minimum gram size.
func (f *EdgeNGramFilter) GetMinGram() int {
	return f.minGram
}

// GetMaxGram returns the maximum gram size.
func (f *EdgeNGramFilter) GetMaxGram() int {
	return f.maxGram
}

// IsPreserveOriginal returns whether the original token is preserved.
func (f *EdgeNGramFilter) IsPreserveOriginal() bool {
	return f.preserveOriginal
}

// Ensure EdgeNGramFilter implements TokenFilter
var _ TokenFilter = (*EdgeNGramFilter)(nil)

// EdgeNGramFilterFactory creates EdgeNGramFilter instances.
type EdgeNGramFilterFactory struct {
	minGram          int
	maxGram          int
	preserveOriginal bool
}

// NewEdgeNGramFilterFactory creates a new EdgeNGramFilterFactory.
//
// Parameters:
//   - minGram: the minimum n-gram size
//   - maxGram: the maximum n-gram size
func NewEdgeNGramFilterFactory(minGram, maxGram int) *EdgeNGramFilterFactory {
	return NewEdgeNGramFilterFactoryWithOptions(minGram, maxGram, false)
}

// NewEdgeNGramFilterFactoryWithOptions creates a new EdgeNGramFilterFactory with options.
//
// Parameters:
//   - minGram: the minimum n-gram size
//   - maxGram: the maximum n-gram size
//   - preserveOriginal: whether to preserve the original token
func NewEdgeNGramFilterFactoryWithOptions(minGram, maxGram int, preserveOriginal bool) *EdgeNGramFilterFactory {
	// Validate parameters
	if minGram < 1 {
		minGram = 1
	}
	if maxGram < minGram {
		maxGram = minGram
	}

	return &EdgeNGramFilterFactory{
		minGram:          minGram,
		maxGram:          maxGram,
		preserveOriginal: preserveOriginal,
	}
}

// Create creates an EdgeNGramFilter wrapping the given input.
func (f *EdgeNGramFilterFactory) Create(input TokenStream) TokenFilter {
	return NewEdgeNGramFilterWithOptions(input, f.minGram, f.maxGram, f.preserveOriginal)
}

// GetMinGram returns the minimum gram size.
func (f *EdgeNGramFilterFactory) GetMinGram() int {
	return f.minGram
}

// GetMaxGram returns the maximum gram size.
func (f *EdgeNGramFilterFactory) GetMaxGram() int {
	return f.maxGram
}

// IsPreserveOriginal returns whether the original token is preserved.
func (f *EdgeNGramFilterFactory) IsPreserveOriginal() bool {
	return f.preserveOriginal
}

// Ensure EdgeNGramFilterFactory implements TokenFilterFactory
var _ TokenFilterFactory = (*EdgeNGramFilterFactory)(nil)
