// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

// LimitTokenOffsetFilter limits tokens based on their start offset.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.LimitTokenOffsetFilter.
//
// The filter stops processing tokens when a token's start offset exceeds the configured
// maximum value. This is useful for limiting indexed text size without breaking tokens.
type LimitTokenOffsetFilter struct {
	*BaseTokenFilter

	// maxStartOffset is the maximum start offset allowed
	maxStartOffset int

	// exceeded is true when the offset limit has been exceeded
	exceeded bool
}

// NewLimitTokenOffsetFilter creates a new LimitTokenOffsetFilter wrapping the given input.
// Tokens with start offset > maxStartOffset will be filtered out.
func NewLimitTokenOffsetFilter(input TokenStream, maxStartOffset int) *LimitTokenOffsetFilter {
	filter := &LimitTokenOffsetFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		maxStartOffset:  maxStartOffset,
		exceeded:        false,
	}
	return filter
}

// IncrementToken advances to the next token, filtering by start offset.
func (f *LimitTokenOffsetFilter) IncrementToken() (bool, error) {
	if f.exceeded {
		return false, nil
	}

	for {
		hasToken, err := f.input.IncrementToken()
		if err != nil {
			return false, err
		}
		if !hasToken {
			return false, nil
		}

		// Check if this token's start offset exceeds the limit
		attrSrc := f.GetAttributeSource()
		if attrSrc != nil {
			if attr := attrSrc.GetAttribute("OffsetAttribute"); attr != nil {
				if offsetAttr, ok := attr.(OffsetAttribute); ok {
					if offsetAttr.StartOffset() > f.maxStartOffset {
						f.exceeded = true
						return false, nil
					}
				}
			}
		}

		return true, nil
	}
}

// GetMaxStartOffset returns the maximum start offset allowed.
func (f *LimitTokenOffsetFilter) GetMaxStartOffset() int {
	return f.maxStartOffset
}

// Ensure LimitTokenOffsetFilter implements TokenFilter
var _ TokenFilter = (*LimitTokenOffsetFilter)(nil)

// LimitTokenOffsetFilterFactory creates LimitTokenOffsetFilter instances.
type LimitTokenOffsetFilterFactory struct {
	maxStartOffset int
}

// NewLimitTokenOffsetFilterFactory creates a new LimitTokenOffsetFilterFactory.
func NewLimitTokenOffsetFilterFactory(maxStartOffset int) *LimitTokenOffsetFilterFactory {
	return &LimitTokenOffsetFilterFactory{
		maxStartOffset: maxStartOffset,
	}
}

// Create creates a LimitTokenOffsetFilter wrapping the given input.
func (f *LimitTokenOffsetFilterFactory) Create(input TokenStream) TokenFilter {
	return NewLimitTokenOffsetFilter(input, f.maxStartOffset)
}

// Ensure LimitTokenOffsetFilterFactory implements TokenFilterFactory
var _ TokenFilterFactory = (*LimitTokenOffsetFilterFactory)(nil)
