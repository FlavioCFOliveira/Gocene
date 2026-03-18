// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
)

// WordDelimiterFilter splits tokens at word boundaries.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.miscellaneous.WordDelimiterFilter.
//
// The filter splits tokens at word boundaries, handling:
//   - camelCase and PascalCase transitions
//   - Delimiters (hyphens, underscores, etc.)
//   - Number-to-letter and letter-to-number transitions
//   - English possessives (trailing "'s")
//
// For example, "PowerShot12Mpx" is split into "Power", "Shot", "12", "Mpx".
//
// The filter can optionally preserve the original token (emit it along with the split parts).
// When splitting, the first token retains the original position increment, while subsequent
// tokens have a position increment of 0, indicating they are at the same position.
type WordDelimiterFilter struct {
	*BaseTokenFilter

	// termAttr holds the CharTermAttribute from the shared attribute source
	termAttr CharTermAttribute

	// offsetAttr holds the OffsetAttribute from the shared attribute source
	offsetAttr OffsetAttribute

	// posIncAttr holds the PositionIncrementAttribute from the shared attribute source
	posIncAttr PositionIncrementAttribute

	// iterator is used to find word boundaries
	iterator *WordDelimiterIterator

	// Configuration options
	splitOnCaseChange     bool
	splitOnNumerics       bool
	stemEnglishPossessive bool
	preserveOriginal      bool

	// State for token emission
	savedTokens       []savedToken
	savedTokenIndex   int
	currentTokenStart int
	currentTokenEnd   int
	currentPosInc     int
	hasOriginalToken  bool
}

// savedToken represents a token to be emitted
type savedToken struct {
	text        string
	startOffset int
	endOffset   int
	posInc      int
}

// NewWordDelimiterFilter creates a new WordDelimiterFilter with the given configuration.
//
// Parameters:
//   - input: the input TokenStream
//   - splitOnCaseChange: if true, causes "PowerShot" to be two tokens
//   - splitOnNumerics: if true, causes "j2se" to be three tokens
//   - stemEnglishPossessive: if true, causes trailing "'s" to be removed
//   - preserveOriginal: if true, the original token is also emitted
func NewWordDelimiterFilter(input TokenStream, splitOnCaseChange, splitOnNumerics, stemEnglishPossessive, preserveOriginal bool) *WordDelimiterFilter {
	filter := &WordDelimiterFilter{
		BaseTokenFilter:       NewBaseTokenFilter(input),
		splitOnCaseChange:     splitOnCaseChange,
		splitOnNumerics:       splitOnNumerics,
		stemEnglishPossessive: stemEnglishPossessive,
		preserveOriginal:      preserveOriginal,
		iterator:              NewWordDelimiterIterator(splitOnCaseChange, splitOnNumerics, stemEnglishPossessive),
		savedTokens:           make([]savedToken, 0, 8),
	}

	// Get attributes from the shared AttributeSource
	attrSrc := filter.GetAttributeSource()
	if attrSrc != nil {
		if attr := attrSrc.GetAttributeByType(reflect.TypeOf(&charTermAttribute{})); attr != nil {
			filter.termAttr = attr.(CharTermAttribute)
		}
		if attr := attrSrc.GetAttributeByType(reflect.TypeOf(&offsetAttribute{})); attr != nil {
			filter.offsetAttr = attr.(OffsetAttribute)
		}
		if attr := attrSrc.GetAttribute("PositionIncrementAttribute"); attr != nil {
			filter.posIncAttr = attr.(PositionIncrementAttribute)
		}
	}

	return filter
}

