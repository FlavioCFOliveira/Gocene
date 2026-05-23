// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package wikipedia

import (
	"io"
	"unicode"
	"unicode/utf8"
)

// Token type integer constants — mirror WikipediaTokenizer.*_ID.
const (
	AlphanumID          = 0
	ApostropheID        = 1
	AcronymID           = 2
	CompanyID           = 3
	EmailID             = 4
	HostID              = 5
	NumID               = 6
	CJID                = 7
	InternalLinkID      = 8
	ExternalLinkID      = 9
	CitationID          = 10
	CategoryID          = 11
	BoldID              = 12
	ItalicsID           = 13
	BoldItalicsID       = 14
	HeadingID           = 15
	SubHeadingID        = 16
	ExternalLinkURLID   = 17

	// YYEOF signals end of input.
	YYEOF = -1
)

// TokenTypes maps token type IDs to their string names.
var TokenTypes = []string{
	"<ALPHANUM>",
	"<APOSTROPHE>",
	"<ACRONYM>",
	"<COMPANY>",
	"<EMAIL>",
	"<HOST>",
	"<NUM>",
	"<CJ>",
	"il",   // INTERNAL_LINK
	"el",   // EXTERNAL_LINK
	"ci",   // CITATION
	"c",    // CATEGORY
	"b",    // BOLD
	"i",    // ITALICS
	"bi",   // BOLD_ITALICS
	"h",    // HEADING
	"sh",   // SUB_HEADING
	"elu",  // EXTERNAL_LINK_URL
}

// scanner states
type wikiState int

const (
	stateInitial          wikiState = iota
	stateCategoryState
	stateInternalLinkState
	stateExternalLinkState
	stateTwoSingleQuotes
	stateThreeSingleQuotes
	stateFiveSingleQuotes
	stateDoubleEquals
	stateDoubleBrace
	stateString
)

// WikipediaTokenizerImpl is a hand-written state-machine scanner that follows
// the JFlex grammar in WikipediaTokenizerImpl.jflex (Apache Lucene 10.4.0).
//
// Go port of org.apache.lucene.analysis.wikipedia.WikipediaTokenizerImpl.
type WikipediaTokenizerImpl struct {
	input io.Reader

	// internal buffer
	buf       []rune
	bufLen    int
	bufPos    int   // current read position in buf
	bufStart  int   // absolute offset of buf[0] in input
	tokenStart int  // absolute start of current token
	tokenEnd   int  // absolute end of current token (exclusive)

	// raw bytes for refill
	rawBuf    []byte
	rawLen    int
	rawPos    int
	eof       bool

	currentTokType    int
	numBalanced       int
	positionInc       int
	numLinkToks       int
	numWikiTokensSeen int

	state wikiState

	// token runes
	tokenText []rune
}

// NewWikipediaTokenizerImpl creates a new scanner reading from r.
func NewWikipediaTokenizerImpl(r io.Reader) *WikipediaTokenizerImpl {
	impl := &WikipediaTokenizerImpl{
		input:       r,
		buf:         make([]rune, 8192),
		rawBuf:      make([]byte, 4096),
		positionInc: 1,
		state:       stateInitial,
	}
	return impl
}

// Reset resets scanner state for a new reader.
func (s *WikipediaTokenizerImpl) Reset(r io.Reader) {
	s.input = r
	s.bufLen = 0
	s.bufPos = 0
	s.bufStart = 0
	s.rawLen = 0
	s.rawPos = 0
	s.eof = false
	s.currentTokType = 0
	s.numBalanced = 0
	s.positionInc = 1
	s.numLinkToks = 0
	s.numWikiTokensSeen = 0
	s.state = stateInitial
}

// YYChar returns the absolute rune offset of the current token start.
func (s *WikipediaTokenizerImpl) YYChar() int { return s.tokenStart }

// YYLength returns the rune length of the last matched token.
func (s *WikipediaTokenizerImpl) YYLength() int { return len(s.tokenText) }

