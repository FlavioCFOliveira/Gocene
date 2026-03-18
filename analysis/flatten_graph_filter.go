// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"sort"
)

// FlattenGraphFilter flattens a token graph into a linear stream.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.FlattenGraphFilter.
//
// This filter is required after graph-producing filters like SynonymGraphFilter
// that produce token graphs with multiple paths. FlattenGraphFilter converts
// the graph into a linear token stream that can be consumed by filters that
// don't understand token graphs.
//
// The filter handles:
//   - Position lengths correctly (tokens spanning multiple positions)
//   - Gaps in the graph (missing positions)
//   - Multiple paths through the graph (synonyms)
//
// The algorithm works by:
//  1. Buffering all tokens from the input stream
//  2. Building a position-based graph structure
//  3. Sorting tokens by position and path depth
//  4. Emitting tokens in linear order with adjusted position increments
//
// Example:
//
//	Input graph ("wifi" synonym of "wireless network"):
//	  Position 0: "wifi" (posLen=2), "wireless" (posLen=1)
//	  Position 1: "network" (posLen=1)
//	Output stream:
//	  "wifi" (posIncr=1, posLen=2)
//	  "wireless" (posIncr=0, posLen=1)
//	  "network" (posIncr=1, posLen=1)
type FlattenGraphFilter struct {
	*BaseTokenFilter

	// termAttr holds the CharTermAttribute from the shared attribute source
	termAttr CharTermAttribute

	// posIncrAttr holds the PositionIncrementAttribute from the shared attribute source
	posIncrAttr PositionIncrementAttribute

	// posLenAttr holds the PositionLengthAttribute from the shared attribute source
	posLenAttr *PositionLengthAttribute

	// offsetAttr holds the OffsetAttribute from the shared attribute source
	offsetAttr OffsetAttribute

	// typeAttr holds the TypeAttribute from the shared attribute source
	typeAttr *TypeAttribute

	// payloadAttr holds the PayloadAttribute from the shared attribute source
	payloadAttr *PayloadAttribute

	// flagsAttr holds the FlagsAttribute from the shared attribute source
	flagsAttr *FlagsAttribute

	// keywordAttr holds the KeywordAttribute from the shared attribute source
	keywordAttr *KeywordAttribute

	// tokenData holds all buffered tokens from the input
	tokenData []*flattenTokenData

	// outputTokens holds tokens sorted for output
	outputTokens []*flattenTokenData

	// currentOutputIndex is the index of the next token to output
	currentOutputIndex int

	// inputExhausted is true when all input tokens have been consumed
	inputExhausted bool

	// endOffset is the final end offset from the input stream
	endOffset int

	// maxPos is the maximum position seen in the input
	maxPos int
}

// flattenTokenData holds the data for a single token in the flatten process.
type flattenTokenData struct {
	// term is the token text
	term string

	// startOffset is the character start offset
	startOffset int

	// endOffset is the character end offset
	endOffset int

	// position is the start position of this token
	position int

	// positionLength is how many positions this token spans
	positionLength int

	// positionIncrement is the position increment for output
	positionIncrement int

	// tokenType is the token type
	tokenType string

	// payload is the optional payload
	payload []byte

	// flags are the token flags
	flags int

	// isKeyword indicates if this is a keyword token
	isKeyword bool

	// outputOrder is the sort order for output
	outputOrder int

	// endPos is the end position (position + positionLength)
	endPos int
}

// NewFlattenGraphFilter creates a new FlattenGraphFilter wrapping the given input.
func NewFlattenGraphFilter(input TokenStream) *FlattenGraphFilter {
	filter := &FlattenGraphFilter{
		BaseTokenFilter:    NewBaseTokenFilter(input),
		tokenData:          make([]*flattenTokenData, 0),
		outputTokens:       make([]*flattenTokenData, 0),
		currentOutputIndex: 0,
		inputExhausted:     false,
		endOffset:          0,
		maxPos:             0,
	}

	filter.initAttributes()
	return filter
}

