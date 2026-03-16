// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

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
// On the first pass, caches all tokens from the input.
// On subsequent passes, returns cached tokens.
func (f *CachingTokenFilter) IncrementToken() (bool, error) {
	// If we have cached tokens and haven't finished caching, continue caching
	if !f.finished {
		hasToken, err := f.input.IncrementToken()
		if err != nil {
			return false, err
		}

		if hasToken {
			// Cache the current token's attributes
			token := cachedToken{}

			if attr := f.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
				if termAttr, ok := attr.(CharTermAttribute); ok {
					token.term = termAttr.String()
				}
			}

			if attr := f.GetAttributeSource().GetAttribute("OffsetAttribute"); attr != nil {
				if offsetAttr, ok := attr.(OffsetAttribute); ok {
					token.startOffset = offsetAttr.StartOffset()
					token.endOffset = offsetAttr.EndOffset()
				}
			}

			if attr := f.GetAttributeSource().GetAttribute("PositionIncrementAttribute"); attr != nil {
				if posAttr, ok := attr.(PositionIncrementAttribute); ok {
					token.positionIncrement = posAttr.GetPositionIncrement()
				}
			}

			f.cachedTokens = append(f.cachedTokens, token)
			return true, nil
		}

		// No more tokens from input
		f.finished = true
	}

	// Return cached tokens
	if f.currentPos < len(f.cachedTokens) {
		token := f.cachedTokens[f.currentPos]
		f.currentPos++

		// Restore token attributes
		if attr := f.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				termAttr.SetEmpty()
				termAttr.AppendString(token.term)
			}
		}

		if attr := f.GetAttributeSource().GetAttribute("OffsetAttribute"); attr != nil {
			if offsetAttr, ok := attr.(OffsetAttribute); ok {
				offsetAttr.SetStartOffset(token.startOffset)
				offsetAttr.SetEndOffset(token.endOffset)
			}
		}

		if attr := f.GetAttributeSource().GetAttribute("PositionIncrementAttribute"); attr != nil {
			if posAttr, ok := attr.(PositionIncrementAttribute); ok {
				posAttr.SetPositionIncrement(token.positionIncrement)
			}
		}

		return true, nil
	}

	return false, nil
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
