// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

/*
 * This algorithm is based on code located at:
 * http://members.unine.ch/jacques.savoy/clef/
 *
 * Copyright (c) 2005, Jacques Savoy
 * All rights reserved.
 *
 * Redistribution and use in source and binary forms, with or without
 * modification, are permitted provided that the following conditions are met:
 * Redistributions of source code must retain the above copyright notice,
 * this list of conditions and the following disclaimer. Redistributions in
 * binary form must reproduce the above copyright notice, this list of
 * conditions and the following disclaimer in the documentation and/or other
 * materials provided with the distribution. Neither the name of the author
 * nor the names of its contributors may be used to endorse or promote
 * products derived from this software without specific prior written
 * permission.
 *
 * THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
 * AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
 * IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
 * ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
 * LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
 * CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
 * SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
 * INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
 * CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
 * ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
 * POSSIBILITY OF SUCH DAMAGE.
 */

import "unicode"

// frenchLightStemmer implements the UniNE light stemming algorithm for French.
//
// Go port of org.apache.lucene.analysis.fr.FrenchLightStemmer (Apache Lucene 10.4.0).
// The Java original is package-private; this implementation is also unexported.
//
// Reference:
//
//	"Light Stemming Approaches for the French, Portuguese, German and Hungarian Languages"
//	Jacques Savoy, CLEF 2005.
type frenchLightStemmer struct{}

