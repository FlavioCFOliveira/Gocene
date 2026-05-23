// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package pt

/*
 * This algorithm is updated based on code located at:
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
 * materials provided with the distribution. Neither the name of the author nor
 * the names of its contributors may be used to endorse or promote products
 * derived from this software without specific prior written permission.
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

// PortugueseLightStemmer implements the UniNE light stemming algorithm for
// Portuguese.
//
// Reference:
//   "Light Stemming Approaches for the French, Portuguese, German and Hungarian
//   Languages" Jacques Savoy.
//
// Go port of org.apache.lucene.analysis.pt.PortugueseLightStemmer (Apache
// Lucene 10.4.0).
type PortugueseLightStemmer struct{}

// NewPortugueseLightStemmer creates a PortugueseLightStemmer.
func NewPortugueseLightStemmer() *PortugueseLightStemmer { return &PortugueseLightStemmer{} }

// Stem applies the UniNE light stemming algorithm to s[:length] in-place and
// returns the new length.
func (PortugueseLightStemmer) Stem(s []rune, length int) int {
	if length < 4 {
		return length
	}

	length = ptRemoveSuffix(s, length)

	if length > 3 && s[length-1] == 'a' {
		length = ptNormFeminine(s, length)
	}

	if length > 4 {
		switch s[length-1] {
		case 'e', 'a', 'o':
			length--
		}
	}

	for i := 0; i < length; i++ {
		switch s[i] {
		case 'à', 'á', 'â', 'ä', 'ã':
			s[i] = 'a'
		case 'ò', 'ó', 'ô', 'ö', 'õ':
			s[i] = 'o'
		case 'è', 'é', 'ê', 'ë':
			s[i] = 'e'
		case 'ù', 'ú', 'û', 'ü':
			s[i] = 'u'
		case 'ì', 'í', 'î', 'ï':
			s[i] = 'i'
		case 'ç':
			s[i] = 'c'
		}
	}
	return length
}

func ptRemoveSuffix(s []rune, length int) int {
	if length > 4 && ptEndsWith(s, length, "es") {
		switch s[length-3] {
		case 'r', 's', 'l', 'z':
			return length - 2
		}
	}

	if length > 3 && ptEndsWith(s, length, "ns") {
		s[length-2] = 'm'
		return length - 1
	}

	if length > 4 && (ptEndsWith(s, length, "eis") || ptEndsWith(s, length, "éis")) {
		s[length-3] = 'e'
		s[length-2] = 'l'
		return length - 1
	}

	if length > 4 && ptEndsWith(s, length, "ais") {
		s[length-2] = 'l'
		return length - 1
	}

	if length > 4 && ptEndsWith(s, length, "óis") {
		s[length-3] = 'o'
		s[length-2] = 'l'
		return length - 1
	}

	if length > 4 && ptEndsWith(s, length, "is") {
		s[length-1] = 'l'
		return length
	}

	if length > 3 && (ptEndsWith(s, length, "ões") || ptEndsWith(s, length, "ães")) {
		length--
		s[length-2] = 'ã'
		s[length-1] = 'o'
		return length
	}

	if length > 6 && ptEndsWith(s, length, "mente") {
		return length - 5
	}

	if length > 3 && s[length-1] == 's' {
		return length - 1
	}
	return length
}

func ptNormFeminine(s []rune, length int) int {
	if length > 7 &&
		(ptEndsWith(s, length, "inha") || ptEndsWith(s, length, "iaca") || ptEndsWith(s, length, "eira")) {
		s[length-1] = 'o'
		return length
	}

	if length > 6 {
		if ptEndsWith(s, length, "osa") || ptEndsWith(s, length, "ica") ||
			ptEndsWith(s, length, "ida") || ptEndsWith(s, length, "ada") ||
			ptEndsWith(s, length, "iva") || ptEndsWith(s, length, "ama") {
			s[length-1] = 'o'
			return length
		}

		if ptEndsWith(s, length, "ona") {
			s[length-3] = 'ã'
			s[length-2] = 'o'
			return length - 1
		}

		if ptEndsWith(s, length, "ora") {
			return length - 1
		}

		if ptEndsWith(s, length, "esa") {
			s[length-3] = 'ê'
			return length - 1
		}

		if ptEndsWith(s, length, "na") {
			s[length-1] = 'o'
			return length
		}
	}
	return length
}

// ptEndsWith reports whether s[:length] ends with suffix (string form).
func ptEndsWith(s []rune, length int, suffix string) bool {
	suffRunes := []rune(suffix)
	if len(suffRunes) > length {
		return false
	}
	for i, r := range suffRunes {
		if s[length-len(suffRunes)+i] != r {
			return false
		}
	}
	return true
}
