// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"unicode"
	"unicode/utf8"
)

// PorterStemFilter applies the Porter Stemming Algorithm to tokens.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.PorterStemFilter.
//
// The Porter Stemming Algorithm is a process for removing the commoner
// morphological and inflexional endings from words in English. Its main
// use is as part of a term normalisation process that is usually done
// when setting up Information Retrieval systems.
type PorterStemFilter struct {
	*BaseTokenFilter

	// stemmer is the Porter stemmer instance
	stemmer *PorterStemmer
}

// PorterStemmer implements the Porter Stemming Algorithm.
type PorterStemmer struct {
	// b is the buffer for the word being stemmed
	b []rune

	// k is the length of the word
	k int

	// j is a general offset to the end
	j int
}

// NewPorterStemFilter creates a new PorterStemFilter wrapping the given input.
func NewPorterStemFilter(input TokenStream) *PorterStemFilter {
	return &PorterStemFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		stemmer:         NewPorterStemmer(),
	}
}

// IncrementToken processes the next token and applies stemming.
func (f *PorterStemFilter) IncrementToken() (bool, error) {
	hasToken, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}

	if hasToken {
		if attr := f.GetAttributeSource().GetAttribute(CharTermAttributeType); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				term := termAttr.String()
				stemmed := f.stemmer.Stem(term)
				if stemmed != term {
					termAttr.SetEmpty()
					termAttr.AppendString(stemmed)
				}
			}
		}
	}

	return hasToken, nil
}

// NewPorterStemmer creates a new PorterStemmer.
func NewPorterStemmer() *PorterStemmer {
	return &PorterStemmer{
		b: make([]rune, 0, 50),
	}
}

// Stem applies the Porter stemming algorithm to the given word.
func (p *PorterStemmer) Stem(word string) string {
	if utf8.RuneCountInString(word) <= 2 {
		return word
	}

	// Convert to lowercase and initialize
	p.b = p.b[:0]
	for _, r := range word {
		p.b = append(p.b, unicode.ToLower(r))
	}
	p.k = len(p.b)
	p.j = 0

	// Apply stemming steps
	p.step1a()
	p.step1b()
	p.step1c()
	p.step2()
	p.step3()
	p.step4()
	p.step5a()
	p.step5b()

	return string(p.b[:p.k])
}

// step1a handles plurals and past participles.
func (p *PorterStemmer) step1a() {
	if p.k > 0 {
		switch p.b[p.k-1] {
		case 's':
			if p.endsWith("sses") {
				p.k -= 2
			} else if p.endsWith("ies") {
				p.k -= 2
			} else if p.b[p.k-2] != 's' {
				p.k--
			}
		}
	}
}

// step1b handles past participles.
func (p *PorterStemmer) step1b() {
	if p.endsWith("eed") {
		if p.measure() > 0 {
			p.k--
		}
	} else if (p.endsWith("ed") || p.endsWith("ing")) && p.hasVowelInStem() {
		p.k = p.j
		if p.endsWith("at") || p.endsWith("bl") || p.endsWith("iz") {
			p.k++
			p.b[p.k-1] = 'e'
		} else if p.isDoubleConsonant(p.k - 1) {
			ch := p.b[p.k-1] // save the removed char before decrementing
			p.k--
			if ch != 'l' && ch != 's' && ch != 'z' {
				// keep the shorter stem (double consonant collapsed)
			} else {
				p.k++ // restore: l/s/z double consonants are kept
			}
		} else if p.measure() == 1 && p.isCVC(p.k-1) {
			p.k++
			p.b[p.k-1] = 'e'
		}
	}
}

// step1c handles final y -> i.
func (p *PorterStemmer) step1c() {
	if p.endsWith("y") && p.hasVowelInStem() {
		p.b[p.k-1] = 'i'
	}
}

