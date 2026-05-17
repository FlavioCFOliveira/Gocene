// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import "reflect"

// SerbianNormalizationFilter remaps Serbian Cyrillic letters to
// "bald" Latin (no diacritics), expanding multi-letter Cyrillic
// graphemes (đ→dj, љ→lj, њ→nj, џ→dz) and demoting Latin diacritics
// (č→c, ć→c, ž→z, š→s).
//
// This is the Go port of
// org.apache.lucene.analysis.sr.SerbianNormalizationFilter from
// Apache Lucene 10.4.0.
//
// The reference comment notes the filter expects lower-cased input;
// the Go port preserves that contract (it does not lower-case).
type SerbianNormalizationFilter struct {
	*BaseTokenFilter

	termAttr CharTermAttribute
}

// NewSerbianNormalizationFilter wraps input.
func NewSerbianNormalizationFilter(input TokenStream) *SerbianNormalizationFilter {
	f := &SerbianNormalizationFilter{
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

// IncrementToken applies the Cyrillic-to-bald-Latin mapping.
func (f *SerbianNormalizationFilter) IncrementToken() (bool, error) {
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
	in := []rune(f.termAttr.String())
	out := make([]rune, 0, len(in)+4)
	for _, r := range in {
		switch r {
		case 'а':
			out = append(out, 'a')
		case 'б':
			out = append(out, 'b')
		case 'в':
			out = append(out, 'v')
		case 'г':
			out = append(out, 'g')
		case 'д':
			out = append(out, 'd')
		case 'ђ', 'đ':
			out = append(out, 'd', 'j')
		case 'е':
			out = append(out, 'e')
		case 'ж', 'з', 'ž':
			out = append(out, 'z')
		case 'и':
			out = append(out, 'i')
		case 'ј':
			out = append(out, 'j')
		case 'к':
			out = append(out, 'k')
		case 'л':
			out = append(out, 'l')
		case 'љ':
			out = append(out, 'l', 'j')
		case 'м':
			out = append(out, 'm')
		case 'н':
			out = append(out, 'n')
		case 'њ':
			out = append(out, 'n', 'j')
		case 'о':
			out = append(out, 'o')
		case 'п':
			out = append(out, 'p')
		case 'р':
			out = append(out, 'r')
		case 'с':
			out = append(out, 's')
		case 'т':
			out = append(out, 't')
		case 'ћ', 'ц', 'ч', 'č', 'ć':
			out = append(out, 'c')
		case 'у':
			out = append(out, 'u')
		case 'ф':
			out = append(out, 'f')
		case 'х':
			out = append(out, 'h')
		case 'џ':
			out = append(out, 'd', 'z')
		case 'ш', 'š':
			out = append(out, 's')
		default:
			out = append(out, r)
		}
	}
	res := string(out)
	if res != f.termAttr.String() {
		f.termAttr.SetEmpty()
		f.termAttr.AppendString(res)
	}
	return true, nil
}

// Ensure SerbianNormalizationFilter implements TokenFilter.
var _ TokenFilter = (*SerbianNormalizationFilter)(nil)

// SerbianNormalizationRegularFilter is the same as
// SerbianNormalizationFilter; the Lucene reference exposes both
// names for backward-compatibility with older configurations.
type SerbianNormalizationRegularFilter = SerbianNormalizationFilter

// SerbianNormalizationFilterFactory creates instances.
type SerbianNormalizationFilterFactory struct{}

// NewSerbianNormalizationFilterFactory returns a fresh factory.
func NewSerbianNormalizationFilterFactory() *SerbianNormalizationFilterFactory {
	return &SerbianNormalizationFilterFactory{}
}

// Create wraps input.
func (f *SerbianNormalizationFilterFactory) Create(input TokenStream) TokenFilter {
	return NewSerbianNormalizationFilter(input)
}

// Ensure factory satisfies TokenFilterFactory.
var _ TokenFilterFactory = (*SerbianNormalizationFilterFactory)(nil)
