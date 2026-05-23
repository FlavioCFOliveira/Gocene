// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package te

// teluguStemmer implements a simple rule-based stemmer for Telugu.
//
// Go port of org.apache.lucene.analysis.te.TeluguStemmer (Apache Lucene
// 10.4.0). The Java original is package-private; this implementation is also
// unexported.
type teluguStemmer struct{}

// runesEndWith reports whether s[:length] ends with the given suffix.
func runesEndWith(s []rune, length int, suffix string) bool {
	sr := []rune(suffix)
	l := len(sr)
	if length < l {
		return false
	}
	for i, r := range sr {
		if s[length-l+i] != r {
			return false
		}
	}
	return true
}

// stem applies Telugu suffix stripping to s[:length] in-place and returns the
// new length.
func (teluguStemmer) stem(s []rune, length int) int {
	// 4-rune suffixes
	if length > 5 && (runesEndWith(s, length, "ళ్ళు") || runesEndWith(s, length, "డ్లు")) {
		return length - 4
	}

	// 2-rune suffixes
	if length > 3 && (runesEndWith(s, length, "డు") ||
		runesEndWith(s, length, "ము") ||
		runesEndWith(s, length, "వు") ||
		runesEndWith(s, length, "లు") ||
		runesEndWith(s, length, "ని") ||
		runesEndWith(s, length, "ను") ||
		runesEndWith(s, length, "చే") ||
		runesEndWith(s, length, "కై") ||
		runesEndWith(s, length, "లో") ||
		runesEndWith(s, length, "ది") ||
		runesEndWith(s, length, "కి") ||
		runesEndWith(s, length, "సు") ||
		runesEndWith(s, length, "వై") ||
		runesEndWith(s, length, "పై")) {
		return length - 2
	}

	// 1-rune suffixes
	if length > 2 && (runesEndWith(s, length, "ి") ||
		runesEndWith(s, length, "ీ") ||
		runesEndWith(s, length, "ు") ||
		runesEndWith(s, length, "ూ") ||
		runesEndWith(s, length, "ె") ||
		runesEndWith(s, length, "ే") ||
		runesEndWith(s, length, "ొ") ||
		runesEndWith(s, length, "ో") ||
		runesEndWith(s, length, "ా")) {
		return length - 1
	}

	return length
}
