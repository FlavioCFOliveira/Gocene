// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"github.com/FlavioCFOliveira/Gocene/analysis/tokenattributes"
)
	

// CachingTokenFilter caches all tokens from the input TokenStream.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.CachingTokenFilter.
//
// CachingTokenFilter can be used to cache tokens from a TokenStream
// and allow multiple iterations over the tokens. This is useful when
// you need to process the same token stream multiple times.
type CachingTokenFilter struct {
	*BaseTokenFilter

	// cachedTokens stores all tokens from the input stream
	cachedTokens []cachedToken

	// currentPos is the current position in the cached tokens
	currentPos int

	// finished is true when all tokens have been cached
	finished bool

	// finalOffset and finalPositionIncrement hold the attribute state captured
	// from the input stream's end() call while the cache was being filled.
	// Subsequent end() calls on this filter restore these values rather than
	// re-invoking the input, mirroring org.apache.lucene.analysis.CachingTokenFilter.end().
	finalOffset             int
	finalPositionIncrement  int
	hasFinalState             bool
}

// cachedToken represents a single cached token with all its attributes.
type cachedToken struct {
	term              string
	startOffset       int
	endOffset         int
	positionIncrement int
}

// NewCachingTokenFilter creates a new CachingTokenFilter wrapping the given input.
func NewCachingTokenFilter(input TokenStream) *CachingTokenFilter {
	return &CachingTokenFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		cachedTokens:    make([]cachedToken, 0),
		currentPos:      0,
		finished:        false,
	}
}

// IncrementToken processes the next token.
// On the first pass, caches all tokens from the input and emits them live.
// After Reset(), returns cached tokens (without re-reading the input).
func (f *CachingTokenFilter) IncrementToken() (bool, error) {
	// First pass: read from input and cache, returning live tokens.
	if !f.finished {
		hasToken, err := f.input.IncrementToken()
		if err != nil {
			return false, err
		}

		if hasToken {
			// Cache the current token's attributes
			token := cachedToken{}

			if attr := f.GetAttributeSource().GetAttribute(CharTermAttributeType); attr != nil {
				if termAttr, ok := attr.(CharTermAttribute); ok {
					token.term = termAttr.String()
				}
			}

			if attr := f.GetAttributeSource().GetAttribute(OffsetAttributeType); attr != nil {
				if offsetAttr, ok := attr.(OffsetAttribute); ok {
					token.startOffset = offsetAttr.StartOffset()
					token.endOffset = offsetAttr.EndOffset()
				}
			}

			if attr := f.GetAttributeSource().GetAttribute(tokenattributes.PositionIncrementAttributeType); attr != nil {
				if posAttr, ok := attr.(tokenattributes.PositionIncrementAttribute); ok {
					token.positionIncrement = posAttr.GetPositionIncrement()
				}
			}

			f.cachedTokens = append(f.cachedTokens, token)
			return true, nil
		}

		// Input exhausted on the first pass. Capture the input stream's final
		// attribute state by invoking input.end() exactly once, then allow
		// callers to invoke Reset() before replaying cached tokens.
		if err := f.input.End(); err != nil {
			return false, err
		}
		f.captureFinalState()
		f.finished = true
		return false, nil
	}

	// Replay cached tokens (after Reset()).
	if f.currentPos < len(f.cachedTokens) {
		token := f.cachedTokens[f.currentPos]
		f.currentPos++

		// Restore token attributes
		if attr := f.GetAttributeSource().GetAttribute(CharTermAttributeType); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				termAttr.SetEmpty()
				termAttr.AppendString(token.term)
			}
		}

		if attr := f.GetAttributeSource().GetAttribute(OffsetAttributeType); attr != nil {
			if offsetAttr, ok := attr.(OffsetAttribute); ok {
				offsetAttr.SetStartOffset(token.startOffset)
				offsetAttr.SetEndOffset(token.endOffset)
			}
		}

		if attr := f.GetAttributeSource().GetAttribute(tokenattributes.PositionIncrementAttributeType); attr != nil {
			if posAttr, ok := attr.(tokenattributes.PositionIncrementAttribute); ok {
				posAttr.SetPositionIncrement(token.positionIncrement)
			}
		}

		return true, nil
	}

	return false, nil
}

// captureFinalState records the offset and position-increment attributes after
// the input stream has reached end-of-stream. This mirrors Lucene's capture of
// finalState during CachingTokenFilter.fillCache().
func (f *CachingTokenFilter) captureFinalState() {
	f.finalOffset = 0
	f.finalPositionIncrement = 0
	if attr := f.GetAttributeSource().GetAttribute(OffsetAttributeType); attr != nil {
		if offsetAttr, ok := attr.(OffsetAttribute); ok {
			f.finalOffset = offsetAttr.EndOffset()
		}
	}
	if attr := f.GetAttributeSource().GetAttribute(tokenattributes.PositionIncrementAttributeType); attr != nil {
		if posAttr, ok := attr.(tokenattributes.PositionIncrementAttribute); ok {
			f.finalPositionIncrement = posAttr.GetPositionIncrement()
		}
	}
	f.hasFinalState = true
}

// End performs end-of-stream operations.
//
// The first time the input reaches end-of-stream, the final attribute state is
// captured inside IncrementToken (via input.end()). Subsequent end() calls on
// this filter restore that captured state and do NOT forward to the input,
// avoiding illegal double-end state on the wrapped tokenizer. This matches
// org.apache.lucene.analysis.CachingTokenFilter.end().
func (f *CachingTokenFilter) End() error {
	if !f.hasFinalState {
		// No cached final state yet (e.g. end() called before any incrementToken
		// loop). Fall back to the standard TokenFilter behaviour.
		return f.BaseTokenFilter.End()
	}
	if attr := f.GetAttributeSource().GetAttribute(OffsetAttributeType); attr != nil {
		if offsetAttr, ok := attr.(OffsetAttribute); ok {
			offsetAttr.SetOffset(f.finalOffset, f.finalOffset)
		}
	}
	if attr := f.GetAttributeSource().GetAttribute(tokenattributes.PositionIncrementAttributeType); attr != nil {
		if posAttr, ok := attr.(tokenattributes.PositionIncrementAttribute); ok {
			posAttr.SetPositionIncrement(f.finalPositionIncrement)
		}
	}
	return nil
}

// Reset resets the filter to allow re-iteration over cached tokens.
func (f *CachingTokenFilter) Reset() error {
	f.currentPos = 0
	return nil
}

// IsCached returns true if all tokens have been cached.
func (f *CachingTokenFilter) IsCached() bool {
	return f.finished
}

// GetCacheSize returns the number of cached tokens.
func (f *CachingTokenFilter) GetCacheSize() int {
	return len(f.cachedTokens)
}
