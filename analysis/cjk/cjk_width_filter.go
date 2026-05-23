// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package cjk

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// kanaNorm maps halfwidth Katakana (0xFF65–0xFF9D) to fullwidth equivalents.
// Index 0 = 0xFF65, index 58 = 0xFF9D.
//
// note: 0xFF9C and 0xFF9D are only mapped to 0x3099 and 0x309A
// as a fallback when they cannot properly combine with a preceding
// character into a composed form.
var kanaNorm = [...]rune{
	0x30fb, 0x30f2, 0x30a1, 0x30a3, 0x30a5, 0x30a7, 0x30a9, 0x30e3, 0x30e5,
	0x30e7, 0x30c3, 0x30fc, 0x30a2, 0x30a4, 0x30a6, 0x30a8, 0x30aa, 0x30ab,
	0x30ad, 0x30af, 0x30b1, 0x30b3, 0x30b5, 0x30b7, 0x30b9, 0x30bb, 0x30bd,
	0x30bf, 0x30c1, 0x30c4, 0x30c6, 0x30c8, 0x30ca, 0x30cb, 0x30cc, 0x30cd,
	0x30ce, 0x30cf, 0x30d2, 0x30d5, 0x30d8, 0x30db, 0x30de, 0x30df, 0x30e0,
	0x30e1, 0x30e2, 0x30e4, 0x30e6, 0x30e8, 0x30e9, 0x30ea, 0x30eb, 0x30ec,
	0x30ed, 0x30ef, 0x30f3, 0x3099, 0x309a,
}

// kanaCombineVoiced contains voiced-mark combining deltas for 0x30A6–0x30FD.
var kanaCombineVoiced = [...]int8{
	78, 0, 0, 0, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1,
	0, 1, 0, 1, 0, 0, 1, 0, 1, 0, 1, 0, 0, 0, 0, 0, 0, 1, 0, 0, 1, 0, 0, 1,
	0, 0, 1, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 8, 8, 8, 8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1,
}

// kanaCombineHalfVoiced contains semi-voiced-mark combining deltas for 0x30A6–0x30FD.
var kanaCombineHalfVoiced = [...]int8{
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2, 0, 0, 2, 0, 0, 2,
	0, 0, 2, 0, 0, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
}

// combineKana attempts to combine a voiced/semi-voiced mark with the preceding
// rune in-place. Returns true if combination occurred.
func combineKana(text []rune, pos int, mark rune) bool {
	prev := text[pos-1]
	if prev >= 0x30a6 && prev <= 0x30fd {
		var delta int8
		if mark == 0xff9f {
			delta = kanaCombineHalfVoiced[prev-0x30a6]
		} else {
			delta = kanaCombineVoiced[prev-0x30a6]
		}
		combined := prev + rune(delta)
		if combined != prev {
			text[pos-1] = combined
			return true
		}
	}
	return false
}

// CJKWidthFilter normalises CJK width differences:
//   - Folds fullwidth ASCII variants (0xFF01–0xFF5E) into basic latin.
//   - Folds halfwidth Katakana variants (0xFF65–0xFF9F) into standard kana.
//
// This is the Go port of
// org.apache.lucene.analysis.cjk.CJKWidthFilter from
// Apache Lucene 10.4.0.
type CJKWidthFilter struct {
	*analysis.BaseTokenFilter

	termAttr analysis.CharTermAttribute
}

// NewCJKWidthFilter creates a CJKWidthFilter wrapping input.
func NewCJKWidthFilter(input analysis.TokenStream) *CJKWidthFilter {
	f := &CJKWidthFilter{
		BaseTokenFilter: analysis.NewBaseTokenFilter(input),
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttribute(analysis.CharTermAttributeType); a != nil {
			f.termAttr = a.(analysis.CharTermAttribute)
		}
	}
	return f
}

// IncrementToken advances to the next token, normalising CJK width.
func (f *CJKWidthFilter) IncrementToken() (bool, error) {
	ok, err := f.GetInput().IncrementToken()
	if !ok || err != nil {
		return ok, err
	}
	if f.termAttr == nil {
		return true, nil
	}

	text := []rune(f.termAttr.String())
	length := len(text)
	for i := 0; i < length; i++ {
		ch := text[i]
		if ch >= 0xff01 && ch <= 0xff5e {
			// fullwidth ASCII → basic latin
			text[i] = ch - 0xfee0
		} else if ch >= 0xff65 && ch <= 0xff9f {
			// halfwidth katakana
			if (ch == 0xff9e || ch == 0xff9f) && i > 0 && combineKana(text, i, ch) {
				// remove the combining mark at position i by shifting left
				text = append(text[:i], text[i+1:]...)
				length--
				i--
			} else {
				text[i] = kanaNorm[ch-0xff65]
			}
		}
	}

	f.termAttr.SetEmpty()
	f.termAttr.AppendString(string(text[:length]))
	return true, nil
}

// Ensure CJKWidthFilter implements TokenFilter.
var _ analysis.TokenFilter = (*CJKWidthFilter)(nil)

// CJKWidthFilterFactory creates CJKWidthFilter instances.
//
// This is the Go port of
// org.apache.lucene.analysis.cjk.CJKWidthFilterFactory from
// Apache Lucene 10.4.0.
type CJKWidthFilterFactory struct{}

// NewCJKWidthFilterFactory creates a CJKWidthFilterFactory.
func NewCJKWidthFilterFactory() *CJKWidthFilterFactory { return &CJKWidthFilterFactory{} }

// Create creates a CJKWidthFilter wrapping input.
func (f *CJKWidthFilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	return NewCJKWidthFilter(input)
}

// Ensure factory implements TokenFilterFactory.
var _ analysis.TokenFilterFactory = (*CJKWidthFilterFactory)(nil)
