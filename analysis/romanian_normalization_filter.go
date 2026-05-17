// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import "reflect"

// Romanian Unicode constants for the cedilla -> comma-below mapping.
const (
	roCapSCommaBelow = 0x0218 // Ș
	roSmlSCommaBelow = 0x0219 // ș
	roCapTCommaBelow = 0x021A // Ț
	roSmlTCommaBelow = 0x021B // ț
	roCapSCedilla    = 0x015E // Ş
	roSmlSCedilla    = 0x015F // ş
	roCapTCedilla    = 0x0162 // Ţ
	roSmlTCedilla    = 0x0163 // ţ
)

// RomanianNormalizer remaps cedilla diacritics (S/T with cedilla) to
// the official Unicode comma-below variants. This is the Go port of
// org.apache.lucene.analysis.ro.RomanianNormalizer.
type RomanianNormalizer struct{}

// NewRomanianNormalizer returns a fresh stateless normaliser.
func NewRomanianNormalizer() *RomanianNormalizer {
	return &RomanianNormalizer{}
}

// Normalize remaps in place and returns the original length (the
// mapping is 1:1 so length is preserved).
func (n *RomanianNormalizer) Normalize(runes []rune, length int) int {
	for i := 0; i < length; i++ {
		switch runes[i] {
		case roCapSCedilla:
			runes[i] = roCapSCommaBelow
		case roSmlSCedilla:
			runes[i] = roSmlSCommaBelow
		case roCapTCedilla:
			runes[i] = roCapTCommaBelow
		case roSmlTCedilla:
			runes[i] = roSmlTCommaBelow
		}
	}
	return length
}

// RomanianNormalizationFilter wraps a TokenStream with
// RomanianNormalizer. This is the Go port of
// org.apache.lucene.analysis.ro.RomanianNormalizationFilter.
type RomanianNormalizationFilter struct {
	*BaseTokenFilter

	normalizer *RomanianNormalizer
	termAttr   CharTermAttribute
}

// NewRomanianNormalizationFilter wraps input.
func NewRomanianNormalizationFilter(input TokenStream) *RomanianNormalizationFilter {
	f := &RomanianNormalizationFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		normalizer:      NewRomanianNormalizer(),
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttributeByType(reflect.TypeOf(&charTermAttribute{})); a != nil {
			f.termAttr = a.(CharTermAttribute)
		}
	}
	return f
}

// IncrementToken applies the normaliser.
func (f *RomanianNormalizationFilter) IncrementToken() (bool, error) {
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
	f.normalizer.Normalize(runes, len(runes))
	res := string(runes)
	if res != f.termAttr.String() {
		f.termAttr.SetEmpty()
		f.termAttr.AppendString(res)
	}
	return true, nil
}

// Ensure RomanianNormalizationFilter implements TokenFilter.
var _ TokenFilter = (*RomanianNormalizationFilter)(nil)

// RomanianNormalizationFilterFactory creates instances.
type RomanianNormalizationFilterFactory struct{}

// NewRomanianNormalizationFilterFactory returns a fresh factory.
func NewRomanianNormalizationFilterFactory() *RomanianNormalizationFilterFactory {
	return &RomanianNormalizationFilterFactory{}
}

// Create wraps input.
func (f *RomanianNormalizationFilterFactory) Create(input TokenStream) TokenFilter {
	return NewRomanianNormalizationFilter(input)
}

// Ensure factory satisfies TokenFilterFactory.
var _ TokenFilterFactory = (*RomanianNormalizationFilterFactory)(nil)
