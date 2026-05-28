// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
	"unicode"
	"unicode/utf8"
)

// Token type constants emitted by StandardTokenizerImpl.
//
// The integer values are aligned with Lucene's
// org.apache.lucene.analysis.standard.StandardTokenizer constants so
// callers can index TokenTypes by the returned token type.
const (
	// TokenTypeAlphanum is the type for general alphanumeric sequences.
	TokenTypeAlphanum = 0
	// TokenTypeNum is the type for numeric sequences.
	TokenTypeNum = 1
	// TokenTypeSoutheastAsian is the type for runs of South-East Asian
	// (Line_Break:SA / Complex_Context) characters.
	TokenTypeSoutheastAsian = 2
	// TokenTypeIdeographic is the type for a single CJKV ideographic
	// character (Han script).
	TokenTypeIdeographic = 3
	// TokenTypeHiragana is the type for a single Hiragana character.
	TokenTypeHiragana = 4
	// TokenTypeKatakana is the type for a run of Katakana characters.
	TokenTypeKatakana = 5
	// TokenTypeHangul is the type for a run of Hangul characters.
	TokenTypeHangul = 6
	// TokenTypeEmoji is the type for an Emoji sequence (including
	// ZWJ sequences, keycap sequences, modifier sequences, regional
	// indicator pairs, and tag sequences).
	TokenTypeEmoji = 7
)

// StandardTokenTypes maps a token type integer (one of TokenType*) to
// the Lucene-faithful string label emitted via TypeAttribute.
var StandardTokenTypes = [...]string{
	"<ALPHANUM>",
	"<NUM>",
	"<SOUTHEAST_ASIAN>",
	"<IDEOGRAPHIC>",
	"<HIRAGANA>",
	"<KATAKANA>",
	"<HANGUL>",
	"<EMOJI>",
}

// MaxTokenLengthLimit is the absolute upper bound for the configured
// maxTokenLength of [StandardTokenizer]. Matches Lucene's
// org.apache.lucene.analysis.standard.StandardTokenizer.MAX_TOKEN_LENGTH_LIMIT.
const MaxTokenLengthLimit = 1024 * 1024

// yyeof is the sentinel returned by [standardTokenizerImpl.getNextToken]
// at end of input.
const yyeof = -1

// standardTokenizerImpl is the UAX#29 Word Break scanner that
// underpins [StandardTokenizer]. It is the Go translation of the
// JFlex grammar in StandardTokenizerImpl.jflex (Lucene 10.4.0), but
// implemented as a hand-rolled longest-match scanner over a buffered
// rune slice. The behaviour follows Lucene's deviations from raw
// UAX#29:
//
//   - Each base word-break class is folded with the
//     (Extend | Format | ZWJ) closure (the *Ex pseudo-classes from
//     the .jflex file).
//   - Lucene's "emoji_sequence" expansion from TR#51 replaces the
//     default WB3c / WB15 / WB16 rules.
//   - Line_Break:SA (Complex_Context) sequences are kept together
//     as a single SOUTHEAST_ASIAN token.
//
// The scanner is not safe for concurrent use.
type standardTokenizerImpl struct {
	// buf is the decoded rune buffer for the current input. It is
	// rebuilt from the io.Reader on every yyreset call.
	buf []rune
	// pos is the current scan position (advance after each match).
	pos int
	// startRead and markedPos are the [start, end) range of the most
	// recent matched token; getText reads them, yychar reports
	// startRead and yylength reports markedPos-startRead.
	startRead int
	markedPos int
}

// newStandardTokenizerImpl returns an empty scanner. Call yyreset
// before invoking getNextToken.
func newStandardTokenizerImpl() *standardTokenizerImpl {
	return &standardTokenizerImpl{}
}

// yyreset attaches a new input reader, draining it into the rune
// buffer. The previous buffer (and any pending state) is discarded.
func (s *standardTokenizerImpl) yyreset(r io.Reader) error {
	s.buf = s.buf[:0]
	s.pos = 0
	s.startRead = 0
	s.markedPos = 0
	if r == nil {
		return nil
	}
	// Read the entire stream into memory. UAX#29 is inherently
	// look-ahead heavy (Lucene's JFlex DFA buffers as well), and the
	// analyser pipeline normally hands us per-field text, so a
	// single buffered pass keeps the implementation simple without
	// changing the observable contract. The read is bounded by
	// MaxTokenizerInputSize so an oversized stream is rejected with
	// ErrInputTooLarge rather than exhausting memory.
	data, err := readAllLimited(r)
	if err != nil {
		return err
	}
	s.buf = decodeUTF8Runes(s.buf, data)
	return nil
}

