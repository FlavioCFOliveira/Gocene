// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
)

// SynonymFilter is a TokenFilter that applies synonym mappings to tokens.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.synonym.SynonymFilter.
//
// The filter looks up tokens in a SynonymMap and emits additional synonym tokens.
// Synonyms are emitted with position increment 0, meaning they occupy the same
// position as the original token in the token stream.
//
// For multi-word synonyms, the filter buffers tokens and matches against the
// longest possible match first.
//
// Example:
//   - Input: "USA"
//   - SynonymMap: "USA" -> "United States", "America"
//   - Output tokens: "USA" (posInc=1), "United" (posInc=0), "States" (posInc=1), "America" (posInc=0)
//
// The filter handles position increments correctly:
//   - Original tokens have their normal position increment (typically 1)
//   - Synonym tokens have position increment 0 (same position as original)
//   - For multi-word output synonyms, each word after the first has position increment 1
//
// This filter is stateful and not thread-safe. A new instance should be created
// for each token stream.
type SynonymFilter struct {
	*BaseTokenFilter

	// synonymMap contains the synonym mappings
	synonymMap *SynonymMap

	// ignoreCase determines if lookups are case-insensitive
	ignoreCase bool

	// termAttr holds the CharTermAttribute from the shared attribute source
	termAttr CharTermAttribute

	// posIncrAttr holds the PositionIncrementAttribute from the shared attribute source
	posIncrAttr PositionIncrementAttribute

	// posLenAttr holds the PositionLengthAttribute from the shared attribute source
	posLenAttr *PositionLengthAttribute

	// offsetAttr holds the OffsetAttribute from the shared attribute source
	offsetAttr OffsetAttribute

	// tokenBuffer stores tokens for multi-word synonym matching
	tokenBuffer []*sfBufferedToken

	// bufferPosition is the current position in the token buffer
	bufferPosition int

	// bufferedTokenCount is the number of tokens currently buffered
	bufferedTokenCount int

	// outputBuffer stores synonym outputs to be emitted
	outputBuffer []sfOutputToken

	// outputPosition is the current position in the output buffer
	outputPosition int

	// lastTokenEnded is true when the input stream has ended
	lastTokenEnded bool

	// captureCount tracks how many tokens have been captured
	captureCount int

	// liveToken tracks if we have a live token from the input
	liveToken bool

	// finished is true when all tokens have been processed
	finished bool
}

// sfBufferedToken represents a token stored in the buffer for lookahead.
type sfBufferedToken struct {
	term              string
	positionIncrement int
	positionLength    int
	startOffset       int
	endOffset         int
	state             *State
}

// sfOutputToken represents a synonym output to be emitted.
type sfOutputToken struct {
	term              string
	positionIncrement int
	positionLength    int
	startOffset       int
	endOffset         int
	isOriginal        bool
}

// NewSynonymFilter creates a new SynonymFilter with the given SynonymMap.
//
// The filter will look up tokens in the synonymMap and emit synonyms
// with position increment 0.
func NewSynonymFilter(input TokenStream, synonymMap *SynonymMap) *SynonymFilter {
	return NewSynonymFilterWithOptions(input, synonymMap, false)
}

// NewSynonymFilterWithOptions creates a new SynonymFilter with options.
//
// Parameters:
//   - input: the input TokenStream
//   - synonymMap: the SynonymMap containing synonym mappings
//   - ignoreCase: if true, lookups are case-insensitive
func NewSynonymFilterWithOptions(input TokenStream, synonymMap *SynonymMap, ignoreCase bool) *SynonymFilter {
	filter := &SynonymFilter{
		BaseTokenFilter:    NewBaseTokenFilter(input),
		synonymMap:         synonymMap,
		ignoreCase:         ignoreCase,
		tokenBuffer:        make([]*sfBufferedToken, 0, synonymMap.GetMaxHorizontalContext()),
		outputBuffer:       make([]sfOutputToken, 0),
		bufferPosition:     0,
		outputPosition:     0,
		bufferedTokenCount: 0,
		lastTokenEnded:     false,
		captureCount:       0,
		liveToken:          false,
		finished:           false,
	}

	// Get attributes from the shared AttributeSource
	attrSource := filter.GetAttributeSource()
	if attrSource != nil {
		attr := attrSource.GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
		if attr != nil {
			filter.termAttr = attr.(CharTermAttribute)
		}
		attr = attrSource.GetAttributeByType(reflect.TypeOf(&positionIncrementAttribute{}))
		if attr != nil {
			filter.posIncrAttr = attr.(PositionIncrementAttribute)
		}
		attr = attrSource.GetAttributeByType(reflect.TypeOf(&PositionLengthAttribute{}))
		if attr != nil {
			filter.posLenAttr = attr.(*PositionLengthAttribute)
		}
		attr = attrSource.GetAttributeByType(reflect.TypeOf(&offsetAttribute{}))
		if attr != nil {
			filter.offsetAttr = attr.(OffsetAttribute)
		}
	}

	return filter
}