// GetPositionIncrement returns the position increment for the last token.
func (s *WikipediaTokenizerImpl) GetPositionIncrement() int { return s.positionInc }

// GetNumWikiTokensSeen returns the number of tokens seen inside current wiki context.
func (s *WikipediaTokenizerImpl) GetNumWikiTokensSeen() int { return s.numWikiTokensSeen }

// GetText copies the current token text into the supplied []rune (returned as string).
func (s *WikipediaTokenizerImpl) GetText() string { return string(s.tokenText) }

// SetText appends the current token text to b and returns the rune count added.
func (s *WikipediaTokenizerImpl) SetText(b *[]rune) int {
	n := len(s.tokenText)
	*b = append(*b, s.tokenText...)
	return n
}

// YYPushback rewinds the buffer by n runes (for push-back after over-scanning).
func (s *WikipediaTokenizerImpl) YYPushback(n int) {
	if n > len(s.tokenText) {
		n = len(s.tokenText)
	}
	s.bufPos -= n
	if s.bufPos < 0 {
		s.bufPos = 0
	}
}

// ─── main scanner ─────────────────────────────────────────────────────────────

// GetNextToken scans the next token from the input and returns its type ID, or
// YYEOF when input is exhausted.
func (s *WikipediaTokenizerImpl) GetNextToken() int {
	for {
		if _, ok := s.peek(0); !ok {
			return YYEOF
		}

		switch s.state {
		case stateInitial:
			tok := s.scanInitial()
			if tok != -2 { // -2 = continue (no token, advance)
				return tok
			}
		case stateCategoryState:
			tok := s.scanCategory()
			if tok != -2 {
				return tok
			}
		case stateInternalLinkState:
			tok := s.scanInternalLink()
			if tok != -2 {
				return tok
			}
		case stateExternalLinkState:
			tok := s.scanExternalLink()
			if tok != -2 {
				return tok
			}
		case stateTwoSingleQuotes:
			tok := s.scanTwoSingleQuotes()
			if tok != -2 {
				return tok
			}
		case stateThreeSingleQuotes:
			tok := s.scanThreeSingleQuotes()
			if tok != -2 {
				return tok
			}
		case stateFiveSingleQuotes:
			tok := s.scanFiveSingleQuotes()
			if tok != -2 {
				return tok
			}
		case stateDoubleEquals:
			tok := s.scanDoubleEquals()
			if tok != -2 {
				return tok
			}
		case stateDoubleBrace:
			tok := s.scanDoubleBrace()
			if tok != -2 {
				return tok
			}
		case stateString:
			tok := s.scanString()
			if tok != -2 {
				return tok
			}
		default:
			// consume unknown
			s.advance(1)
		}
	}
}

// ─── state handlers ───────────────────────────────────────────────────────────

