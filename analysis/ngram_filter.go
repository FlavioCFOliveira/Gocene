// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"unicode/utf8"
)

// NGramFilter generates n-grams from tokens.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.ngram.NGramTokenFilter.
//
// NGramFilter takes tokens from the input stream and generates n-grams of specified
// lengths. For example, with minGram=2 and maxGram=3, the token "hello" would
// generate: "he", "hel", "el", "ell", "ll", "llo", "lo".
//
// The filter handles position increments correctly:
// - The first n-gram for each input token has the original position increment
// - Subsequent n-grams have position increment 0 (same position as the first)
//
// Offsets are preserved to reflect the position of each n-gram in the original text.
type NGramFilter struct {
	*BaseTokenFilter

	// minGram is the minimum n-gram size
	minGram int

	// maxGram is the maximum n-gram size
	maxGram int

	// termAttr holds the CharTermAttribute from the shared attribute source
	termAttr CharTermAttribute

	// posIncrAttr holds the PositionIncrementAttribute from the shared attribute source
	posIncrAttr PositionIncrementAttribute

	// offsetAttr holds the OffsetAttribute from the shared attribute source
	offsetAttr OffsetAttribute

	// curTerm holds the current token text as a string
	curTerm string

	// curTermRunes holds the current token text as runes for Unicode support
	curTermRunes []rune

	// curStartOffset holds the start offset of the current token
	curStartOffset int

	// curEndOffset holds the end offset of the current token
	curEndOffset int

	// curPosIncr holds the position increment of the current token
	curPosIncr int

	// curGramSize is the current n-gram size being generated
	curGramSize int

	// curPos is the current starting position within the token (in runes)
	curPos int

	// firstGram indicates if this is the first n-gram for the current token
	firstGram bool

	// hasToken indicates if there's a token waiting to be processed
	hasToken bool
}

// NewNGramFilter creates a new NGramFilter with the specified min and max gram sizes.
//
// Parameters:
//   - input: the input TokenStream
//   - minGram: the minimum n-gram size (must be >= 1)
//   - maxGram: the maximum n-gram size (must be >= minGram)
//
// Returns a new NGramFilter.
func NewNGramFilter(input TokenStream, minGram, maxGram int) *NGramFilter {
	if minGram < 1 {
		minGram = 1
	}
	if maxGram < minGram {
		maxGram = minGram
	}

	filter := &NGramFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		minGram:         minGram,
		maxGram:         maxGram,
		curGramSize:     minGram,
		curPos:          0,
		firstGram:       true,
		hasToken:        false,
	}

	// Get attributes from the shared AttributeSource
	attrSrc := filter.GetAttributeSource()
	if attrSrc != nil {
		if attr := attrSrc.GetAttributeByType(reflect.TypeOf(&charTermAttribute{})); attr != nil {
			filter.termAttr = attr.(CharTermAttribute)
		}
		if attr := attrSrc.GetAttributeByType(reflect.TypeOf(&positionIncrementAttribute{})); attr != nil {
			filter.posIncrAttr = attr.(PositionIncrementAttribute)
		}
		if attr := attrSrc.GetAttributeByType(reflect.TypeOf(&offsetAttribute{})); attr != nil {
			filter.offsetAttr = attr.(OffsetAttribute)
		}
	}

	return filter
}

