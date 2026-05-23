// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package ko

import (
	"math/big"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// noNumeral is the sentinel value meaning "not a Hangul digit 0–9".
const noNumeral = rune(0xFFFF)

// numerals maps Hangul digit characters (BMP only) to their decimal value
// (0–9). Characters not present hold noNumeral.
var numerals [0x10000]rune

// exponents maps Hangul power-of-ten characters (BMP only) to their exponent
// (1–20). Characters not present hold 0.
var exponents [0x10000]int

func init() {
	for i := range numerals {
		numerals[i] = noNumeral
	}
	numerals['영'] = 0 // U+C601 0
	numerals['일'] = 1 // U+C77C 1
	numerals['이'] = 2 // U+C774 2
	numerals['삼'] = 3 // U+C0BC 3
	numerals['사'] = 4 // U+C0AC 4
	numerals['오'] = 5 // U+C624 5
	numerals['육'] = 6 // U+C721 6
	numerals['칠'] = 7 // U+CE60 7
	numerals['팔'] = 8 // U+D314 8
	numerals['구'] = 9 // U+AD6C 9

	exponents['십'] = 1  // U+C2ED 10
	exponents['백'] = 2  // U+BC31 100
	exponents['천'] = 3  // U+CC9C 1,000
	exponents['만'] = 4  // U+B9CC 10,000
	exponents['억'] = 8  // U+C5B5 100,000,000
	exponents['조'] = 12 // U+C870 1,000,000,000,000
	exponents['경'] = 16 // U+ACBD 10,000,000,000,000,000
	exponents['해'] = 20 // U+D574 100,000,000,000,000,000,000
}

// NumberBuffer holds a Korean number string and a parse-position cursor.
//
// This is the Go port of
// org.apache.lucene.analysis.ko.KoreanNumberFilter.NumberBuffer from Apache
// Lucene 10.4.0.
type NumberBuffer struct {
	s   []rune
	pos int
}

// NewNumberBuffer creates a NumberBuffer from s.
func NewNumberBuffer(s string) *NumberBuffer { return &NumberBuffer{s: []rune(s)} }

// CharAt returns the rune at index i.
func (b *NumberBuffer) CharAt(i int) rune { return b.s[i] }

// Length returns the number of runes in the buffer.
func (b *NumberBuffer) Length() int { return len(b.s) }

// Advance advances the cursor by one.
func (b *NumberBuffer) Advance() { b.pos++ }

// Position returns the current cursor position.
func (b *NumberBuffer) Position() int { return b.pos }

// KoreanNumberFilter normalises Korean numbers to regular Arabic decimal
// numbers in half-width characters.
//
// This is the Go port of
// org.apache.lucene.analysis.ko.KoreanNumberFilter from Apache Lucene 10.4.0.
type KoreanNumberFilter struct {
	*analysis.BaseTokenFilter

	termAttr    analysis.CharTermAttribute
	offsetAttr  analysis.OffsetAttribute
	keywordAttr analysis.KeywordAttribute
	posIncrAttr analysis.PositionIncrementAttribute
	posLenAttr  analysis.PositionLengthAttribute

	savedState        *util.AttributeState
	numeral           strings.Builder
	fallThroughTokens int
	exhausted         bool
}

// NewKoreanNumberFilter creates a KoreanNumberFilter wrapping input.
func NewKoreanNumberFilter(input analysis.TokenStream) *KoreanNumberFilter {
	f := &KoreanNumberFilter{
		BaseTokenFilter: analysis.NewBaseTokenFilter(input),
	}
	src := f.BaseTokenFilter.GetAttributeSource()
	if src != nil {
		if a := src.GetAttribute(analysis.CharTermAttributeType); a != nil {
			f.termAttr, _ = a.(analysis.CharTermAttribute)
		}
		if a := src.GetAttribute(analysis.OffsetAttributeType); a != nil {
			f.offsetAttr, _ = a.(analysis.OffsetAttribute)
		}
		if a := src.GetAttribute(analysis.KeywordAttributeType); a != nil {
			f.keywordAttr, _ = a.(analysis.KeywordAttribute)
		}
		if a := src.GetAttribute(analysis.PositionIncrementAttributeType); a != nil {
			f.posIncrAttr, _ = a.(analysis.PositionIncrementAttribute)
		}
		if a := src.GetAttribute(analysis.PositionLengthAttributeType); a != nil {
			f.posLenAttr, _ = a.(analysis.PositionLengthAttribute)
		}
	}
	return f
}

// Reset resets the filter state for re-use with a new token stream.
func (f *KoreanNumberFilter) Reset() error {
	f.fallThroughTokens = 0
	f.numeral.Reset()
	f.savedState = nil
	f.exhausted = false
	if in := f.BaseTokenFilter.GetInput(); in != nil {
		if r, ok := in.(interface{ Reset() error }); ok {
			return r.Reset()
		}
	}
	return nil
}

// IncrementToken advances to the next token, composing and normalising Korean
// number sequences as needed.
func (f *KoreanNumberFilter) IncrementToken() (bool, error) {
	src := f.BaseTokenFilter.GetAttributeSource()

	// Emit a previously captured token we read past.
	if f.savedState != nil {
		src.RestoreState(f.savedState)
		f.savedState = nil
		return true, nil
	}

	if f.exhausted {
		return false, nil
	}

	ok, err := f.BaseTokenFilter.IncrementToken()
	if err != nil {
		return false, err
	}
	if !ok {
		f.exhausted = true
		return false, nil
	}

	if f.keywordAttr != nil && f.keywordAttr.IsKeywordToken() {
		return true, nil
	}

	if f.fallThroughTokens > 0 {
		f.fallThroughTokens--
		return true, nil
	}

	if f.posIncrAttr != nil && f.posIncrAttr.GetPositionIncrement() == 0 {
		posLen := 1
		if f.posLenAttr != nil {
			posLen = f.posLenAttr.GetPositionLength()
		}
		f.fallThroughTokens = posLen - 1
		return true, nil
	}

	moreTokens := true
	composedNumber := false
	startOffset := 0
	endOffset := 0

	preCompState := src.CaptureState()
	term := f.termStr()
	numeralTerm := f.IsNumeral(term)

	for moreTokens && numeralTerm {
		if !composedNumber {
			if f.offsetAttr != nil {
				startOffset = f.offsetAttr.StartOffset()
			}
			composedNumber = true
		}
		if f.offsetAttr != nil {
			endOffset = f.offsetAttr.EndOffset()
		}

		ok, err = f.BaseTokenFilter.IncrementToken()
		if err != nil {
			return false, err
		}
		if !ok {
			f.exhausted = true
		}
		moreTokens = ok

		if moreTokens && f.posIncrAttr != nil && f.posIncrAttr.GetPositionIncrement() == 0 {
			posLen := 1
			if f.posLenAttr != nil {
				posLen = f.posLenAttr.GetPositionLength()
			}
			f.fallThroughTokens = posLen - 1
			f.savedState = src.CaptureState()
			src.RestoreState(preCompState)
			return moreTokens, nil
		}

		f.numeral.WriteString(term)

		if moreTokens {
			term = f.termStr()
			numeralTerm = f.IsNumeral(term) || f.IsNumeralPunctuation(term)
		}
	}

	if composedNumber {
		if moreTokens {
			f.savedState = src.CaptureState()
		}

		normalized := f.NormalizeNumber(f.numeral.String())
		if f.termAttr != nil {
			f.termAttr.SetValue(normalized)
		}
		if f.offsetAttr != nil {
			f.offsetAttr.SetOffset(startOffset, endOffset)
		}

		f.numeral.Reset()
		return true, nil
	}
	return moreTokens, nil
}

func (f *KoreanNumberFilter) termStr() string {
	if f.termAttr == nil {
		return ""
	}
	return f.termAttr.String()
}

// NormalizeNumber normalises a Korean number string to an Arabic decimal
// string. Returns the input unchanged on parse error.
func (f *KoreanNumberFilter) NormalizeNumber(number string) string {
	result := parseKoreanNumber(NewNumberBuffer(number))
	if result == nil {
		return number
	}
	s := result.Text('f', -1)
	// Strip trailing zeros after decimal point, matching BigDecimal.stripTrailingZeros().toPlainString().
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	return s
}

// IsNumeral reports whether every character in s is a numeral (Arabic, Hangul
// digit 영–구, or Hangul exponent 십/백/천/만/억/조/경/해).
func (f *KoreanNumberFilter) IsNumeral(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if !isNumeralRune(c) {
			return false
		}
	}
	return true
}