// initAttributes retrieves attributes from the shared AttributeSource.
func (f *FlattenGraphFilter) initAttributes() {
	attrSource := f.GetAttributeSource()
	if attrSource == nil {
		return
	}

	// Get CharTermAttribute
	if attr := attrSource.GetAttributeByType(reflect.TypeOf(&charTermAttribute{})); attr != nil {
		f.termAttr = attr.(CharTermAttribute)
	}

	// Get PositionIncrementAttribute
	if attr := attrSource.GetAttributeByType(reflect.TypeOf(&positionIncrementAttribute{})); attr != nil {
		f.posIncrAttr = attr.(PositionIncrementAttribute)
	}

	// Get PositionLengthAttribute
	if attr := attrSource.GetAttributeByType(reflect.TypeOf(&PositionLengthAttribute{})); attr != nil {
		f.posLenAttr = attr.(*PositionLengthAttribute)
	}

	// Get OffsetAttribute
	if attr := attrSource.GetAttributeByType(reflect.TypeOf(&offsetAttribute{})); attr != nil {
		f.offsetAttr = attr.(OffsetAttribute)
	}

	// Get TypeAttribute
	if attr := attrSource.GetAttributeByType(reflect.TypeOf(&TypeAttribute{})); attr != nil {
		f.typeAttr = attr.(*TypeAttribute)
	}

	// Get PayloadAttribute
	if attr := attrSource.GetAttributeByType(reflect.TypeOf(&PayloadAttribute{})); attr != nil {
		f.payloadAttr = attr.(*PayloadAttribute)
	}

	// Get FlagsAttribute
	if attr := attrSource.GetAttributeByType(reflect.TypeOf(&FlagsAttribute{})); attr != nil {
		f.flagsAttr = attr.(*FlagsAttribute)
	}

	// Get KeywordAttribute
	if attr := attrSource.GetAttributeByType(reflect.TypeOf(&KeywordAttribute{})); attr != nil {
		f.keywordAttr = attr.(*KeywordAttribute)
	}
}

// IncrementToken advances to the next token in the flattened stream.
func (f *FlattenGraphFilter) IncrementToken() (bool, error) {
	// First call: buffer all tokens from input
	if !f.inputExhausted {
		if err := f.bufferInput(); err != nil {
			return false, err
		}
	}

	// Check if we've output all tokens
	if f.currentOutputIndex >= len(f.outputTokens) {
		return false, nil
	}

	// Get the next token to output
	token := f.outputTokens[f.currentOutputIndex]
	f.currentOutputIndex++

	// Clear and set attributes
	f.ClearAttributes()

	// Set term
	if f.termAttr != nil {
		f.termAttr.SetValue(token.term)
	}

	// Set position increment
	if f.posIncrAttr != nil {
		f.posIncrAttr.SetPositionIncrement(token.positionIncrement)
	}

	// Set position length
	if f.posLenAttr != nil {
		f.posLenAttr.SetPositionLength(token.positionLength)
	}

	// Set offsets
	if f.offsetAttr != nil {
		f.offsetAttr.SetStartOffset(token.startOffset)
		f.offsetAttr.SetEndOffset(token.endOffset)
	}

	// Set type
	if f.typeAttr != nil {
		f.typeAttr.SetType(token.tokenType)
	}

	// Set payload
	if f.payloadAttr != nil {
		f.payloadAttr.SetPayload(token.payload)
	}

	// Set flags
	if f.flagsAttr != nil {
		f.flagsAttr.SetFlags(token.flags)
	}

	// Set keyword
	if f.keywordAttr != nil {
		f.keywordAttr.SetKeyword(token.isKeyword)
	}

	return true, nil
}

// bufferInput reads all tokens from the input stream and builds the output order.
func (f *FlattenGraphFilter) bufferInput() error {
	f.inputExhausted = true

	// Read all tokens from input
	currentPos := 0
	for {
		hasToken, err := f.input.IncrementToken()
		if err != nil {
			return err
		}
		if !hasToken {
			break
		}

		// Capture token data
		data := &flattenTokenData{
			position:       currentPos,
			positionLength: 1,
			tokenType:      "word",
		}

		// Get term
		if f.termAttr != nil {
			data.term = f.termAttr.String()
		}

		// Get position increment and update current position
		if f.posIncrAttr != nil {
			posIncr := f.posIncrAttr.GetPositionIncrement()
			if posIncr == 0 {
				// Token at same position as previous
				data.position = currentPos
			} else {
				// Token at new position
				currentPos += posIncr
				data.position = currentPos
			}
		}

		// Get position length
		if f.posLenAttr != nil {
			data.positionLength = f.posLenAttr.GetPositionLength()
		}

		// Get offsets
		if f.offsetAttr != nil {
			data.startOffset = f.offsetAttr.StartOffset()
			data.endOffset = f.offsetAttr.EndOffset()
		}

		// Get type
		if f.typeAttr != nil {
			data.tokenType = f.typeAttr.GetType()
		}

		// Get payload
		if f.payloadAttr != nil {
			data.payload = f.payloadAttr.GetPayload()
		}

		// Get flags
		if f.flagsAttr != nil {
			data.flags = f.flagsAttr.GetFlags()
		}

		// Get keyword
		if f.keywordAttr != nil {
			data.isKeyword = f.keywordAttr.IsKeywordToken()
		}

		// Calculate end position
		data.endPos = data.position + data.positionLength

		// Track max position
		if data.endPos > f.maxPos {
			f.maxPos = data.endPos
		}

		f.tokenData = append(f.tokenData, data)
	}

	// Get end offset from input
	if err := f.input.End(); err != nil {
		return err
	}

	// Get final end offset from offset attribute
	if f.offsetAttr != nil {
		f.endOffset = f.offsetAttr.EndOffset()
	}

	// Build output order
	f.buildOutputOrder()

	return nil
}

