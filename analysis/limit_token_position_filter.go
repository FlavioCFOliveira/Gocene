// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

// LimitTokenPositionFilter limits tokens based on their cumulative position.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.LimitTokenPositionFilter.
//
// The filter stops processing tokens when a token's cumulative position exceeds
// maxTokenPosition. The first token always occupies position 1 (positionIncrement=1).
type LimitTokenPositionFilter struct {
	*BaseTokenFilter

	// maxTokenPosition is the maximum cumulative position allowed
	maxTokenPosition int

	// tokenPosition is the current cumulative position
	tokenPosition int

	// consumed is true when the position limit has been exceeded
	consumed bool
}

// NewLimitTokenPositionFilter creates a new LimitTokenPositionFilter wrapping the given input.
// Tokens with position > maxTokenPosition will be filtered out.
func NewLimitTokenPositionFilter(input TokenStream, maxTokenPosition int) *LimitTokenPositionFilter {
	filter := &LimitTokenPositionFilter{
		BaseTokenFilter:  NewBaseTokenFilter(input),
		maxTokenPosition: maxTokenPosition,
		consumed:         false,
	}
	return filter
}

// IncrementToken advances to the next token, filtering by cumulative position.
// The cumulative position is incremented by each token's position increment.
// Tokens whose cumulative position exceeds maxTokenPosition end the stream.
func (f *LimitTokenPositionFilter) IncrementToken() (bool, error) {
	if f.consumed {
		return false, nil
	}

	hasToken, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}
	if !hasToken {
		f.consumed = true
		return false, nil
	}

	// Accumulate position using the token's position increment (default=1).
	posIncrement := 1
	attrSrc := f.GetAttributeSource()
	if attrSrc != nil {
		if attr := attrSrc.GetAttribute(PositionIncrementAttributeType); attr != nil {
			if posAttr, ok := attr.(PositionIncrementAttribute); ok {
				posIncrement = posAttr.GetPositionIncrement()
			}
		}
	}
	f.tokenPosition += posIncrement

	if f.tokenPosition <= f.maxTokenPosition {
		return true, nil
	}
	f.consumed = true
	return false, nil
}

// GetMaxTokenPosition returns the maximum token position allowed.
func (f *LimitTokenPositionFilter) GetMaxTokenPosition() int {
	return f.maxTokenPosition
}

// Ensure LimitTokenPositionFilter implements TokenFilter
var _ TokenFilter = (*LimitTokenPositionFilter)(nil)

// LimitTokenPositionFilterFactory creates LimitTokenPositionFilter instances.
type LimitTokenPositionFilterFactory struct {
	maxTokenPosition int
}

// NewLimitTokenPositionFilterFactory creates a new LimitTokenPositionFilterFactory.
func NewLimitTokenPositionFilterFactory(maxTokenPosition int) *LimitTokenPositionFilterFactory {
	return &LimitTokenPositionFilterFactory{
		maxTokenPosition: maxTokenPosition,
	}
}

// Create creates a LimitTokenPositionFilter wrapping the given input.
func (f *LimitTokenPositionFilterFactory) Create(input TokenStream) TokenFilter {
	return NewLimitTokenPositionFilter(input, f.maxTokenPosition)
}

// Ensure LimitTokenPositionFilterFactory implements TokenFilterFactory
var _ TokenFilterFactory = (*LimitTokenPositionFilterFactory)(nil)
