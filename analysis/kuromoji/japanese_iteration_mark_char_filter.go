// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package kuromoji

import (
	"io"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// Iteration mark characters.
const (
	kanjiIterationMark           = '々' // 々
	hiraganaIterationMark        = 'ゝ' // ゝ
	hiraganaVoicedIterationMark  = 'ゞ' // ゞ
	katakanaIterationMark        = 'ヽ' // ヽ
	katakanaVoicedIterationMark  = 'ヾ' // ヾ
	fullStopPunctuation          = '。' // 。

	// NormalizeKanjiDefault is the default for kanji iteration mark normalization.
	NormalizeKanjiDefault = true
	// NormalizeKanaDefault is the default for kana iteration mark normalization.
	NormalizeKanaDefault = true
)

// Hiragana to dakuten map (indexed by code point - 0x304b / か).
var h2d [50]rune

// Katakana to dakuten map (indexed by code point - 0x30ab / カ).
var k2d [50]rune

func init() {
	h2d[0] = 'が'  // か => が
	h2d[1] = 'が'  // が => が
	h2d[2] = 'ぎ'  // き => ぎ
	h2d[3] = 'ぎ'  // ぎ => ぎ
	h2d[4] = 'ぐ'  // く => ぐ
	h2d[5] = 'ぐ'  // ぐ => ぐ
	h2d[6] = 'げ'  // け => げ
	h2d[7] = 'げ'  // げ => げ
	h2d[8] = 'ご'  // こ => ご
	h2d[9] = 'ご'  // ご => ご
	h2d[10] = 'ざ' // さ => ざ
	h2d[11] = 'ざ' // ざ => ざ
	h2d[12] = 'じ' // し => じ
	h2d[13] = 'じ' // じ => じ
	h2d[14] = 'ず' // す => ず
	h2d[15] = 'ず' // ず => ず
	h2d[16] = 'ぜ' // せ => ぜ
	h2d[17] = 'ぜ' // ぜ => ぜ
	h2d[18] = 'ぞ' // そ => ぞ
	h2d[19] = 'ぞ' // ぞ => ぞ
	h2d[20] = 'だ' // た => だ
	h2d[21] = 'だ' // だ => だ
	h2d[22] = 'ぢ' // ち => ぢ
	h2d[23] = 'ぢ' // ぢ => ぢ
	h2d[24] = 'っ'
	h2d[25] = 'づ' // つ => づ
	h2d[26] = 'づ' // づ => づ
	h2d[27] = 'で' // て => で
	h2d[28] = 'で' // で => で
	h2d[29] = 'ど' // と => ど
	h2d[30] = 'ど' // ど => ど
	h2d[31] = 'な'
	h2d[32] = 'に'
	h2d[33] = 'ぬ'
	h2d[34] = 'ね'
	h2d[35] = 'の'
	h2d[36] = 'ば' // は => ば
	h2d[37] = 'ば' // ば => ば
	h2d[38] = 'ぱ'
	h2d[39] = 'び' // ひ => び
	h2d[40] = 'び' // び => び
	h2d[41] = 'ぴ'
	h2d[42] = 'ぶ' // ふ => ぶ
	h2d[43] = 'ぶ' // ぶ => ぶ
	h2d[44] = 'ぷ'
	h2d[45] = 'べ' // へ => べ
	h2d[46] = 'べ' // べ => べ
	h2d[47] = 'ぺ'
	h2d[48] = 'ぼ' // ほ => ぼ
	h2d[49] = 'ぼ' // ぼ => ぼ

	const diff = 'カ' - 'か' // カ - か
	for i := range k2d {
		k2d[i] = h2d[i] + diff
	}
}

// JapaneseIterationMarkCharFilter normalizes Japanese horizontal iteration
// marks (odoriji) to their expanded form.
//
// Kanji iteration mark: 々 (U+3005)
// Hiragana iteration marks: ゝ (U+309D), ゞ (U+309E)
// Katakana iteration marks: ヽ (U+30FD), ヾ (U+30FE)
//
// This is the Go port of
// org.apache.lucene.analysis.ja.JapaneseIterationMarkCharFilter from Apache
// Lucene 10.4.0.
type JapaneseIterationMarkCharFilter struct {
	*analysis.CharFilter
	// runes holds the entire input as a rune slice (buffered up to 。 or EOF).
	runes  []rune
	pos    int
	output []rune // fully expanded output

	normalizeKanji bool
	normalizeKana  bool
}

// NewJapaneseIterationMarkCharFilter creates a filter that normalizes
// iteration marks with independent kanji and kana normalization flags.
func NewJapaneseIterationMarkCharFilter(input io.Reader, normalizeKanji, normalizeKana bool) *JapaneseIterationMarkCharFilter {
	f := &JapaneseIterationMarkCharFilter{
		CharFilter:     analysis.NewCharFilter(input),
		normalizeKanji: normalizeKanji,
		normalizeKana:  normalizeKana,
	}
	return f
}

// NewJapaneseIterationMarkCharFilterDefault creates a filter with both kanji
// and kana normalization enabled.
func NewJapaneseIterationMarkCharFilterDefault(input io.Reader) *JapaneseIterationMarkCharFilter {
	return NewJapaneseIterationMarkCharFilter(input, NormalizeKanjiDefault, NormalizeKanaDefault)
}

// Read satisfies io.Reader. Expands iteration marks in the input and writes
// normalized bytes into p.
func (f *JapaneseIterationMarkCharFilter) Read(p []byte) (int, error) {
	if f.output == nil {
		if err := f.loadAndExpand(); err != nil && err != io.EOF {
			return 0, err
		}
	}
	if f.pos >= len(f.output) {
		return 0, io.EOF
	}
	// encode remaining output runes into p
	s := string(f.output[f.pos:])
	b := []byte(s)
	n := copy(p, b)
	// advance by runes corresponding to the n bytes consumed
	consumed := string(b[:n])
	f.pos += len([]rune(consumed))
	return n, nil
}

// loadAndExpand reads the entire input and expands iteration marks.
func (f *JapaneseIterationMarkCharFilter) loadAndExpand() error {
	var sb strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := f.CharFilter.Read(buf)
		if n > 0 {
			sb.Write(buf[:n])
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
	}
	runes := []rune(sb.String())
	f.output = expandIterationMarks(runes, f.normalizeKanji, f.normalizeKana)
	f.pos = 0
	return nil
}

// expandIterationMarks returns a new rune slice with iteration marks replaced.
func expandIterationMarks(runes []rune, normalizeKanji, normalizeKana bool) []rune {
	out := make([]rune, 0, len(runes))
	spanEnd := 0 // position past last handled iteration-mark span

	for i, c := range runes {
		if !isIterationMark(c, normalizeKanji, normalizeKana) {
			out = append(out, c)
			if c == fullStopPunctuation {
				spanEnd = i + 1
			}
			continue
		}
		// iteration mark
		if i < spanEnd {
			// inside previous span
			src := sourceChar(runes, i, out, spanEnd)
			out = append(out, normalizeRune(src, c, normalizeKanji, normalizeKana))
			continue
		}
		if i == spanEnd {
			// illegal: starts exactly where previous span ended
			out = append(out, c)
			spanEnd = i + 1
			continue
		}
		// new span
		spanSize := countSpan(runes, i, normalizeKanji, normalizeKana, spanEnd)
		spanEnd = i + spanSize
		src := sourceCharForNewSpan(runes, i, spanSize, out)
		out = append(out, normalizeRune(src, c, normalizeKanji, normalizeKana))
	}
	return out
}

func sourceChar(runes []rune, pos int, out []rune, spanEnd int) rune {
	// The source character is spanSize positions back in the output.
	// spanSize is stored implicitly: pos - (last span start) characters back.
	// Simpler: look spanSize back in out.
	_ = spanEnd
	if len(out) == 0 {
		return runes[pos]
	}
	return out[len(out)-1]
}

func sourceCharForNewSpan(runes []rune, pos, spanSize int, out []rune) rune {
	// Source is spanSize positions back in the output buffer.
	backPos := len(out) - spanSize
	if backPos < 0 || backPos >= len(out) {
		// Can't look back that far — return the raw char at pos-spanSize in input.
		srcIdx := pos - spanSize
		if srcIdx >= 0 && srcIdx < len(runes) {
			return runes[srcIdx]
		}
		return runes[pos]
	}
	return out[backPos]
}

func countSpan(runes []rune, start int, normalizeKanji, normalizeKana bool, prevSpanEnd int) int {
	count := 0
	for i := start; i < len(runes) && isIterationMark(runes[i], normalizeKanji, normalizeKana); i++ {
		count++
	}
	// Restrict so we don't go past the previous span end.
	if start-count < prevSpanEnd {
		count = start - prevSpanEnd
	}
	if count < 0 {
		count = 0
	}
	return count
}

func isIterationMark(c rune, normalizeKanji, normalizeKana bool) bool {
	if normalizeKanji && c == kanjiIterationMark {
		return true
	}
	if normalizeKana {
		return c == hiraganaIterationMark || c == hiraganaVoicedIterationMark ||
			c == katakanaIterationMark || c == katakanaVoicedIterationMark
	}
	return false
}

func normalizeRune(src, mark rune, _, normalizeKana bool) rune {
	if !normalizeKana {
		// kanji: just return src
		return src
	}
	switch mark {
	case hiraganaIterationMark:
		if isHiraganaDakuten(src) {
			return src - 1
		}
		return src
	case hiraganaVoicedIterationMark:
		return lookupH2D(src)
	case katakanaIterationMark:
		if isKatakanaDakuten(src) {
			return src - 1
		}
		return src
	case katakanaVoicedIterationMark:
		return lookupK2D(src)
	}
	return src // kanji or unrecognised
}

func lookupH2D(c rune) rune {
	const base = 'か'
	idx := c - base
	if idx >= 0 && int(idx) < len(h2d) {
		return h2d[idx]
	}
	return c
}

func lookupK2D(c rune) rune {
	const base = 'カ'
	idx := c - base
	if idx >= 0 && int(idx) < len(k2d) {
		return k2d[idx]
	}
	return c
}

func isHiraganaDakuten(c rune) bool {
	const base = 'か'
	idx := c - base
	if idx < 0 || int(idx) >= len(h2d) {
		return false
	}
	return h2d[idx] == c
}

func isKatakanaDakuten(c rune) bool {
	const base = 'カ'
	idx := c - base
	if idx < 0 || int(idx) >= len(k2d) {
		return false
	}
	return k2d[idx] == c
}
