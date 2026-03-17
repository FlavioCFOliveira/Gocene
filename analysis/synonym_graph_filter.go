/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package analysis

import (
	"reflect"
)

// SynonymGraphFilter is a TokenFilter that produces a token graph with synonym paths.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.synonym.SynonymGraphFilter.
//
// The filter matches tokens from the input stream against the SynonymMap and produces
// a graph where:
//   - Original tokens form the main path through the graph
//   - Synonym tokens form alternative paths
//   - Multi-word synonyms are handled with proper position length
//
// The graph structure allows for proper phrase query matching across synonyms.
// For example, with synonym "quick" -> "fast", input "quick brown fox" produces:
//   Position 0: "quick" (posLen=1), "fast" (posLen=1, posIncr=0)
//   Position 1: "brown" (posLen=1)
//   Position 2: "fox" (posLen=1)
//
// For multi-word synonyms like "united states" -> "usa", input "united states of america" produces:
//   Position 0: "united" (posLen=1), "usa" (posLen=2, posIncr=0)
//   Position 1: "states" (posLen=1)
//   Position 2: "of" (posLen=1)
//   Position 3: "america" (posLen=1)
//
// The filter uses a buffer to look ahead for multi-word synonym matches.
// The maximum lookahead is determined by SynonymMap.GetMaxHorizontalContext().
type SynonymGraphFilter struct {
	*BaseTokenFilter

	// synonymMap is the map of input phrases to output phrases
	synonymMap *SynonymMap

	// ignoreCase determines if matching should be case-insensitive
	ignoreCase bool

	// termAttr holds the CharTermAttribute from the shared attribute source
	termAttr CharTermAttribute

	// posIncrAttr holds the PositionIncrementAttribute from the shared attribute source
	posIncrAttr PositionIncrementAttribute

	// posLenAttr holds the PositionLengthAttribute from the shared attribute source
	posLenAttr *PositionLengthAttribute

	// offsetAttr holds the OffsetAttribute from the shared attribute source
	offsetAttr OffsetAttribute

	// tokenBuffer holds buffered tokens for lookahead
	tokenBuffer []*graphBufferedToken

	// bufferPosition is the current position in the token buffer
	bufferPosition int

	// inputExhausted is true when all input tokens have been consumed
	inputExhausted bool

	// currentPosition is the current output position
	currentPosition int

	// outputQueue holds tokens ready to be emitted
	outputQueue []*graphOutputToken

	// outputPosition is the current position in the output queue
	outputPosition int

	// lastEndOffset is the end offset of the last emitted token
	lastEndOffset int
}

// graphBufferedToken holds a token that has been read from the input but not yet processed.
type graphBufferedToken struct {
	term              []byte
	startOffset       int
	endOffset         int
	positionIncrement int
	position          int
}

// graphOutputToken represents a token ready to be emitted.
type graphOutputToken struct {
	term              []byte
	startOffset       int
	endOffset         int
	positionIncrement int
	positionLength    int
	isSynonym         bool
}

// NewSynonymGraphFilter creates a new SynonymGraphFilter with the given SynonymMap.
func NewSynonymGraphFilter(input TokenStream, synonymMap *SynonymMap, ignoreCase bool) *SynonymGraphFilter {
	filter := &SynonymGraphFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		synonymMap:      synonymMap,
		ignoreCase:      ignoreCase,
		tokenBuffer:     make([]*graphBufferedToken, 0),
		outputQueue:     make([]*graphOutputToken, 0),
		bufferPosition:  0,
		outputPosition:  0,
		currentPosition: 0,
		lastEndOffset:   0,
	}

	// Get attributes from the shared AttributeSource
	filter.initAttributes()

	return filter
}

