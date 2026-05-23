// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package kuromoji

import (
	"math/big"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// noNumeral is the sentinel value indicating a character is not a kanji numeral.
const noNumeral = rune(0xFFFF) // Character.MAX_VALUE equivalent

// numerals maps kanji numeral characters (0-9) to their numeric values.
// Index by rune; non-numerals hold noNumeral.
var numerals [0x10000]rune

// exponents maps kanji multiplier characters to their power-of-10 exponents.
var exponents [0x10000]byte

func init() {
	for i := range numerals {
		numerals[i] = noNumeral
	}
	numerals['〇'] = 0
	numerals['一'] = 1
	numerals['二'] = 2
	numerals['三'] = 3
	numerals['四'] = 4
	numerals['五'] = 5
	numerals['六'] = 6
	numerals['七'] = 7
	numerals['八'] = 8
	numerals['九'] = 9

	exponents['十'] = 1
	exponents['百'] = 2
	exponents['千'] = 3
	exponents['万'] = 4
	exponents['億'] = 8
	exponents['兆'] = 12
	exponents['京'] = 16
	exponents['垓'] = 20
}

// NumberBuffer holds a Japanese number string together with a parse position.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.JapaneseNumberFilter.NumberBuffer from Apache
// Lucene 10.4.0.
type NumberBuffer struct {
	s   string
	pos int
}

// NewNumberBuffer creates a NumberBuffer for the given string.
func NewNumberBuffer(s string) *NumberBuffer { return &NumberBuffer{s: s} }

// CharAt returns the rune at index i.
func (b *NumberBuffer) CharAt(i int) rune { r := []rune(b.s); return r[i] }

// Length returns the number of runes in the buffer.
func (b *NumberBuffer) Length() int { return len([]rune(b.s)) }

// Advance increments the parse position by one.
func (b *NumberBuffer) Advance() { b.pos++ }

// Position returns the current parse position.
func (b *NumberBuffer) Position() int { return b.pos }

// JapaneseNumberFilter normalizes Japanese numbers (kansūji) to regular Arabic
// decimal numbers in half-width characters.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.JapaneseNumberFilter from Apache Lucene
// 10.4.0.
type JapaneseNumberFilter struct {
	*analysis.BaseTokenFilter
	termAttr    analysis.CharTermAttribute
	offsetAttr  analysis.OffsetAttribute
	keywordAttr analysis.KeywordAttribute
	posIncrAttr analysis.PositionIncrementAttribute
	posLenAttr  analysis.PositionLengthAttribute

	state            *util.AttributeState
	numeral          strings.Builder
	fallThroughTokens int
	exhausted        bool
}

// NewJapaneseNumberFilter creates a new JapaneseNumberFilter wrapping input.
func NewJapaneseNumberFilter(input analysis.TokenStream) *JapaneseNumberFilter {
	f := &JapaneseNumberFilter{
		BaseTokenFilter: analysis.NewBaseTokenFilter(input),
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttribute(analysis.CharTermAttributeType); a != nil {
			f.termAttr = a.(analysis.CharTermAttribute)
		}
		if a := src.GetAttribute(analysis.OffsetAttributeType); a != nil {
			f.offsetAttr = a.(analysis.OffsetAttribute)
		}
		if a := src.GetAttribute(analysis.KeywordAttributeType); a != nil {
			f.keywordAttr = a.(analysis.KeywordAttribute)
		}
		if a := src.GetAttribute(analysis.PositionIncrementAttributeType); a != nil {
			f.posIncrAttr = a.(analysis.PositionIncrementAttribute)
		}
		if a := src.GetAttribute(analysis.PositionLengthAttributeType); a != nil {
			f.posLenAttr = a.(analysis.PositionLengthAttribute)
		}
	}
	return f
}

// Reset resets the filter state.
func (f *JapaneseNumberFilter) Reset() error {
	if r, ok := f.GetInput().(interface{ Reset() error }); ok {
		if err := r.Reset(); err != nil {
			return err
		}
	}
	f.fallThroughTokens = 0
	f.numeral.Reset()
	f.state = nil
	f.exhausted = false
	return nil
}

// IncrementToken advances to the next token, composing and normalizing
// consecutive numeral tokens.
func (f *JapaneseNumberFilter) IncrementToken() (bool, error) {
	src := f.GetAttributeSource()

	// Emit previously captured token we read past earlier.
	if f.state != nil {
		src.RestoreState(f.state)
		f.state = nil
		return true, nil
	}

	if f.exhausted {
		return false, nil
	}

	ok, err := f.GetInput().IncrementToken()
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
	composedNumberToken := false
	startOffset := 0
	endOffset := 0
	preCompositionState := src.CaptureState()
	term := f.termAttr.String()
	numeralTerm := f.isNumeral(term)

	for moreTokens && numeralTerm {
		if !composedNumberToken {
			if f.offsetAttr != nil {
				startOffset = f.offsetAttr.StartOffset()
			}
			composedNumberToken = true
		}
		if f.offsetAttr != nil {
			endOffset = f.offsetAttr.EndOffset()
		}
		moreTokens, err = f.GetInput().IncrementToken()
		if err != nil {
			return false, err
		}
		if !moreTokens {
			f.exhausted = true
		}

		if f.posIncrAttr != nil && f.posIncrAttr.GetPositionIncrement() == 0 {
			// Stacked synonym token: save it, restore pre-composition state.
			posLen := 1
			if f.posLenAttr != nil {
				posLen = f.posLenAttr.GetPositionLength()
			}
			f.fallThroughTokens = posLen - 1
			f.state = src.CaptureState()
			src.RestoreState(preCompositionState)
			return moreTokens, nil
		}

		f.numeral.WriteString(term)

		if moreTokens {
			term = f.termAttr.String()
			numeralTerm = f.isNumeral(term) || f.isNumeralPunctuation(term)
		}
	}

	if composedNumberToken {
		if moreTokens {
			f.state = src.CaptureState()
		}
		normalizedNumber := f.NormalizeNumber(f.numeral.String())
		if f.termAttr != nil {
			f.termAttr.SetValue(normalizedNumber)
		}
		if f.offsetAttr != nil {
			f.offsetAttr.SetOffset(startOffset, endOffset)
		}
		f.numeral.Reset()
		return true, nil
	}
	return moreTokens, nil
}

// NormalizeNumber normalizes a Japanese number string to Arabic decimal form.
func (f *JapaneseNumberFilter) NormalizeNumber(number string) string {
	defer func() {
		recover() //nolint:errcheck // on arithmetic panic, return original number
	}()
	buf := NewNumberBuffer(number)
	result := parseNumber(buf)
	if result == nil {
		return number
	}
	return stripTrailingZerosRat(result)
}

// stripTrailingZerosRat converts a *big.Rat to plain string, removing trailing
// zeros, mirroring BigDecimal.stripTrailingZeros().toPlainString().
func stripTrailingZerosRat(r *big.Rat) string {
	if r == nil {
		return "0"
	}
	// Use floating-point representation
	f := new(big.Float).SetRat(r)
	s := f.Text('f', 20) // enough precision
	// Strip trailing zeros after decimal point
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	return s
}

func parseNumber(buf *NumberBuffer) *big.Rat {
	sum := new(big.Rat)
	result := parseLargePair(buf)
	if result == nil {
		return nil
	}
	for result != nil {
		sum.Add(sum, result)
		result = parseLargePair(buf)
	}
	return sum
}

func parseLargePair(buf *NumberBuffer) *big.Rat {
	first := parseMediumNumber(buf)
	second := parseLargeKanjiNumeral(buf)
	if first == nil && second == nil {
		return nil
	}
	if second == nil {
		return first
	}
	if first == nil {
		return second
	}
	return new(big.Rat).Mul(first, second)
}

func parseMediumNumber(buf *NumberBuffer) *big.Rat {
	sum := new(big.Rat)
	result := parseMediumPair(buf)
	if result == nil {
		return nil
	}
	for result != nil {
		sum.Add(sum, result)
		result = parseMediumPair(buf)
	}
	return sum
}

func parseMediumPair(buf *NumberBuffer) *big.Rat {
	first := parseBasicNumber(buf)
	second := parseMediumKanjiNumeral(buf)
	if first == nil && second == nil {
		return nil
	}
	if second == nil {
		return first
	}
	if first == nil {
		return second
	}
	return new(big.Rat).Mul(first, second)
}

func parseBasicNumber(buf *NumberBuffer) *big.Rat {
	var sb strings.Builder
	i := buf.Position()
	runes := []rune(buf.s)
	for i < len(runes) {
		c := runes[i]
		if isArabicNumeral(c) {
			sb.WriteRune(rune('0' + arabicNumeralValue(c)))
		} else if c < 0x10000 && numerals[c] != noNumeral {
			sb.WriteRune(rune('0' + numerals[c]))
		} else if isDecimalPoint(c) {
			sb.WriteByte('.')
		} else if isThousandSeparator(c) {
			// skip
		} else {
			break
		}
		i++
		buf.Advance()
	}
	if sb.Len() == 0 {
		return nil
	}
	r, ok := new(big.Rat).SetString(sb.String())
	if !ok {
		return nil
	}
	return r
}

func parseLargeKanjiNumeral(buf *NumberBuffer) *big.Rat {
	if buf.Position() >= buf.Length() {
		return nil
	}
	c := buf.CharAt(buf.Position())
	if c >= 0x10000 {
		return nil
	}
	power := int(exponents[c])
	if power > 3 {
		buf.Advance()
		return ratPow10(power)
	}
	return nil
}

func parseMediumKanjiNumeral(buf *NumberBuffer) *big.Rat {
	if buf.Position() >= buf.Length() {
		return nil
	}
	c := buf.CharAt(buf.Position())
	if c >= 0x10000 {
		return nil
	}
	power := int(exponents[c])
	if power >= 1 && power <= 3 {
		buf.Advance()
		return ratPow10(power)
	}
	return nil
}

func ratPow10(n int) *big.Rat {
	r := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n)), nil)
	return new(big.Rat).SetInt(r)
}

