// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package segmentation

// BreakIteratorWrapper wraps a RuleBasedBreakIterator, making object reuse
// convenient and emitting a rule status for emoji sequences.
//
// Go port of
// org.apache.lucene.analysis.icu.segmentation.BreakIteratorWrapper
// (Apache Lucene 10.4.0).
type BreakIteratorWrapper struct {
	textIterator *CharArrayIterator
	rbbi         RuleBasedBreakIterator
	text         []rune
	start        int
	status       int
}

// NewBreakIteratorWrapper creates a BreakIteratorWrapper around rbbi.
func NewBreakIteratorWrapper(rbbi RuleBasedBreakIterator) *BreakIteratorWrapper {
	return &BreakIteratorWrapper{
		textIterator: NewCharArrayIterator(),
		rbbi:         rbbi,
	}
}

// Current returns the current break position.
func (bw *BreakIteratorWrapper) Current() int {
	return bw.rbbi.Current()
}

// GetRuleStatus returns the rule-status for the most recent Next() call.
func (bw *BreakIteratorWrapper) GetRuleStatus() int {
	return bw.status
}

// Next advances to the next break position and returns it, or Done.
func (bw *BreakIteratorWrapper) Next() int {
	current := bw.rbbi.Current()
	next := bw.rbbi.Next()
	bw.status = bw.calcStatus(current, next)
	return next
}

// calcStatus determines the effective rule status for the span [current, next).
// Emoji sequences get the EmojiSequenceStatus override.
func (bw *BreakIteratorWrapper) calcStatus(current, next int) int {
	if next != Done && bw.isEmoji(current, next) {
		return EmojiSequenceStatus
	}
	return bw.rbbi.GetRuleStatus()
}

// isEmoji reports whether the code point at text[bw.start+current] is an
// emoji character.
func (bw *BreakIteratorWrapper) isEmoji(current, next int) bool {
	if bw.text == nil {
		return false
	}
	begin := bw.start + current
	end := bw.start + next
	if begin >= len(bw.text) || end > len(bw.text) {
		return false
	}
	r := bw.text[begin]
	// Emoji characters are in the Emoji unicode property.
	if !isEmojiRune(r) {
		return false
	}
	if isEmojiRKRune(r) {
		// RK emoji only count as emoji if followed by a presentation selector
		// (U+FE0F) or a combining enclosing keycap (U+20E3).
		trailer := begin + 1
		return trailer < end && (bw.text[trailer] == 0xFE0F || bw.text[trailer] == 0x20E3)
	}
	return true
}

// SetText sets the text region to analyse.
func (bw *BreakIteratorWrapper) SetText(text []rune, start, length int) {
	bw.text = text
	bw.start = start
	bw.textIterator.SetText(text, start, length)
	bw.rbbi.SetText(text, start, length)
	bw.status = RuleStatusWordNone
}