// scanInitial handles the YYINITIAL state.
func (s *WikipediaTokenizerImpl) scanInitial() int {
	// Try various patterns in priority order.

	// [[Category: or [[:Category:
	if s.hasPrefix("[[Category:") || s.hasPrefix("[[:Category:") {
		s.numWikiTokensSeen = 0
		s.positionInc = 1
		s.currentTokType = CategoryID
		if s.hasPrefix("[[:Category:") {
			s.advance(12)
		} else {
			s.advance(11)
		}
		s.state = stateCategoryState
		return -2
	}
	// [[
	if s.hasPrefix("[[") {
		s.numWikiTokensSeen = 0
		s.positionInc = 1
		s.currentTokType = InternalLinkID
		s.advance(2)
		s.state = stateInternalLinkState
		return -2
	}
	// <ref>
	if s.hasPrefix("<ref>") {
		s.numWikiTokensSeen = 0
		s.positionInc = 1
		s.currentTokType = CitationID
		s.advance(5)
		s.state = stateDoubleBrace
		return -2
	}
	// {{
	if s.hasPrefix("{{") {
		s.numWikiTokensSeen = 0
		s.positionInc = 1
		s.currentTokType = CitationID
		s.advance(2)
		s.state = stateDoubleBrace
		return -2
	}
	// [
	if s.matchRune('[') {
		s.numWikiTokensSeen = 0
		s.positionInc = 1
		s.currentTokType = ExternalLinkURLID
		s.advance(1)
		s.state = stateExternalLinkState
		return -2
	}
	// '''''
	if s.hasPrefix("'''''") {
		s.numWikiTokensSeen = 0
		s.positionInc = 1
		if s.numBalanced == 0 {
			s.numBalanced++
			s.advance(5)
			s.state = stateFiveSingleQuotes
		} else {
			s.numBalanced = 0
			s.advance(5)
		}
		return -2
	}
	// '''
	if s.hasPrefix("'''") {
		s.numWikiTokensSeen = 0
		s.positionInc = 1
		if s.numBalanced == 0 {
			s.numBalanced++
			s.advance(3)
			s.state = stateThreeSingleQuotes
		} else {
			s.numBalanced = 0
			s.advance(3)
		}
		return -2
	}
	// ''
	if s.hasPrefix("''") {
		s.numWikiTokensSeen = 0
		s.positionInc = 1
		if s.numBalanced == 0 {
			s.numBalanced++
			s.advance(2)
			s.state = stateTwoSingleQuotes
		} else {
			s.numBalanced = 0
			s.advance(2)
		}
		return -2
	}
	// ==
	if s.hasPrefix("==") {
		s.numWikiTokensSeen = 0
		s.positionInc = 1
		s.advance(2)
		s.state = stateDoubleEquals
		return -2
	}

	// ALPHANUM / APOSTROPHE / ACRONYM / COMPANY / EMAIL / HOST / NUM / CJ
	if tok := s.scanWordToken(); tok >= 0 {
		return tok
	}

	// CJ (CJK/Japanese/Korean)
	if cjRune, cjOk := s.peek(0); cjOk && isCJ(cjRune) {
		s.tokenStart = s.absPos()
		s.tokenText = s.tokenText[:0]
		for {
			r2, ok2 := s.peek(0)
			if !ok2 || !isCJ(r2) {
				break
			}
			s.tokenText = append(s.tokenText, r2)
			s.bufPos++
		}
		s.tokenEnd = s.absPos()
		s.positionInc = 1
		return CJID
	}

	// ignore everything else
	s.advance(1)
	s.numWikiTokensSeen = 0
	s.positionInc = 1
	return -2
}

// scanWordToken tries to match ALPHANUM, APOSTROPHE, ACRONYM, COMPANY, EMAIL,
// HOST, NUM patterns at the current position. Returns -1 if no match.
func (s *WikipediaTokenizerImpl) scanWordToken() int {
	r, ok := s.peek(0)
	if !ok {
		return -1
	}
	if !isAlpha(r) && !isDigit(r) {
		return -1
	}

	// Accumulate runes directly to avoid buffer-compaction invalidating slice indices.
	s.tokenStart = s.absPos()
	s.tokenText = s.tokenText[:0]
	for {
		r2, ok2 := s.peek(0)
		if !ok2 {
			break
		}
		if isAlpha(r2) || isDigit(r2) || r2 == '\'' || r2 == '.' || r2 == '@' || r2 == '&' ||
			r2 == '-' || r2 == '_' || r2 == '/' || r2 == ',' || r2 == '?' || r2 == '=' || r2 == '#' {
			s.tokenText = append(s.tokenText, r2)
			s.bufPos++
		} else {
			break
		}
	}
	if len(s.tokenText) == 0 {
		return -1
	}
	s.tokenEnd = s.absPos()
	s.positionInc = 1

	text := string(s.tokenText)
	return classifyToken(text)
}