// decodeUTF8Runes decodes the bytes into a rune slice, reusing the
// backing array of dst when possible. Invalid bytes are passed
// through as U+FFFD, matching Go's standard string conversion.
func decodeUTF8Runes(dst []rune, data []byte) []rune {
	if cap(dst) >= len(data) {
		dst = dst[:0]
	} else {
		dst = make([]rune, 0, len(data))
	}
	for i := 0; i < len(data); {
		r, size := decodeRune(data[i:])
		dst = append(dst, r)
		i += size
	}
	return dst
}

// decodeRune is a thin wrapper around utf8.DecodeRune. It returns
// U+FFFD on invalid sequences, matching the behaviour expected by
// Lucene-style analysers (which operate on Java chars, but where
// Gocene's contract is "interpret bytes as UTF-8").
func decodeRune(b []byte) (rune, int) {
	return utf8.DecodeRune(b)
}

// getNextToken advances the scanner past whitespace and other
// non-matching characters and returns the type of the next token,
// or yyeof at end of input.
//
// The token's text is exposed via getText / yylength / yychar.
//
// Rule selection uses longest match; ties are broken by the order in
// which rules appear in the JFlex source (Emoji > Num > Hangul >
// Katakana > Word > Southeast Asian > Ideographic > Hiragana).
func (s *standardTokenizerImpl) getNextToken() int {
	n := len(s.buf)
	for s.pos < n {
		bestEnd := 0
		bestType := -1

		if end, ok := s.matchEmoji(s.pos); ok && end-s.pos > bestEnd {
			bestEnd = end - s.pos
			bestType = TokenTypeEmoji
		}
		if end, ok := s.matchNumeric(s.pos); ok && end-s.pos > bestEnd {
			bestEnd = end - s.pos
			bestType = TokenTypeNum
		}
		if end, ok := s.matchHangul(s.pos); ok && end-s.pos > bestEnd {
			bestEnd = end - s.pos
			bestType = TokenTypeHangul
		}
		if end, ok := s.matchKatakana(s.pos); ok && end-s.pos > bestEnd {
			bestEnd = end - s.pos
			bestType = TokenTypeKatakana
		}
		if end, ok := s.matchWord(s.pos); ok && end-s.pos > bestEnd {
			bestEnd = end - s.pos
			bestType = TokenTypeAlphanum
		}
		if end, ok := s.matchSoutheastAsian(s.pos); ok && end-s.pos > bestEnd {
			bestEnd = end - s.pos
			bestType = TokenTypeSoutheastAsian
		}
		if end, ok := s.matchIdeographic(s.pos); ok && end-s.pos > bestEnd {
			bestEnd = end - s.pos
			bestType = TokenTypeIdeographic
		}
		if end, ok := s.matchHiragana(s.pos); ok && end-s.pos > bestEnd {
			bestEnd = end - s.pos
			bestType = TokenTypeHiragana
		}

		if bestType < 0 {
			// No rule matched -- skip the rune.
			s.pos++
			continue
		}
		s.startRead = s.pos
		s.pos += bestEnd
		s.markedPos = s.pos
		return bestType
	}
	return yyeof
}

// yychar returns the start offset (in runes) of the most recent
// match.
func (s *standardTokenizerImpl) yychar() int {
	return s.startRead
}

// yylength returns the length (in runes) of the most recent match.
func (s *standardTokenizerImpl) yylength() int {
	return s.markedPos - s.startRead
}

// getText copies the rune slice of the most recent match into the
// CharTermAttribute as UTF-8 bytes.
func (s *standardTokenizerImpl) getText(t CharTermAttribute) {
	t.SetEmpty()
	chunk := s.buf[s.startRead:s.markedPos]
	buf := t.Grow(len(chunk) * utf8MaxRuneBytes)
	w := 0
	for _, r := range chunk {
		w += encodeRune(buf[w:], r)
	}
	t.SetLength(w)
}

// utf8MaxRuneBytes is the upper bound for any rune's UTF-8
// representation (4 bytes).
const utf8MaxRuneBytes = 4

// encodeRune writes r as UTF-8 into dst and returns the byte count.
// Defers to the standard library to stay byte-for-byte compatible
// with Go's UTF-8 representation.
func encodeRune(dst []byte, r rune) int {
	return utf8.EncodeRune(dst, r)
}

