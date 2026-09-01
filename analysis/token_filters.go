// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

// CachingTokenFilter caches all token states locally in a slice when the first call to Next is called.
// Subsequent calls will use the cache.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.CachingTokenFilter.
type CachingTokenFilter struct {
	source TokenStream
	cache  []Token
	pos    int
}

func NewCachingTokenFilter(source TokenStream) *CachingTokenFilter {
	return &CachingTokenFilter{
		source: source,
	}
}

func (f *CachingTokenFilter) Reset() error {
	if f.cache == nil {
		return f.source.Reset()
	}
	f.pos = 0
	return nil
}

func (f *CachingTokenFilter) Next() (Token, bool) {
	if f.cache == nil {
		f.cache = make([]Token, 0, 64)
		for {
			token, ok := f.source.Next()
			if !ok {
				break
			}
			f.cache = append(f.cache, token)
		}
		f.pos = 0
	}

	if f.pos >= len(f.cache) {
		return Token{}, false
	}

	token := f.cache[f.pos]
	f.pos++
	return token, true
}

func (f *CachingTokenFilter) Close() error {
	return f.source.Close()
}

// TokenPredicate is a function that decides if a token should be preserved.
type TokenPredicate func(Token) bool

// FilteringTokenFilter is a token filter that removes tokens based on a predicate.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.FilteringTokenFilter.
type FilteringTokenFilter struct {
	source TokenStream
	accept TokenPredicate
}

func NewFilteringTokenFilter(source TokenStream, accept TokenPredicate) *FilteringTokenFilter {
	return &FilteringTokenFilter{
		source: source,
		accept: accept,
	}
}

func (f *FilteringTokenFilter) Reset() error {
	return f.source.Reset()
}

func (f *FilteringTokenFilter) Next() (Token, bool) {
	skippedPositions := 0
	for {
		token, ok := f.source.Next()
		if !ok {
			return Token{}, false
		}
		if f.accept(token) {
			if skippedPositions != 0 {
				token.PositionInc += skippedPositions
			}
			return token, true
		}
		skippedPositions += token.PositionInc
	}
}

func (f *FilteringTokenFilter) Close() error {
	return f.source.Close()
}