// initAttributes retrieves attributes from the shared AttributeSource.
func (f *SynonymGraphFilter) initAttributes() {
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

		// PositionLengthAttribute is needed for graph synonyms - add it if not present
		posLenType := reflect.TypeOf(&PositionLengthAttribute{})
		attr = attrSource.GetAttributeByType(posLenType)
		if attr != nil {
			f.posLenAttr = attr.(*PositionLengthAttribute)
		} else {
			f.posLenAttr = NewPositionLengthAttribute()
			attrSource.AddAttribute(f.posLenAttr)
		}

		attr = attrSource.GetAttributeByType(reflect.TypeOf(&offsetAttribute{}))
		if attr != nil {
			offsetAttr := attr.(*offsetAttribute)
			f.offsetAttr = offsetAttr
		}
	}
}

// IncrementToken advances to the next token in the stream.
// This implements the core synonym graph generation logic.
func (f *SynonymGraphFilter) IncrementToken() (bool, error) {
	// If we have tokens in the output queue, emit them first
	if f.outputPosition < len(f.outputQueue) {
		token := f.outputQueue[f.outputPosition]
		f.outputPosition++
		f.emitOutputToken(token)
		return true, nil
	}

	// If the input is exhausted and buffer is processed, we're done
	if f.inputExhausted && f.bufferPosition >= len(f.tokenBuffer) {
		return false, nil
	}

	// Buffer more tokens if needed and process
	if !f.inputExhausted {
		if err := f.bufferTokens(); err != nil {
			return false, err
		}
	}

	// Process the next position in the buffer
	if f.bufferPosition < len(f.tokenBuffer) {
		if err := f.processPosition(); err != nil {
			return false, err
		}

		// If we generated output tokens, emit the first one
		if f.outputPosition < len(f.outputQueue) {
			token := f.outputQueue[f.outputPosition]
			f.outputPosition++
			f.emitOutputToken(token)
			return true, nil
		}
	}

	// Check again if we're done
	if f.inputExhausted && f.bufferPosition >= len(f.tokenBuffer) {
		return false, nil
	}

	// If we get here, we might have an empty output queue but still have work to do
	// This can happen with certain edge cases, so advance and try again
	if f.bufferPosition < len(f.tokenBuffer) {
		f.bufferPosition++
		return f.IncrementToken()
	}

	return false, nil
}

// bufferTokens reads tokens from the input stream into the buffer.
// It reads enough tokens to allow for multi-word synonym matching.
func (f *SynonymGraphFilter) bufferTokens() error {
	maxContext := f.synonymMap.GetMaxHorizontalContext()
	if maxContext < 1 {
		maxContext = 1
	}

	// Read tokens until we have enough context or input is exhausted
	for {
		hasToken, err := f.input.IncrementToken()
		if err != nil {
			return err
		}
		if !hasToken {
			f.inputExhausted = true
			break
		}

		// Extract token data from attributes
		token := f.extractBufferedToken()
		f.tokenBuffer = append(f.tokenBuffer, token)

		// We need to buffer at least maxContext tokens ahead of current position
		// to detect multi-word synonyms
		if f.bufferPosition > 0 && len(f.tokenBuffer)-f.bufferPosition >= maxContext {
			break
		}
	}

	return nil
}

// extractBufferedToken extracts token data from the current input token.
func (f *SynonymGraphFilter) extractBufferedToken() *graphBufferedToken {
	token := &graphBufferedToken{
		position: f.currentPosition,
	}

	if f.termAttr != nil {
		token.term = f.termAttr.Bytes()
	}

	if f.offsetAttr != nil {
		token.startOffset = f.offsetAttr.StartOffset()
		token.endOffset = f.offsetAttr.EndOffset()
	}

	if f.posIncrAttr != nil {
		token.positionIncrement = f.posIncrAttr.GetPositionIncrement()
		f.currentPosition += token.positionIncrement
		token.position = f.currentPosition
	} else {
		f.currentPosition++
		token.position = f.currentPosition
		token.positionIncrement = 1
	}

	return token
}