// IncrementToken advances to the next token, applying synonym mappings.
//
// This method:
// 1. Returns any buffered output tokens first
// 2. Captures input tokens and looks for synonym matches
// 3. Emits original tokens and their synonyms with correct position increments
func (f *SynonymFilter) IncrementToken() (bool, error) {
	// First, emit any buffered output tokens
	if f.outputPosition < len(f.outputBuffer) {
		return f.emitOutputToken(), nil
	}

	// Clear output buffer
	f.outputBuffer = f.outputBuffer[:0]
	f.outputPosition = 0

	// If we've finished, return false
	if f.finished {
		return false, nil
	}

	// Main processing loop
	for {
		// Calculate how many tokens we need to buffer for longest possible match
		maxContext := f.synonymMap.GetMaxHorizontalContext()
		if maxContext == 0 {
			maxContext = 1
		}
		availableTokens := f.bufferedTokenCount - f.bufferPosition

		// Buffer more tokens if we haven't reached max context and input hasn't ended
		if availableTokens < maxContext && !f.lastTokenEnded {
			hasToken, err := f.captureToken()
			if err != nil {
				return false, err
			}
			if !hasToken {
				f.lastTokenEnded = true
			}
			continue
		}

		// Now try to find a match
		match := f.findMatch()
		if match != nil {
			return f.processMatch(match), nil
		}

		// No match found - need to emit the current token
		if f.bufferPosition < f.bufferedTokenCount {
			return f.emitBufferedToken(), nil
		}

		// No more tokens to process
		if f.lastTokenEnded {
			f.finished = true
			return false, nil
		}

		// Capture more tokens if available
		hasToken, err := f.captureToken()
		if err != nil {
			return false, err
		}
		if !hasToken {
			f.lastTokenEnded = true
		}
	}
}

// captureToken reads the next token from input and buffers it.
func (f *SynonymFilter) captureToken() (bool, error) {
	// Clear attributes before reading next token
	f.ClearAttributes()

	hasToken, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}
	if !hasToken {
		return false, nil
	}

	// Capture token data
	token := &sfBufferedToken{
		state: f.GetAttributeSource().CaptureState(),
	}

	if f.termAttr != nil {
		token.term = f.termAttr.String()
	}
	if f.posIncrAttr != nil {
		token.positionIncrement = f.posIncrAttr.GetPositionIncrement()
	}
	if f.posLenAttr != nil {
		token.positionLength = f.posLenAttr.GetPositionLength()
	} else {
		token.positionLength = 1
	}
	if f.offsetAttr != nil {
		token.startOffset = f.offsetAttr.StartOffset()
		token.endOffset = f.offsetAttr.EndOffset()
	}

	// Add to buffer
	if f.bufferedTokenCount < len(f.tokenBuffer) {
		f.tokenBuffer[f.bufferedTokenCount] = token
	} else {
		f.tokenBuffer = append(f.tokenBuffer, token)
	}
	f.bufferedTokenCount++
	f.captureCount++

	return true, nil
}