func isArabicNumeral(c rune) bool {
	return (c >= '0' && c <= '9') || (c >= '０' && c <= '９')
}

func arabicNumeralValue(c rune) int {
	if c >= '0' && c <= '9' {
		return int(c - '0')
	}
	return int(c - '０')
}

func isDecimalPoint(c rune) bool { return c == '.' || c == '．' }

func isThousandSeparator(c rune) bool { return c == ',' || c == '，' }

// IsNumeral returns true if every character in input is a numeral character.
func (f *JapaneseNumberFilter) isNumeral(input string) bool {
	for _, c := range input {
		if !f.isNumeralChar(c) {
			return false
		}
	}
	return len(input) > 0
}

func (f *JapaneseNumberFilter) isNumeralChar(c rune) bool {
	return isArabicNumeral(c) ||
		(c < 0x10000 && numerals[c] != noNumeral) ||
		(c < 0x10000 && exponents[c] > 0)
}

// IsNumeralPunctuation returns true if every character in input is numeral
// punctuation (decimal point or thousand separator).
func (f *JapaneseNumberFilter) isNumeralPunctuation(input string) bool {
	for _, c := range input {
		if !isDecimalPoint(c) && !isThousandSeparator(c) {
			return false
		}
	}
	return len(input) > 0
}

// Ensure JapaneseNumberFilter implements analysis.TokenFilter.
var _ analysis.TokenFilter = (*JapaneseNumberFilter)(nil)
