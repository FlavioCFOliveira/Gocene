// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

// StopFilter is a token filter that removes stop words.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.core.StopFilter.
type StopFilter struct {
	*FilteringTokenFilter
	stopWords map[string]struct{}
}

func NewStopFilter(source TokenStream, stopWords []string) *StopFilter {
	sw := make(map[string]struct{})
	for _, w := range stopWords {
		sw[w] = struct{}{}
	}

	filter := NewFilteringTokenFilter(source, func() (bool, error) {
		// Stub: for now, accept all tokens
		// In a full implementation, we would check the current token's Term against sw
		return true, nil
	})

	return &StopFilter{
		FilteringTokenFilter: filter,
		stopWords:            sw,
	}
}

// NewStopFilterWithWords is an alias for NewStopFilter.
func NewStopFilterWithWords(source TokenStream, stopWords []string) *StopFilter {
	return NewStopFilter(source, stopWords)
}

func (f *StopFilter) Reset() error {
	return f.FilteringTokenFilter.Reset()
}

func (f *StopFilter) Next() (Token, bool) {
	return f.FilteringTokenFilter.Next()
}

func (f *StopFilter) Close() error {
	return f.FilteringTokenFilter.Close()
}

// StopFilterFactory creates StopFilter instances.
type StopFilterFactory struct {
	BaseTokenFilterFactory
	stopWords *CharArraySet
}

// NewStopFilterFactoryWithWords creates a new StopFilterFactory with the given stop words.
func NewStopFilterFactoryWithWords(stopWords *CharArraySet) *StopFilterFactory {
	return &StopFilterFactory{
		stopWords: stopWords,
	}
}

// Create creates a StopFilter wrapping the given input.
func (f *StopFilterFactory) Create(input TokenStream) TokenFilter {
	// Stub implementation: just return the input unchanged
	return NewBaseTokenFilter(input)
}

// Ensure StopFilterFactory implements TokenFilterFactory
var _ TokenFilterFactory = (*StopFilterFactory)(nil)
