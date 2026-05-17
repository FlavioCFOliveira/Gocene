// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

// StemmerUtil provides the small set of buffer-mutating helpers
// shared by the language-specific stemmers.
//
// This is the Go port of
// org.apache.lucene.analysis.util.StemmerUtil from Apache Lucene
// 10.4.0.
//
// The Lucene reference operates on char[] (UTF-16). Most Gocene
// stemmers already use []rune (one code point per element); the
// helpers below mirror the reference signatures for that
// representation. Where a stemmer prefers a byte-level scan over an
// ASCII-safe payload, the same helpers work on []byte as long as
// the caller passes a non-Unicode prefix.
type StemmerUtil struct{}

// StemmerStartsWith reports whether the prefix matches the head of
// runes[:length].
func StemmerStartsWith(runes []rune, length int, prefix string) bool {
	pr := []rune(prefix)
	if len(pr) > length {
		return false
	}
	for i, r := range pr {
		if runes[i] != r {
			return false
		}
	}
	return true
}

// StemmerEndsWith reports whether the suffix matches the tail of
// runes[:length]. This is a convenience wrapper around
// runesEndWith for callers that prefer the StemmerUtil-prefixed
// name.
func StemmerEndsWith(runes []rune, length int, suffix string) bool {
	return runesEndWith(runes, length, suffix)
}

// StemmerDelete removes the rune at pos from runes[:length] and
// returns the new length. Equivalent to Lucene's
// StemmerUtil.delete(char[], int, int).
func StemmerDelete(runes []rune, pos, length int) int {
	if pos < 0 || pos >= length {
		return length
	}
	return runeDelete(runes, pos, length)
}

// StemmerDeleteN removes nChars runes starting at pos and returns the
// new length. Equivalent to Lucene's
// StemmerUtil.deleteN(char[], int, int, int).
func StemmerDeleteN(runes []rune, pos, length, nChars int) int {
	if nChars <= 0 || pos < 0 || pos+nChars > length {
		return length
	}
	copy(runes[pos:length-nChars], runes[pos+nChars:length])
	return length - nChars
}