// -----------------------------------------------------------------
// Word-break helpers
//
// The helpers come in two flavours. The *Cls predicates classify a
// single rune against one of the rule's base classes. The consume*
// helpers consume one occurrence of an "Ex" pseudo-class (a base
// rune followed by zero or more Extend|Format|ZWJ runes), advancing
// the position. They return ok=false (and leave pos unchanged) when
// the base class does not match at the given position.
// -----------------------------------------------------------------

// runeAt returns the rune at position i, or -1 when i is past the
// end of the buffer.
func (s *standardTokenizerImpl) runeAt(i int) rune {
	if i < 0 || i >= len(s.buf) {
		return -1
	}
	return s.buf[i]
}

// isExtFmtZwj reports whether r is in WB:Extend, WB:Format or WB:ZWJ.
func isExtFmtZwj(r rune) bool {
	return unicode.Is(wbPropExtend, r) ||
		unicode.Is(wbPropFormat, r) ||
		unicode.Is(wbPropZWJ, r)
}

// isExtFmtZwjSansPresSel matches isExtFmtZwj but excludes the two
// emoji-presentation selectors (U+FE0E text, U+FE0F emoji).
func isExtFmtZwjSansPresSel(r rune) bool {
	if r == 0xFE0E || r == 0xFE0F {
		return false
	}
	return isExtFmtZwj(r)
}

// skipExtFmtZwj advances past any run of (Extend|Format|ZWJ)
// starting at i and returns the new index.
func skipExtFmtZwj(buf []rune, i int) int {
	for i < len(buf) && isExtFmtZwj(buf[i]) {
		i++
	}
	return i
}

// skipExtFmtZwjSansPresSel is the variant used by the emoji rules:
// it does not consume the presentation selectors.
func skipExtFmtZwjSansPresSel(buf []rune, i int) int {
	for i < len(buf) && isExtFmtZwjSansPresSel(buf[i]) {
		i++
	}
	return i
}

// ---- Word/Numeric base classes ----

func isAHLetter(r rune) bool {
	return unicode.Is(wbPropALetter, r) || isWBHebrewLetter(r)
}

func isWBHebrewLetter(r rune) bool {
	return unicode.Is(wbPropHebrewLetter, r)
}

func isNumeric(r rune) bool {
	return unicode.Is(wbPropNumeric, r)
}

func isExtendNumLet(r rune) bool {
	return unicode.Is(wbPropExtendNumLet, r)
}

func isWBKatakana(r rune) bool {
	return unicode.Is(wbPropKatakana, r)
}

func isHangul(r rune) bool {
	return unicode.Is(scriptHangul, r) && isAHLetter(r)
}

func isMidLetter(r rune) bool {
	return unicode.Is(wbPropMidLetter, r) ||
		unicode.Is(wbPropMidNumLet, r) ||
		unicode.Is(wbPropSingleQuote, r)
}

func isMidNumeric(r rune) bool {
	return unicode.Is(wbPropMidNum, r) ||
		unicode.Is(wbPropMidNumLet, r) ||
		unicode.Is(wbPropSingleQuote, r)
}

func isSingleQuote(r rune) bool {
	return unicode.Is(wbPropSingleQuote, r)
}

func isDoubleQuote(r rune) bool {
	return unicode.Is(wbPropDoubleQuote, r)
}

func isComplexContext(r rune) bool {
	return unicode.Is(lbComplexContext, r)
}

func isHanIdeograph(r rune) bool {
	return unicode.Is(scriptHan, r)
}

func isHiragana(r rune) bool {
	return unicode.Is(scriptHiragana, r)
}

func isRegionalIndicator(r rune) bool {
	return unicode.Is(wbPropRegionalIndicator, r)
}

// ---- Emoji helpers ----

// isKeyCapBaseChar matches [0-9#*].
func isKeyCapBaseChar(r rune) bool {
	return (r >= '0' && r <= '9') || r == '#' || r == '*'
}

// isAccidentalEmoji matches [©®™〰〽].
func isAccidentalEmoji(r rune) bool {
	return r == 0x00A9 || r == 0x00AE || r == 0x2122 || r == 0x3030 || r == 0x303D
}

// isEmojiModifier reports whether r is in \p{Emoji_Modifier}.
func isEmojiModifier(r rune) bool {
	return unicode.Is(emojiPropEmojiModifier, r)
}

// isEmojiModifierBase reports whether r is in \p{Emoji_Modifier_Base}.
func isEmojiModifierBase(r rune) bool {
	return unicode.Is(emojiPropEmojiModifierBase, r)
}