func classifyToken(text string) int {
	runes := []rune(text)
	n := len(runes)

	// APOSTROPHE: alpha ' alpha (internal apostrophes)
	for i := 1; i < n-1; i++ {
		if runes[i] == '\'' && isAlphaRune(runes[i-1]) && isAlphaRune(runes[i+1]) {
			return ApostropheID
		}
	}

	// ACRONYM: alpha "." (alpha ".")+ e.g. U.S.A.
	if n >= 3 {
		isAcronym := true
		i := 0
		for i < n {
			if !isAlphaRune(runes[i]) {
				isAcronym = false
				break
			}
			i++
			if i >= n {
				isAcronym = false
				break
			}
			if runes[i] != '.' {
				isAcronym = false
				break
			}
			i++
		}
		if isAcronym && i == n {
			return AcronymID
		}
	}

	// COMPANY: alpha ("&"|"@") alpha
	for i := 1; i < n-1; i++ {
		if (runes[i] == '&' || runes[i] == '@') && isAlphaRune(runes[i-1]) && isAlphaRune(runes[i+1]) {
			return CompanyID
		}
	}

	// EMAIL: contains @
	for i, r := range runes {
		if r == '@' && i > 0 && i < n-1 {
			return EmailID
		}
	}

	// HOST: alphanum ("." alphanum)+  — multiple dots, no special chars
	hasDot := false
	for _, r := range runes {
		if r == '.' {
			hasDot = true
		} else if !isAlphaRune(r) && !isDigitRune(r) {
			hasDot = false
			break
		}
	}
	if hasDot {
		return HostID
	}

	// NUM: has digit and punctuation
	hasDigit := false
	hasPunct := false
	for _, r := range runes {
		if isDigitRune(r) {
			hasDigit = true
		} else if r == '-' || r == '_' || r == '/' || r == '.' || r == ',' {
			hasPunct = true
		}
	}
	if hasDigit && hasPunct {
		return NumID
	}

	return AlphanumID
}

func (s *WikipediaTokenizerImpl) scanCategory() int {
	if tok := s.scanAlphanumToken(CategoryID); tok >= 0 {
		s.numWikiTokensSeen++
		return tok
	}
	if s.hasPrefix("]]") {
		s.advance(2)
		s.state = stateInitial
		return -2
	}
	s.advance(1)
	s.positionInc = 1
	return -2
}

func (s *WikipediaTokenizerImpl) scanInternalLink() int {
	if tok := s.scanAlphanumToken(s.currentTokType); tok >= 0 {
		s.numWikiTokensSeen++
		s.state = stateInternalLinkState
		return tok
	}
	if s.hasPrefix("]]") {
		s.numLinkToks = 0
		s.advance(2)
		s.state = stateInitial
		return -2
	}
	s.advance(1)
	s.positionInc = 1
	return -2
}

func (s *WikipediaTokenizerImpl) scanExternalLink() int {
	// URL
	if s.hasPrefix("http://") || s.hasPrefix("https://") {
		s.tokenStart = s.absPos()
		s.tokenText = s.tokenText[:0]
		// consume until whitespace or ]
		for {
			r, ok := s.peek(0)
			if !ok || r == ']' || isWhitespace(r) {
				break
			}
			s.tokenText = append(s.tokenText, r)
			s.bufPos++
		}
		s.tokenEnd = s.absPos()
		s.positionInc = 1
		s.numWikiTokensSeen++
		s.state = stateExternalLinkState
		return s.currentTokType
	}
	// ALPHANUM
	if tok := s.scanAlphanumToken(s.currentTokType); tok >= 0 {
		if s.numLinkToks == 0 {
			s.positionInc = 0
		} else {
			s.positionInc = 1
		}
		s.numWikiTokensSeen++
		s.currentTokType = ExternalLinkID
		s.state = stateExternalLinkState
		s.numLinkToks++
		return ExternalLinkID
	}
	if s.matchRune(']') {
		s.numLinkToks = 0
		s.positionInc = 0
		s.advance(1)
		s.state = stateInitial
		return -2
	}
	if r, ok := s.peek(0); ok && isWhitespace(r) {
		s.advance(1)
		s.positionInc = 1
		return -2
	}
	s.advance(1)
	return -2
}

