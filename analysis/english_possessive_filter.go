// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

// EnglishPossessiveFilter strips trailing "'s" or "'S" suffixes
// (with ASCII apostrophe '\”, right single quotation mark
// U+2019, or fullwidth apostrophe U+FF07) from each incoming token.
//
// This is the Go port of
// org.apache.lucene.analysis.en.EnglishPossessiveFilter from Apache
// Lucene 10.4.0.
//
// Deviation from Lucene: the Java filter scans a char[] (UTF-16).
// The Go port works on the UTF-8 buffer; the three apostrophe
// variants encode as 0x27 (ASCII), 0xE2 0x80 0x99 (U+2019), and
// 0xEF 0xBC 0x87 (U+FF07). The check operates on the rune-aware
// string representation to keep the matching readable.
type EnglishPossessiveFilter struct {
	*BaseTokenFilter

	termAttr CharTermAttribute
}

// NewEnglishPossessiveFilter wraps input.
func NewEnglishPossessiveFilter(input TokenStream) *EnglishPossessiveFilter {
	f := &EnglishPossessiveFilter{
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

// IncrementToken strips the possessive suffix if present.
func (f *EnglishPossessiveFilter) IncrementToken() (bool, error) {
	ok, err := f.input.IncrementToken()
	if err != nil || !ok {
		return ok, err
	}
	if f.termAttr == nil {
		return true, nil
	}
	runes := []rune(f.termAttr.String())
	if len(runes) >= 2 {
		last := runes[len(runes)-1]
		prev := runes[len(runes)-2]
		if (prev == '\'' || prev == 0x2019 || prev == 0xFF07) &&
			(last == 's' || last == 'S') {
			f.termAttr.SetEmpty()
			f.termAttr.AppendString(string(runes[:len(runes)-2]))
		}
	}
	return true, nil
}

// Ensure EnglishPossessiveFilter implements TokenFilter.
var _ TokenFilter = (*EnglishPossessiveFilter)(nil)

// EnglishPossessiveFilterFactory creates instances.
type EnglishPossessiveFilterFactory struct{}

// NewEnglishPossessiveFilterFactory returns a fresh factory.
func NewEnglishPossessiveFilterFactory() *EnglishPossessiveFilterFactory {
	return &EnglishPossessiveFilterFactory{}
}

// Create wraps input.
func (f *EnglishPossessiveFilterFactory) Create(input TokenStream) TokenFilter {
	return NewEnglishPossessiveFilter(input)
}

// Ensure factory satisfies TokenFilterFactory.
var _ TokenFilterFactory = (*EnglishPossessiveFilterFactory)(nil)
