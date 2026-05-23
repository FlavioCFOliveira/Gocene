// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hunspell

import "unicode"

// WordCase classifies the casing of a word.
//
// This is the Go port of
// org.apache.lucene.analysis.hunspell.WordCase from Apache Lucene 10.4.0.
type WordCase int

const (
	// WordCaseUpper means all uppercase, e.g. WORD.
	WordCaseUpper WordCase = iota
	// WordCaseTitle means title case, e.g. Word.
	WordCaseTitle
	// WordCaseLower means all lowercase, e.g. word.
	WordCaseLower
	// WordCaseMixed means mixed case, e.g. WoRd or wOrd.
	WordCaseMixed
	// WordCaseNeutral means no alphabetic case, e.g. "-" or "/" or "42".
	WordCaseNeutral
)

func (wc WordCase) String() string {
	switch wc {
	case WordCaseUpper:
		return "UPPER"
	case WordCaseTitle:
		return "TITLE"
	case WordCaseLower:
		return "LOWER"
	case WordCaseMixed:
		return "MIXED"
	case WordCaseNeutral:
		return "NEUTRAL"
	default:
		return "UNKNOWN"
	}
}

// charCase classifies a single rune.
type charCase int

const (
	charCaseUpper charCase = iota
	charCaseLower
	charCaseNeutral
)

func charCaseOf(r rune) charCase {
	if unicode.IsUpper(r) {
		return charCaseUpper
	}
	if unicode.IsLower(r) && unicode.ToUpper(r) != r {
		return charCaseLower
	}
	return charCaseNeutral
}

// wordCaseGet derives a WordCase from the start character's case and whether
// upper/lower characters were seen in the remainder.
func wordCaseGet(start charCase, seenUpper, seenLower bool) WordCase {
	if seenLower && seenUpper {
		return WordCaseMixed
	}
	switch start {
	case charCaseLower:
		if seenUpper {
			return WordCaseMixed
		}
		return WordCaseLower
	case charCaseUpper:
		if !seenLower {
			return WordCaseUpper
		}
		return WordCaseTitle
	default: // charCaseNeutral
		if seenLower {
			return WordCaseLower
		}
		if seenUpper {
			return WordCaseUpper
		}
		return WordCaseNeutral
	}
}

// CaseOfRunes returns the WordCase of the rune slice word[:length].
func CaseOfRunes(word []rune, length int) WordCase {
	if length == 0 {
		return WordCaseNeutral
	}
	startCase := charCaseOf(word[0])

	seenUpper := false
	seenLower := false
	for i := 1; i < length; i++ {
		cc := charCaseOf(word[i])
		seenUpper = seenUpper || cc == charCaseUpper
		seenLower = seenLower || cc == charCaseLower
		if seenUpper && seenLower {
			break
		}
	}
	return wordCaseGet(startCase, seenUpper, seenLower)
}

// CaseOfString returns the WordCase of s.
func CaseOfString(s string) WordCase {
	runes := []rune(s)
	return CaseOfRunes(runes, len(runes))
}

// CaseOfStringN returns the WordCase of the first length runes of s.
func CaseOfStringN(s string, length int) WordCase {
	runes := []rune(s)
	if length > len(runes) {
		length = len(runes)
	}
	return CaseOfRunes(runes, length)
}
