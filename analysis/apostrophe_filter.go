// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import "reflect"

// ApostropheFilter truncates each incoming token at the first
// occurrence of an ASCII apostrophe (U+0027) or the right single
// quotation mark (U+2019). This is the customary handling for
// Turkish text, where the apostrophe separates suffixes from proper
// names.
//
// This is the Go port of
// org.apache.lucene.analysis.tr.ApostropheFilter from Apache Lucene
// 10.4.0.
//
// Deviation from Lucene: the reference scans a char[] (UTF-16). The
// Go port operates on the UTF-8 byte buffer directly; both target
// code points are encoded such that no inner byte of one can be
// confused with the ASCII apostrophe byte 0x27, so a byte-level
// search remains correct. The right single quotation mark U+2019
// is U+2019 = 0xE2 0x80 0x99 in UTF-8 and is detected as a
// three-byte sequence.
type ApostropheFilter struct {
	*BaseTokenFilter

	termAttr CharTermAttribute
}

// NewApostropheFilter wraps input with the apostrophe filter.
func NewApostropheFilter(input TokenStream) *ApostropheFilter {
	f := &ApostropheFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttributeByType(reflect.TypeOf(&charTermAttribute{})); a != nil {
			f.termAttr = a.(CharTermAttribute)
		}
	}
	return f
}

// IncrementToken truncates the current token at the first apostrophe
// or right single quotation mark.
func (f *ApostropheFilter) IncrementToken() (bool, error) {
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
	buf := f.termAttr.Buffer()
	length := f.termAttr.Length()
	for i := 0; i < length; i++ {
		b := buf[i]
		// ASCII apostrophe
		if b == '\'' {
			f.termAttr.SetLength(i)
			return true, nil
		}
		// Right single quotation mark U+2019 = 0xE2 0x80 0x99
		if b == 0xE2 && i+2 < length && buf[i+1] == 0x80 && buf[i+2] == 0x99 {
			f.termAttr.SetLength(i)
			return true, nil
		}
	}
	return true, nil
}

// Ensure ApostropheFilter implements TokenFilter.
var _ TokenFilter = (*ApostropheFilter)(nil)

// ApostropheFilterFactory creates ApostropheFilter instances.
type ApostropheFilterFactory struct{}

// NewApostropheFilterFactory returns a fresh factory.
func NewApostropheFilterFactory() *ApostropheFilterFactory {
	return &ApostropheFilterFactory{}
}

// Create returns an ApostropheFilter wrapping input.
func (f *ApostropheFilterFactory) Create(input TokenStream) TokenFilter {
	return NewApostropheFilter(input)
}

// Ensure ApostropheFilterFactory implements TokenFilterFactory.
var _ TokenFilterFactory = (*ApostropheFilterFactory)(nil)
