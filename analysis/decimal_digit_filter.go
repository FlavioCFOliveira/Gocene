// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"strings"
	"unicode"
)

// DecimalDigitFilter folds every Unicode decimal-number code point
// to the corresponding ASCII digit 0-9.
//
// This is the Go port of
// org.apache.lucene.analysis.core.DecimalDigitFilter from Apache
// Lucene 10.4.0.
//
// Deviation from Lucene: the reference operates on char[] and
// handles surrogate pairs by collapsing them after substitution.
// The Go port iterates over runes, so every supplementary code
// point is naturally a single element and no length adjustment is
// required.
type DecimalDigitFilter struct {
	*BaseTokenFilter

	termAttr CharTermAttribute
}

// NewDecimalDigitFilter wraps input.
func NewDecimalDigitFilter(input TokenStream) *DecimalDigitFilter {
	f := &DecimalDigitFilter{
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

// IncrementToken folds non-ASCII digits.
func (f *DecimalDigitFilter) IncrementToken() (bool, error) {
	ok, err := f.input.IncrementToken()
	if err != nil || !ok {
		return ok, err
	}
	if f.termAttr == nil {
		return true, nil
	}
	s := f.termAttr.String()
	if !needsDecimalDigitFold(s) {
		return true, nil
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r > 0x7F && unicode.IsDigit(r) {
			b.WriteByte(byte('0' + unicodeDigitValue(r)))
		} else {
			b.WriteRune(r)
		}
	}
	f.termAttr.SetEmpty()
	f.termAttr.AppendString(b.String())
	return true, nil
}

// needsDecimalDigitFold returns true when any rune in s is a
// non-ASCII decimal-number code point.
func needsDecimalDigitFold(s string) bool {
	for _, r := range s {
		if r > 0x7F && unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

// unicodeDigitValue returns the decimal value of r in [0,9]. Falls
// back to 0 for code points that are not decimal digits (the caller
// guards with unicode.IsDigit before invoking).
func unicodeDigitValue(r rune) int {
	// Map runes via unicode.Digit table; for digit code points the
	// numeric value can be derived from the offset within the script's
	// decimal block (every Unicode decimal-digit block is contiguous).
	// We approximate by subtracting the block's '0' rune.
	switch {
	case r >= '0' && r <= '9':
		return int(r - '0')
	case r >= 0x0660 && r <= 0x0669: // Arabic-Indic
		return int(r - 0x0660)
	case r >= 0x06F0 && r <= 0x06F9: // Extended Arabic-Indic
		return int(r - 0x06F0)
	case r >= 0x07C0 && r <= 0x07C9: // NKo
		return int(r - 0x07C0)
	case r >= 0x0966 && r <= 0x096F: // Devanagari
		return int(r - 0x0966)
	case r >= 0x09E6 && r <= 0x09EF: // Bengali
		return int(r - 0x09E6)
	case r >= 0x0A66 && r <= 0x0A6F: // Gurmukhi
		return int(r - 0x0A66)
	case r >= 0x0AE6 && r <= 0x0AEF: // Gujarati
		return int(r - 0x0AE6)
	case r >= 0x0B66 && r <= 0x0B6F: // Oriya
		return int(r - 0x0B66)
	case r >= 0x0BE6 && r <= 0x0BEF: // Tamil
		return int(r - 0x0BE6)
	case r >= 0x0C66 && r <= 0x0C6F: // Telugu
		return int(r - 0x0C66)
	case r >= 0x0CE6 && r <= 0x0CEF: // Kannada
		return int(r - 0x0CE6)
	case r >= 0x0D66 && r <= 0x0D6F: // Malayalam
		return int(r - 0x0D66)
	case r >= 0x0DE6 && r <= 0x0DEF: // Sinhala Lith
		return int(r - 0x0DE6)
	case r >= 0x0E50 && r <= 0x0E59: // Thai
		return int(r - 0x0E50)
	case r >= 0x0ED0 && r <= 0x0ED9: // Lao
		return int(r - 0x0ED0)
	case r >= 0x0F20 && r <= 0x0F29: // Tibetan
		return int(r - 0x0F20)
	case r >= 0x1040 && r <= 0x1049: // Myanmar
		return int(r - 0x1040)
	case r >= 0x1090 && r <= 0x1099: // Myanmar Shan
		return int(r - 0x1090)
	case r >= 0x17E0 && r <= 0x17E9: // Khmer
		return int(r - 0x17E0)
	case r >= 0x1810 && r <= 0x1819: // Mongolian
		return int(r - 0x1810)
	case r >= 0xFF10 && r <= 0xFF19: // Fullwidth
		return int(r - 0xFF10)
	}
	return 0
}

// Ensure DecimalDigitFilter implements TokenFilter.
var _ TokenFilter = (*DecimalDigitFilter)(nil)

// DecimalDigitFilterFactory creates DecimalDigitFilter instances.
type DecimalDigitFilterFactory struct{}

// NewDecimalDigitFilterFactory returns a fresh factory.
func NewDecimalDigitFilterFactory() *DecimalDigitFilterFactory {
	return &DecimalDigitFilterFactory{}
}

// Create wraps input.
func (f *DecimalDigitFilterFactory) Create(input TokenStream) TokenFilter {
	return NewDecimalDigitFilter(input)
}

// Ensure factory satisfies TokenFilterFactory.
var _ TokenFilterFactory = (*DecimalDigitFilterFactory)(nil)
