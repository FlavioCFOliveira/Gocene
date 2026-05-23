// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

// czechStemmer implements a light stemmer for Czech.
//
// Go port of org.apache.lucene.analysis.cz.CzechStemmer (Apache Lucene 10.4.0).
// The Java original is package-private; this implementation is also unexported.
//
// Reference:
//
//	"Indexing and stemming approaches for the Czech language"
//	http://portal.acm.org/citation.cfm?id=1598600
type czechStemmer struct{}

// stem applies Czech stemming to s[:length] in-place and returns the new length.
//
// NOTE: Input is expected to be in lowercase, but with diacritical marks.
func (czechStemmer) stem(s []rune, length int) int {
	length = czechRemoveCase(s, length)
	length = czechRemovePossessives(s, length)
	if length > 0 {
		length = czechNormalize(s, length)
	}
	return length
}

func czechRemoveCase(s []rune, length int) int {
	if length > 7 && runesEndWith(s, length, "atech") {
		return length - 5
	}

	if length > 6 && (runesEndWith(s, length, "ětem") ||
		runesEndWith(s, length, "etem") ||
		runesEndWith(s, length, "atům")) {
		return length - 4
	}

	if length > 5 && (runesEndWith(s, length, "ech") ||
		runesEndWith(s, length, "ich") ||
		runesEndWith(s, length, "ích") ||
		runesEndWith(s, length, "ého") ||
		runesEndWith(s, length, "ěmi") ||
		runesEndWith(s, length, "emi") ||
		runesEndWith(s, length, "ému") ||
		runesEndWith(s, length, "ěte") ||
		runesEndWith(s, length, "ete") ||
		runesEndWith(s, length, "ěti") ||
		runesEndWith(s, length, "eti") ||
		runesEndWith(s, length, "ího") ||
		runesEndWith(s, length, "iho") ||
		runesEndWith(s, length, "ími") ||
		runesEndWith(s, length, "ímu") ||
		runesEndWith(s, length, "imu") ||
		runesEndWith(s, length, "ách") ||
		runesEndWith(s, length, "ata") ||
		runesEndWith(s, length, "aty") ||
		runesEndWith(s, length, "ých") ||
		runesEndWith(s, length, "ama") ||
		runesEndWith(s, length, "ami") ||
		runesEndWith(s, length, "ové") ||
		runesEndWith(s, length, "ovi") ||
		runesEndWith(s, length, "ými")) {
		return length - 3
	}

	if length > 4 && (runesEndWith(s, length, "em") ||
		runesEndWith(s, length, "es") ||
		runesEndWith(s, length, "ém") ||
		runesEndWith(s, length, "ím") ||
		runesEndWith(s, length, "ům") ||
		runesEndWith(s, length, "at") ||
		runesEndWith(s, length, "ám") ||
		runesEndWith(s, length, "os") ||
		runesEndWith(s, length, "us") ||
		runesEndWith(s, length, "ým") ||
		runesEndWith(s, length, "mi") ||
		runesEndWith(s, length, "ou")) {
		return length - 2
	}

	if length > 3 {
		switch s[length-1] {
		case 'a', 'e', 'i', 'o', 'u', 'ů', 'y', 'á', 'é', 'í', 'ý', 'ě':
			return length - 1
		}
	}

	return length
}

func czechRemovePossessives(s []rune, length int) int {
	if length > 5 && (runesEndWith(s, length, "ov") ||
		runesEndWith(s, length, "in") ||
		runesEndWith(s, length, "ův")) {
		return length - 2
	}
	return length
}

func czechNormalize(s []rune, length int) int {
	if runesEndWith(s, length, "čt") { // čt -> ck
		s[length-2] = 'c'
		s[length-1] = 'k'
		return length
	}

	if runesEndWith(s, length, "št") { // št -> sk
		s[length-2] = 's'
		s[length-1] = 'k'
		return length
	}

	switch s[length-1] {
	case 'c', 'č': // [cč] -> k
		s[length-1] = 'k'
		return length
	case 'z', 'ž': // [zž] -> h
		s[length-1] = 'h'
		return length
	}

	if length > 1 && s[length-2] == 'e' {
		s[length-2] = s[length-1] // e* -> *
		return length - 1
	}

	if length > 2 && s[length-2] == 'ů' {
		s[length-2] = 'o' // *ů* -> *o*
		return length
	}

	return length
}
