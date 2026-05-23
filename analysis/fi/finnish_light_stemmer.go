// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package fi provides Finnish language analysis components.
package fi

// FinnishLightStemmer is a light stemmer for Finnish.
//
// The algorithm is described in:
// Report on CLEF-2003 Monolingual Tracks, Jacques Savoy.
//
// This is the Go port of
// org.apache.lucene.analysis.fi.FinnishLightStemmer from
// Apache Lucene 10.4.0.
type FinnishLightStemmer struct{}

// Stem reduces s[:length] in-place and returns the new length.
func (st *FinnishLightStemmer) Stem(s []rune, length int) int {
	if length < 4 {
		return length
	}

	for i := 0; i < length; i++ {
		switch s[i] {
		case 'ä', 'å':
			s[i] = 'a'
		case 'ö':
			s[i] = 'o'
		}
	}

	length = st.step1(s, length)
	length = st.step2(s, length)
	length = st.step3(s, length)
	length = st.norm1(s, length)
	length = st.norm2(s, length)
	return length
}

// StemString stems the given string and returns the result.
func (st *FinnishLightStemmer) StemString(term string) string {
	runes := []rune(term)
	n := st.Stem(runes, len(runes))
	return string(runes[:n])
}

func (st *FinnishLightStemmer) step1(s []rune, len int) int {
	if len > 8 {
		if endsWith(s, len, "kin") {
			return st.step1(s, len-3)
		}
		if endsWith(s, len, "ko") {
			return st.step1(s, len-2)
		}
	}
	if len > 11 {
		if endsWith(s, len, "dellinen") {
			return len - 8
		}
		if endsWith(s, len, "dellisuus") {
			return len - 9
		}
	}
	return len
}

func (st *FinnishLightStemmer) step2(s []rune, len int) int {
	if len > 5 {
		if endsWith(s, len, "lla") || endsWith(s, len, "tse") || endsWith(s, len, "sti") {
			return len - 3
		}
		if endsWith(s, len, "ni") {
			return len - 2
		}
		if endsWith(s, len, "aa") {
			return len - 1
		}
	}
	return len
}

func (st *FinnishLightStemmer) step3(s []rune, ln int) int {
	if ln > 8 {
		if endsWith(s, ln, "nnen") {
			s[ln-4] = 's'
			return ln - 3
		}
		if endsWith(s, ln, "ntena") {
			s[ln-5] = 's'
			return ln - 4
		}
		if endsWith(s, ln, "tten") {
			return ln - 4
		}
		if endsWith(s, ln, "eiden") {
			return ln - 5
		}
	}
	if ln > 6 {
		if endsWith(s, ln, "neen") ||
			endsWith(s, ln, "niin") ||
			endsWith(s, ln, "seen") ||
			endsWith(s, ln, "teen") ||
			endsWith(s, ln, "inen") {
			return ln - 4
		}
		if s[ln-3] == 'h' && isVowel(s[ln-2]) && s[ln-1] == 'n' {
			return ln - 3
		}
		if endsWith(s, ln, "den") {
			s[ln-3] = 's'
			return ln - 2
		}
		if endsWith(s, ln, "ksen") {
			s[ln-4] = 's'
			return ln - 3
		}
		if endsWith(s, ln, "ssa") ||
			endsWith(s, ln, "sta") ||
			endsWith(s, ln, "lla") ||
			endsWith(s, ln, "lta") ||
			endsWith(s, ln, "tta") ||
			endsWith(s, ln, "ksi") ||
			endsWith(s, ln, "lle") {
			return ln - 3
		}
	}
	if ln > 5 {
		if endsWith(s, ln, "na") || endsWith(s, ln, "ne") {
			return ln - 2
		}
		if endsWith(s, ln, "nei") {
			return ln - 3
		}
	}
	if ln > 4 {
		if endsWith(s, ln, "ja") || endsWith(s, ln, "ta") {
			return ln - 2
		}
		if s[ln-1] == 'a' {
			return ln - 1
		}
		if s[ln-1] == 'n' && isVowel(s[ln-2]) {
			return ln - 2
		}
		if s[ln-1] == 'n' {
			return ln - 1
		}
	}
	return ln
}

func (st *FinnishLightStemmer) norm1(s []rune, len int) int {
	if len > 5 && endsWith(s, len, "hde") {
		s[len-3] = 'k'
		s[len-2] = 's'
		s[len-1] = 'i'
	}
	if len > 4 {
		if endsWith(s, len, "ei") || endsWith(s, len, "at") {
			return len - 2
		}
	}
	if len > 3 {
		switch s[len-1] {
		case 't', 's', 'j', 'e', 'a', 'i':
			return len - 1
		}
	}
	return len
}

func (st *FinnishLightStemmer) norm2(s []rune, len int) int {
	if len > 8 {
		ch := s[len-1]
		if ch == 'e' || ch == 'o' || ch == 'u' {
			len--
		}
	}
	if len > 4 {
		if s[len-1] == 'i' {
			len--
		}
		if len > 4 {
			ch := s[0]
			for i := 1; i < len; i++ {
				if s[i] == ch && (ch == 'k' || ch == 'p' || ch == 't') {
					len = runeDelete(s, i, len)
					i--
				} else {
					ch = s[i]
				}
			}
		}
	}
	return len
}

// endsWith reports whether s[:length] ends with the given suffix.
func endsWith(s []rune, length int, suffix string) bool {
	sr := []rune(suffix)
	if len(sr) > length {
		return false
	}
	offset := length - len(sr)
	for i, r := range sr {
		if s[offset+i] != r {
			return false
		}
	}
	return true
}

// isVowel reports whether ch is a Finnish vowel.
func isVowel(ch rune) bool {
	switch ch {
	case 'a', 'e', 'i', 'o', 'u', 'y':
		return true
	default:
		return false
	}
}

// runeDelete removes the rune at pos from s[:length] and returns the new length.
func runeDelete(s []rune, pos, length int) int {
	copy(s[pos:length-1], s[pos+1:length])
	return length - 1
}
