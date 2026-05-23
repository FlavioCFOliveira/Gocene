// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package classic

import (
	"io"
	"regexp"
	"strings"
)

// Token type constants matching ClassicTokenizer.
const (
	// TokenAlphaNum is a pure alphabetic or alphanumeric token.
	TokenAlphaNum = 0
	// TokenApostrophe is a possessive word like O'Reilly.
	TokenApostrophe = 1
	// TokenAcronym is a dotted acronym like U.S.A.
	TokenAcronym = 2
	// TokenCompany is a word containing an ampersand like AT&T.
	TokenCompany = 3
	// TokenEmail is an email address.
	TokenEmail = 4
	// TokenHost is an internet hostname.
	TokenHost = 5
	// TokenNum is a numeric token, possibly with decimal/comma separators.
	TokenNum = 6
	// TokenCJ is a CJK character token.
	TokenCJ = 7
	// TokenAcronymDep is a deprecated acronym form (digits in dotted sequence).
	TokenAcronymDep = 8
)

// TokenTypes maps token type constants to their string labels.
var TokenTypes = []string{
	"<ALPHANUM>",
	"<APOSTROPHE>",
	"<ACRONYM>",
	"<COMPANY>",
	"<EMAIL>",
	"<HOST>",
	"<NUM>",
	"<CJ>",
	"<ACRONYM_DEP>",
}

// classicToken is a single token scanned by ClassicTokenizerImpl.
type classicToken struct {
	text      string
	tokenType int
	startOff  int
	endOff    int
}

// classicScanResult holds all scanned tokens from an input reader.
type classicScanResult struct {
	tokens []classicToken
}

// regexp patterns used by ClassicTokenizerImpl.
// Order matters: more specific patterns must precede general ones.
var (
	reEmail    = regexp.MustCompile(`[a-zA-Z0-9][a-zA-Z0-9_.+-]*@[a-zA-Z0-9][a-zA-Z0-9-]*\.[a-zA-Z0-9.]+`)
	reAcronym  = regexp.MustCompile(`[A-Za-z]\.[A-Za-z](?:\.[A-Za-z])*\.?`)
	reAcroNum  = regexp.MustCompile(`[A-Za-z0-9]\.[A-Za-z0-9](?:\.[A-Za-z0-9])*`)
	reHost     = regexp.MustCompile(`[a-zA-Z0-9][a-zA-Z0-9-]*(?:\.[a-zA-Z0-9][a-zA-Z0-9-]*)+`)
	reCompany  = regexp.MustCompile(`[a-zA-Z0-9]+(?:&[a-zA-Z0-9]+)+`)
	reApostrophe = regexp.MustCompile(`[a-zA-Z]+'[a-zA-Z]+`)
	reNum      = regexp.MustCompile(`[0-9]+(?:[.,][0-9]+)*`)
	reAlpha    = regexp.MustCompile(`[a-zA-Z0-9]+`)
	reCJ       = regexp.MustCompile(`[\x{3040}-\x{318F}\x{31F0}-\x{31FF}\x{3400}-\x{4DBF}\x{4E00}-\x{9FFF}\x{A000}-\x{A4CF}\x{AC00}-\x{D7AF}]`)
)

// ClassicTokenizerImpl is the scanner component for ClassicTokenizer.
//
// This is the Go port of
// org.apache.lucene.analysis.classic.ClassicTokenizerImpl from
// Apache Lucene 10.4.0.
//
// Deviation: the Java reference is JFlex-generated. This Go port uses
// regexp-based scanning to approximate the same token type detection. The
// behavioural output matches for typical inputs; edge cases requiring the
// full JFlex DFA may differ.
type ClassicTokenizerImpl struct {
	input  string
	pos    int
	tokens []classicToken
	idx    int
}

// NewClassicTokenizerImpl creates an impl from an io.Reader.
func NewClassicTokenizerImpl(r io.Reader) *ClassicTokenizerImpl {
	data, _ := io.ReadAll(r)
	impl := &ClassicTokenizerImpl{input: string(data)}
	impl.scan()
	return impl
}

// Reset resets the scanner over a new reader.
func (s *ClassicTokenizerImpl) Reset(r io.Reader) {
	data, _ := io.ReadAll(r)
	s.input = string(data)
	s.pos = 0
	s.tokens = s.tokens[:0]
	s.idx = 0
	s.scan()
}

// ResetIndex rewinds the token cursor to the beginning without re-reading
// the input. Use this when the scanner has already eagerly consumed its
// reader and a Reset is needed (e.g., for repeated tokenisation).
func (s *ClassicTokenizerImpl) ResetIndex() {
	s.idx = 0
}

