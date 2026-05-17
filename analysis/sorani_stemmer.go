// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

// SoraniStemmer is a light suffix stemmer for Sorani (Central
// Kurdish). It removes a small fixed set of postpositional,
// possessive, definite/indefinite, and demonstrative suffixes.
//
// This is the Go port of org.apache.lucene.analysis.ckb.SoraniStemmer
// from Apache Lucene 10.4.0.
//
// Deviation from Lucene: the reference operates on a char[] (UTF-16
// code units) and removes 1-4 code units at a time. The Sorani
// alphabet is in the Arabic script (BMP only), so one rune == one
// char and the truncation counts translate directly.
type SoraniStemmer struct{}

// NewSoraniStemmer returns a fresh stemmer. The receiver carries no
// state and is safe to share.
func NewSoraniStemmer() *SoraniStemmer {
	return &SoraniStemmer{}
}

// soraniSuffix lists the Sorani suffixes that the stemmer strips,
// ordered by the priority and minimum-length guards of the Lucene
// reference. The first element of each row is the suffix in Sorani
// script; the second is the minimum word length (number of runes) at
// which the suffix is eligible to be stripped.
//
// Where two rows in the original Java method belong to the same
// "guard return" group (definite, plural, etc.), they appear here in
// the same first-match-wins order. The "step" field encodes whether
// the loop should fall through to subsequent rules (1 = continue,
// 0 = return after stripping).

// Sorani suffix definitions, ordered by their position in
// org.apache.lucene.analysis.ckb.SoraniStemmer.stem.
var soraniSuffixes = []soraniSuffixRule{
	// Postpositions: continue after stripping.
	{suffix: "دا", minLen: 6, strip: 2, terminal: false},
	{suffix: "نا", minLen: 5, strip: 1, terminal: false},
	{suffix: "ەوە", minLen: 7, strip: 3, terminal: false},

	// Possessive pronouns: continue after stripping.
	{suffix: "مان", minLen: 7, strip: 3, terminal: false},
	{suffix: "یان", minLen: 7, strip: 3, terminal: false, possessive: true},
	{suffix: "تان", minLen: 7, strip: 3, terminal: false},
}

// soraniTerminalRules lists the suffix rules that, when matched, are
// returned immediately (the Java method uses an early return for each
// of these branches).
var soraniTerminalRules = []soraniSuffixRule{
	{suffix: "ێکی", minLen: 7, strip: 3},
	{suffix: "یەکی", minLen: 8, strip: 4},
	{suffix: "ێک", minLen: 6, strip: 2},
	{suffix: "یەک", minLen: 7, strip: 3},
	{suffix: "ەکە", minLen: 7, strip: 3},
	{suffix: "کە", minLen: 6, strip: 2},
	{suffix: "ەکان", minLen: 8, strip: 4},
	{suffix: "کان", minLen: 7, strip: 3},
	{suffix: "یانی", minLen: 8, strip: 4},
	{suffix: "انی", minLen: 7, strip: 3},
	{suffix: "یان", minLen: 7, strip: 3},
	{suffix: "ان", minLen: 6, strip: 2},
	{suffix: "یانە", minLen: 8, strip: 4},
	{suffix: "انە", minLen: 7, strip: 3},
	{suffix: "ایە", minLen: 6, strip: 2},
	{suffix: "ەیە", minLen: 6, strip: 2},
	{suffix: "ە", minLen: 5, strip: 1},
	{suffix: "ی", minLen: 5, strip: 1},
}

type soraniSuffixRule struct {
	suffix     string
	minLen     int
	strip      int
	terminal   bool
	possessive bool
}

// runesEndWith reports whether the trailing portion of runes[:length]
// matches suffix when interpreted as a rune string.
func runesEndWith(runes []rune, length int, suffix string) bool {
	sufRunes := []rune(suffix)
	if len(sufRunes) > length {
		return false
	}
	off := length - len(sufRunes)
	for i, r := range sufRunes {
		if runes[off+i] != r {
			return false
		}
	}
	return true
}

// Stem removes Sorani suffixes from runes[:length] and returns the
// new length. The caller must slice runes down to the returned value.
func (s *SoraniStemmer) Stem(runes []rune, length int) int {
	// Postposition group (first match wins; continues to next group).
	for _, r := range soraniSuffixes[:3] {
		if length > r.minLen && runesEndWith(runes, length, r.suffix) {
			length -= r.strip
			break
		}
	}
	// Possessive pronoun group.
	for _, r := range soraniSuffixes[3:] {
		if length > r.minLen && runesEndWith(runes, length, r.suffix) {
			length -= r.strip
			break
		}
	}
	// Terminal-return rules: first match wins, returned immediately.
	for _, r := range soraniTerminalRules {
		if length > r.minLen && runesEndWith(runes, length, r.suffix) {
			return length - r.strip
		}
	}
	return length
}

// StemString is the string-oriented convenience entry point. It
// returns the stem of s, suitable for use from filters that hold
// term text as a UTF-8 string.
func (s *SoraniStemmer) StemString(input string) string {
	if input == "" {
		return ""
	}
	runes := []rune(input)
	n := s.Stem(runes, len(runes))
	return string(runes[:n])
}