func (s *WikipediaTokenizerImpl) scanTwoSingleQuotes() int {
	if s.hasPrefix("'") {
		s.advance(1)
		s.currentTokType = BoldID
		s.state = stateThreeSingleQuotes
		return -2
	}
	if s.hasPrefix("'''") {
		s.advance(3)
		s.currentTokType = BoldItalicsID
		s.state = stateFiveSingleQuotes
		return -2
	}
	if tok := s.scanAlphanumToken(ItalicsID); tok >= 0 {
		s.currentTokType = ItalicsID
		s.numWikiTokensSeen++
		s.state = stateString
		return ItalicsID
	}
	// handle [[, [[Category:, [
	if s.hasPrefix("[[Category:") || s.hasPrefix("[[:Category:") {
		s.currentTokType = CategoryID
		s.numWikiTokensSeen = 0
		skip := 11
		if s.hasPrefix("[[:Category:") {
			skip = 12
		}
		s.advance(skip)
		s.state = stateCategoryState
		return -2
	}
	if s.hasPrefix("[[") {
		s.currentTokType = InternalLinkID
		s.numWikiTokensSeen = 0
		s.advance(2)
		s.state = stateInternalLinkState
		return -2
	}
	if s.matchRune('[') {
		s.currentTokType = ExternalLinkID
		s.numWikiTokensSeen = 0
		s.advance(1)
		s.state = stateExternalLinkState
		return -2
	}
	s.advance(1)
	return -2
}

func (s *WikipediaTokenizerImpl) scanThreeSingleQuotes() int {
	if tok := s.scanAlphanumToken(s.currentTokType); tok >= 0 {
		s.numWikiTokensSeen++
		s.state = stateString
		return tok
	}
	if s.handleLinkStart() {
		return -2
	}
	s.advance(1)
	return -2
}

func (s *WikipediaTokenizerImpl) scanFiveSingleQuotes() int {
	if tok := s.scanAlphanumToken(s.currentTokType); tok >= 0 {
		s.numWikiTokensSeen++
		s.state = stateString
		return tok
	}
	if s.handleLinkStart() {
		return -2
	}
	s.advance(1)
	return -2
}

func (s *WikipediaTokenizerImpl) scanDoubleEquals() int {
	if s.matchRune('=') {
		s.advance(1)
		s.currentTokType = SubHeadingID
		s.numWikiTokensSeen = 0
		s.state = stateString
		return -2
	}
	if tok := s.scanAlphanumToken(HeadingID); tok >= 0 {
		s.currentTokType = HeadingID
		s.numWikiTokensSeen++
		s.state = stateDoubleEquals
		return HeadingID
	}
	if s.hasPrefix("==") {
		s.advance(2)
		s.state = stateInitial
		return -2
	}
	s.advance(1)
	return -2
}

func (s *WikipediaTokenizerImpl) scanDoubleBrace() int {
	if tok := s.scanAlphanumToken(s.currentTokType); tok >= 0 {
		s.numWikiTokensSeen = 0
		s.state = stateDoubleBrace
		return tok
	}
	if s.hasPrefix("}}") {
		s.advance(2)
		s.state = stateInitial
		return -2
	}
	if s.hasPrefix("</ref>") {
		s.advance(6)
		s.state = stateInitial
		return -2
	}
	s.advance(1)
	return -2
}

func (s *WikipediaTokenizerImpl) scanString() int {
	if s.hasPrefix("'''''") {
		s.numBalanced = 0
		s.currentTokType = AlphanumID
		s.advance(5)
		s.state = stateInitial
		return -2
	}
	if s.hasPrefix("'''") {
		s.numBalanced = 0
		s.currentTokType = AlphanumID
		s.advance(3)
		s.state = stateInitial
		return -2
	}
	if s.hasPrefix("''") {
		s.numBalanced = 0
		s.currentTokType = AlphanumID
		s.advance(2)
		s.state = stateInitial
		return -2
	}
	if s.hasPrefix("===") {
		s.numBalanced = 0
		s.currentTokType = AlphanumID
		s.advance(3)
		s.state = stateInitial
		return -2
	}
	if tok := s.scanAlphanumToken(s.currentTokType); tok >= 0 {
		s.numWikiTokensSeen++
		s.state = stateString
		return tok
	}
	if s.handleLinkStart() {
		return -2
	}
	if s.matchRune('|') {
		s.advance(1)
		return s.currentTokType
	}
	s.advance(1)
	return -2
}