// isExtendedPictographic reports whether r is in \p{Extended_Pictographic}.
func isExtendedPictographic(r rune) bool {
	return unicode.Is(emojiPropExtendedPictographic, r)
}

// isEmojiRKAM matches the JFlex macro
// [\p{WB:Regional_Indicator}{KeyCapBaseChar}{AccidentalEmoji}\p{Emoji_Modifier}].
func isEmojiRKAM(r rune) bool {
	return isRegionalIndicator(r) ||
		isKeyCapBaseChar(r) ||
		isAccidentalEmoji(r) ||
		isEmojiModifier(r)
}

// isEmojiSansRKAM matches \p{Emoji} minus EmojiRKAM.
func isEmojiSansRKAM(r rune) bool {
	if !unicode.Is(emojiPropEmoji, r) {
		return false
	}
	return !isEmojiRKAM(r)
}

// isEmojiChar matches (\p{Extended_Pictographic} | EmojiSansRKAM).
func isEmojiChar(r rune) bool {
	return isExtendedPictographic(r) || isEmojiSansRKAM(r)
}

// isZWJ matches \p{WB:ZWJ} (U+200D).
func isZWJ(r rune) bool {
	return unicode.Is(wbPropZWJ, r)
}

// isTagSpec matches [\u{E0020}-\u{E007E}].
func isTagSpec(r rune) bool {
	return r >= 0xE0020 && r <= 0xE007E
}

// isTagTerm matches \u{E007F}.
func isTagTerm(r rune) bool {
	return r == 0xE007F
}

// keyCapCodepoint is U+20E3 COMBINING ENCLOSING KEYCAP.
const keyCapCodepoint = 0x20E3

// emojiPresentationSelector is U+FE0F.
const emojiPresentationSelector = 0xFE0F

// -----------------------------------------------------------------
// Ex-consumer helpers
//
// Each tryConsume*Ex helper attempts to match one occurrence of a
// JFlex {ClassEx} pseudo-class: a base rune satisfying the predicate
// followed by the WB4 closure {ExtFmtZwj}. They return the position
// just past the consumed run and ok=true on success, or (i, false)
// when the base predicate fails.
// -----------------------------------------------------------------

func tryConsumeAHLetterEx(buf []rune, i int) (int, bool) {
	if i >= len(buf) || !isAHLetter(buf[i]) {
		return i, false
	}
	return skipExtFmtZwj(buf, i+1), true
}

func tryConsumeHebrewLetterEx(buf []rune, i int) (int, bool) {
	if i >= len(buf) || !isWBHebrewLetter(buf[i]) {
		return i, false
	}
	return skipExtFmtZwj(buf, i+1), true
}

func tryConsumeNumericEx(buf []rune, i int) (int, bool) {
	if i >= len(buf) || !isNumeric(buf[i]) {
		return i, false
	}
	return skipExtFmtZwj(buf, i+1), true
}

func tryConsumeKatakanaEx(buf []rune, i int) (int, bool) {
	if i >= len(buf) || !isWBKatakana(buf[i]) {
		return i, false
	}
	return skipExtFmtZwj(buf, i+1), true
}

func tryConsumeHangulEx(buf []rune, i int) (int, bool) {
	if i >= len(buf) || !isHangul(buf[i]) {
		return i, false
	}
	return skipExtFmtZwj(buf, i+1), true
}

func tryConsumeMidLetterEx(buf []rune, i int) (int, bool) {
	if i >= len(buf) || !isMidLetter(buf[i]) {
		return i, false
	}
	return skipExtFmtZwj(buf, i+1), true
}

func tryConsumeMidNumericEx(buf []rune, i int) (int, bool) {
	if i >= len(buf) || !isMidNumeric(buf[i]) {
		return i, false
	}
	return skipExtFmtZwj(buf, i+1), true
}

func tryConsumeExtendNumLetEx(buf []rune, i int) (int, bool) {
	if i >= len(buf) || !isExtendNumLet(buf[i]) {
		return i, false
	}
	return skipExtFmtZwj(buf, i+1), true
}

func tryConsumeSingleQuoteEx(buf []rune, i int) (int, bool) {
	if i >= len(buf) || !isSingleQuote(buf[i]) {
		return i, false
	}
	return skipExtFmtZwj(buf, i+1), true
}

func tryConsumeDoubleQuoteEx(buf []rune, i int) (int, bool) {
	if i >= len(buf) || !isDoubleQuote(buf[i]) {
		return i, false
	}
	return skipExtFmtZwj(buf, i+1), true
}

