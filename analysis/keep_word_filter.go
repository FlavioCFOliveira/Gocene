// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
)

// KeepWordFilter keeps only tokens that are in a specified word set.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.KeepWordFilter.
//
// This filter is useful for filtering tokens to only a specific vocabulary.
type KeepWordFilter struct {
	*BaseTokenFilter

	// keepWords is the set of words to keep
	keepWords map[string]bool

	// termAttr holds the CharTermAttribute from the shared attribute source
	termAttr CharTermAttribute
}

// NewKeepWordFilter creates a new KeepWordFilter wrapping the given input.
// Only tokens that are in the keepWords set are passed through.
func NewKeepWordFilter(input TokenStream, keepWords map[string]bool) *KeepWordFilter {
	filter := &KeepWordFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		keepWords:       keepWords,
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

// IncrementToken advances to the next token, keeping only words in the set.
func (f *KeepWordFilter) IncrementToken() (bool, error) {
	for {
		hasToken, err := f.input.IncrementToken()
		if err != nil {
			return false, err
		}
		if !hasToken {
			return false, nil
		}

		// Check if token is in the keep set
		if f.termAttr != nil {
			text := f.termAttr.String()
			if f.keepWords[text] {
				return true, nil
			}
			// Token not in keep set, skip it
			continue
		}

		// If no term attribute, pass through
		return true, nil
	}
}

// GetKeepWords returns the set of words to keep.
func (f *KeepWordFilter) GetKeepWords() map[string]bool {
	return f.keepWords
}

// Ensure KeepWordFilter implements TokenFilter
var _ TokenFilter = (*KeepWordFilter)(nil)

// KeepWordFilterFactory creates KeepWordFilter instances.
type KeepWordFilterFactory struct {
	keepWords map[string]bool
}

// NewKeepWordFilterFactory creates a new KeepWordFilterFactory.
func NewKeepWordFilterFactory(keepWords map[string]bool) *KeepWordFilterFactory {
	return &KeepWordFilterFactory{
		keepWords: keepWords,
	}
}

// Create creates a KeepWordFilter wrapping the given input.
func (f *KeepWordFilterFactory) Create(input TokenStream) TokenFilter {
	return NewKeepWordFilter(input, f.keepWords)
}

// Ensure KeepWordFilterFactory implements TokenFilterFactory
var _ TokenFilterFactory = (*KeepWordFilterFactory)(nil)
