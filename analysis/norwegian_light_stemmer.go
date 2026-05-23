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

// NorwegianVariant selects which Norwegian variant(s) to stem.
type NorwegianVariant int

const (
	// NorwegianBokmaal enables removal of Bokmål-specific endings.
	NorwegianBokmaal NorwegianVariant = 1
	// NorwegianNynorsk enables removal of Nynorsk-specific endings.
	NorwegianNynorsk NorwegianVariant = 2
)

// norwegianLightStemmer implements a light stemmer for Norwegian (Bokmål and/or
// Nynorsk).
//
// Go port of org.apache.lucene.analysis.no.NorwegianLightStemmer (Apache
// Lucene 10.4.0). The Java original is package-private; this implementation
// is also unexported.
//
// Parts of this stemmer are adapted from SwedishLightStemmer, except that
// while the Swedish one has a pre-defined rule set and a corresponding corpus
// to validate against, the Norwegian one is hand crafted.
type norwegianLightStemmer struct {
	useBokmaal bool
	useNynorsk bool
}

// newNorwegianLightStemmer creates a stemmer for the given variant(s).
// flags should be NorwegianBokmaal, NorwegianNynorsk, or both OR'd together.
func newNorwegianLightStemmer(flags NorwegianVariant) norwegianLightStemmer {
	if flags <= 0 || int(flags) > int(NorwegianBokmaal)+int(NorwegianNynorsk) {
		panic("norwegianLightStemmer: invalid flags")
	}
	return norwegianLightStemmer{
		useBokmaal: flags&NorwegianBokmaal != 0,
		useNynorsk: flags&NorwegianNynorsk != 0,
	}
}

// stem applies the Norwegian light stemming algorithm to s[:length] in-place
// and returns the new length.
func (n norwegianLightStemmer) stem(s []rune, length int) int {
	// Remove possessive -s (bilens -> bilen) and continue checking.
	if length > 4 && s[length-1] == 's' {
		length--
	}

	// Remove common endings (5-char), single-pass.
	if length > 7 &&
		((runesEndWith(s, length, "heter") && n.useBokmaal) ||
			(runesEndWith(s, length, "heten") && n.useBokmaal) ||
			(runesEndWith(s, length, "heita") && n.useNynorsk)) {
		return length - 5
	}

	// Remove Nynorsk common endings (6-char), single-pass.
	if length > 8 && n.useNynorsk &&
		(runesEndWith(s, length, "heiter") ||
			runesEndWith(s, length, "leiken") ||
			runesEndWith(s, length, "leikar")) {
		return length - 6
	}

	if length > 5 &&
		(runesEndWith(s, length, "dom") ||
			(runesEndWith(s, length, "het") && n.useBokmaal)) {
		return length - 3
	}

	if length > 6 && n.useNynorsk &&
		(runesEndWith(s, length, "heit") ||
			runesEndWith(s, length, "semd") ||
			runesEndWith(s, length, "leik")) {
		return length - 4
	}

	if length > 7 &&
		(runesEndWith(s, length, "elser") || runesEndWith(s, length, "elsen")) {
		return length - 5
	}

	if length > 6 &&
		((runesEndWith(s, length, "ende") && n.useBokmaal) ||
			(runesEndWith(s, length, "ande") && n.useNynorsk) ||
			runesEndWith(s, length, "else") ||
			(runesEndWith(s, length, "este") && n.useBokmaal) ||
			(runesEndWith(s, length, "aste") && n.useNynorsk) ||
			(runesEndWith(s, length, "eren") && n.useBokmaal) ||
			(runesEndWith(s, length, "aren") && n.useNynorsk)) {
		return length - 4
	}

	if length > 5 &&
		((runesEndWith(s, length, "ere") && n.useBokmaal) ||
			(runesEndWith(s, length, "are") && n.useNynorsk) ||
			(runesEndWith(s, length, "est") && n.useBokmaal) ||
			(runesEndWith(s, length, "ast") && n.useNynorsk) ||
			runesEndWith(s, length, "ene") ||
			(runesEndWith(s, length, "ane") && n.useNynorsk)) {
		return length - 3
	}

	if length > 4 &&
		(runesEndWith(s, length, "er") ||
			runesEndWith(s, length, "en") ||
			runesEndWith(s, length, "et") ||
			(runesEndWith(s, length, "ar") && n.useNynorsk) ||
			(runesEndWith(s, length, "st") && n.useBokmaal) ||
			runesEndWith(s, length, "te")) {
		return length - 2
	}

	if length > 3 {
		switch s[length-1] {
		case 'a', 'e', 'n':
			return length - 1
		}
	}

	return length
}
