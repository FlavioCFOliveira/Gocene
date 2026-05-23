// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package es

// SpanishLightStemmer implements a light stemming algorithm for Spanish.
//
// This is the Go port of
// org.apache.lucene.analysis.es.SpanishLightStemmer from Apache Lucene 10.4.0.
//
// The algorithm is described in:
// "Report on CLEF-2001 Experiments" — Jacques Savoy.
// Original code copyright (c) 2005, Jacques Savoy; BSD licensed.
//
// Deviation: Java operates on a char[] in-place and returns the new length.
// Go operates on a []rune in-place (for correct Unicode support) and returns
// the new length.
type SpanishLightStemmer struct{}

// NewSpanishLightStemmer creates a new SpanishLightStemmer.
func NewSpanishLightStemmer() *SpanishLightStemmer { return &SpanishLightStemmer{} }

// Stem applies light stemming to s[:length] in-place and returns the new length.
// The caller must ensure len(s) >= length.
func (st *SpanishLightStemmer) Stem(s []rune, length int) int {
	if length < 5 {
		return length
	}
	// Normalise accented vowels.
	for i := 0; i < length; i++ {
		switch s[i] {
		case 'à', 'á', 'â', 'ä':
			s[i] = 'a'
		case 'ò', 'ó', 'ô', 'ö':
			s[i] = 'o'
		case 'è', 'é', 'ê', 'ë':
			s[i] = 'e'
		case 'ù', 'ú', 'û', 'ü':
			s[i] = 'u'
		case 'ì', 'í', 'î', 'ï':
			s[i] = 'i'
		}
	}
	switch s[length-1] {
	case 'o', 'a', 'e':
		return length - 1
	case 's':
		if s[length-2] == 'e' && s[length-3] == 's' && s[length-4] == 'e' {
			return length - 2
		}
		if s[length-2] == 'e' && s[length-3] == 'c' {
			s[length-3] = 'z'
			return length - 2
		}
		if s[length-2] == 'o' || s[length-2] == 'a' || s[length-2] == 'e' {
			return length - 2
		}
	}
	return length
}
