// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package completion

// IsLowercaseAlphabets reports whether every character in s is a lowercase
// alphabet (half-width or full-width).
//
// This is the Go port of
// org.apache.lucene.analysis.ja.completion.CharSequenceUtils.isLowercaseAlphabets
// from Apache Lucene 10.4.0.
func IsLowercaseAlphabets(s string) bool {
	for _, c := range s {
		if !isHalfWidthLowercaseAlphabet(c) && !isFullWidthLowercaseAlphabet(c) {
			return false
		}
	}
	return true
}

// IsKana reports whether every character in s is hiragana or katakana.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.completion.CharSequenceUtils.isKana
// from Apache Lucene 10.4.0.
func IsKana(s string) bool {
	for _, c := range s {
		if !isHiragana(c) && !isKatakana(c) {
			return false
		}
	}
	return true
}

// IsKatakanaOrHWAlphabets reports whether every character in s is katakana or
// a half-width lowercase alphabet.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.completion.CharSequenceUtils.isKatakanaOrHWAlphabets
// from Apache Lucene 10.4.0.
func IsKatakanaOrHWAlphabets(s string) bool {
	for _, c := range s {
		if !isKatakana(c) && !isHalfWidthLowercaseAlphabet(c) {
			return false
		}
	}
	return true
}

// IsFullWidthLowercaseAlphabet reports whether c is a full-width lowercase
// Latin letter (U+FF41–U+FF5A).
func IsFullWidthLowercaseAlphabet(c rune) bool { return isFullWidthLowercaseAlphabet(c) }

// ToKatakana converts all hiragana characters in s to their katakana
// equivalents; other characters are left unchanged.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.completion.CharSequenceUtils.toKatakana
// from Apache Lucene 10.4.0.
func ToKatakana(s string) string {
	runes := []rune(s)
	for i, c := range runes {
		if (c >= 0x3041 && c <= 0x3096) || c == 0x309D || c == 0x309E {
			runes[i] = c + 0x60
		}
	}
	return string(runes)
}

func isHiragana(c rune) bool             { return c >= 0x3040 && c <= 0x309F }
func isKatakana(c rune) bool             { return c >= 0x30A0 && c <= 0x30FF }
func isHalfWidthLowercaseAlphabet(c rune) bool { return c >= 0x61 && c <= 0x7A }
func isFullWidthLowercaseAlphabet(c rune) bool { return c >= 0xFF41 && c <= 0xFF5A }
