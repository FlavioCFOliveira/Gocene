// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"strings"
)

// Persian Unicode code points used by the stemmer suffixes.
const (
	faALEF = 0x0627
	faHEH  = 0x0647
	faTEH  = 0x062A
	faREH  = 0x0631
	faNOON = 0x0646
	faYEH  = 0x064A
	faZWNJ = 0x200C
)

// persianSuffixes lists the suffix strings the Persian stemmer
// strips, in the order applied by org.apache.lucene.analysis.fa.PersianStemmer.
var persianSuffixes = []string{
	string(rune(faALEF)) + string(rune(faTEH)),                                             // ات
	string(rune(faALEF)) + string(rune(faNOON)),                                            // ان
	string(rune(faTEH)) + string(rune(faREH)) + string(rune(faYEH)) + string(rune(faNOON)), // ترین
	string(rune(faTEH)) + string(rune(faREH)),                                              // تر
	string(rune(faYEH)) + string(rune(faYEH)),                                              // یی
	string(rune(faYEH)),                        // ی
	string(rune(faHEH)) + string(rune(faALEF)), // ها
	string(rune(faZWNJ)),                       // ZWNJ
}

// PersianStemmer applies a fixed list of Persian inflectional
// suffixes to a token, stripping the first match in priority order
// and continuing through the remaining suffixes (the reference loops
// over the array without early return).
//
// This is the Go port of org.apache.lucene.analysis.fa.PersianStemmer
// from Apache Lucene 10.4.0.
type PersianStemmer struct{}

// NewPersianStemmer returns a fresh stateless stemmer.
func NewPersianStemmer() *PersianStemmer {
	return &PersianStemmer{}
}

// Stem applies the suffix list to runes[:length] and returns the new
// length. The reference requires at least two runes remain after
// each strip, mirrored here by the length > sufLen+1 guard.
func (s *PersianStemmer) Stem(runes []rune, length int) int {
	for _, suf := range persianSuffixes {
		sufRunes := []rune(suf)
		if length < len(sufRunes)+2 {
			continue
		}
		if runesEndWith(runes, length, suf) {
			length -= len(sufRunes)
		}
	}
	return length
}

// StemString stems s.
func (s *PersianStemmer) StemString(input string) string {
	if input == "" {
		return ""
	}
	runes := []rune(input)
	n := s.Stem(runes, len(runes))
	return string(runes[:n])
}

// PersianStemFilter applies PersianStemmer to every non-keyword
// token.
//
// This is the Go port of
// org.apache.lucene.analysis.fa.PersianStemFilter.
type PersianStemFilter struct {
	*BaseTokenFilter

	stemmer     *PersianStemmer
	termAttr    CharTermAttribute
	keywordAttr KeywordAttribute
}

// NewPersianStemFilter wraps input with the Persian stemmer.
func NewPersianStemFilter(input TokenStream) *PersianStemFilter {
	f := &PersianStemFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		stemmer:         NewPersianStemmer(),
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttributeByType(reflect.TypeOf(&charTermAttribute{})); a != nil {
			f.termAttr = a.(CharTermAttribute)
		}
		if a := src.GetAttributeByType(KeywordAttributeType); a != nil {
			f.keywordAttr = a.(KeywordAttribute)
		}
	}
	return f
}

// IncrementToken stems the current token unless it is marked as a
// keyword.
func (f *PersianStemFilter) IncrementToken() (bool, error) {
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
	if f.keywordAttr != nil && f.keywordAttr.IsKeywordToken() {
		return true, nil
	}
	s := f.termAttr.String()
	st := f.stemmer.StemString(s)
	if st != s {
		f.termAttr.SetEmpty()
		f.termAttr.AppendString(st)
	}
	return true, nil
}

// Ensure PersianStemFilter implements TokenFilter.
var _ TokenFilter = (*PersianStemFilter)(nil)

// PersianStemFilterFactory creates PersianStemFilter instances.
type PersianStemFilterFactory struct{}

// NewPersianStemFilterFactory returns a fresh factory.
func NewPersianStemFilterFactory() *PersianStemFilterFactory {
	return &PersianStemFilterFactory{}
}

// Create returns a PersianStemFilter wrapping input.
func (f *PersianStemFilterFactory) Create(input TokenStream) TokenFilter {
	return NewPersianStemFilter(input)
}

// Ensure PersianStemFilterFactory implements TokenFilterFactory.
var _ TokenFilterFactory = (*PersianStemFilterFactory)(nil)

// PersianCharFilter replaces every Zero Width Non-Joiner (U+200C)
// in the input with a regular ASCII space (0x20). Length and offsets
// are preserved because ZWNJ is a single UTF-8 code point that
// occupies more than one byte; the replacement is performed at the
// rune level and the resulting string is re-encoded.
//
// This is the Go port of
// org.apache.lucene.analysis.fa.PersianCharFilter from Apache Lucene
// 10.4.0.
//
// Deviation from Lucene: the reference extends CharFilter and
// implements the char-buffer Reader contract. Gocene exposes
// CharFilter as a string-to-string transformation (NormalizeChar);
// callers wrap the result in a Reader as required.
type PersianCharFilter struct{}

// NewPersianCharFilter returns a fresh stateless char filter.
func NewPersianCharFilter() *PersianCharFilter {
	return &PersianCharFilter{}
}

// NormalizeChar replaces every ZWNJ in input with a space.
func (f *PersianCharFilter) NormalizeChar(input string) string {
	if input == "" {
		return ""
	}
	return strings.ReplaceAll(input, "‌", " ")
}

// PersianCharFilterFactory creates PersianCharFilter instances.
type PersianCharFilterFactory struct{}

// NewPersianCharFilterFactory returns a fresh factory.
func NewPersianCharFilterFactory() *PersianCharFilterFactory {
	return &PersianCharFilterFactory{}
}

// Create returns a PersianCharFilter.
func (f *PersianCharFilterFactory) Create() *PersianCharFilter {
	return NewPersianCharFilter()
}
