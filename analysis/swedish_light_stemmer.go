// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

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

// swedishLightStemmer implements the UniNE light stemming algorithm for Swedish.
//
// Go port of org.apache.lucene.analysis.sv.SwedishLightStemmer (Apache Lucene
// 10.4.0). The Java original is package-private; this implementation is also
// unexported.
//
// Reference:
//
//	"Report on CLEF-2003 Monolingual Tracks" Jacques Savoy.
type swedishLightStemmer struct{}

// stem applies the Swedish light stemming algorithm to s[:length] in-place
// and returns the new length.
func (swedishLightStemmer) stem(s []rune, length int) int {
	if length > 4 && s[length-1] == 's' {
		length--
	}

	if length > 7 && (runesEndWith(s, length, "elser") || runesEndWith(s, length, "heten")) {
		return length - 5
	}

	if length > 6 && (runesEndWith(s, length, "arne") ||
		runesEndWith(s, length, "erna") ||
		runesEndWith(s, length, "ande") ||
		runesEndWith(s, length, "else") ||
		runesEndWith(s, length, "aste") ||
		runesEndWith(s, length, "orna") ||
		runesEndWith(s, length, "aren")) {
		return length - 4
	}

	if length > 5 && (runesEndWith(s, length, "are") ||
		runesEndWith(s, length, "ast") ||
		runesEndWith(s, length, "het")) {
		return length - 3
	}

	if length > 4 && (runesEndWith(s, length, "ar") ||
		runesEndWith(s, length, "er") ||
		runesEndWith(s, length, "or") ||
		runesEndWith(s, length, "en") ||
		runesEndWith(s, length, "at") ||
		runesEndWith(s, length, "te") ||
		runesEndWith(s, length, "et")) {
		return length - 2
	}

	if length > 3 {
		switch s[length-1] {
		case 't', 'a', 'e', 'n':
			return length - 1
		}
	}

	return length
}
