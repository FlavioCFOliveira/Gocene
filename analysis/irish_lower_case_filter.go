// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"unicode"
)

// IrishLowerCaseFilter lower-cases its input while applying Irish
// linguistic rules for t-prothesis and n-eclipsis: a leading 'n' or
// 't' followed by an upper-case vowel triggers the insertion of a
// hyphen, so "nAthair" becomes "n-athair".
//
// This is the Go port of
// org.apache.lucene.analysis.ga.IrishLowerCaseFilter from Apache
// Lucene 10.4.0.
type IrishLowerCaseFilter struct {
	*BaseTokenFilter

	termAttr CharTermAttribute
}

// NewIrishLowerCaseFilter wraps input with the Irish lower-case
// filter.
func NewIrishLowerCaseFilter(input TokenStream) *IrishLowerCaseFilter {
	f := &IrishLowerCaseFilter{
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

// IncrementToken normalises the current token according to the
// t-prothesis / n-eclipsis rules described above and then
// lower-cases all remaining characters.
func (f *IrishLowerCaseFilter) IncrementToken() (bool, error) {
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
	runes := []rune(f.termAttr.String())
	idx := 0
	if len(runes) > 1 && (runes[0] == 'n' || runes[0] == 't') && isIrishUpperVowel(runes[1]) {
		// Insert '-' at index 1; everything from the original index 1
		// onwards shifts one position to the right.
		newRunes := make([]rune, 0, len(runes)+1)
		newRunes = append(newRunes, runes[0], '-')
		newRunes = append(newRunes, runes[1:]...)
		runes = newRunes
		idx = 2
	}
	for i := idx; i < len(runes); i++ {
		runes[i] = unicode.ToLower(runes[i])
	}
	res := string(runes)
	f.termAttr.SetEmpty()
	f.termAttr.AppendString(res)
	return true, nil
}

// isIrishUpperVowel reports whether r is an upper-case English or
// Irish vowel (the Irish set includes acute-accented vowels).
func isIrishUpperVowel(r rune) bool {
	switch r {
	case 'A', 'E', 'I', 'O', 'U',
		0x00C1, 0x00C9, 0x00CD, 0x00D3, 0x00DA:
		return true
	}
	return false
}

// Ensure IrishLowerCaseFilter implements TokenFilter.
var _ TokenFilter = (*IrishLowerCaseFilter)(nil)

// IrishLowerCaseFilterFactory creates IrishLowerCaseFilter
// instances.
type IrishLowerCaseFilterFactory struct{}

// NewIrishLowerCaseFilterFactory returns a fresh factory.
func NewIrishLowerCaseFilterFactory() *IrishLowerCaseFilterFactory {
	return &IrishLowerCaseFilterFactory{}
}

// Create returns an IrishLowerCaseFilter wrapping input.
func (f *IrishLowerCaseFilterFactory) Create(input TokenStream) TokenFilter {
	return NewIrishLowerCaseFilter(input)
}

// Ensure IrishLowerCaseFilterFactory implements TokenFilterFactory.
var _ TokenFilterFactory = (*IrishLowerCaseFilterFactory)(nil)