// IsNumeralPunctuation reports whether every character in s is numeral
// punctuation (decimal point or thousand separator).
func (f *KoreanNumberFilter) IsNumeralPunctuation(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if !isKoDecimalPoint(c) && !isKoThousandSeparator(c) {
			return false
		}
	}
	return true
}

// IsArabicNumeral reports whether c is a half-width or full-width Arabic numeral.
func (f *KoreanNumberFilter) IsArabicNumeral(c rune) bool { return isKoArabicNumeral(c) }

// ParseLargeHangulNumeral parses ten-thousands (만) or larger Hangul exponents
// from buf.
func (f *KoreanNumberFilter) ParseLargeHangulNumeral(buf *NumberBuffer) *big.Float {
	return parseLargeHangulNumeral(buf)
}

// ParseMediumHangulNumeral parses tens (십), hundreds (백), thousands (천) from buf.
func (f *KoreanNumberFilter) ParseMediumHangulNumeral(buf *NumberBuffer) *big.Float {
	return parseMediumHangulNumeral(buf)
}

// --- package-level parsing helpers -------------------------------------------

func parseKoreanNumber(buf *NumberBuffer) *big.Float {
	result := parseLargePair(buf)
	if result == nil {
		return nil
	}
	sum := new(big.Float).SetPrec(256).Set(result)
	for {
		result = parseLargePair(buf)
		if result == nil {
			break
		}
		sum.Add(sum, result)
	}
	return sum
}