func tryConsumeComplexContextEx(buf []rune, i int) (int, bool) {
	if i >= len(buf) || !isComplexContext(buf[i]) {
		return i, false
	}
	return skipExtFmtZwj(buf, i+1), true
}

func tryConsumeHanEx(buf []rune, i int) (int, bool) {
	if i >= len(buf) || !isHanIdeograph(buf[i]) {
		return i, false
	}
	return skipExtFmtZwj(buf, i+1), true
}

func tryConsumeHiraganaEx(buf []rune, i int) (int, bool) {
	if i >= len(buf) || !isHiragana(buf[i]) {
		return i, false
	}
	return skipExtFmtZwj(buf, i+1), true
}

func tryConsumeRegionalIndicatorEx(buf []rune, i int) (int, bool) {
	if i >= len(buf) || !isRegionalIndicator(buf[i]) {
		return i, false
	}
	return skipExtFmtZwj(buf, i+1), true
}

// tryConsumeKeyCapBaseCharEx consumes a key-cap base char (0-9, #, *)
// followed by the sans-PresSel WB4 closure.
func tryConsumeKeyCapBaseCharEx(buf []rune, i int) (int, bool) {
	if i >= len(buf) || !isKeyCapBaseChar(buf[i]) {
		return i, false
	}
	return skipExtFmtZwjSansPresSel(buf, i+1), true
}

// tryConsumeKeyCapEx consumes the keycap combining mark U+20E3
// followed by the sans-PresSel WB4 closure.
func tryConsumeKeyCapEx(buf []rune, i int) (int, bool) {
	if i >= len(buf) || buf[i] != keyCapCodepoint {
		return i, false
	}
	return skipExtFmtZwjSansPresSel(buf, i+1), true
}

// tryConsumeEmojiCharEx consumes one EmojiChar with the sans-PresSel
// closure.
func tryConsumeEmojiCharEx(buf []rune, i int) (int, bool) {
	if i >= len(buf) || !isEmojiChar(buf[i]) {
		return i, false
	}
	return skipExtFmtZwjSansPresSel(buf, i+1), true
}

// tryConsumeEmojiModifierBaseEx consumes one Emoji_Modifier_Base
// with the sans-PresSel closure.
func tryConsumeEmojiModifierBaseEx(buf []rune, i int) (int, bool) {
	if i >= len(buf) || !isEmojiModifierBase(buf[i]) {
		return i, false
	}
	return skipExtFmtZwjSansPresSel(buf, i+1), true
}

// tryConsumeEmojiModifierEx consumes one Emoji_Modifier with the
// sans-PresSel closure.
func tryConsumeEmojiModifierEx(buf []rune, i int) (int, bool) {
	if i >= len(buf) || !isEmojiModifier(buf[i]) {
		return i, false
	}
	return skipExtFmtZwjSansPresSel(buf, i+1), true
}

// -----------------------------------------------------------------
// Rule matchers
// -----------------------------------------------------------------

// matchSoutheastAsian implements {ComplexContextEx}+.
func (s *standardTokenizerImpl) matchSoutheastAsian(start int) (int, bool) {
	i, ok := tryConsumeComplexContextEx(s.buf, start)
	if !ok {
		return start, false
	}
	for {
		next, ok := tryConsumeComplexContextEx(s.buf, i)
		if !ok {
			break
		}
		i = next
	}
	return i, true
}

// matchIdeographic implements {HanEx} (single occurrence).
func (s *standardTokenizerImpl) matchIdeographic(start int) (int, bool) {
	return tryConsumeHanEx(s.buf, start)
}

// matchHiragana implements {HiraganaEx} (single occurrence).
func (s *standardTokenizerImpl) matchHiragana(start int) (int, bool) {
	return tryConsumeHiraganaEx(s.buf, start)
}

// matchHangul implements {HangulEx}+.
func (s *standardTokenizerImpl) matchHangul(start int) (int, bool) {
	i, ok := tryConsumeHangulEx(s.buf, start)
	if !ok {
		return start, false
	}
	for {
		next, ok := tryConsumeHangulEx(s.buf, i)
		if !ok {
			break
		}
		i = next
	}
	return i, true
}

// matchKatakana implements {KatakanaEx}+.
func (s *standardTokenizerImpl) matchKatakana(start int) (int, bool) {
	i, ok := tryConsumeKatakanaEx(s.buf, start)
	if !ok {
		return start, false
	}
	for {
		next, ok := tryConsumeKatakanaEx(s.buf, i)
		if !ok {
			break
		}
		i = next
	}
	return i, true
}