// GetNextToken returns the next scanned token or nil if exhausted.
func (s *ClassicTokenizerImpl) GetNextToken() *classicToken {
	if s.idx >= len(s.tokens) {
		return nil
	}
	t := &s.tokens[s.idx]
	s.idx++
	return t
}

// scan tokenises the entire input into s.tokens.
func (s *ClassicTokenizerImpl) scan() {
	runes := []rune(s.input)
	// Build a byte-offset map: runeIndex → byte offset.
	byteOff := make([]int, len(runes)+1)
	b := 0
	for i, r := range runes {
		byteOff[i] = b
		b += len(string(r))
	}
	byteOff[len(runes)] = b

	i := 0
	for i < len(runes) {
		r := runes[i]
		// Skip whitespace.
		if isSpace(r) {
			i++
			continue
		}
		// Try each pattern in priority order.
		sub := string(runes[i:])
		// Email
		if m := reEmail.FindStringIndex(sub); m != nil && m[0] == 0 {
			end := i + len([]rune(sub[:m[1]]))
			s.tokens = append(s.tokens, classicToken{
				text:      string(runes[i:end]),
				tokenType: TokenEmail,
				startOff:  byteOff[i],
				endOff:    byteOff[end],
			})
			i = end
			continue
		}
		// Acronym (letter-only dotted)
		if m := reAcronym.FindStringIndex(sub); m != nil && m[0] == 0 {
			end := i + len([]rune(sub[:m[1]]))
			s.tokens = append(s.tokens, classicToken{
				text:      string(runes[i:end]),
				tokenType: TokenAcronym,
				startOff:  byteOff[i],
				endOff:    byteOff[end],
			})
			i = end
			continue
		}
		// Host (dot-connected alphanums)
		if m := reHost.FindStringIndex(sub); m != nil && m[0] == 0 {
			tok := sub[:m[1]]
			// Only classify as host if it contains a dot.
			if strings.ContainsRune(tok, '.') {
				end := i + len([]rune(tok))
				// Avoid matching simple numbers as hosts.
				if !reNum.MatchString(tok) {
					s.tokens = append(s.tokens, classicToken{
						text:      string(runes[i:end]),
						tokenType: TokenHost,
						startOff:  byteOff[i],
						endOff:    byteOff[end],
					})
					i = end
					continue
				}
			}
		}
		// Company
		if m := reCompany.FindStringIndex(sub); m != nil && m[0] == 0 {
			end := i + len([]rune(sub[:m[1]]))
			s.tokens = append(s.tokens, classicToken{
				text:      string(runes[i:end]),
				tokenType: TokenCompany,
				startOff:  byteOff[i],
				endOff:    byteOff[end],
			})
			i = end
			continue
		}
		// Apostrophe
		if m := reApostrophe.FindStringIndex(sub); m != nil && m[0] == 0 {
			end := i + len([]rune(sub[:m[1]]))
			s.tokens = append(s.tokens, classicToken{
				text:      string(runes[i:end]),
				tokenType: TokenApostrophe,
				startOff:  byteOff[i],
				endOff:    byteOff[end],
			})
			i = end
			continue
		}
		// Number
		if m := reNum.FindStringIndex(sub); m != nil && m[0] == 0 {
			end := i + len([]rune(sub[:m[1]]))
			s.tokens = append(s.tokens, classicToken{
				text:      string(runes[i:end]),
				tokenType: TokenNum,
				startOff:  byteOff[i],
				endOff:    byteOff[end],
			})
			i = end
			continue
		}
		// CJK single character
		if isCJK(r) {
			s.tokens = append(s.tokens, classicToken{
				text:      string(r),
				tokenType: TokenCJ,
				startOff:  byteOff[i],
				endOff:    byteOff[i+1],
			})
			i++
			continue
		}
		// AlphaNum
		if m := reAlpha.FindStringIndex(sub); m != nil && m[0] == 0 {
			end := i + len([]rune(sub[:m[1]]))
			s.tokens = append(s.tokens, classicToken{
				text:      string(runes[i:end]),
				tokenType: TokenAlphaNum,
				startOff:  byteOff[i],
				endOff:    byteOff[end],
			})
			i = end
			continue
		}
		// Skip unknown character.
		i++
	}
}

func isSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f'
}

func isCJK(r rune) bool {
	return (r >= 0x3040 && r <= 0x318F) || // Hiragana/Katakana
		(r >= 0x31F0 && r <= 0x31FF) ||
		(r >= 0x3400 && r <= 0x4DBF) ||
		(r >= 0x4E00 && r <= 0x9FFF) ||
		(r >= 0xA000 && r <= 0xA4CF) ||
		(r >= 0xAC00 && r <= 0xD7AF)
}
