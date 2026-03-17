// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

// LimitTokenCountFilter limits the number of tokens passed through.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.LimitTokenCountFilter.
//
// The filter stops processing tokens after reaching the configured maximum count.
type LimitTokenCountFilter struct {
	*BaseTokenFilter

	// maxTokenCount is the maximum number of tokens to allow
	maxTokenCount int

	// consumed is true when the maximum token count has been reached
	consumed bool
}

// NewLimitTokenCountFilter creates a new LimitTokenCountFilter wrapping the given input.
// Only the first maxTokenCount tokens will be passed through.
func NewLimitTokenCountFilter(input TokenStream, maxTokenCount int) *LimitTokenCountFilter {
	filter := &LimitTokenCountFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		maxTokenCount:   maxTokenCount,
		consumed:        false,
	}
	return filter
}

// IncrementToken advances to the next token, stopping after maxTokenCount tokens.
func (f *LimitTokenCountFilter) IncrementToken() (bool, error) {
	if f.consumed {
		return false, nil
	}

	hasToken, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}
	if !hasToken {
		return false, nil
	}

	f.maxTokenCount--
	if f.maxTokenCount <= 0 {
		f.consumed = true
	}

	return true, nil
}

// GetMaxTokenCount returns the maximum number of tokens allowed.
func (f *LimitTokenCountFilter) GetMaxTokenCount() int {
	return f.maxTokenCount
}

// Ensure LimitTokenCountFilter implements TokenFilter
var _ TokenFilter = (*LimitTokenCountFilter)(nil)

// LimitTokenCountFilterFactory creates LimitTokenCountFilter instances.
type LimitTokenCountFilterFactory struct {
	maxTokenCount int
}

// NewLimitTokenCountFilterFactory creates a new LimitTokenCountFilterFactory.
func NewLimitTokenCountFilterFactory(maxTokenCount int) *LimitTokenCountFilterFactory {
	return &LimitTokenCountFilterFactory{
		maxTokenCount: maxTokenCount,
	}
}

// Create creates a LimitTokenCountFilter wrapping the given input.
func (f *LimitTokenCountFilterFactory) Create(input TokenStream) TokenFilter {
	return NewLimitTokenCountFilter(input, f.maxTokenCount)
}

// Ensure LimitTokenCountFilterFactory implements TokenFilterFactory
var _ TokenFilterFactory = (*LimitTokenCountFilterFactory)(nil)
