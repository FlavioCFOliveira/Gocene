// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package te provides analysis components for Telugu text.
package te

// TeluguNormalizer normalizes Telugu text to remove some differences in
// spelling variations.
//
// Go port of org.apache.lucene.analysis.te.TeluguNormalizer (Apache Lucene
// 10.4.0). The Java original is package-private; this implementation is also
// unexported.
type TeluguNormalizer struct{}

// NewTeluguNormalizer creates a new TeluguNormalizer.
func NewTeluguNormalizer() *TeluguNormalizer {
	return &TeluguNormalizer{}
}

// Normalize applies Telugu normalization to s[:length] in-place and returns
// the new length.
func (n *TeluguNormalizer) Normalize(s []rune, length int) int {
	for i := 0; i < length; i++ {
		switch s[i] {
		// candrabindu (ఀ and ఁ) -> bindu (ం)
		case 'ఀ', 'ఁ':
			s[i] = 'ం'

		// delete visarga (ః)
		case 'ః':
			length = runeDelete(s, i, length)
			i--

		// zwj/zwnj -> delete
		case '‍', '‌':
			length = runeDelete(s, i, length)
			i--

		// long -> short vowels
		case 'ఔ': // ఔ -> ఓ
			s[i] = 'ఓ'
		case 'ఐ': // ఐ -> ఏ
			s[i] = 'ఏ'
		case 'ఆ': // ఆ -> అ
			s[i] = 'అ'
		case 'ఈ': // ఈ -> ఇ
			s[i] = 'ఇ'
		case 'ఊ': // ఊ -> ఉ
			s[i] = 'ఉ'

		// long -> short vowels matras
		case 'ీ': // ీ -> ి
			s[i] = 'ి'
		case 'ూ': // ూ -> ు
			s[i] = 'ు'
		case 'ే': // ే -> ె
			s[i] = 'ె'
		case 'ో': // ో -> ొ
			s[i] = 'ొ'

		// decomposed diphthong (ె + ౖ) -> precomposed diphthong vowel sign (ై)
		case 'ె':
			if i+1 < length && s[i+1] == 'ౖ' {
				s[i] = 'ై'
				length = runeDelete(s, i+1, length)
			}

		// composed oo or au -> oo or au
		case 'ఒ':
			if i+1 < length && s[i+1] == 'ౕ' {
				// (ఒ + ౕ) -> oo (ఓ)
				s[i] = 'ఓ'
				length = runeDelete(s, i+1, length)
			} else if i+1 < length && s[i+1] == 'ౌ' {
				// (ఒ + ౌ) -> au (ఔ)
				s[i] = 'ఔ'
				length = runeDelete(s, i+1, length)
			}
		}
	}
	return length
}

// runeDelete removes the rune at position pos from s[:length] in-place and
// returns the new length. Mirrors StemmerUtil.delete from Java.
func runeDelete(s []rune, pos, length int) int {
	length--
	copy(s[pos:], s[pos+1:length+1])
	return length
}
