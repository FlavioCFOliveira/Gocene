// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"unicode"
)

// GreekLowerCaseFilter normalises Greek text to lower case while
// stripping tonos and dialytika marks and standardising the final
// sigma U+03C2 to U+03C3.
//
// This is the Go port of
// org.apache.lucene.analysis.el.GreekLowerCaseFilter from Apache
// Lucene 10.4.0.
type GreekLowerCaseFilter struct {
	*BaseTokenFilter

	termAttr CharTermAttribute
}

// NewGreekLowerCaseFilter wraps input with the Greek lower-case
// filter.
func NewGreekLowerCaseFilter(input TokenStream) *GreekLowerCaseFilter {
	f := &GreekLowerCaseFilter{
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

// IncrementToken lower-cases the current token using greekLowerCase
// for every rune.
func (f *GreekLowerCaseFilter) IncrementToken() (bool, error) {
	ok, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	if f.termAttr == nil {
		return true, nil
	}
	s := f.termAttr.String()
	out := make([]rune, 0, len(s))
	for _, r := range s {
		out = append(out, greekLowerCase(r))
	}
	res := string(out)
	if res != s {
		f.termAttr.SetEmpty()
		f.termAttr.AppendString(res)
	}
	return true, nil
}

// greekLowerCase mirrors GreekLowerCaseFilter.lowerCase in Lucene
// 10.4.0. It handles the small set of Greek code points that need
// custom mapping and falls back to unicode.ToLower for everything
// else.
func greekLowerCase(r rune) rune {
	switch r {
	case 0x03C2: // small final sigma
		return 0x03C3 // small sigma
	case 0x0386, 0x03AC: // capital/small alpha with tonos
		return 0x03B1
	case 0x0388, 0x03AD: // capital/small epsilon with tonos
		return 0x03B5
	case 0x0389, 0x03AE: // capital/small eta with tonos
		return 0x03B7
	case 0x038A, 0x03AA, 0x03AF, 0x03CA, 0x0390:
		return 0x03B9 // small iota family
	case 0x038E, 0x03AB, 0x03CD, 0x03CB, 0x03B0:
		return 0x03C5 // small upsilon family
	case 0x038C, 0x03CC:
		return 0x03BF // small omicron family
	case 0x038F, 0x03CE:
		return 0x03C9 // small omega family
	case 0x03A2:
		return 0x03C2 // reserved -> small final sigma
	default:
		return unicode.ToLower(r)
	}
}

// Ensure GreekLowerCaseFilter implements TokenFilter.
var _ TokenFilter = (*GreekLowerCaseFilter)(nil)

// GreekLowerCaseFilterFactory creates GreekLowerCaseFilter instances.
type GreekLowerCaseFilterFactory struct{}

// NewGreekLowerCaseFilterFactory returns a fresh factory.
func NewGreekLowerCaseFilterFactory() *GreekLowerCaseFilterFactory {
	return &GreekLowerCaseFilterFactory{}
}

// Create returns a GreekLowerCaseFilter wrapping input.
func (f *GreekLowerCaseFilterFactory) Create(input TokenStream) TokenFilter {
	return NewGreekLowerCaseFilter(input)
}

// Ensure GreekLowerCaseFilterFactory implements TokenFilterFactory.
var _ TokenFilterFactory = (*GreekLowerCaseFilterFactory)(nil)
