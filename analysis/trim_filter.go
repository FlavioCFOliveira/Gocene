// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"strings"
)

// TrimFilter trims leading and trailing whitespace from tokens.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.TrimFilter.
//
// This filter uses strings.TrimSpace to remove Unicode whitespace from both ends
// of the token text.
type TrimFilter struct {
	*BaseTokenFilter

	// termAttr holds the CharTermAttribute from the shared attribute source
	termAttr CharTermAttribute
}

// NewTrimFilter creates a new TrimFilter wrapping the given input.
func NewTrimFilter(input TokenStream) *TrimFilter {
	filter := &TrimFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
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

// IncrementToken advances to the next token and trims whitespace.
func (f *TrimFilter) IncrementToken() (bool, error) {
	hasToken, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}
	if !hasToken {
		return false, nil
	}

	// Trim whitespace from the token
	if f.termAttr != nil {
		text := f.termAttr.String()
		trimmed := strings.TrimSpace(text)
		f.termAttr.SetValue(trimmed)
	}

	return true, nil
}

// Ensure TrimFilter implements TokenFilter
var _ TokenFilter = (*TrimFilter)(nil)

// TrimFilterFactory creates TrimFilter instances.
type TrimFilterFactory struct{}

// NewTrimFilterFactory creates a new TrimFilterFactory.
func NewTrimFilterFactory() *TrimFilterFactory {
	return &TrimFilterFactory{}
}

// Create creates a TrimFilter wrapping the given input.
func (f *TrimFilterFactory) Create(input TokenStream) TokenFilter {
	return NewTrimFilter(input)
}

// Ensure TrimFilterFactory implements TokenFilterFactory
var _ TokenFilterFactory = (*TrimFilterFactory)(nil)