// matchNumeric implements
//
//	{ExtendNumLetEx}* {NumericEx}
//	( ( {ExtendNumLetEx}* | {MidNumericEx} ) {NumericEx} )*
//	{ExtendNumLetEx}*
//
// The leading ExtendNumLet must not consume the entire match (the
// regex requires at least one NumericEx).
func (s *standardTokenizerImpl) matchNumeric(start int) (int, bool) {
	i := start
	// Leading {ExtendNumLetEx}*.
	for {
		next, ok := tryConsumeExtendNumLetEx(s.buf, i)
		if !ok {
			break
		}
		i = next
	}
	// Mandatory first NumericEx.
	i, ok := tryConsumeNumericEx(s.buf, i)
	if !ok {
		return start, false
	}
	// Trailing groups: ( ({ExtendNumLetEx}* | {MidNumericEx}) {NumericEx} )*
	for {
		j := i
		// Try {ExtendNumLetEx}*.
		for {
			next, ok := tryConsumeExtendNumLetEx(s.buf, j)
			if !ok {
				break
			}
			j = next
		}
		// If no ENL was consumed, try a single MidNumericEx (the
		// regex alternatives are { ENL* | MidNum }; an empty ENL*
		// branch is still valid, so we attempt MidNum only when the
		// ENL closure produced nothing -- but JFlex picks the
		// alternative that allows the longest overall match, which
		// in practice means: if a NumericEx follows directly, the
		// ENL*-empty branch wins; otherwise try MidNum.)
		var sep int
		if j > i {
			sep = j
		} else if mid, ok := tryConsumeMidNumericEx(s.buf, i); ok {
			sep = mid
		} else {
			sep = i // empty ENL branch
		}
		next, ok := tryConsumeNumericEx(s.buf, sep)
		if !ok {
			break
		}
		i = next
	}
	// Trailing {ExtendNumLetEx}*.
	for {
		next, ok := tryConsumeExtendNumLetEx(s.buf, i)
		if !ok {
			break
		}
		i = next
	}
	return i, true
}

// matchWord implements the WORD_TYPE production from
// StandardTokenizerImpl.jflex.
//
// The grammar (abbreviated):
//
//	{ENL}*
//	   ( {KatakanaEx}+
//	   | ( {HebrewLetterEx} ({SingleQuoteEx} | {DoubleQuoteEx}{HebrewLetterEx})
//	     | {NumericEx}      ( ({ENL}* | {MidNumericEx}) {NumericEx} )*
//	     | {AHLetterEx}     ( ({ENL}* | {MidLetterEx}) {AHLetterEx} )*
//	     )+
//	   )
//	   ( {ENL}+
//	     ( {KatakanaEx}+
//	     | ( {HebrewLetterEx} ({SingleQuoteEx} | {DoubleQuoteEx}{HebrewLetterEx})
//	       | {NumericEx} ( ({ENL}* | {MidNumericEx}) {NumericEx} )*
//	       | {AHLetterEx} ( ({ENL}* | {MidLetterEx}) {AHLetterEx} )*
//	       )+
//	     )
//	   )*
//	{ENL}*
//
// The inner pieces (one Hebrew / Numeric / AHLetter "run") are
// factored into matchWordRun; the outer structure orchestrates the
// optional ENL bridges and trailing closure.
func (s *standardTokenizerImpl) matchWord(start int) (int, bool) {
	i := start
	// Leading {ExtendNumLetEx}*.
	for {
		next, ok := tryConsumeExtendNumLetEx(s.buf, i)
		if !ok {
			break
		}
		i = next
	}
	end, ok := matchWordCore(s.buf, i)
	if !ok {
		return start, false
	}
	i = end
	// ( {ENL}+ core )*
	for {
		j := i
		enl := j
		for {
			next, ok := tryConsumeExtendNumLetEx(s.buf, enl)
			if !ok {
				break
			}
			enl = next
		}
		if enl == j {
			break
		}
		end, ok := matchWordCore(s.buf, enl)
		if !ok {
			break
		}
		i = end
	}
	// Trailing {ENL}*.
	for {
		next, ok := tryConsumeExtendNumLetEx(s.buf, i)
		if !ok {
			break
		}
		i = next
	}
	return i, true
}