// handleLinkStart handles [[ / [[Category: / [ transitions inside markup states.
// Returns true if a state transition was made.
func (s *WikipediaTokenizerImpl) handleLinkStart() bool {
	if s.hasPrefix("[[Category:") || s.hasPrefix("[[:Category:") {
		s.numBalanced = 0
		s.numWikiTokensSeen = 0
		s.currentTokType = CategoryID
		skip := 11
		if s.hasPrefix("[[:Category:") {
			skip = 12
		}
		s.advance(skip)
		s.state = stateCategoryState
		return true
	}
	if s.hasPrefix("[[") {
		s.numBalanced = 0
		s.numWikiTokensSeen = 0
		s.currentTokType = InternalLinkID
		s.advance(2)
		s.state = stateInternalLinkState
		return true
	}
	if s.matchRune('[') {
		s.numBalanced = 0
		s.numWikiTokensSeen = 0
		s.currentTokType = ExternalLinkID
		s.advance(1)
		s.state = stateExternalLinkState
		return true
	}
	return false
}

// scanAlphanumToken reads a run of alpha/digit runes and emits tokType.
// Returns -1 if no alpha/digit at current position.
func (s *WikipediaTokenizerImpl) scanAlphanumToken(tokType int) int {
	r, ok := s.peek(0)
	if !ok || (!isAlpha(r) && !isDigit(r)) {
		return -1
	}
	s.tokenStart = s.absPos()
	s.tokenText = s.tokenText[:0]
	for {
		r2, ok2 := s.peek(0)
		if !ok2 || (!isAlpha(r2) && !isDigit(r2)) {
			break
		}
		s.tokenText = append(s.tokenText, r2)
		s.bufPos++
	}
	s.tokenEnd = s.absPos()
	return tokType
}

// ─── buffer management ────────────────────────────────────────────────────────

// absPos returns the current absolute rune offset.
func (s *WikipediaTokenizerImpl) absPos() int { return s.bufStart + s.bufPos }

// peek returns the rune n positions ahead of bufPos without consuming it.
func (s *WikipediaTokenizerImpl) peek(n int) (rune, bool) {
	if s.bufPos+n < s.bufLen {
		return s.buf[s.bufPos+n], true
	}
	if s.eof {
		return 0, false
	}
	// Refill.
	if !s.refill() {
		return 0, false
	}
	if s.bufPos+n < s.bufLen {
		return s.buf[s.bufPos+n], true
	}
	return 0, false
}

// advance moves bufPos forward by n runes.
func (s *WikipediaTokenizerImpl) advance(n int) {
	// Ensure buffer has enough data.
	for s.bufPos+n > s.bufLen && !s.eof {
		s.refill()
	}
	if s.bufPos+n <= s.bufLen {
		s.bufPos += n
	} else {
		s.bufPos = s.bufLen
	}
}

// matchRune returns true if the current rune equals r (without consuming).
func (s *WikipediaTokenizerImpl) matchRune(r rune) bool {
	got, ok := s.peek(0)
	return ok && got == r
}

// hasPrefix returns true if the buffer (starting at bufPos) starts with the
// given string.
func (s *WikipediaTokenizerImpl) hasPrefix(prefix string) bool {
	prunes := []rune(prefix)
	for i, r := range prunes {
		got, ok := s.peek(i)
		if !ok || got != r {
			return false
		}
	}
	return true
}