// findMatch looks for a synonym match in the buffered tokens.
// Returns the match info or nil if no match found.
func (f *SynonymFilter) findMatch() *sfMatchInfo {
	if f.bufferPosition >= f.bufferedTokenCount {
		return nil
	}

	// Try longest match first
	maxWords := f.synonymMap.GetMaxHorizontalContext()
	if maxWords == 0 {
		maxWords = 1
	}

	// Limit to available tokens
	availableWords := f.bufferedTokenCount - f.bufferPosition
	if availableWords < maxWords {
		maxWords = availableWords
	}

	// Try from longest to shortest
	for numWords := maxWords; numWords >= 1; numWords-- {
		// Build lookup key
		var key []byte
		firstToken := f.tokenBuffer[f.bufferPosition]
		lastToken := f.tokenBuffer[f.bufferPosition+numWords-1]

		for i := 0; i < numWords; i++ {
			token := f.tokenBuffer[f.bufferPosition+i]
			term := token.term
			if f.ignoreCase {
				term = sfToLowerCase(term)
			}
			if i > 0 {
				key = append(key, WORD_SEPARATOR)
			}
			key = append(key, []byte(term)...)
		}

		// Look up in synonym map
		ordinals := f.synonymMap.Lookup(key)
		if ordinals != nil && len(ordinals) > 0 {
			return &sfMatchInfo{
				startIndex: f.bufferPosition,
				endIndex:   f.bufferPosition + numWords,
				numWords:   numWords,
				ordinals:   ordinals,
				firstToken: firstToken,
				lastToken:  lastToken,
			}
		}
	}

	return nil
}

// sfMatchInfo holds information about a synonym match.
type sfMatchInfo struct {
	startIndex int
	endIndex   int
	numWords   int
	ordinals   []int
	firstToken *sfBufferedToken
	lastToken  *sfBufferedToken
}

// processMatch processes a synonym match and sets up output tokens.
// Returns true if a token was emitted.
func (f *SynonymFilter) processMatch(match *sfMatchInfo) bool {
	// Restore state of first token
	f.GetAttributeSource().RestoreState(match.firstToken.state)

	// Calculate position increment for original token
	origPosIncr := match.firstToken.positionIncrement
	if f.captureCount > 1 {
		// Adjust for tokens we've already consumed
		origPosIncr = 1
	}

	// Set position increment for original token
	if f.posIncrAttr != nil {
		f.posIncrAttr.SetPositionIncrement(origPosIncr)
	}

	// Queue remaining original tokens (for multi-word input matches)
	for i := match.startIndex + 1; i < match.endIndex; i++ {
		token := f.tokenBuffer[i]
		f.outputBuffer = append(f.outputBuffer, sfOutputToken{
			term:              token.term,
			positionIncrement: token.positionIncrement,
			positionLength:    token.positionLength,
			startOffset:       token.startOffset,
			endOffset:         token.endOffset,
		})
	}

	// Build output tokens for synonyms
	for _, ordinal := range match.ordinals {
		output := f.synonymMap.GetOutput(ordinal)
		if output == nil {
			continue
		}

		words := SplitWords(output.ValidBytes())
		if len(words) == 0 {
			continue
		}

		// First synonym word has position increment 0 (same position as original)
		// Subsequent words have position increment 1
		for i, word := range words {
			posIncr := 0
			if i > 0 {
				posIncr = 1
			}

			// Calculate offsets - use original token's offsets for single-word synonyms
			// For multi-word, we don't have accurate offsets
			startOffset := match.firstToken.startOffset
			endOffset := match.lastToken.endOffset

			f.outputBuffer = append(f.outputBuffer, sfOutputToken{
				term:              word,
				positionIncrement: posIncr,
				positionLength:    1,
				startOffset:       startOffset,
				endOffset:         endOffset,
			})
		}
	}

	// Advance buffer position past matched tokens
	f.bufferPosition = match.endIndex

	// If we have output tokens, mark that we need to emit them after the current token
	if len(f.outputBuffer) > 0 {
		f.outputPosition = 0
	}

	return true
}