// NewWordDelimiterFilterWithTable creates a new WordDelimiterFilter with a custom character type table.
//
// Parameters:
//   - input: the input TokenStream
//   - charTypeTable: custom character type table (should be at least 256 bytes for ASCII)
//   - splitOnCaseChange: if true, causes "PowerShot" to be two tokens
//   - splitOnNumerics: if true, causes "j2se" to be three tokens
//   - stemEnglishPossessive: if true, causes trailing "'s" to be removed
//   - preserveOriginal: if true, the original token is also emitted
func NewWordDelimiterFilterWithTable(input TokenStream, charTypeTable []byte, splitOnCaseChange, splitOnNumerics, stemEnglishPossessive, preserveOriginal bool) *WordDelimiterFilter {
	filter := &WordDelimiterFilter{
		BaseTokenFilter:       NewBaseTokenFilter(input),
		splitOnCaseChange:     splitOnCaseChange,
		splitOnNumerics:       splitOnNumerics,
		stemEnglishPossessive: stemEnglishPossessive,
		preserveOriginal:      preserveOriginal,
		iterator:              NewWordDelimiterIteratorWithTable(charTypeTable, splitOnCaseChange, splitOnNumerics, stemEnglishPossessive),
		savedTokens:           make([]savedToken, 0, 8),
	}

	// Get attributes from the shared AttributeSource
	attrSrc := filter.GetAttributeSource()
	if attrSrc != nil {
		if attr := attrSrc.GetAttributeByType(reflect.TypeOf(&charTermAttribute{})); attr != nil {
			filter.termAttr = attr.(CharTermAttribute)
		}
		if attr := attrSrc.GetAttributeByType(reflect.TypeOf(&offsetAttribute{})); attr != nil {
			filter.offsetAttr = attr.(OffsetAttribute)
		}
		if attr := attrSrc.GetAttribute("PositionIncrementAttribute"); attr != nil {
			filter.posIncAttr = attr.(PositionIncrementAttribute)
		}
	}

	return filter
}

// IncrementToken advances to the next token, splitting at word boundaries.
func (f *WordDelimiterFilter) IncrementToken() (bool, error) {
	// If we have saved tokens to emit, emit the next one
	if f.savedTokenIndex < len(f.savedTokens) {
		token := f.savedTokens[f.savedTokenIndex]
		f.savedTokenIndex++
		f.emitToken(token)
		return true, nil
	}

	// Clear saved tokens
	f.savedTokens = f.savedTokens[:0]
	f.savedTokenIndex = 0

	// Get the next token from input
	hasToken, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}
	if !hasToken {
		return false, nil
	}

	// Get the current token text
	if f.termAttr == nil {
		// No term attribute, just pass through
		return true, nil
	}

	tokenText := f.termAttr.String()
	if tokenText == "" {
		return true, nil
	}

	// Get the current offsets
	var startOffset, endOffset int
	if f.offsetAttr != nil {
		startOffset = f.offsetAttr.StartOffset()
		endOffset = f.offsetAttr.EndOffset()
	}

	// Get the current position increment
	var posInc int = 1
	if f.posIncAttr != nil {
		posInc = f.posIncAttr.GetPositionIncrement()
	}

	// Convert token text to runes for the iterator
	runes := []rune(tokenText)
	if len(runes) == 0 {
		return true, nil
	}

	// Set up the iterator
	f.iterator.SetText(runes, len(runes))

	// Collect all subwords
	subwords := make([]savedToken, 0, 4)
	for f.iterator.Next() != DONE {
		start := f.iterator.Current()
		end := f.iterator.End()
		if start < end && end <= len(runes) {
			subword := string(runes[start:end])
			// Calculate offsets based on rune positions
			// We need to convert rune positions to byte positions for the original text
			subwordStartOffset := startOffset + f.runeIndexToByteIndex(tokenText, start)
			subwordEndOffset := startOffset + f.runeIndexToByteIndex(tokenText, end)
			subwords = append(subwords, savedToken{
				text:        subword,
				startOffset: subwordStartOffset,
				endOffset:   subwordEndOffset,
				posInc:      0, // Will be set later
			})
		}
	}

	// If no subwords were found, or the token wasn't split, emit the original
	if len(subwords) == 0 {
		return true, nil
	}

	// Check if the token was actually split
	wasSplit := len(subwords) > 1 || subwords[0].text != tokenText

	if !wasSplit {
		// Token wasn't split, emit as-is
		return true, nil
	}

	// Token was split - set up the saved tokens
	// First subword gets the original position increment
	// Subsequent subwords get position increment 0
	for i := range subwords {
		if i == 0 {
			subwords[i].posInc = posInc
		} else {
			subwords[i].posInc = 0
		}
	}

	// If preserving original, add it as the first token
	if f.preserveOriginal {
		f.savedTokens = append(f.savedTokens, savedToken{
			text:        tokenText,
			startOffset: startOffset,
			endOffset:   endOffset,
			posInc:      posInc,
		})
		// All split parts get position increment 0 when original is preserved
		for i := range subwords {
			subwords[i].posInc = 0
		}
	}

	// Add the subwords to saved tokens
	f.savedTokens = append(f.savedTokens, subwords...)

	// Emit the first saved token
	if len(f.savedTokens) > 0 {
		token := f.savedTokens[0]
		f.savedTokenIndex = 1
		f.emitToken(token)
		return true, nil
	}

	return true, nil
}

