// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
)

// LengthFilter filters tokens based on their length.
// It removes tokens that are shorter than minLength or longer than maxLength.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.LengthFilter.
//
// Note: Unlike Lucene's implementation which uses inclusive length range,
// this filter follows the same semantics: minLength <= length <= maxLength.
type LengthFilter struct {
	*BaseTokenFilter

	// minLength is the minimum token length (inclusive)
	minLength int

	// maxLength is the maximum token length (inclusive)
	maxLength int

	// termAttr holds the CharTermAttribute from the shared attribute source
	termAttr CharTermAttribute
}

// NewLengthFilter creates a new LengthFilter wrapping the given input.
// Only tokens with lengths between minLength and maxLength (inclusive) are kept.
func NewLengthFilter(input TokenStream, minLength, maxLength int) *LengthFilter {
	filter := &LengthFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		minLength:       minLength,
		maxLength:       maxLength,
	}

	// Get the CharTermAttribute from the shared AttributeSource
	attrSrc := filter.GetAttributeSource()
	if attrSrc != nil {
		attr := attrSrc.GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
		if attr != nil {
			filter.termAttr = attr.(CharTermAttribute)
		}
	}

	return filter
}

// IncrementToken advances to the next token and filters by length.
// Returns false if there are no more tokens or if the token length is outside the range.
func (f *LengthFilter) IncrementToken() (bool, error) {
	for {
		hasToken, err := f.input.IncrementToken()
		if err != nil {
			return false, err
		}
		if !hasToken {
			return false, nil
		}

		// Check token length
		if f.termAttr != nil {
			length := f.termAttr.Length()
			if length >= f.minLength && length <= f.maxLength {
				return true, nil
			}
			// Token length is outside range, skip it and continue
			continue
		}

		// If no term attribute, pass through
		return true, nil
	}
}

// GetMinLength returns the minimum token length.
func (f *LengthFilter) GetMinLength() int {
	return f.minLength
}

// GetMaxLength returns the maximum token length.
func (f *LengthFilter) GetMaxLength() int {
	return f.maxLength
}

// Ensure LengthFilter implements TokenFilter
var _ TokenFilter = (*LengthFilter)(nil)

// LengthFilterFactory creates LengthFilter instances.
type LengthFilterFactory struct {
	minLength int
	maxLength int
}

// NewLengthFilterFactory creates a new LengthFilterFactory.
func NewLengthFilterFactory(minLength, maxLength int) *LengthFilterFactory {
	return &LengthFilterFactory{
		minLength: minLength,
		maxLength: maxLength,
	}
}

// Create creates a LengthFilter wrapping the given input.
func (f *LengthFilterFactory) Create(input TokenStream) TokenFilter {
	return NewLengthFilter(input, f.minLength, f.maxLength)
}

// Ensure LengthFilterFactory implements TokenFilterFactory
var _ TokenFilterFactory = (*LengthFilterFactory)(nil)
