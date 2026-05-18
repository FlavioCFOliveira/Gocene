// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"strings"
)

// UpperCaseFilter normalises every token's text to upper case.
//
// This is the Go port of
// org.apache.lucene.analysis.core.UpperCaseFilter from Apache Lucene
// 10.4.0. Lucene notes that upper-casing can lose information for
// certain code points; the same caveat applies here.
type UpperCaseFilter struct {
	*BaseTokenFilter

	termAttr CharTermAttribute
}

// NewUpperCaseFilter wraps input.
func NewUpperCaseFilter(input TokenStream) *UpperCaseFilter {
	f := &UpperCaseFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttribute(CharTermAttributeType); a != nil {
			f.termAttr = a.(CharTermAttribute)
		}
	}
	return f
}

// IncrementToken upper-cases the current token.
func (f *UpperCaseFilter) IncrementToken() (bool, error) {
	ok, err := f.input.IncrementToken()
	if err != nil || !ok {
		return ok, err
	}
	if f.termAttr == nil {
		return true, nil
	}
	s := f.termAttr.String()
	upper := strings.ToUpper(s)
	if upper != s {
		f.termAttr.SetEmpty()
		f.termAttr.AppendString(upper)
	}
	return true, nil
}

// Ensure UpperCaseFilter implements TokenFilter.
var _ TokenFilter = (*UpperCaseFilter)(nil)

// UpperCaseFilterFactory creates UpperCaseFilter instances.
type UpperCaseFilterFactory struct{}

// NewUpperCaseFilterFactory returns a fresh factory.
func NewUpperCaseFilterFactory() *UpperCaseFilterFactory {
	return &UpperCaseFilterFactory{}
}

// Create wraps input.
func (f *UpperCaseFilterFactory) Create(input TokenStream) TokenFilter {
	return NewUpperCaseFilter(input)
}

// Ensure factory satisfies TokenFilterFactory.
var _ TokenFilterFactory = (*UpperCaseFilterFactory)(nil)