// matchWordCore matches the inner production of the WORD rule:
//
//	( KatakanaEx+
//	| ( HebrewLetterEx (SingleQuoteEx | DoubleQuoteEx HebrewLetterEx)
//	  | NumericEx      ( (ENL* | MidNumericEx) NumericEx )*
//	  | AHLetterEx     ( (ENL* | MidLetterEx)  AHLetterEx )*
//	  )+
//	)
//
// Returns the post-match position and ok=true when at least one
// alternative produced characters.
func matchWordCore(buf []rune, start int) (int, bool) {
	// First try the {KatakanaEx}+ alternative.
	if end, ok := matchKatakanaRun(buf, start); ok {
		return end, true
	}
	// Otherwise iterate the AHLetter/Hebrew/Numeric alternative
	// at least once.
	i := start
	progressed := false
	for {
		next, ok := matchAHLetterHebrewNumeric(buf, i)
		if !ok {
			break
		}
		progressed = true
		i = next
	}
	if !progressed {
		return start, false
	}
	return i, true
}

// matchKatakanaRun consumes one or more KatakanaEx.
func matchKatakanaRun(buf []rune, start int) (int, bool) {
	i, ok := tryConsumeKatakanaEx(buf, start)
	if !ok {
		return start, false
	}
	for {
		next, ok := tryConsumeKatakanaEx(buf, i)
		if !ok {
			break
		}
		i = next
	}
	return i, true
}

// matchAHLetterHebrewNumeric matches exactly one of the three
// inner alternatives:
//
//	HebrewLetterEx (SingleQuoteEx | DoubleQuoteEx HebrewLetterEx)
//	NumericEx      ( (ENL* | MidNumericEx) NumericEx )*
//	AHLetterEx     ( (ENL* | MidLetterEx)  AHLetterEx )*
//
// The Hebrew alternative is greedier than the AHLetter alternative,
// but is only valid when the leading rune is a Hebrew letter AND a
// SingleQuote or DoubleQuote+Hebrew suffix follows; otherwise the
// shared AHLetter branch handles Hebrew runes uniformly.
func matchAHLetterHebrewNumeric(buf []rune, start int) (int, bool) {
	// Track the longest valid match across the three branches.
	bestEnd := start
	bestOK := false

	// Hebrew alternative.
	if i, ok := tryConsumeHebrewLetterEx(buf, start); ok {
		// Followed by SingleQuoteEx?
		if j, ok := tryConsumeSingleQuoteEx(buf, i); ok {
			if j > bestEnd {
				bestEnd = j
				bestOK = true
			}
		}
		// Or DoubleQuoteEx HebrewLetterEx?
		if j, ok := tryConsumeDoubleQuoteEx(buf, i); ok {
			if k, ok2 := tryConsumeHebrewLetterEx(buf, j); ok2 {
				if k > bestEnd {
					bestEnd = k
					bestOK = true
				}
			}
		}
	}

	// Numeric alternative: {NumericEx} ( ({ENL}* | {MidNumericEx}) {NumericEx} )*
	if i, ok := tryConsumeNumericEx(buf, start); ok {
		end := i
		for {
			j := end
			// Optional ENL* separator.
			enl := j
			for {
				next, ok := tryConsumeExtendNumLetEx(buf, enl)
				if !ok {
					break
				}
				enl = next
			}
			var sep int
			if enl > j {
				sep = enl
			} else if mid, ok := tryConsumeMidNumericEx(buf, j); ok {
				sep = mid
			} else {
				sep = j // empty branch
			}
			next, ok := tryConsumeNumericEx(buf, sep)
			if !ok {
				break
			}
			end = next
		}
		if end > bestEnd {
			bestEnd = end
			bestOK = true
		}
	}

	// AHLetter alternative: {AHLetterEx} ( ({ENL}* | {MidLetterEx}) {AHLetterEx} )*
	if i, ok := tryConsumeAHLetterEx(buf, start); ok {
		end := i
		for {
			j := end
			enl := j
			for {
				next, ok := tryConsumeExtendNumLetEx(buf, enl)
				if !ok {
					break
				}
				enl = next
			}
			var sep int
			if enl > j {
				sep = enl
			} else if mid, ok := tryConsumeMidLetterEx(buf, j); ok {
				sep = mid
			} else {
				sep = j // empty branch
			}
			next, ok := tryConsumeAHLetterEx(buf, sep)
			if !ok {
				break
			}
			end = next
		}
		if end > bestEnd {
			bestEnd = end
			bestOK = true
		}
	}

	if !bestOK {
		return start, false
	}
	return bestEnd, true
}