// refill reads more bytes from input and decodes them as runes into buf.
// Returns false if EOF is reached with no new data.
func (s *WikipediaTokenizerImpl) refill() bool {
	// Compact: keep unread runes at the front.
	if s.bufPos > 0 {
		copy(s.buf, s.buf[s.bufPos:s.bufLen])
		s.bufStart += s.bufPos
		s.bufLen -= s.bufPos
		s.bufPos = 0
	}

	// Read raw bytes.
	n, err := s.input.Read(s.rawBuf[s.rawLen:])
	s.rawLen += n
	if err != nil {
		s.eof = true
		if s.rawLen == 0 {
			return false
		}
	}

	// Decode runes from rawBuf into buf.
	raw := s.rawBuf[:s.rawLen]
	decoded := 0
	for len(raw) > 0 && s.bufLen < len(s.buf)-4 {
		r, size := utf8.DecodeRune(raw)
		if size == 0 {
			break
		}
		if r == utf8.RuneError && size == 1 && !s.eof {
			break // incomplete sequence at end — wait for more bytes
		}
		if s.bufLen >= len(s.buf) {
			// Grow buf.
			newBuf := make([]rune, len(s.buf)*2)
			copy(newBuf, s.buf[:s.bufLen])
			s.buf = newBuf
		}
		s.buf[s.bufLen] = r
		s.bufLen++
		raw = raw[size:]
		decoded += size
	}
	// Shift consumed bytes out of rawBuf.
	copy(s.rawBuf, s.rawBuf[decoded:s.rawLen])
	s.rawLen -= decoded

	return s.bufLen > s.bufPos
}

// ─── character classification ─────────────────────────────────────────────────

// isAlpha matches LETTER in the JFlex grammar (extended Latin + many scripts).
func isAlpha(r rune) bool {
	return (r >= 0x0041 && r <= 0x005a) || // A-Z
		(r >= 0x0061 && r <= 0x007a) || // a-z
		(r >= 0x00c0 && r <= 0x00d6) ||
		(r >= 0x00d8 && r <= 0x00f6) ||
		(r >= 0x00f8 && r <= 0x00ff) ||
		(r >= 0x0100 && r <= 0x1fff) ||
		(r >= 0xffa0 && r <= 0xffdc)
}

func isAlphaRune(r rune) bool { return isAlpha(r) }

// isDigit matches DIGIT in the JFlex grammar (many Unicode digit ranges).
func isDigit(r rune) bool {
	return (r >= 0x0030 && r <= 0x0039) || // 0-9
		(r >= 0x0660 && r <= 0x0669) ||
		(r >= 0x06f0 && r <= 0x06f9) ||
		(r >= 0x0966 && r <= 0x096f) ||
		(r >= 0x09e6 && r <= 0x09ef) ||
		(r >= 0x0a66 && r <= 0x0a6f) ||
		(r >= 0x0ae6 && r <= 0x0aef) ||
		(r >= 0x0b66 && r <= 0x0b6f) ||
		(r >= 0x0be7 && r <= 0x0bef) ||
		(r >= 0x0c66 && r <= 0x0c6f) ||
		(r >= 0x0ce6 && r <= 0x0cef) ||
		(r >= 0x0d66 && r <= 0x0d6f) ||
		(r >= 0x0e50 && r <= 0x0e59) ||
		(r >= 0x0ed0 && r <= 0x0ed9) ||
		(r >= 0x1040 && r <= 0x1049)
}

func isDigitRune(r rune) bool { return isDigit(r) }

// isCJ matches CJ (Chinese/Japanese) in the JFlex grammar.
func isCJ(r rune) bool {
	return (r >= 0x3040 && r <= 0x318f) ||
		(r >= 0x3100 && r <= 0x312f) ||
		(r >= 0x30a0 && r <= 0x30ff) ||
		(r >= 0x31f0 && r <= 0x31ff) ||
		(r >= 0x3300 && r <= 0x337f) ||
		(r >= 0x3400 && r <= 0x4dbf) ||
		(r >= 0x4e00 && r <= 0x9fff) ||
		(r >= 0xf900 && r <= 0xfaff) ||
		(r >= 0xff65 && r <= 0xff9f)
}

func isWhitespace(r rune) bool {
	return unicode.IsSpace(r)
}