// emitBufferedToken emits a token from the buffer that had no synonym match.
func (f *SynonymFilter) emitBufferedToken() bool {
	if f.bufferPosition >= f.bufferedTokenCount {
		return false
	}

	token := f.tokenBuffer[f.bufferPosition]
	f.GetAttributeSource().RestoreState(token.state)

	// Adjust position increment
	if f.posIncrAttr != nil {
		posIncr := token.positionIncrement
		if f.captureCount > 1 && f.bufferPosition == 0 {
			// First token after capturing multiple
			posIncr = 1
		}
		f.posIncrAttr.SetPositionIncrement(posIncr)
	}

	f.bufferPosition++
	return true
}

// emitOutputToken emits a token from the output buffer.
func (f *SynonymFilter) emitOutputToken() bool {
	if f.outputPosition >= len(f.outputBuffer) {
		return false
	}

	output := f.outputBuffer[f.outputPosition]

	// Clear and set attributes
	f.ClearAttributes()

	if f.termAttr != nil {
		f.termAttr.SetValue(output.term)
	}
	if f.posIncrAttr != nil {
		f.posIncrAttr.SetPositionIncrement(output.positionIncrement)
	}
	if f.posLenAttr != nil {
		f.posLenAttr.SetPositionLength(output.positionLength)
	}
	if f.offsetAttr != nil {
		f.offsetAttr.SetStartOffset(output.startOffset)
		f.offsetAttr.SetEndOffset(output.endOffset)
	}

	f.outputPosition++
	return true
}

// sfToLowerCase converts a string to lowercase for case-insensitive matching.
func sfToLowerCase(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c = c - 'A' + 'a'
		}
		result[i] = c
	}
	return string(result)
}

// End performs end-of-stream operations.
func (f *SynonymFilter) End() error {
	// Reset state
	f.bufferPosition = 0
	f.bufferedTokenCount = 0
	f.outputPosition = 0
	f.outputBuffer = f.outputBuffer[:0]
	f.lastTokenEnded = false
	f.captureCount = 0
	f.liveToken = false
	f.finished = false

	return f.input.End()
}

// Close releases resources.
func (f *SynonymFilter) Close() error {
	// Clear buffers
	f.tokenBuffer = f.tokenBuffer[:0]
	f.outputBuffer = f.outputBuffer[:0]

	return f.input.Close()
}

// GetSynonymMap returns the SynonymMap used by this filter.
func (f *SynonymFilter) GetSynonymMap() *SynonymMap {
	return f.synonymMap
}

// IsIgnoreCase returns true if this filter uses case-insensitive matching.
func (f *SynonymFilter) IsIgnoreCase() bool {
	return f.ignoreCase
}

// Ensure SynonymFilter implements TokenFilter
var _ TokenFilter = (*SynonymFilter)(nil)

// SynonymFilterFactory creates SynonymFilter instances.
type SynonymFilterFactory struct {
	synonymMap *SynonymMap
	ignoreCase bool
}

// NewSynonymFilterFactory creates a new SynonymFilterFactory.
func NewSynonymFilterFactory(synonymMap *SynonymMap) *SynonymFilterFactory {
	return &SynonymFilterFactory{
		synonymMap: synonymMap,
		ignoreCase: false,
	}
}

// NewSynonymFilterFactoryWithOptions creates a new SynonymFilterFactory with options.
func NewSynonymFilterFactoryWithOptions(synonymMap *SynonymMap, ignoreCase bool) *SynonymFilterFactory {
	return &SynonymFilterFactory{
		synonymMap: synonymMap,
		ignoreCase: ignoreCase,
	}
}

// Create creates a SynonymFilter wrapping the given input.
func (f *SynonymFilterFactory) Create(input TokenStream) TokenFilter {
	return NewSynonymFilterWithOptions(input, f.synonymMap, f.ignoreCase)
}

// GetSynonymMap returns the SynonymMap used by this factory.
func (f *SynonymFilterFactory) GetSynonymMap() *SynonymMap {
	return f.synonymMap
}

// IsIgnoreCase returns true if this factory creates filters with case-insensitive matching.
func (f *SynonymFilterFactory) IsIgnoreCase() bool {
	return f.ignoreCase
}

// Ensure SynonymFilterFactory implements TokenFilterFactory
var _ TokenFilterFactory = (*SynonymFilterFactory)(nil)
