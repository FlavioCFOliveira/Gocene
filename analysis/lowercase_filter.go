// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"strings"
	"unicode"
)

// LowerCaseFilter converts tokens to lowercase using Unicode case folding.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.LowerCaseFilter.
//
// This filter converts the text of each token to lowercase using
// unicode.ToLower, which handles full Unicode case folding.
type LowerCaseFilter struct {
	*BaseTokenFilter

	// termAttr holds the CharTermAttribute from the shared attribute source
	termAttr CharTermAttribute
}

// NewLowerCaseFilter creates a new LowerCaseFilter wrapping the given input.
func NewLowerCaseFilter(input TokenStream) *LowerCaseFilter {
	filter := &LowerCaseFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
	}

	// Get the CharTermAttribute from the shared AttributeSource
	// Note: We need to use the correct type - the tokenizer stores *charTermAttribute
	attrSrc := filter.GetAttributeSource()
	if attrSrc != nil {
		attr := attrSrc.GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
		if attr != nil {
			filter.termAttr = attr.(CharTermAttribute)
		}
	}

	return filter
}

// IncrementToken advances to the next token and converts it to lowercase.
func (f *LowerCaseFilter) IncrementToken() (bool, error) {
	hasToken, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}
	if !hasToken {
		return false, nil
	}

	// Convert the token to lowercase
	if f.termAttr != nil {
		text := f.termAttr.String()
		lowerText := strings.Map(func(r rune) rune {
			return unicode.ToLower(r)
		}, text)
		f.termAttr.SetValue(lowerText)
	}

	return true, nil
}

// Ensure LowerCaseFilter implements TokenFilter
var _ TokenFilter = (*LowerCaseFilter)(nil)