// IncrementToken advances to the next n-gram.
// Returns true if a token is available, false if at end of stream.
func (f *NGramFilter) IncrementToken() (bool, error) {
	for {
		// If we don't have a token to process, get one from input
		if !f.hasToken {
			hasToken, err := f.input.IncrementToken()
			if err != nil {
				return false, err
			}
			if !hasToken {
				return false, nil
			}

			// Save the current token info
			if f.termAttr != nil {
				f.curTerm = f.termAttr.String()
				// Convert to runes for proper Unicode handling
				f.curTermRunes = []rune(f.curTerm)
			}
			if f.offsetAttr != nil {
				f.curStartOffset = f.offsetAttr.StartOffset()
				f.curEndOffset = f.offsetAttr.EndOffset()
			}
			if f.posIncrAttr != nil {
				f.curPosIncr = f.posIncrAttr.GetPositionIncrement()
			} else {
				f.curPosIncr = 1
			}

			// Initialize n-gram generation
			f.curGramSize = f.minGram
			f.curPos = 0
			f.firstGram = true
			f.hasToken = true
		}

		// Generate n-grams: iterate by position first, then by gram size
		// This matches Lucene's NGramTokenFilter behavior
		for f.hasToken {
			// Check if we've exhausted all positions
			if f.curPos >= len(f.curTermRunes) {
				// Exhausted all positions, move to next token
				f.hasToken = false
				break // Break inner loop to get next token
			}

			// Check if current gram size is valid
			if f.curGramSize > f.maxGram {
				// Exhausted all gram sizes for this position, move to next position
				f.curPos++
				f.curGramSize = f.minGram
				continue
			}

			// Check if we can generate an n-gram at current position with current size
			if f.curPos+f.curGramSize <= len(f.curTermRunes) {
				// Generate the n-gram
				gramRunes := f.curTermRunes[f.curPos : f.curPos+f.curGramSize]
				gram := string(gramRunes)

				// Calculate byte offsets for the n-gram
				startByteOffset := f.curStartOffset
				endByteOffset := f.curStartOffset

				// Calculate byte offset from rune position
				// We need to find the byte offset of the start rune
				if f.curPos > 0 {
					// Count bytes up to the start rune
					for i := 0; i < f.curPos && i < len(f.curTermRunes); i++ {
						startByteOffset += utf8.RuneLen(f.curTermRunes[i])
					}
				}

				// Calculate end byte offset
				endByteOffset = startByteOffset
				for i := f.curPos; i < f.curPos+f.curGramSize && i < len(f.curTermRunes); i++ {
					endByteOffset += utf8.RuneLen(f.curTermRunes[i])
				}

				// Ensure offsets don't exceed original token bounds
				if startByteOffset < f.curStartOffset {
					startByteOffset = f.curStartOffset
				}
				if endByteOffset > f.curEndOffset {
					endByteOffset = f.curEndOffset
				}

				// Set the term
				if f.termAttr != nil {
					f.termAttr.SetValue(gram)
				}

				// Set the offset
				if f.offsetAttr != nil {
					f.offsetAttr.SetStartOffset(startByteOffset)
					f.offsetAttr.SetEndOffset(endByteOffset)
				}

				// Set the position increment
				if f.posIncrAttr != nil {
					if f.firstGram {
						f.posIncrAttr.SetPositionIncrement(f.curPosIncr)
						f.firstGram = false
					} else {
						f.posIncrAttr.SetPositionIncrement(0)
					}
				}

				// Move to next gram size for same position
				f.curGramSize++

				return true, nil
			}

			// Current gram size too large for this position, try next position
			f.curPos++
			f.curGramSize = f.minGram
		}
	}
}

// GetMinGram returns the minimum n-gram size.
func (f *NGramFilter) GetMinGram() int {
	return f.minGram
}

// GetMaxGram returns the maximum n-gram size.
func (f *NGramFilter) GetMaxGram() int {
	return f.maxGram
}

// Ensure NGramFilter implements TokenFilter
var _ TokenFilter = (*NGramFilter)(nil)

// NGramFilterFactory creates NGramFilter instances.
type NGramFilterFactory struct {
	minGram int
	maxGram int
}

// NewNGramFilterFactory creates a new NGramFilterFactory.
//
// Parameters:
//   - minGram: the minimum n-gram size (must be >= 1)
//   - maxGram: the maximum n-gram size (must be >= minGram)
func NewNGramFilterFactory(minGram, maxGram int) *NGramFilterFactory {
	if minGram < 1 {
		minGram = 1
	}
	if maxGram < minGram {
		maxGram = minGram
	}
	return &NGramFilterFactory{
		minGram: minGram,
		maxGram: maxGram,
	}
}

// Create creates an NGramFilter wrapping the given input.
func (f *NGramFilterFactory) Create(input TokenStream) TokenFilter {
	return NewNGramFilter(input, f.minGram, f.maxGram)
}

// GetMinGram returns the minimum n-gram size.
func (f *NGramFilterFactory) GetMinGram() int {
	return f.minGram
}

// GetMaxGram returns the maximum n-gram size.
func (f *NGramFilterFactory) GetMaxGram() int {
	return f.maxGram
}

// Ensure NGramFilterFactory implements TokenFilterFactory
var _ TokenFilterFactory = (*NGramFilterFactory)(nil)