// matchEmoji implements the EMOJI_TYPE production:
//
//	{EmojiCharOrPresSeqOrModSeq}
//	   ( ( {ZWJ} {EmojiCharOrPresSeqOrModSeq} )*
//	   | {TagSpec}+ {TagTerm}
//	   )
//	| {KeyCapBaseCharEx} {EmojiPresentationSelector}? {KeyCapEx}
//	| {RegionalIndicatorEx}{2}
//
// The three alternatives are tried independently and the longest is
// returned.
func (s *standardTokenizerImpl) matchEmoji(start int) (int, bool) {
	bestEnd := start
	bestOK := false

	// Alternative A: EmojiCharOrPresSeqOrModSeq ( ZWJ ... | TagSpec+ TagTerm )?
	if i, ok := matchEmojiCharOrPresSeqOrModSeq(s.buf, start); ok {
		end := i
		// (ZWJ EmojiCharOrPresSeqOrModSeq)*
		for {
			j := end
			if j >= len(s.buf) || !isZWJ(s.buf[j]) {
				break
			}
			k, ok := matchEmojiCharOrPresSeqOrModSeq(s.buf, j+1)
			if !ok {
				break
			}
			end = k
		}
		// TagSpec+ TagTerm alternative (only valid if no ZWJ run
		// followed the first segment, which our loop above tries
		// greedily anyway; the JFlex grammar uses a separate '|'
		// branch, so we explore it independently from end == i).
		if tagEnd := tryConsumeTagSequence(s.buf, i); tagEnd > end {
			end = tagEnd
		}
		if end > bestEnd {
			bestEnd = end
			bestOK = true
		}
	}

	// Alternative B: KeyCapBaseCharEx PresSel? KeyCapEx
	if i, ok := tryConsumeKeyCapBaseCharEx(s.buf, start); ok {
		j := i
		if j < len(s.buf) && s.buf[j] == emojiPresentationSelector {
			j++
		}
		if k, ok := tryConsumeKeyCapEx(s.buf, j); ok {
			if k > bestEnd {
				bestEnd = k
				bestOK = true
			}
		}
	}

	// Alternative C: RegionalIndicatorEx{2}
	if i, ok := tryConsumeRegionalIndicatorEx(s.buf, start); ok {
		if j, ok := tryConsumeRegionalIndicatorEx(s.buf, i); ok {
			if j > bestEnd {
				bestEnd = j
				bestOK = true
			}
		}
	}

	if !bestOK {
		return start, false
	}
	return bestEnd, true
}

// matchEmojiCharOrPresSeqOrModSeq matches the macro
//
//	( {ZWJ}* {EmojiCharEx} {EmojiPresentationSelector}? )
//	| ( ( {ZWJ}* {EmojiModifierBaseEx} )? {EmojiModifierEx} )
func matchEmojiCharOrPresSeqOrModSeq(buf []rune, start int) (int, bool) {
	bestEnd := start
	bestOK := false

	// First alternative: ZWJ* EmojiCharEx PresSel?
	{
		i := start
		for i < len(buf) && isZWJ(buf[i]) {
			i++
		}
		if j, ok := tryConsumeEmojiCharEx(buf, i); ok {
			if j < len(buf) && buf[j] == emojiPresentationSelector {
				j++
			}
			if j > bestEnd {
				bestEnd = j
				bestOK = true
			}
		}
	}

	// Second alternative: (ZWJ* EmojiModifierBaseEx)? EmojiModifierEx
	{
		i := start
		// Try with the optional base sequence first.
		baseStart := i
		for baseStart < len(buf) && isZWJ(buf[baseStart]) {
			baseStart++
		}
		if k, ok := tryConsumeEmojiModifierBaseEx(buf, baseStart); ok {
			if mod, ok2 := tryConsumeEmojiModifierEx(buf, k); ok2 {
				if mod > bestEnd {
					bestEnd = mod
					bestOK = true
				}
			}
		}
		// And without (bare EmojiModifierEx).
		if mod, ok := tryConsumeEmojiModifierEx(buf, i); ok {
			if mod > bestEnd {
				bestEnd = mod
				bestOK = true
			}
		}
	}

	if !bestOK {
		return start, false
	}
	return bestEnd, true
}

// tryConsumeTagSequence consumes a non-empty TagSpec+ run followed
// by a TagTerm. Returns the post-match position and start when no
// valid sequence is present.
func tryConsumeTagSequence(buf []rune, start int) int {
	i := start
	specs := 0
	for i < len(buf) && isTagSpec(buf[i]) {
		i++
		specs++
	}
	if specs == 0 {
		return start
	}
	if i >= len(buf) || !isTagTerm(buf[i]) {
		return start
	}
	return i + 1
}