// emitToken sets the attributes for the given saved token.
func (f *WordDelimiterFilter) emitToken(token savedToken) {
	if f.termAttr != nil {
		f.termAttr.SetValue(token.text)
	}
	if f.offsetAttr != nil {
		f.offsetAttr.SetStartOffset(token.startOffset)
		f.offsetAttr.SetEndOffset(token.endOffset)
	}
	if f.posIncAttr != nil {
		f.posIncAttr.SetPositionIncrement(token.posInc)
	}
}

// runeIndexToByteIndex converts a rune index to a byte index in the given string.
func (f *WordDelimiterFilter) runeIndexToByteIndex(s string, runeIndex int) int {
	if runeIndex <= 0 {
		return 0
	}
	currentRuneIndex := 0
	for i := range s {
		if currentRuneIndex == runeIndex {
			return i
		}
		currentRuneIndex++
	}
	return len(s)
}

// Reset resets the filter state.
func (f *WordDelimiterFilter) Reset() error {
	f.savedTokens = f.savedTokens[:0]
	f.savedTokenIndex = 0
	if f.input != nil {
		if resettable, ok := f.input.(interface{ Reset() error }); ok {
			return resettable.Reset()
		}
	}
	return nil
}

// Ensure WordDelimiterFilter implements TokenFilter
var _ TokenFilter = (*WordDelimiterFilter)(nil)

// WordDelimiterFilterFactory creates WordDelimiterFilter instances.
type WordDelimiterFilterFactory struct {
	splitOnCaseChange     bool
	splitOnNumerics       bool
	stemEnglishPossessive bool
	preserveOriginal      bool
}

// NewWordDelimiterFilterFactory creates a new WordDelimiterFilterFactory.
//
// Parameters:
//   - splitOnCaseChange: if true, causes "PowerShot" to be two tokens
//   - splitOnNumerics: if true, causes "j2se" to be three tokens
//   - stemEnglishPossessive: if true, causes trailing "'s" to be removed
//   - preserveOriginal: if true, the original token is also emitted
func NewWordDelimiterFilterFactory(splitOnCaseChange, splitOnNumerics, stemEnglishPossessive, preserveOriginal bool) *WordDelimiterFilterFactory {
	return &WordDelimiterFilterFactory{
		splitOnCaseChange:     splitOnCaseChange,
		splitOnNumerics:       splitOnNumerics,
		stemEnglishPossessive: stemEnglishPossessive,
		preserveOriginal:      preserveOriginal,
	}
}

// Create creates a WordDelimiterFilter wrapping the given input.
func (f *WordDelimiterFilterFactory) Create(input TokenStream) TokenFilter {
	return NewWordDelimiterFilter(input, f.splitOnCaseChange, f.splitOnNumerics, f.stemEnglishPossessive, f.preserveOriginal)
}

// Ensure WordDelimiterFilterFactory implements TokenFilterFactory
var _ TokenFilterFactory = (*WordDelimiterFilterFactory)(nil)
