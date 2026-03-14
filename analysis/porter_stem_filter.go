// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"unicode"
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
		if attr := f.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
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
	if len(word) <= 2 {
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
		} else if p.isDoubleConsonant(p.k-1) {
			p.k--
			if p.endsWith("l") || p.endsWith("s") || p.endsWith("z") {
				p.k++
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

// step4 handles more suffixes.
func (p *PorterStemmer) step4() {
	if p.k > 1 {
		switch p.b[p.k-2] {
		case 'a':
			if p.endsWith("al") && p.measure() > 1 {
				p.k -= 2
			}
		case 'c':
			if p.endsWith("ance") && p.measure() > 1 {
				p.k -= 4
			} else if p.endsWith("ence") && p.measure() > 1 {
				p.k -= 4
			}
		case 'e':
			if p.endsWith("er") && p.measure() > 1 {
				p.k -= 2
			}
		case 'i':
			if p.endsWith("ic") && p.measure() > 1 {
				p.k -= 2
			}
		case 'l':
			if p.endsWith("able") && p.measure() > 1 {
				p.k -= 4
			} else if p.endsWith("ible") && p.measure() > 1 {
				p.k -= 4
			}
		case 'n':
			if p.endsWith("ant") && p.measure() > 1 {
				p.k -= 3
			} else if p.endsWith("ement") && p.measure() > 1 {
				p.k -= 5
			} else if p.endsWith("ment") && p.measure() > 1 {
				p.k -= 4
			} else if p.endsWith("ent") && p.measure() > 1 {
				p.k -= 3
			}
		case 'o':
			if (p.endsWith("ion") && p.k > 4) && p.measure() > 1 {
				p.k -= 3
			} else if p.endsWith("ou") && p.measure() > 1 {
				p.k -= 2
			}
		case 's':
			if p.endsWith("ism") && p.measure() > 1 {
				p.k -= 3
			}
		case 't':
			if p.endsWith("ate") && p.measure() > 1 {
				p.k -= 3
			} else if p.endsWith("iti") && p.measure() > 1 {
				p.k -= 3
			}
		case 'u':
			if p.endsWith("ous") && p.measure() > 1 {
				p.k -= 3
			}
		case 'v':
			if p.endsWith("ive") && p.measure() > 1 {
				p.k -= 3
			}
		case 'z':
			if p.endsWith("ize") && p.measure() > 1 {
				p.k -= 3
			}
		}
	}
}

// step5a handles final e.
func (p *PorterStemmer) step5a() {
	if p.endsWith("e") {
		a := p.measure()
		if a > 1 || (a == 1 && !p.isCVC(p.k-2)) {
			p.k--
		}
	}
}

// step5b handles double consonants.
func (p *PorterStemmer) step5b() {
	if p.isDoubleConsonant(p.k-1) && p.measure() > 1 {
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

func (p *PorterStemmer) measure() int {
	n := 0
	i := 0

	// Skip initial consonants
	for i < p.k && !p.isVowel(i) {
		i++
	}

	// Count VC sequences
	for i < p.k {
		// Skip vowels
		for i < p.k && p.isVowel(i) {
			i++
		}
		if i >= p.k {
			break
		}
		// Skip consonants
		for i < p.k && !p.isVowel(i) {
			i++
		}
		n++
	}

	return n
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
