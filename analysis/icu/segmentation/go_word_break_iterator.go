// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package segmentation

import "unicode"

// goWordBreakIterator is a Go-native approximation of ICU4J's
// RuleBasedBreakIterator for UAX#29 word segmentation.
//
// Deviation: ICU4J loads compiled binary rule files (.brk) and applies a
// full RBBI algorithm including dictionary-based segmentation for CJK, Thai,
// Lao, Myanmar, and Khmer. This implementation uses Go's stdlib unicode
// range tables to classify runes, which approximates word boundaries but does
// not perform dictionary-based segmentation. For CJK text, each ideographic
// character is treated as its own word token (matching Lucene's IDEOGRAPHIC
// token type behaviour). Thai/Lao/Myanmar/Khmer text is left unsegmented
// within script runs (no dictionary). This is sufficient for the ICUTokenizer
// infrastructure and for scripts that do not require dictionary segmentation.
type goWordBreakIterator struct {
	text    []rune
	start   int
	length  int
	current int
	status  int
}

// newGoWordBreakIterator creates a new iterator. SetText must be called before use.
func newGoWordBreakIterator() *goWordBreakIterator {
	return &goWordBreakIterator{}
}

// SetText configures the iterator to scan text[start : start+length].
func (g *goWordBreakIterator) SetText(text []rune, start, length int) {
	g.text = text
	g.start = start
	g.length = length
	g.current = 0
	g.status = RuleStatusWordNone
}

// Current returns the current break position (relative to the script run start).
func (g *goWordBreakIterator) Current() int {
	return g.current
}

// GetRuleStatus returns the rule-status tag for the most recent break.
func (g *goWordBreakIterator) GetRuleStatus() int {
	return g.status
}

// Next advances to the next break position and returns it, or Done.
//
// The algorithm:
//  1. Skip any whitespace/punctuation at the current position (these are
//     non-token spans — reported with RuleStatusWordNone).
//  2. Find the end of the next word run (letters/digits/CJK/etc.).
//  3. Return the new position.
//
// This produces the same effect as ICU4J's word break iterator for the common
// case: alternating non-token and token spans.
func (g *goWordBreakIterator) Next() int {
	limit := g.length
	pos := g.current

	if pos >= limit {
		g.status = RuleStatusWordNone
		return Done
	}

	r := g.text[g.start+pos]

	if isWordRune(r) {
		// Scan forward to the end of the word run.
		// CJK ideographs are each their own token.
		if isIdeographic(r) {
			pos++
			g.status = RuleStatusWordIdeo
			g.current = pos
			return pos
		}
		end := pos + 1
		for end < limit {
			next := g.text[g.start+end]
			if !isWordRune(next) || isIdeographic(next) {
				break
			}
			end++
		}
		// Determine status from the first rune.
		g.status = wordStatus(r)
		g.current = end
		return end
	}

	// Non-word span: consume until the next word rune.
	end := pos + 1
	for end < limit {
		next := g.text[g.start+end]
		if isWordRune(next) {
			break
		}
		end++
	}
	g.status = RuleStatusWordNone
	g.current = end
	return end
}

// Clone returns an independent copy of this break iterator.
func (g *goWordBreakIterator) Clone() RuleBasedBreakIterator {
	cp := *g
	return &cp
}

// isWordRune reports whether r is a word character (letter, digit, or
// CJK ideograph/syllable/kana).
func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) ||
		unicode.Is(unicode.Han, r) ||
		unicode.Is(unicode.Hiragana, r) ||
		unicode.Is(unicode.Katakana, r) ||
		unicode.Is(unicode.Hangul, r) ||
		unicode.Is(unicode.Thai, r) ||
		unicode.Is(unicode.Lao, r) ||
		unicode.Is(unicode.Myanmar, r) ||
		unicode.Is(unicode.Khmer, r)
}

// isIdeographic reports whether r is a CJK ideographic character (Han only).
// Hiragana and Katakana are kept together with adjacent kana runes.
func isIdeographic(r rune) bool {
	return unicode.Is(unicode.Han, r)
}

// wordStatus returns the RuleStatus appropriate for rune r.
func wordStatus(r rune) int {
	switch {
	case unicode.Is(unicode.Han, r):
		return RuleStatusWordIdeo
	case unicode.Is(unicode.Hiragana, r):
		return RuleStatusWordKana
	case unicode.Is(unicode.Katakana, r):
		return RuleStatusWordKana
	case unicode.IsDigit(r):
		return RuleStatusWordNumber
	default:
		return RuleStatusWordLetter
	}
}

// Ensure goWordBreakIterator implements RuleBasedBreakIterator.
var _ RuleBasedBreakIterator = (*goWordBreakIterator)(nil)