// buildOutputOrder sorts tokens for linear output.
// Tokens are sorted by:
// 1. Start position (ascending)
// 2. End position descending (longer spans first)
// 3. Original order for stability
func (f *FlattenGraphFilter) buildOutputOrder() {
	if len(f.tokenData) == 0 {
		return
	}

	// Group tokens by their start position
	posToTokens := make(map[int][]*flattenTokenData)
	for _, token := range f.tokenData {
		posToTokens[token.position] = append(posToTokens[token.position], token)
	}

	// Sort tokens at each position by end position (descending) for stability
	for pos := range posToTokens {
		sort.Slice(posToTokens[pos], func(i, j int) bool {
			// Sort by end position descending (longer spans first)
			if posToTokens[pos][i].endPos != posToTokens[pos][j].endPos {
				return posToTokens[pos][i].endPos > posToTokens[pos][j].endPos
			}
			// Stable sort: preserve original order
			return false
		})
	}

	// Build output order using a breadth-first approach
	// We need to track which positions have been "covered" by output tokens
	f.outputTokens = make([]*flattenTokenData, 0, len(f.tokenData))
	coveredEndPos := 0

	for coveredEndPos < f.maxPos {
		// Find tokens starting at or before coveredEndPos
		var candidates []*flattenTokenData
		for pos := 0; pos <= coveredEndPos; pos++ {
			for _, token := range posToTokens[pos] {
				// Only include tokens that haven't been output yet
				// and that start at or extend to coveredEndPos
				if token.outputOrder == 0 && token.position <= coveredEndPos {
					candidates = append(candidates, token)
				}
			}
		}

		if len(candidates) == 0 {
			// No candidates found, advance position
			coveredEndPos++
			continue
		}

		// Sort candidates by position (asc), then by endPos (desc)
		sort.Slice(candidates, func(i, j int) bool {
			if candidates[i].position != candidates[j].position {
				return candidates[i].position < candidates[j].position
			}
			return candidates[i].endPos > candidates[j].endPos
		})

		// Output the first candidate
		token := candidates[0]
		token.outputOrder = len(f.outputTokens) + 1

		// Calculate position increment
		if len(f.outputTokens) == 0 {
			// First token
			token.positionIncrement = token.position
		} else {
			prevToken := f.outputTokens[len(f.outputTokens)-1]
			if token.position <= prevToken.position {
				// Same position as previous token
				token.positionIncrement = 0
			} else {
				// New position
				token.positionIncrement = token.position - prevToken.position
			}
		}

		f.outputTokens = append(f.outputTokens, token)

		// Update covered end position
		if token.endPos > coveredEndPos {
			coveredEndPos = token.endPos
		}
	}

	// Handle any remaining tokens that weren't included in the main path
	// (tokens that are completely contained within other tokens' spans)
	for _, token := range f.tokenData {
		if token.outputOrder == 0 {
			token.outputOrder = len(f.outputTokens) + 1

			// Calculate position increment
			if len(f.outputTokens) == 0 {
				token.positionIncrement = token.position
			} else {
				prevToken := f.outputTokens[len(f.outputTokens)-1]
				if token.position <= prevToken.position {
					token.positionIncrement = 0
				} else {
					token.positionIncrement = token.position - prevToken.position
				}
			}

			f.outputTokens = append(f.outputTokens, token)
		}
	}
}

// End performs end-of-stream operations.
func (f *FlattenGraphFilter) End() error {
	// Ensure we've consumed all input
	if !f.inputExhausted {
		if err := f.bufferInput(); err != nil {
			return err
		}
	}

	// Clear attributes
	f.ClearAttributes()

	// Set final end offset
	if f.offsetAttr != nil {
		f.offsetAttr.SetStartOffset(f.endOffset)
		f.offsetAttr.SetEndOffset(f.endOffset)
	}

	return nil
}

// Reset resets the filter for reuse.
func (f *FlattenGraphFilter) Reset() error {
	f.tokenData = f.tokenData[:0]
	f.outputTokens = f.outputTokens[:0]
	f.currentOutputIndex = 0
	f.inputExhausted = false
	f.endOffset = 0
	f.maxPos = 0

	return f.input.Close()
}

// Ensure FlattenGraphFilter implements TokenFilter
var _ TokenFilter = (*FlattenGraphFilter)(nil)
