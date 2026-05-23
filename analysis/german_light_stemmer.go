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

// germanLightStemmer implements the "UniNE" light stemming algorithm for German.
//
// Go port of org.apache.lucene.analysis.de.GermanLightStemmer (Apache Lucene 10.4.0).
// The Java original is package-private; this implementation is also unexported.
//
// Reference:
//
//	"Light Stemming Approaches for the French, Portuguese, German and Hungarian Languages"
//	Jacques Savoy, CLEF 2005.
type germanLightStemmer struct{}

// stem normalises diacritics in s[:len] and then applies two suffix-stripping
// steps. It returns the new length after stemming (the slice is modified
// in-place).
func (germanLightStemmer) stem(s []rune, length int) int {
	for i := 0; i < length; i++ {
		switch s[i] {
		case 'ä', 'à', 'á', 'â':
			s[i] = 'a'
		case 'ö', 'ò', 'ó', 'ô':
			s[i] = 'o'
		case 'ï', 'ì', 'í', 'î':
			s[i] = 'i'
		case 'ü', 'ù', 'ú', 'û':
			s[i] = 'u'
		}
	}
	length = germanLightStep1(s, length)
	return germanLightStep2(s, length)
}

// stEnding reports whether ch is one of the "st-ending" consonants.
func germanLightStEnding(ch rune) bool {
	switch ch {
	case 'b', 'd', 'f', 'g', 'h', 'k', 'l', 'm', 'n', 't':
		return true
	}
	return false
}

// germanLightStep1 strips plural / case suffixes.
func germanLightStep1(s []rune, length int) int {
	if length > 5 && s[length-3] == 'e' && s[length-2] == 'r' && s[length-1] == 'n' {
		return length - 3
	}
	if length > 4 && s[length-2] == 'e' {
		switch s[length-1] {
		case 'm', 'n', 'r', 's':
			return length - 2
		}
	}
	if length > 3 && s[length-1] == 'e' {
		return length - 1
	}
	if length > 3 && s[length-1] == 's' && germanLightStEnding(s[length-2]) {
		return length - 1
	}
	return length
}

// germanLightStep2 strips verb / adjective suffixes.
func germanLightStep2(s []rune, length int) int {
	if length > 5 && s[length-3] == 'e' && s[length-2] == 's' && s[length-1] == 't' {
		return length - 3
	}
	if length > 4 && s[length-2] == 'e' && (s[length-1] == 'r' || s[length-1] == 'n') {
		return length - 2
	}
	if length > 4 && s[length-2] == 's' && s[length-1] == 't' && germanLightStEnding(s[length-3]) {
		return length - 2
	}
	return length
}