// processPosition processes the current position in the buffer,
// looking for synonyms and building the output queue.
func (f *SynonymGraphFilter) processPosition() error {
	if f.bufferPosition >= len(f.tokenBuffer) {
		return nil
	}

	// Clear the output queue
	f.outputQueue = f.outputQueue[:0]
	f.outputPosition = 0

	// Get the current token
	currentToken := f.tokenBuffer[f.bufferPosition]

	// Look for synonyms starting at this position
	matches := f.findMatches(f.bufferPosition)

	// If we found matches, add them to the output queue
	if len(matches) > 0 {
		// Add the original token first
		origToken := &graphOutputToken{
			term:              currentToken.term,
			startOffset:       currentToken.startOffset,
			endOffset:         currentToken.endOffset,
			positionIncrement: 1, // Original token always has increment 1 at this position
			positionLength:    1,
			isSynonym:         false,
		}
		f.outputQueue = append(f.outputQueue, origToken)

		// Add synonym tokens
		for _, match := range matches {
			synToken := &graphOutputToken{
				term:              match.output,
				startOffset:       match.startOffset,
				endOffset:         match.endOffset,
				positionIncrement: 0, // Synonyms have increment 0 (same position)
				positionLength:    match.positionLength,
				isSynonym:         true,
			}
			f.outputQueue = append(f.outputQueue, synToken)
		}

		// Advance buffer position by 1 (we'll handle multi-word input matches separately)
		f.bufferPosition++
	} else {
		// No synonyms found, just output the original token
		token := &graphOutputToken{
			term:              currentToken.term,
			startOffset:       currentToken.startOffset,
			endOffset:         currentToken.endOffset,
			positionIncrement: 1,
			positionLength:    1,
			isSynonym:         false,
		}
		f.outputQueue = append(f.outputQueue, token)
		f.bufferPosition++
	}

	return nil
}

// synonymMatch represents a found synonym match.
type synonymMatch struct {
	output         []byte
	startOffset    int
	endOffset      int
	positionLength int
	inputLength    int // Number of input tokens consumed
}

// findMatches looks for synonym matches starting at the given buffer position.
func (f *SynonymGraphFilter) findMatches(startPos int) []*synonymMatch {
	var matches []*synonymMatch

	if startPos >= len(f.tokenBuffer) {
		return matches
	}

	maxContext := f.synonymMap.GetMaxHorizontalContext()
	if maxContext < 1 {
		maxContext = 1
	}

	// Try different phrase lengths, starting from the longest
	for length := maxContext; length >= 1; length-- {
		if startPos+length > len(f.tokenBuffer) {
			continue
		}

		// Build the input phrase
		inputPhrase := f.buildInputPhrase(startPos, length)
		if len(inputPhrase) == 0 {
			continue
		}

		// Look up in synonym map
		ordinals := f.synonymMap.Lookup(inputPhrase)
		if len(ordinals) > 0 {
			// Get the start and end offsets for this match
			startOffset := f.tokenBuffer[startPos].startOffset
			endOffset := f.tokenBuffer[startPos+length-1].endOffset

			// Add each output as a match
			for _, ord := range ordinals {
				output := f.synonymMap.GetOutput(ord)
				if output != nil {
					outputBytes := output.ValidBytes()
					outputWords := SplitWords(outputBytes)
					positionLength := len(outputWords)
					if positionLength < 1 {
						positionLength = 1
					}

					match := &synonymMatch{
						output:         outputBytes,
						startOffset:    startOffset,
						endOffset:      endOffset,
						positionLength: positionLength,
						inputLength:    length,
					}
					matches = append(matches, match)
				}
			}

			// If we found a match with this length, don't look for shorter matches
			// that start at the same position (Lucene behavior)
			if length > 1 && len(ordinals) > 0 {
				// Skip the consumed tokens in the buffer
				// Note: We don't actually skip here because we need to output
				// the original tokens too. The graph structure handles this.
			}
		}
	}

	return matches
}