// stem applies the French UniNE algorithm to s[:length].
// The slice is modified in-place; the new length is returned.
//
// IMPORTANT: The Java source uses sequential if-statements (not if/else-if),
// which means some cases fall through to subsequent checks. This structure
// must be preserved exactly — most notably the "trice" case transforms the
// suffix to "teur" and then falls through to the "teur" check.
func (frenchLightStemmer) stem(s []rune, length int) int {
	if length > 5 && s[length-1] == 'x' {
		if s[length-3] == 'a' && s[length-2] == 'u' && s[length-4] != 'e' {
			s[length-2] = 'l'
		}
		length--
	}
	if length > 3 && s[length-1] == 'x' {
		length--
	}
	if length > 3 && s[length-1] == 's' {
		length--
	}

	if length > 9 && runesEndWith(s, length, "issement") {
		length -= 6
		s[length-1] = 'r'
		return frenchLightNorm(s, length)
	}

	if length > 8 && runesEndWith(s, length, "issant") {
		length -= 4
		s[length-1] = 'r'
		return frenchLightNorm(s, length)
	}

	if length > 6 && runesEndWith(s, length, "ement") {
		length -= 4
		if length > 3 && runesEndWith(s, length, "ive") {
			length--
			s[length-1] = 'f'
		}
		return frenchLightNorm(s, length)
	}

	if length > 11 && runesEndWith(s, length, "ficatrice") {
		length -= 5
		s[length-2] = 'e'
		s[length-1] = 'r'
		return frenchLightNorm(s, length)
	}

	if length > 10 && runesEndWith(s, length, "ficateur") {
		length -= 4
		s[length-2] = 'e'
		s[length-1] = 'r'
		return frenchLightNorm(s, length)
	}

	if length > 9 && runesEndWith(s, length, "catrice") {
		length -= 3
		s[length-4] = 'q'
		s[length-3] = 'u'
		s[length-2] = 'e'
		// s[length-1] = 'r' — unnecessary, already 'r'.
		return frenchLightNorm(s, length)
	}

	if length > 8 && runesEndWith(s, length, "cateur") {
		length -= 2
		s[length-4] = 'q'
		s[length-3] = 'u'
		s[length-2] = 'e'
		s[length-1] = 'r'
		return frenchLightNorm(s, length)
	}

	if length > 8 && runesEndWith(s, length, "atrice") {
		length -= 4
		s[length-2] = 'e'
		s[length-1] = 'r'
		return frenchLightNorm(s, length)
	}

	if length > 7 && runesEndWith(s, length, "ateur") {
		length -= 3
		s[length-2] = 'e'
		s[length-1] = 'r'
		return frenchLightNorm(s, length)
	}

	// NOTE: "trice" does NOT return — it transforms the suffix to "teur" and
	// falls through to the "teur" check below, matching the Java if-statement
	// structure (sequential, not if/else-if).
	if length > 6 && runesEndWith(s, length, "trice") {
		length--
		s[length-3] = 'e'
		s[length-2] = 'u'
		s[length-1] = 'r'
	}

	if length > 5 && runesEndWith(s, length, "ième") {
		return frenchLightNorm(s, length-4)
	}

	if length > 7 && runesEndWith(s, length, "teuse") {
		length -= 2
		s[length-1] = 'r'
		return frenchLightNorm(s, length)
	}

	if length > 6 && runesEndWith(s, length, "teur") {
		length--
		s[length-1] = 'r'
		return frenchLightNorm(s, length)
	}

	if length > 5 && runesEndWith(s, length, "euse") {
		return frenchLightNorm(s, length-2)
	}

	if length > 8 && runesEndWith(s, length, "ère") {
		length--
		s[length-2] = 'e'
		return frenchLightNorm(s, length)
	}

	if length > 7 && runesEndWith(s, length, "ive") {
		length--
		s[length-1] = 'f'
		return frenchLightNorm(s, length)
	}

	if length > 4 && (runesEndWith(s, length, "folle") || runesEndWith(s, length, "molle")) {
		length -= 2
		s[length-1] = 'u'
		return frenchLightNorm(s, length)
	}

	if length > 9 && runesEndWith(s, length, "nnelle") {
		return frenchLightNorm(s, length-5)
	}

	if length > 9 && runesEndWith(s, length, "nnel") {
		return frenchLightNorm(s, length-3)
	}

	// NOTE: "ète" does NOT return — it modifies and falls through to subsequent
	// checks, matching the Java if-statement structure.
	if length > 4 && runesEndWith(s, length, "ète") {
		length--
		s[length-2] = 'e'
	}

	// NOTE: "ique" does NOT return — it modifies and falls through.
	if length > 8 && runesEndWith(s, length, "ique") {
		length -= 4
	}

	if length > 8 && runesEndWith(s, length, "esse") {
		return frenchLightNorm(s, length-3)
	}

	if length > 7 && runesEndWith(s, length, "inage") {
		return frenchLightNorm(s, length-3)
	}

	if length > 9 && runesEndWith(s, length, "isation") {
		length -= 7
		if length > 5 && runesEndWith(s, length, "ual") {
			s[length-2] = 'e'
		}
		return frenchLightNorm(s, length)
	}

	if length > 9 && runesEndWith(s, length, "isateur") {
		return frenchLightNorm(s, length-7)
	}

	if length > 8 && runesEndWith(s, length, "ation") {
		return frenchLightNorm(s, length-5)
	}

	if length > 8 && runesEndWith(s, length, "ition") {
		return frenchLightNorm(s, length-5)
	}

	return frenchLightNorm(s, length)
}

// frenchLightNorm applies diacritic normalisation and deduplication to s[:length].
func frenchLightNorm(s []rune, length int) int {
	if length > 4 {
		for i := 0; i < length; i++ {
			switch s[i] {
			case 'à', 'á', 'â':
				s[i] = 'a'
			case 'ô':
				s[i] = 'o'
			case 'è', 'é', 'ê':
				s[i] = 'e'
			case 'ù', 'û':
				s[i] = 'u'
			case 'î':
				s[i] = 'i'
			case 'ç':
				s[i] = 'c'
			}
		}

		// Remove consecutive duplicate letters.
		ch := s[0]
		for i := 1; i < length; i++ {
			if s[i] == ch && unicode.IsLetter(ch) {
				length = runeDelete(s, i, length)
				i--
			} else {
				ch = s[i]
			}
		}
	}

	if length > 4 && runesEndWith(s, length, "ie") {
		length -= 2
	}

	if length > 4 {
		if s[length-1] == 'r' {
			length--
		}
		if length > 4 && s[length-1] == 'e' {
			length--
		}
		if length > 4 && s[length-1] == 'e' {
			length--
		}
		if length > 4 && s[length-1] == s[length-2] && unicode.IsLetter(s[length-1]) {
			length--
		}
	}
	return length
}