func parseLargePair(buf *NumberBuffer) *big.Float {
	first := parseMediumNumber(buf)
	second := parseLargeHangulNumeral(buf)
	if first == nil && second == nil {
		return nil
	}
	if second == nil {
		return first
	}
	if first == nil {
		return second
	}
	return new(big.Float).SetPrec(256).Mul(first, second)
}

func parseMediumNumber(buf *NumberBuffer) *big.Float {
	result := parseMediumPair(buf)
	if result == nil {
		return nil
	}
	sum := new(big.Float).SetPrec(256).Set(result)
	for {
		result = parseMediumPair(buf)
		if result == nil {
			break
		}
		sum.Add(sum, result)
	}
	return sum
}

func parseMediumPair(buf *NumberBuffer) *big.Float {
	first := parseBasicNumber(buf)
	second := parseMediumHangulNumeral(buf)
	if first == nil && second == nil {
		return nil
	}
	if second == nil {
		return first
	}
	if first == nil {
		return second
	}
	return new(big.Float).SetPrec(256).Mul(first, second)
}

func parseBasicNumber(buf *NumberBuffer) *big.Float {
	var sb strings.Builder
	for buf.Position() < buf.Length() {
		c := buf.CharAt(buf.Position())
		if isKoArabicNumeral(c) {
			sb.WriteByte(byte('0' + arabicNumeralVal(c)))
		} else if c < 0x10000 && numerals[c] != noNumeral {
			sb.WriteByte(byte('0' + numerals[c]))
		} else if isKoDecimalPoint(c) {
			sb.WriteByte('.')
		} else if isKoThousandSeparator(c) {
			// skip
		} else {
			break
		}
		buf.Advance()
	}
	if sb.Len() == 0 {
		return nil
	}
	v, _, err := big.ParseFloat(sb.String(), 10, 256, big.ToNearestEven)
	if err != nil {
		return nil
	}
	return v
}

func parseLargeHangulNumeral(buf *NumberBuffer) *big.Float {
	if buf.Position() >= buf.Length() {
		return nil
	}
	c := buf.CharAt(buf.Position())
	if c >= 0x10000 {
		return nil
	}
	power := exponents[c]
	if power > 3 {
		buf.Advance()
		return bigTenPow(power)
	}
	return nil
}

func parseMediumHangulNumeral(buf *NumberBuffer) *big.Float {
	if buf.Position() >= buf.Length() {
		return nil
	}
	c := buf.CharAt(buf.Position())
	if c >= 0x10000 {
		return nil
	}
	power := exponents[c]
	if power >= 1 && power <= 3 {
		buf.Advance()
		return bigTenPow(power)
	}
	return nil
}

// --- character predicates ----------------------------------------------------

func isNumeralRune(c rune) bool {
	return isKoArabicNumeral(c) ||
		(c < 0x10000 && numerals[c] != noNumeral) ||
		(c < 0x10000 && exponents[c] > 0)
}

func isKoArabicNumeral(c rune) bool {
	return (c >= '0' && c <= '9') || (c >= '０' && c <= '９')
}

func arabicNumeralVal(c rune) int {
	if c >= '0' && c <= '9' {
		return int(c - '0')
	}
	return int(c - '０')
}

func isKoDecimalPoint(c rune) bool {
	return c == '.' || c == '．' // U+002E, U+FF0E
}

func isKoThousandSeparator(c rune) bool {
	return c == ',' || c == '，' // U+002C, U+FF0C
}

func bigTenPow(power int) *big.Float {
	i := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(power)), nil)
	return new(big.Float).SetPrec(256).SetInt(i)
}

// Ensure KoreanNumberFilter satisfies TokenStream.
var _ analysis.TokenStream = (*KoreanNumberFilter)(nil)