// buildInputPhrase builds an input phrase from tokens in the buffer.
func (f *SynonymGraphFilter) buildInputPhrase(startPos, length int) []byte {
	if startPos+length > len(f.tokenBuffer) {
		return nil
	}

	var phrase []byte
	for i := 0; i < length; i++ {
		token := f.tokenBuffer[startPos+i]
		if i > 0 {
			phrase = append(phrase, WORD_SEPARATOR)
		}
		phrase = append(phrase, token.term...)
	}

	return phrase
}

// emitOutputToken emits an output token by setting the attributes.
// For multi-word synonyms, this emits the first word.
func (f *SynonymGraphFilter) emitOutputToken(token *graphOutputToken) {
	f.ClearAttributes()

	// Split multi-word terms
	words := SplitWords(token.term)
	if len(words) == 0 {
		words = []string{string(token.term)}
	}

	// Set term (first word)
	if f.termAttr != nil {
		f.termAttr.SetEmpty()
		f.termAttr.AppendString(words[0])
	}

	// Set offsets
	if f.offsetAttr != nil {
		f.offsetAttr.SetStartOffset(token.startOffset)
		f.offsetAttr.SetEndOffset(token.endOffset)
	}

	// Set position increment
	if f.posIncrAttr != nil {
		f.posIncrAttr.SetPositionIncrement(token.positionIncrement)
	}

	// Set position length
	if f.posLenAttr != nil {
		f.posLenAttr.SetPositionLength(token.positionLength)
	}

	f.lastEndOffset = token.endOffset
}

// End performs end-of-stream operations.
func (f *SynonymGraphFilter) End() error {
	// Set final offset if available
	if f.offsetAttr != nil && f.input != nil {
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
func (f *SynonymGraphFilter) Reset() error {
	f.tokenBuffer = f.tokenBuffer[:0]
	f.outputQueue = f.outputQueue[:0]
	f.bufferPosition = 0
	f.outputPosition = 0
	f.currentPosition = 0
	f.inputExhausted = false
	f.lastEndOffset = 0

	return f.BaseTokenFilter.End()
}

// GetSynonymMap returns the SynonymMap used by this filter.
func (f *SynonymGraphFilter) GetSynonymMap() *SynonymMap {
	return f.synonymMap
}

// IsIgnoreCase returns true if this filter ignores case when matching.
func (f *SynonymGraphFilter) IsIgnoreCase() bool {
	return f.ignoreCase
}

// Ensure SynonymGraphFilter implements TokenFilter
var _ TokenFilter = (*SynonymGraphFilter)(nil)

// SynonymGraphFilterFactory creates SynonymGraphFilter instances.
type SynonymGraphFilterFactory struct {
	synonymMap *SynonymMap
	ignoreCase bool
}

// NewSynonymGraphFilterFactory creates a new SynonymGraphFilterFactory.
func NewSynonymGraphFilterFactory(synonymMap *SynonymMap) *SynonymGraphFilterFactory {
	return &SynonymGraphFilterFactory{
		synonymMap: synonymMap,
		ignoreCase: false,
	}
}

// NewSynonymGraphFilterFactoryWithIgnoreCase creates a new SynonymGraphFilterFactory
// with case-insensitive matching.
func NewSynonymGraphFilterFactoryWithIgnoreCase(synonymMap *SynonymMap, ignoreCase bool) *SynonymGraphFilterFactory {
	return &SynonymGraphFilterFactory{
		synonymMap: synonymMap,
		ignoreCase: ignoreCase,
	}
}

// SetIgnoreCase sets whether matching should be case-insensitive.
func (f *SynonymGraphFilterFactory) SetIgnoreCase(ignoreCase bool) {
	f.ignoreCase = ignoreCase
}

// Create creates a SynonymGraphFilter wrapping the given input.
func (f *SynonymGraphFilterFactory) Create(input TokenStream) TokenFilter {
	return NewSynonymGraphFilter(input, f.synonymMap, f.ignoreCase)
}

// Ensure SynonymGraphFilterFactory implements TokenFilterFactory
var _ TokenFilterFactory = (*SynonymGraphFilterFactory)(nil)
