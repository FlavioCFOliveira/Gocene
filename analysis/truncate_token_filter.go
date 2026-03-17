// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
)

// TruncateTokenFilter truncates tokens to a maximum length.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.TruncateTokenFilter.
//
// Tokens longer than maxLength are truncated to maxLength characters.
type TruncateTokenFilter struct {
	*BaseTokenFilter

	// maxLength is the maximum token length
	maxLength int

	// termAttr holds the CharTermAttribute from the shared attribute source
	termAttr CharTermAttribute
}

// NewTruncateTokenFilter creates a new TruncateTokenFilter wrapping the given input.
// Tokens longer than maxLength will be truncated.
func NewTruncateTokenFilter(input TokenStream, maxLength int) *TruncateTokenFilter {
	filter := &TruncateTokenFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
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

// IncrementToken advances to the next token and truncates if necessary.
func (f *TruncateTokenFilter) IncrementToken() (bool, error) {
	hasToken, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}
	if !hasToken {
		return false, nil
	}

	// Truncate the token if necessary
	if f.termAttr != nil {
		text := f.termAttr.String()
		if len(text) > f.maxLength {
			f.termAttr.SetValue(text[:f.maxLength])
		}
	}

	return true, nil
}

// GetMaxLength returns the maximum token length.
func (f *TruncateTokenFilter) GetMaxLength() int {
	return f.maxLength
}

// Ensure TruncateTokenFilter implements TokenFilter
var _ TokenFilter = (*TruncateTokenFilter)(nil)

// TruncateTokenFilterFactory creates TruncateTokenFilter instances.
type TruncateTokenFilterFactory struct {
	maxLength int
}

// NewTruncateTokenFilterFactory creates a new TruncateTokenFilterFactory.
func NewTruncateTokenFilterFactory(maxLength int) *TruncateTokenFilterFactory {
	return &TruncateTokenFilterFactory{
		maxLength: maxLength,
	}
}

// Create creates a TruncateTokenFilter wrapping the given input.
func (f *TruncateTokenFilterFactory) Create(input TokenStream) TokenFilter {
	return NewTruncateTokenFilter(input, f.maxLength)
}

// Ensure TruncateTokenFilterFactory implements TokenFilterFactory
var _ TokenFilterFactory = (*TruncateTokenFilterFactory)(nil)
