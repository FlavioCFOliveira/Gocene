// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
)

// RemoveDuplicatesTokenFilter removes duplicate tokens at the same position.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.RemoveDuplicatesTokenFilter.
//
// This filter removes tokens that have the same text and position as a previous
// token. It is useful for removing duplicates that may be introduced by
// synonym expansion or other token expansion mechanisms.
//
// The filter tracks tokens at each position and removes subsequent duplicates.
// Position increments are preserved correctly - when a duplicate is removed,
// the position increment of the next non-duplicate token is adjusted.
//
// Example:
//
//	Input:  "A"(pos=1) "A"(pos=0) "B"(pos=1) "B"(pos=0)
//	Output: "A"(pos=1) "B"(pos=1)
//
// Note: This filter only removes duplicates at the same position. Tokens with
// the same text at different positions are preserved.
type RemoveDuplicatesTokenFilter struct {
	*BaseTokenFilter

	// termAttr holds the CharTermAttribute from the shared attribute source
	termAttr CharTermAttribute

	// posIncrAttr holds the PositionIncrementAttribute from the shared attribute source
	posIncrAttr PositionIncrementAttribute

	// seenTokens tracks tokens that have been seen at the current position
	seenTokens map[string]struct{}

	// currentPosition tracks the current position in the stream
	currentPosition int

	// savedIncrement holds the position increment to apply to the next token
	// when duplicates are skipped
	savedIncrement int

	// hasSavedIncrement indicates if we have a saved increment to apply
	hasSavedIncrement bool
}

// NewRemoveDuplicatesTokenFilter creates a new RemoveDuplicatesTokenFilter wrapping the given input.
func NewRemoveDuplicatesTokenFilter(input TokenStream) *RemoveDuplicatesTokenFilter {
	filter := &RemoveDuplicatesTokenFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		seenTokens:      make(map[string]struct{}),
		currentPosition: -1,
		savedIncrement:  0,
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
	}

	return filter
}

// IncrementToken advances to the next token, removing duplicates at the same position.
func (f *RemoveDuplicatesTokenFilter) IncrementToken() (bool, error) {
	for {
		hasToken, err := f.input.IncrementToken()
		if err != nil {
			return false, err
		}
		if !hasToken {
			return false, nil
		}

		// Get the token text
		var tokenText string
		if f.termAttr != nil {
			tokenText = f.termAttr.String()
		}

		// Get the position increment
		posIncr := 1
		if f.posIncrAttr != nil {
			posIncr = f.posIncrAttr.GetPositionIncrement()
		}

		// Calculate the actual position of this token
		if posIncr > 0 {
			// Moving to a new position - clear the seen set
			f.currentPosition += posIncr
			for key := range f.seenTokens {
				delete(f.seenTokens, key)
			}
		}

		// Check if this token is a duplicate at the current position
		if _, exists := f.seenTokens[tokenText]; exists {
			// This is a duplicate - skip it and accumulate its position increment
			if f.posIncrAttr != nil {
				f.savedIncrement += f.posIncrAttr.GetPositionIncrement()
				f.hasSavedIncrement = true
			}
			continue
		}

		// Not a duplicate - mark it as seen
		f.seenTokens[tokenText] = struct{}{}

		// Apply any saved position increment
		if f.hasSavedIncrement && f.posIncrAttr != nil {
			f.posIncrAttr.SetPositionIncrement(f.posIncrAttr.GetPositionIncrement() + f.savedIncrement)
			f.savedIncrement = 0
			f.hasSavedIncrement = false
		}

		return true, nil
	}
}

// End performs end-of-stream operations.
// Delegates to the input TokenStream and clears internal state.
func (f *RemoveDuplicatesTokenFilter) End() error {
	// Clear the seen tokens map
	for key := range f.seenTokens {
		delete(f.seenTokens, key)
	}
	f.currentPosition = -1
	f.savedIncrement = 0
	f.hasSavedIncrement = false

	return f.BaseTokenFilter.End()
}

// Ensure RemoveDuplicatesTokenFilter implements TokenFilter
var _ TokenFilter = (*RemoveDuplicatesTokenFilter)(nil)

// RemoveDuplicatesTokenFilterFactory creates RemoveDuplicatesTokenFilter instances.
type RemoveDuplicatesTokenFilterFactory struct{}

// NewRemoveDuplicatesTokenFilterFactory creates a new RemoveDuplicatesTokenFilterFactory.
func NewRemoveDuplicatesTokenFilterFactory() *RemoveDuplicatesTokenFilterFactory {
	return &RemoveDuplicatesTokenFilterFactory{}
}

// Create creates a RemoveDuplicatesTokenFilter wrapping the given input.
func (f *RemoveDuplicatesTokenFilterFactory) Create(input TokenStream) TokenFilter {
	return NewRemoveDuplicatesTokenFilter(input)
}

// Ensure RemoveDuplicatesTokenFilterFactory implements TokenFilterFactory
var _ TokenFilterFactory = (*RemoveDuplicatesTokenFilterFactory)(nil)
