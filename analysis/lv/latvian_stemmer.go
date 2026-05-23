// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lv

// latvianAffix encodes one suffix entry from the Latvian stemmer table.
type latvianAffix struct {
	affix       []rune
	vc          int  // minimum vowel count required in the remaining stem
	palatalizes bool // fire palatalization rules after stripping
}

// affixes is the ordered list of suffixes tried by LatvianStemmer.
// Order is significant: longer/more-specific suffixes must appear first.
var affixes = [...]latvianAffix{
	{[]rune("ajiem"), 3, false},
	{[]rune("ajai"), 3, false},
	{[]rune("ajam"), 2, false},
	{[]rune("ajām"), 2, false},
	{[]rune("ajos"), 2, false},
	{[]rune("ajās"), 2, false},
	{[]rune("iem"), 2, true},
	{[]rune("ajā"), 2, false},
	{[]rune("ais"), 2, false},
	{[]rune("ai"), 2, false},
	{[]rune("ei"), 2, false},
	{[]rune("ām"), 1, false},
	{[]rune("am"), 1, false},
	{[]rune("ēm"), 1, false},
	{[]rune("īm"), 1, false},
	{[]rune("im"), 1, false},
	{[]rune("um"), 1, false},
	{[]rune("us"), 1, true},
	{[]rune("as"), 1, false},
	{[]rune("ās"), 1, false},
	{[]rune("es"), 1, false},
	{[]rune("os"), 1, true},
	{[]rune("ij"), 1, false},
	{[]rune("īs"), 1, false},
	{[]rune("ēs"), 1, false},
	{[]rune("is"), 1, false},
	{[]rune("ie"), 1, false},
	{[]rune("u"), 1, true},
	{[]rune("a"), 1, true},
	{[]rune("i"), 1, true},
	{[]rune("e"), 1, false},
	{[]rune("ā"), 1, false},
	{[]rune("ē"), 1, false},
	{[]rune("ī"), 1, false},
	{[]rune("ū"), 1, false},
	{[]rune("o"), 1, false},
	{[]rune("s"), 0, false},
	{[]rune("š"), 0, false},
}

// LatvianStemmer is a light stemmer for Latvian.
//
// This is a light version of the algorithm in Karlis Kreslin's PhD thesis
// "A stemming algorithm for Latvian".
//
// This is the Go port of
// org.apache.lucene.analysis.lv.LatvianStemmer from
// Apache Lucene 10.4.0.
type LatvianStemmer struct{}

// Stem reduces s[:length] in-place and returns the new length.
func (st *LatvianStemmer) Stem(s []rune, length int) int {
	numV := numVowels(s, length)
	for i := range affixes {
		aff := &affixes[i]
		if numV > aff.vc &&
			length >= len(aff.affix)+3 &&
			endsWith(s, length, aff.affix) {
			length -= len(aff.affix)
			if aff.palatalizes {
				return unpalatalize(s, length)
			}
			return length
		}
	}
	return length
}

// StemString stems a string-level input.
func (st *LatvianStemmer) StemString(term string) string {
	runes := []rune(term)
	n := st.Stem(runes, len(runes))
	return string(runes[:n])
}

// unpalatalize applies palatalization reversal after suffix stripping.
// The caller guarantees len(s) > length so that s[length] (first removed
// char) is accessible without bounds error.
func unpalatalize(s []rune, length int) int {
	// removed char is s[length]
	if s[length] == 'u' {
		// kš -> kst: extend by one (s[length] slot holds 'u', reuse it for 't')
		if endsWith(s, length, []rune("kš")) {
			s[length-1] = 's'
			s[length] = 't'
			return length + 1
		}
		// ņņ -> nn
		if endsWith(s, length, []rune("ņņ")) {
			s[length-2] = 'n'
			s[length-1] = 'n'
			return length
		}
	}

	switch {
	case endsWith(s, length, []rune("pj")),
		endsWith(s, length, []rune("bj")),
		endsWith(s, length, []rune("mj")),
		endsWith(s, length, []rune("vj")):
		return length - 1
	case endsWith(s, length, []rune("šņ")):
		s[length-2] = 's'
		s[length-1] = 'n'
	case endsWith(s, length, []rune("žņ")):
		s[length-2] = 'z'
		s[length-1] = 'n'
	case endsWith(s, length, []rune("šļ")):
		s[length-2] = 's'
		s[length-1] = 'l'
	case endsWith(s, length, []rune("žļ")):
		s[length-2] = 'z'
		s[length-1] = 'l'
	case endsWith(s, length, []rune("ļņ")):
		s[length-2] = 'l'
		s[length-1] = 'n'
	case endsWith(s, length, []rune("ļļ")):
		s[length-2] = 'l'
		s[length-1] = 'l'
	default:
		if length > 0 {
			switch s[length-1] {
			case 'č':
				s[length-1] = 'c'
			case 'ļ':
				s[length-1] = 'l'
			case 'ņ':
				s[length-1] = 'n'
			}
		}
	}
	return length
}

// numVowels counts the vowels in s[:length].
func numVowels(s []rune, length int) int {
	n := 0
	for i := 0; i < length; i++ {
		switch s[i] {
		case 'a', 'e', 'i', 'o', 'u', 'ā', 'ī', 'ē', 'ū':
			n++
		}
	}
	return n
}

// endsWith reports whether s[:length] ends with suffix.
func endsWith(s []rune, length int, suffix []rune) bool {
	if len(suffix) > length {
		return false
	}
	offset := length - len(suffix)
	for i, r := range suffix {
		if s[offset+i] != r {
			return false
		}
	}
	return true
}