// isEmojiRune reports whether r has the Emoji or Extended_Pictographic property.
//
// Deviation: the Java implementation uses ICU4J's UnicodeSet for a comprehensive
// emoji property lookup. This Go implementation uses a curated set of known emoji
// codepoint ranges derived from Unicode 15 Emoji data, sufficient for tokenization.
// The range 0x2122..0x3299 used previously was too broad (it included ideographic
// punctuation like U+3002 IDEOGRAPHIC FULL STOP). Only specific symbols in that
// block that carry the Emoji property are included here.
func isEmojiRune(r rune) bool {
	switch {
	// Latin/common emoji
	case r == 0x00A9 || r == 0x00AE: // ©, ®
		return true
	case r == 0x203C || r == 0x2049: // ‼, ⁉
		return true
	// Specific emoji symbols (not the full 0x2122..0x3299 range)
	case r == 0x2122: // ™
		return true
	case r == 0x2139: // ℹ
		return true
	case r >= 0x2194 && r <= 0x2199: // ↔ through ↙
		return true
	case r >= 0x21A9 && r <= 0x21AA: // ↩ ↪
		return true
	case r == 0x231A || r == 0x231B: // ⌚ ⌛
		return true
	case r == 0x2328: // ⌨
		return true
	case r == 0x23CF: // ⏏
		return true
	case r >= 0x23E9 && r <= 0x23F3: // ⏩ through ⏳
		return true
	case r >= 0x23F8 && r <= 0x23FA: // ⏸ ⏹ ⏺
		return true
	case r == 0x24C2: // Ⓜ
		return true
	case r >= 0x25AA && r <= 0x25AB: // ▪ ▫
		return true
	case r == 0x25B6 || r == 0x25C0: // ▶ ◀
		return true
	case r >= 0x25FB && r <= 0x25FE: // ◻ ◼ ◽ ◾
		return true
	case r >= 0x2600 && r <= 0x2604: // ☀ through ☄
		return true
	case r == 0x260E: // ☎
		return true
	case r >= 0x2611 && r <= 0x2614: // ☑ ☒ ☔
		return true
	case r == 0x2618 || r == 0x261D: // ☘ ☝
		return true
	case r == 0x2620 || r == 0x2622 || r == 0x2623: // ☠ ☢ ☣
		return true
	case r == 0x2626 || r == 0x262A || r == 0x262E || r == 0x262F: // ☦ ☪ ☮ ☯
		return true
	case r >= 0x2638 && r <= 0x263A: // ☸ ☹ ☺
		return true
	case r == 0x2640 || r == 0x2642: // ♀ ♂
		return true
	case r >= 0x2648 && r <= 0x2653: // ♈ through ♓
		return true
	case r >= 0x265F && r <= 0x2660: // ♟ ♠
		return true
	case r == 0x2663 || r == 0x2665 || r == 0x2666 || r == 0x2668: // ♣ ♥ ♦ ♨
		return true
	case r >= 0x267B && r <= 0x267F: // ♻ through ♿
		return true
	case r >= 0x2692 && r <= 0x2697: // ⚒ through ⚗
		return true
	case r == 0x2699 || r == 0x269B || r == 0x269C: // ⚙ ⚛ ⚜
		return true
	case r >= 0x26A0 && r <= 0x26A1: // ⚠ ⚡
		return true
	case r >= 0x26AA && r <= 0x26AB: // ⚪ ⚫
		return true
	case r >= 0x26B0 && r <= 0x26B1: // ⚰ ⚱
		return true
	case r >= 0x26BD && r <= 0x26BE: // ⚽ ⚾
		return true
	case r >= 0x26C4 && r <= 0x26C5: // ⛄ ⛅
		return true
	case r >= 0x26CE && r <= 0x26CF: // ⛎ ⛏
		return true
	case r == 0x26D1 || r == 0x26D3 || r == 0x26D4: // ⛑ ⛓ ⛔
		return true
	case r >= 0x26E9 && r <= 0x26EA: // ⛩ ⛪
		return true
	case r >= 0x26F0 && r <= 0x26F5: // ⛰ through ⛵
		return true
	case r >= 0x26F7 && r <= 0x26FA: // ⛷ through ⛺
		return true
	case r == 0x26FD: // ⛽
		return true
	case r == 0x2702 || r == 0x2705: // ✂ ✅
		return true
	case r >= 0x2708 && r <= 0x270D: // ✈ through ✍
		return true
	case r == 0x270F: // ✏
		return true
	case r == 0x2712 || r == 0x2714 || r == 0x2716: // ✒ ✔ ✖
		return true
	case r == 0x271D: // ✝
		return true
	case r == 0x2721 || r == 0x2728: // ✡ ✨
		return true
	case r == 0x2733 || r == 0x2734: // ✳ ✴
		return true
	case r == 0x2744 || r == 0x2747: // ❄ ❇
		return true
	case r == 0x274C || r == 0x274E: // ❌ ❎
		return true
	case r >= 0x2753 && r <= 0x2755: // ❓ ❔ ❕
		return true
	case r == 0x2757: // ❗
		return true
	case r >= 0x2763 && r <= 0x2764: // ❣ ❤
		return true
	case r >= 0x2795 && r <= 0x2797: // ➕ ➖ ➗
		return true
	case r == 0x27A1: // ➡
		return true
	case r == 0x27B0 || r == 0x27BF: // ➰ ➿
		return true
	case r >= 0x2934 && r <= 0x2935: // ⤴ ⤵
		return true
	case r >= 0x2B05 && r <= 0x2B07: // ⬅ ⬆ ⬇
		return true
	case r >= 0x2B1B && r <= 0x2B1C: // ⬛ ⬜
		return true
	case r == 0x2B50 || r == 0x2B55: // ⭐ ⭕
		return true
	case r == 0x3030 || r == 0x303D: // 〰 〽
		return true
	case r == 0x3297 || r == 0x3299: // ㊗ ㊙
		return true
	// Supplementary planes
	case r >= 0x1F000 && r <= 0x1FFFF: // Mahjong, cards, misc symbols, emoticons, etc.
		return true
	default:
		return false
	}
}

// isEmojiRKRune reports whether r is in the "EmojiRK" set — characters that
// are only treated as emoji when followed by a presentation selector or
// keycap combining character.
// Matches BreakIteratorWrapper.EMOJI_RK = "[*#0-9©®™〰〽]".
func isEmojiRKRune(r rune) bool {
	return r == '*' || r == '#' ||
		(r >= '0' && r <= '9') ||
		r == 0x00A9 || r == 0x00AE || r == 0x2122 ||
		r == 0x3030 || r == 0x303D
}