// step2 handles various suffixes.
func (p *PorterStemmer) step2() {
	switch p.b[p.k-2] {
	case 'a':
		if p.endsWith("ational") {
			p.replaceSuffix("ational", "ate")
		} else if p.endsWith("tional") {
			p.replaceSuffix("tional", "tion")
		}
	case 'c':
		if p.endsWith("enci") {
			p.replaceSuffix("enci", "ence")
		} else if p.endsWith("anci") {
			p.replaceSuffix("anci", "ance")
		}
	case 'e':
		if p.endsWith("izer") {
			p.replaceSuffix("izer", "ize")
		}
	case 'l':
		if p.endsWith("bli") {
			p.replaceSuffix("bli", "ble")
		} else if p.endsWith("alli") {
			p.replaceSuffix("alli", "al")
		} else if p.endsWith("entli") {
			p.replaceSuffix("entli", "ent")
		} else if p.endsWith("eli") {
			p.replaceSuffix("eli", "e")
		} else if p.endsWith("ousli") {
			p.replaceSuffix("ousli", "ous")
		}
	case 'o':
		if p.endsWith("ization") {
			p.replaceSuffix("ization", "ize")
		} else if p.endsWith("ation") {
			p.replaceSuffix("ation", "ate")
		} else if p.endsWith("ator") {
			p.replaceSuffix("ator", "ate")
		}
	case 's':
		if p.endsWith("alism") {
			p.replaceSuffix("alism", "al")
		} else if p.endsWith("iveness") {
			p.replaceSuffix("iveness", "ive")
		} else if p.endsWith("fulness") {
			p.replaceSuffix("fulness", "ful")
		} else if p.endsWith("ousness") {
			p.replaceSuffix("ousness", "ous")
		}
	case 't':
		if p.endsWith("aliti") {
			p.replaceSuffix("aliti", "al")
		} else if p.endsWith("iviti") {
			p.replaceSuffix("iviti", "ive")
		} else if p.endsWith("biliti") {
			p.replaceSuffix("biliti", "ble")
		}
	case 'g':
		if p.endsWith("logi") {
			p.replaceSuffix("logi", "log")
		}
	}
}

// step3 handles more suffixes.
func (p *PorterStemmer) step3() {
	switch p.b[p.k-1] {
	case 'e':
		if p.endsWith("icate") {
			p.replaceSuffix("icate", "ic")
		} else if p.endsWith("ative") {
			p.replaceSuffix("ative", "")
		} else if p.endsWith("alize") {
			p.replaceSuffix("alize", "al")
		}
	case 'i':
		if p.endsWith("iciti") {
			p.replaceSuffix("iciti", "ic")
		}
	case 'l':
		if p.endsWith("ical") {
			p.replaceSuffix("ical", "ic")
		} else if p.endsWith("ful") {
			p.replaceSuffix("ful", "")
		}
	case 's':
		if p.endsWith("ness") {
			p.replaceSuffix("ness", "")
		}
	}
}

// step4 corresponds to Lucene's step5. It identifies the LONGEST matching suffix
// and then checks m()>1 ONCE — matching Lucene's break-then-check pattern.
// Unlike the previous else-if chain, a suffix match locks in j; a shorter suffix
// can never override it even if m() for the longer one is ≤1.
func (p *PorterStemmer) step4() {
	if p.k <= 1 {
		return
	}

	matched := false
	switch p.b[p.k-2] {
	case 'a':
		matched = p.endsWith("al")
	case 'c':
		matched = p.endsWith("ance") || p.endsWith("ence")
	case 'e':
		matched = p.endsWith("er")
	case 'i':
		matched = p.endsWith("ic")
	case 'l':
		matched = p.endsWith("able") || p.endsWith("ible")
	case 'n':
		// Longest-first to match Lucene's suffix priority order.
		matched = p.endsWith("ant") || p.endsWith("ement") ||
			p.endsWith("ment") || p.endsWith("ent")
	case 'o':
		// "ion" requires the char before it to be 's' or 't' (Lucene Bug-2 fix).
		if p.endsWith("ion") {
			if p.j > 0 && (p.b[p.j-1] == 's' || p.b[p.j-1] == 't') {
				matched = true
			}
		} else {
			matched = p.endsWith("ou")
		}
	case 's':
		matched = p.endsWith("ism")
	case 't':
		matched = p.endsWith("ate") || p.endsWith("iti")
	case 'u':
		matched = p.endsWith("ous")
	case 'v':
		matched = p.endsWith("ive")
	case 'z':
		matched = p.endsWith("ize")
	}

	// Remove the suffix only when the stem has m > 1 (Lucene: if (m() > 1) k = j).
	if matched && p.measure() > 1 {
		p.k = p.j
	}
}

