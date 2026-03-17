// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
)

// KeywordRepeatFilter emits tokens twice - once as keyword and once as regular token.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.KeywordRepeatFilter.
//
// This filter is useful for cases where you want to index both the original keyword
// and a modified version (e.g., stemmed). The first emission has the keyword attribute
// set to true, the second has it set to false.
type KeywordRepeatFilter struct {
	*BaseTokenFilter

	// keywordAttr holds the KeywordAttribute from the shared attribute source
	keywordAttr *KeywordAttribute

	// termAttr holds the CharTermAttribute from the shared attribute source
	termAttr CharTermAttribute

	// savedToken holds the token text for the second emission
	savedToken string

	// emitSaved indicates whether we should emit the saved token
	emitSaved bool
}

// NewKeywordRepeatFilter creates a new KeywordRepeatFilter wrapping the given input.
func NewKeywordRepeatFilter(input TokenStream) *KeywordRepeatFilter {
	filter := &KeywordRepeatFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
	}

	// Get attributes from the shared AttributeSource
	attrSrc := filter.GetAttributeSource()
	if attrSrc != nil {
		if attr := attrSrc.GetAttribute("KeywordAttribute"); attr != nil {
			filter.keywordAttr = attr.(*KeywordAttribute)
		}

		if attr := attrSrc.GetAttributeByType(reflect.TypeOf(&charTermAttribute{})); attr != nil {
			filter.termAttr = attr.(CharTermAttribute)
		}
	}

	return filter
}

// IncrementToken advances to the next token, emitting each token twice.
// First emission: keyword=true, second emission: keyword=false.
func (f *KeywordRepeatFilter) IncrementToken() (bool, error) {
	// If we have a saved token, emit it now
	if f.emitSaved {
		f.emitSaved = false
		if f.keywordAttr != nil {
			f.keywordAttr.SetKeyword(false)
		}
		if f.termAttr != nil {
			f.termAttr.SetValue(f.savedToken)
		}
		return true, nil
	}

	// Get the next token from input
	hasToken, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}
	if !hasToken {
		return false, nil
	}

	// Save the token text for the second emission
	if f.termAttr != nil {
		f.savedToken = f.termAttr.String()
	}

	// Set keyword attribute to true for first emission
	if f.keywordAttr != nil {
		f.keywordAttr.SetKeyword(true)
	}

	// Mark that we have a saved token to emit next
	f.emitSaved = true

	return true, nil
}

// Ensure KeywordRepeatFilter implements TokenFilter
var _ TokenFilter = (*KeywordRepeatFilter)(nil)

// KeywordRepeatFilterFactory creates KeywordRepeatFilter instances.
type KeywordRepeatFilterFactory struct{}

// NewKeywordRepeatFilterFactory creates a new KeywordRepeatFilterFactory.
func NewKeywordRepeatFilterFactory() *KeywordRepeatFilterFactory {
	return &KeywordRepeatFilterFactory{}
}

// Create creates a KeywordRepeatFilter wrapping the given input.
func (f *KeywordRepeatFilterFactory) Create(input TokenStream) TokenFilter {
	return NewKeywordRepeatFilter(input)
}

// Ensure KeywordRepeatFilterFactory implements TokenFilterFactory
var _ TokenFilterFactory = (*KeywordRepeatFilterFactory)(nil)