// step5a and step5b correspond to Lucene's step6, which begins by setting
// j = k (measuring the full current word) before any suffix check.
// This is critical for the correct VC-sequence count in both sub-steps.

// step5a removes a final 'e' when the stem has m>1, or m==1 and is not CVC.
func (p *PorterStemmer) step5a() {
	// Set j to the full word length so measure() covers the entire word.
	p.j = p.k
	if p.b[p.k-1] == 'e' {
		a := p.measure()
		if a > 1 || (a == 1 && !p.isCVC(p.k-2)) {
			p.k--
		}
	}
}

// step5b collapses a final double-l when m>1.
func (p *PorterStemmer) step5b() {
	// j is still p.k (set by step5a above); no change needed.
	if p.b[p.k-1] == 'l' && p.isDoubleConsonant(p.k-1) && p.measure() > 1 {
		p.k--
	}
}

// Helper functions

func (p *PorterStemmer) endsWith(suffix string) bool {
	suffixLen := len(suffix)
	if p.k < suffixLen {
		return false
	}
	for i := 0; i < suffixLen; i++ {
		if p.b[p.k-suffixLen+i] != rune(suffix[i]) {
			return false
		}
	}
	p.j = p.k - suffixLen
	return true
}

func (p *PorterStemmer) replaceSuffix(oldSuffix, newSuffix string) {
	if p.measure() > 0 {
		p.k = p.j
		for _, r := range newSuffix {
			p.b[p.k] = r
			p.k++
		}
	}
}

// measure counts VC sequences in the stem b[0..j-1], where j is the start
// index of the suffix set by the most recent endsWith call. This mirrors Lucene's
// m() which uses j as an inclusive 0-indexed end (Lucene j == Gocene j-1).
func (p *PorterStemmer) measure() int {
	n := 0
	i := 0

	// Skip initial consonants; stop when a vowel is found.
	for {
		if i >= p.j {
			return n
		}
		if p.isVowel(i) {
			break
		}
		i++
	}
	i++ // advance past the first vowel

	for {
		// Skip vowels until a consonant is found.
		for {
			if i >= p.j {
				return n
			}
			if !p.isVowel(i) {
				break
			}
			i++
		}
		i++ // advance past the consonant
		n++

		// Skip consonants until a vowel is found.
		for {
			if i >= p.j {
				return n
			}
			if p.isVowel(i) {
				break
			}
			i++
		}
		i++ // advance past the vowel
	}
}

func (p *PorterStemmer) isVowel(i int) bool {
	switch p.b[i] {
	case 'a', 'e', 'i', 'o', 'u':
		return true
	case 'y':
		return i > 0 && !p.isVowel(i-1)
	}
	return false
}

func (p *PorterStemmer) hasVowelInStem() bool {
	for i := 0; i < p.j; i++ {
		if p.isVowel(i) {
			return true
		}
	}
	return false
}

func (p *PorterStemmer) isDoubleConsonant(j int) bool {
	if j < 1 {
		return false
	}
	if p.b[j] != p.b[j-1] {
		return false
	}
	return !p.isVowel(j)
}

func (p *PorterStemmer) isCVC(j int) bool {
	if j < 2 {
		return false
	}
	if p.isVowel(j) || !p.isVowel(j-1) || p.isVowel(j-2) {
		return false
	}
	// Check if ends with w, x, or y
	c := p.b[j]
	return c != 'w' && c != 'x' && c != 'y'
}
