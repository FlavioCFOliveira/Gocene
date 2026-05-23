// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

/*
Copyright © 2003,
Center for Intelligent Information Retrieval,
University of Massachusetts, Amherst.
All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

1. Redistributions of source code must retain the above copyright notice,
this list of conditions and the following disclaimer.

2. Redistributions in binary form must reproduce the above copyright notice,
this list of conditions and the following disclaimer in the documentation
and/or other materials provided with the distribution.

3. The names "Center for Intelligent Information Retrieval" and
"University of Massachusetts" must not be used to endorse or promote products
derived from this software without prior written permission.

THIS SOFTWARE IS PROVIDED BY UNIVERSITY OF MASSACHUSETTS AND OTHER CONTRIBUTORS
"AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO,
THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDERS OR CONTRIBUTORS BE
LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE
GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION)
HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT
LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY
OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH
DAMAGE.
*/

// Package en provides English language analysis components.
package en

import (
	"sync"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// maxWordLen is the maximum word length processed by KStemmer.
const maxWordLen = 50

// dictEntry holds a stem and an exception flag.
type dictEntry struct {
	root      string // empty string means "word is its own stem"
	exception bool
}

// kStemDict is the shared, lazily-initialised dictionary.
var (
	kStemDictOnce sync.Once
	kStemDict     *analysis.CharArrayMap[*dictEntry]
)

func getKStemDict() *analysis.CharArrayMap[*dictEntry] {
	kStemDictOnce.Do(func() {
		kStemDict = initKStemDict()
	})
	return kStemDict
}

// initKStemDict builds the static dictionary from all word lists.
func initKStemDict() *analysis.CharArrayMap[*dictEntry] {
	d := analysis.NewCharArrayMap[*dictEntry](30000, false)

	// exception words — stem is the word itself, marked as exception
	for _, w := range kStemExceptionWords {
		entry := &dictEntry{root: w, exception: true}
		d.Put(w, entry)
	}

	// direct conflations — word maps to a different stem
	for _, pair := range kStemDirectConflations {
		entry := &dictEntry{root: pair[1], exception: false}
		d.Put(pair[0], entry)
	}

	// country-nationality pairs
	for _, pair := range kStemCountryNationality {
		entry := &dictEntry{root: pair[1], exception: false}
		d.Put(pair[0], entry)
	}

	// head word lists — word is its own stem
	defaultEntry := &dictEntry{}
	for _, list := range [][]string{
		kStemData1, kStemData2, kStemData3, kStemData4,
		kStemData5, kStemData6, kStemData7, kStemData8,
	} {
		for _, w := range list {
			if d.GetString(w) == nil {
				d.Put(w, defaultEntry)
			}
		}
	}

	// supplement dict
	for _, w := range kStemSupplementDict {
		if d.GetString(w) == nil {
			d.Put(w, defaultEntry)
		}
	}

	// proper nouns
	for _, w := range kStemProperNouns {
		if d.GetString(w) == nil {
			d.Put(w, defaultEntry)
		}
	}

	return d
}

// kStemmer implements the KStem stemming algorithm for English.
//
// Go port of org.apache.lucene.analysis.en.KStemmer (Apache Lucene 10.4.0).
// This is package-private in Java; exported here for use by KStemFilter.
type kStemmer struct {
	word []rune // mutable word buffer (like Java OpenStringBuilder)
	k    int    // index of last character (word length = k+1)
	j    int    // index of last character of the stem

	matchedEntry *dictEntry
	result       string
}

// newKStemmer creates a new kStemmer instance.
func newKStemmer() *kStemmer {
	return &kStemmer{word: make([]rune, 0, maxWordLen+10)}
}

// wordLength returns the current length of the word buffer.
func (s *kStemmer) wordLength() int { return s.k + 1 }

func (s *kStemmer) charAt(i int) rune { return s.word[i] }
func (s *kStemmer) setCharAt(i int, ch rune) { s.word[i] = ch }
func (s *kStemmer) setLength(n int) { s.word = s.word[:n] }
func (s *kStemmer) appendRune(ch rune) { s.word = append(s.word, ch) }
func (s *kStemmer) appendStr(str string) {
	for _, r := range str {
		s.word = append(s.word, r)
	}
}

func (s *kStemmer) penultChar() rune { return s.word[s.k-1] }

func (s *kStemmer) isVowel(index int) bool { return !s.isCons(index) }

func (s *kStemmer) isCons(index int) bool {
	ch := s.word[index]
	if ch == 'a' || ch == 'e' || ch == 'i' || ch == 'o' || ch == 'u' {
		return false
	}
	if ch != 'y' || index == 0 {
		return true
	}
	return !s.isCons(index - 1)
}

func (s *kStemmer) stemLength() int { return s.j + 1 }

func (s *kStemmer) endsInRunes(suffix []rune) bool {
	if len(suffix) > s.k {
		return false
	}
	r := len(s.word) - len(suffix)
	for i, ch := range suffix {
		if ch != s.word[r+i] {
			return false
		}
	}
	s.j = r - 1
	return true
}

func (s *kStemmer) endsIn2(a, b rune) bool {
	if 2 > s.k {
		return false
	}
	if s.word[s.k-1] == a && s.word[s.k] == b {
		s.j = s.k - 2
		return true
	}
	return false
}

func (s *kStemmer) endsIn3(a, b, c rune) bool {
	if 3 > s.k {
		return false
	}
	if s.word[s.k-2] == a && s.word[s.k-1] == b && s.word[s.k] == c {
		s.j = s.k - 3
		return true
	}
	return false
}

func (s *kStemmer) endsIn4(a, b, c, d rune) bool {
	if 4 > s.k {
		return false
	}
	if s.word[s.k-3] == a && s.word[s.k-2] == b && s.word[s.k-1] == c && s.word[s.k] == d {
		s.j = s.k - 4
		return true
	}
	return false
}

func (s *kStemmer) wordInDict() *dictEntry {
	if s.matchedEntry != nil {
		return s.matchedEntry
	}
	e := getKStemDict().Get(s.word, 0, len(s.word))
	if e != nil && !e.exception {
		s.matchedEntry = e
	}
	return e
}

func (s *kStemmer) setSuffix(str string) {
	s.setSuff(str, len([]rune(str)))
}

func (s *kStemmer) setSuff(str string, length int) {
	s.setLength(s.j + 1)
	runes := []rune(str)
	for i := 0; i < length; i++ {
		s.appendRune(runes[i])
	}
	s.k = s.j + length
}

func (s *kStemmer) lookup() bool {
	s.matchedEntry = getKStemDict().Get(s.word, 0, len(s.word))
	return s.matchedEntry != nil
}

func (s *kStemmer) matched() bool {
	return s.matchedEntry != nil
}

func (s *kStemmer) doubleC(i int) bool {
	if i < 1 {
		return false
	}
	if s.word[i] != s.word[i-1] {
		return false
	}
	return s.isCons(i)
}

func (s *kStemmer) vowelInStem() bool {
	for i := 0; i < s.stemLength(); i++ {
		if s.isVowel(i) {
			return true
		}
	}
	return false
}

func (s *kStemmer) plural() {
	if s.word[s.k] != 's' {
		return
	}
	if s.endsIn3('i', 'e', 's') {
		s.setLength(s.j + 3)
		s.k--
		if s.lookup() {
			return
		}
		s.k++
		s.appendRune('s')
		s.setSuffix("y")
		s.lookup()
	} else if s.endsIn2('e', 's') {
		s.setLength(s.j + 2)
		s.k--
		tryE := s.j > 0 && !((s.word[s.j] == 's') && (s.word[s.j-1] == 's'))
		if tryE && s.lookup() {
			return
		}
		s.setLength(s.j + 1)
		s.k--
		if s.lookup() {
			return
		}
		s.appendRune('e')
		s.k++
		if !tryE {
			s.lookup()
		}
	} else {
		if len(s.word) > 3 && s.penultChar() != 's' && !s.endsIn3('o', 'u', 's') {
			s.setLength(s.k)
			s.k--
			s.lookup()
		}
	}
}

func (s *kStemmer) pastTense() {
	if len(s.word) <= 4 {
		return
	}
	if s.endsIn3('i', 'e', 'd') {
		s.setLength(s.j + 3)
		s.k--
		if s.lookup() {
			return
		}
		s.k++
		s.appendRune('d')
		s.setSuffix("y")
		s.lookup()
		return
	}
	if s.endsIn2('e', 'd') && s.vowelInStem() {
		s.setLength(s.j + 2)
		s.k = s.j + 1
		entry := s.wordInDict()
		if entry != nil && !entry.exception {
			return
		}
		s.setLength(s.j + 1)
		s.k = s.j
		if s.lookup() {
			return
		}
		if s.doubleC(s.k) {
			s.setLength(s.k)
			s.k--
			if s.lookup() {
				return
			}
			s.appendRune(s.word[s.k])
			s.k++
			s.lookup()
			return
		}
		if s.word[0] == 'u' && s.word[1] == 'n' {
			s.appendRune('e')
			s.appendRune('d')
			s.k += 2
			return
		}
		s.setLength(s.j + 1)
		s.appendRune('e')
		s.k = s.j + 1
	}
}

func (s *kStemmer) aspect() {
	if len(s.word) <= 5 {
		return
	}
	if s.endsIn3('i', 'n', 'g') && s.vowelInStem() {
		s.setCharAt(s.j+1, 'e')
		s.setLength(s.j + 2)
		s.k = s.j + 1
		entry := s.wordInDict()
		if entry != nil && !entry.exception {
			return
		}
		s.setLength(s.k)
		s.k--
		if s.lookup() {
			return
		}
		if s.doubleC(s.k) {
			s.k--
			s.setLength(s.k + 1)
			if s.lookup() {
				return
			}
			s.appendRune(s.word[s.k])
			s.k++
			s.lookup()
			return
		}
		if s.j > 0 && s.isCons(s.j) && s.isCons(s.j-1) {
			s.k = s.j
			s.setLength(s.k + 1)
			return
		}
		s.setLength(s.j + 1)
		s.appendRune('e')
		s.k = s.j + 1
	}
}

func (s *kStemmer) ityEndings() {
	oldK := s.k
	if s.endsIn3('i', 't', 'y') {
		s.setLength(s.j + 1)
		s.k = s.j
		if s.lookup() {
			return
		}
		s.appendRune('e')
		s.k = s.j + 1
		if s.lookup() {
			return
		}
		s.setCharAt(s.j+1, 'i')
		s.appendStr("ty")
		s.k = oldK
		if s.j > 0 && s.word[s.j-1] == 'i' && s.word[s.j] == 'l' {
			s.setLength(s.j - 1)
			s.appendStr("le")
			s.k = s.j
			s.lookup()
			return
		}
		if s.j > 0 && s.word[s.j-1] == 'i' && s.word[s.j] == 'v' {
			s.setLength(s.j + 1)
			s.appendRune('e')
			s.k = s.j + 1
			s.lookup()
			return
		}
		if s.j > 0 && s.word[s.j-1] == 'a' && s.word[s.j] == 'l' {
			s.setLength(s.j + 1)
			s.k = s.j
			s.lookup()
			return
		}
		if s.lookup() {
			return
		}
		s.setLength(s.j + 1)
		s.k = s.j
	}
}

func (s *kStemmer) ncessEndings() {
	oldK := s.k
	if s.endsIn3('n', 'c', 'e') {
		wordChar := s.word[s.j]
		if wordChar != 'e' && wordChar != 'a' {
			return
		}
		s.setLength(s.j)
		s.appendRune('e')
		s.k = s.j
		if s.lookup() {
			return
		}
		s.setLength(s.j)
		s.k = s.j - 1
		if s.lookup() {
			return
		}
		s.appendRune(wordChar)
		s.appendStr("nce")
		s.k = oldK
	}
}

func (s *kStemmer) nessEndings() {
	if s.endsIn4('n', 'e', 's', 's') {
		s.setLength(s.j + 1)
		s.k = s.j
		if s.word[s.j] == 'i' {
			s.setCharAt(s.j, 'y')
		}
		s.lookup()
	}
}

func (s *kStemmer) ismEndings() {
	if s.endsIn3('i', 's', 'm') {
		s.setLength(s.j + 1)
		s.k = s.j
		s.lookup()
	}
}

func (s *kStemmer) mentEndings() {
	oldK := s.k
	if s.endsIn4('m', 'e', 'n', 't') {
		s.setLength(s.j + 1)
		s.k = s.j
		if s.lookup() {
			return
		}
		s.appendStr("ment")
		s.k = oldK
	}
}

func (s *kStemmer) izeEndings() {
	oldK := s.k
	if s.endsIn3('i', 'z', 'e') {
		s.setLength(s.j + 1)
		s.k = s.j
		if s.lookup() {
			return
		}
		s.appendRune('i')
		if s.doubleC(s.j) {
			s.setLength(s.j)
			s.k = s.j - 1
			if s.lookup() {
				return
			}
			s.appendRune(s.word[s.j-1])
		}
		s.setLength(s.j + 1)
		s.appendRune('e')
		s.k = s.j + 1
		if s.lookup() {
			return
		}
		s.setLength(s.j + 1)
		s.appendStr("ize")
		s.k = oldK
	}
}

func (s *kStemmer) ncyEndings() {
	if s.endsIn3('n', 'c', 'y') {
		if s.word[s.j] != 'e' && s.word[s.j] != 'a' {
			return
		}
		s.setCharAt(s.j+2, 't')
		s.setLength(s.j + 3)
		s.k = s.j + 2
		if s.lookup() {
			return
		}
		s.setCharAt(s.j+2, 'c')
		s.appendRune('e')
		s.k = s.j + 3
		s.lookup()
	}
}

func (s *kStemmer) bleEndings() {
	oldK := s.k
	if s.endsIn3('b', 'l', 'e') {
		if s.word[s.j] != 'a' && s.word[s.j] != 'i' {
			return
		}
		wordChar := s.word[s.j]
		s.setLength(s.j)
		s.k = s.j - 1
		if s.lookup() {
			return
		}
		if s.doubleC(s.k) {
			s.setLength(s.k)
			s.k--
			if s.lookup() {
				return
			}
			s.k++
			s.appendRune(s.word[s.k-1])
		}
		s.setLength(s.j)
		s.appendRune('e')
		s.k = s.j
		if s.lookup() {
			return
		}
		s.setLength(s.j)
		s.appendStr("ate")
		s.k = s.j + 2
		if s.lookup() {
			return
		}
		s.setLength(s.j)
		s.appendRune(wordChar)
		s.appendStr("ble")
		s.k = oldK
	}
}

func (s *kStemmer) icEndings() {
	if s.endsIn2('i', 'c') {
		s.setLength(s.j + 3)
		s.appendStr("al")
		s.k = s.j + 4
		if s.lookup() {
			return
		}
		s.setCharAt(s.j+1, 'y')
		s.setLength(s.j + 2)
		s.k = s.j + 1
		if s.lookup() {
			return
		}
		s.setCharAt(s.j+1, 'e')
		if s.lookup() {
			return
		}
		s.setLength(s.j + 1)
		s.k = s.j
		if s.lookup() {
			return
		}
		s.appendStr("ic")
		s.k = s.j + 2
	}
}

var (
	kIzation = []rune("ization")
	kItion   = []rune("ition")
	kAtion   = []rune("ation")
	kIcation = []rune("ication")
)

func (s *kStemmer) ionEndings() {
	oldK := s.k
	if !s.endsIn3('i', 'o', 'n') {
		return
	}
	if s.endsInRunes(kIzation) {
		s.setLength(s.j + 3)
		s.appendRune('e')
		s.k = s.j + 3
		s.lookup()
		return
	}
	if s.endsInRunes(kItion) {
		s.setLength(s.j + 1)
		s.appendRune('e')
		s.k = s.j + 1
		if s.lookup() {
			return
		}
		s.setLength(s.j + 1)
		s.appendStr("ition")
		s.k = oldK
	} else if s.endsInRunes(kAtion) {
		s.setLength(s.j + 3)
		s.appendRune('e')
		s.k = s.j + 3
		if s.lookup() {
			return
		}
		s.setLength(s.j + 1)
		s.appendRune('e')
		s.k = s.j + 1
		if s.lookup() {
			return
		}
		s.setLength(s.j + 1)
		s.k = s.j
		if s.lookup() {
			return
		}
		s.setLength(s.j + 1)
		s.appendStr("ation")
		s.k = oldK
	}
	if s.endsInRunes(kIcation) {
		s.setLength(s.j + 1)
		s.appendRune('y')
		s.k = s.j + 1
		if s.lookup() {
			return
		}
		s.setLength(s.j + 1)
		s.appendStr("ication")
		s.k = oldK
	}
	// fall through to -ion
	{
		s.j = s.k - 3
		s.setLength(s.j + 1)
		s.appendRune('e')
		s.k = s.j + 1
		if s.lookup() {
			return
		}
		s.setLength(s.j + 1)
		s.k = s.j
		if s.lookup() {
			return
		}
		s.setLength(s.j + 1)
		s.appendStr("ion")
		s.k = oldK
	}
}

func (s *kStemmer) erAndOrEndings() {
	oldK := s.k
	if s.word[s.k] != 'r' {
		return
	}
	if s.endsIn4('i', 'z', 'e', 'r') {
		s.setLength(s.j + 4)
		s.k = s.j + 3
		s.lookup()
		return
	}
	if s.endsIn2('e', 'r') || s.endsIn2('o', 'r') {
		wordChar := s.word[s.j+1]
		if s.doubleC(s.j) {
			s.setLength(s.j)
			s.k = s.j - 1
			if s.lookup() {
				return
			}
			s.appendRune(s.word[s.j-1])
		}
		if s.word[s.j] == 'i' {
			s.setCharAt(s.j, 'y')
			s.setLength(s.j + 1)
			s.k = s.j
			if s.lookup() {
				return
			}
			s.setCharAt(s.j, 'i')
			s.appendRune('e')
		}
		if s.word[s.j] == 'e' {
			s.setLength(s.j)
			s.k = s.j - 1
			if s.lookup() {
				return
			}
			s.appendRune('e')
		}
		s.setLength(s.j + 2)
		s.k = s.j + 1
		if s.lookup() {
			return
		}
		s.setLength(s.j + 1)
		s.k = s.j
		if s.lookup() {
			return
		}
		s.appendRune('e')
		s.k = s.j + 1
		if s.lookup() {
			return
		}
		s.setLength(s.j + 1)
		s.appendRune(wordChar)
		s.appendRune('r')
		s.k = oldK
	}
}

func (s *kStemmer) lyEndings() {
	oldK := s.k
	if s.endsIn2('l', 'y') {
		s.setCharAt(s.j+2, 'e')
		if s.lookup() {
			return
		}
		s.setCharAt(s.j+2, 'y')
		s.setLength(s.j + 1)
		s.k = s.j
		if s.lookup() {
			return
		}
		if s.j > 0 && s.word[s.j-1] == 'a' && s.word[s.j] == 'l' {
			return
		}
		s.appendStr("ly")
		s.k = oldK
		if s.j > 0 && s.word[s.j-1] == 'a' && s.word[s.j] == 'b' {
			s.setCharAt(s.j+2, 'e')
			s.k = s.j + 2
			return
		}
		if s.word[s.j] == 'i' {
			s.setLength(s.j)
			s.appendRune('y')
			s.k = s.j
			if s.lookup() {
				return
			}
			s.setLength(s.j)
			s.appendStr("ily")
			s.k = oldK
		}
		s.setLength(s.j + 1)
		s.k = s.j
	}
}

func (s *kStemmer) alEndings() {
	oldK := s.k
	if len(s.word) < 4 {
		return
	}
	if s.endsIn2('a', 'l') {
		s.setLength(s.j + 1)
		s.k = s.j
		if s.lookup() {
			return
		}
		if s.doubleC(s.j) {
			s.setLength(s.j)
			s.k = s.j - 1
			if s.lookup() {
				return
			}
			s.appendRune(s.word[s.j-1])
		}
		s.setLength(s.j + 1)
		s.appendRune('e')
		s.k = s.j + 1
		if s.lookup() {
			return
		}
		s.setLength(s.j + 1)
		s.appendStr("um")
		s.k = s.j + 2
		if s.lookup() {
			return
		}
		s.setLength(s.j + 1)
		s.appendStr("al")
		s.k = oldK
		if s.j > 0 && s.word[s.j-1] == 'i' && s.word[s.j] == 'c' {
			s.setLength(s.j - 1)
			s.k = s.j - 2
			if s.lookup() {
				return
			}
			s.setLength(s.j - 1)
			s.appendRune('y')
			s.k = s.j - 1
			if s.lookup() {
				return
			}
			s.setLength(s.j - 1)
			s.appendStr("ic")
			s.k = s.j
			s.lookup()
			return
		}
		if s.word[s.j] == 'i' {
			s.setLength(s.j)
			s.k = s.j - 1
			if s.lookup() {
				return
			}
			s.appendStr("ial")
			s.k = oldK
			s.lookup()
		}
	}
}

func (s *kStemmer) iveEndings() {
	oldK := s.k
	if s.endsIn3('i', 'v', 'e') {
		s.setLength(s.j + 1)
		s.k = s.j
		if s.lookup() {
			return
		}
		s.appendRune('e')
		s.k = s.j + 1
		if s.lookup() {
			return
		}
		s.setLength(s.j + 1)
		s.appendStr("ive")
		if s.j > 0 && s.word[s.j-1] == 'a' && s.word[s.j] == 't' {
			s.setCharAt(s.j-1, 'e')
			s.setLength(s.j)
			s.k = s.j - 1
			if s.lookup() {
				return
			}
			s.setLength(s.j - 1)
			if s.lookup() {
				return
			}
			s.appendStr("ative")
			s.k = oldK
		}
		s.setCharAt(s.j+2, 'o')
		s.setCharAt(s.j+3, 'n')
		if s.lookup() {
			return
		}
		s.setCharAt(s.j+2, 'v')
		s.setCharAt(s.j+3, 'e')
		s.k = oldK
	}
}

// stem applies the KStem algorithm to term and returns the stemmed form.
// It returns the original term unchanged if the word is short or not alphabetic.
func (s *kStemmer) stem(term string) string {
	runes := []rune(term)
	changed := s.stemRunes(runes, len(runes))
	if !changed {
		return term
	}
	return s.asString()
}

// stemRunes runs the algorithm on a rune slice. Returns true if the word changed.
func (s *kStemmer) stemRunes(term []rune, length int) bool {
	s.result = ""
	s.k = length - 1

	if s.k <= 1 || s.k >= maxWordLen-1 {
		return false
	}

	// check the dictionary first
	entry := getKStemDict().Get(term, 0, length)
	if entry != nil {
		if entry.root != "" {
			s.result = entry.root
			return true
		}
		return false
	}

	// copy chars to internal buffer, reject non-alpha
	s.word = s.word[:0]
	s.word = append(s.word, make([]rune, length+10)...)
	s.word = s.word[:0]
	for i := 0; i < length; i++ {
		ch := term[i]
		if !isAlpha(ch) {
			return false
		}
		s.word = append(s.word, ch)
	}

	s.matchedEntry = nil

	for {
		s.plural()
		if s.matched() {
			break
		}
		s.pastTense()
		if s.matched() {
			break
		}
		s.aspect()
		if s.matched() {
			break
		}
		s.ityEndings()
		if s.matched() {
			break
		}
		s.nessEndings()
		if s.matched() {
			break
		}
		s.ionEndings()
		if s.matched() {
			break
		}
		s.erAndOrEndings()
		if s.matched() {
			break
		}
		s.lyEndings()
		if s.matched() {
			break
		}
		s.alEndings()
		if s.matched() {
			break
		}
		s.wordInDict()
		s.iveEndings()
		if s.matched() {
			break
		}
		s.izeEndings()
		if s.matched() {
			break
		}
		s.mentEndings()
		if s.matched() {
			break
		}
		s.bleEndings()
		if s.matched() {
			break
		}
		s.ismEndings()
		if s.matched() {
			break
		}
		s.icEndings()
		if s.matched() {
			break
		}
		s.ncyEndings()
		if s.matched() {
			break
		}
		s.ncessEndings()
		s.matched()
		break
	}

	entry = s.matchedEntry
	if entry != nil {
		s.result = entry.root
	}

	return true
}

// asString returns the current word buffer as a string.
func (s *kStemmer) asString() string {
	if s.result != "" {
		return s.result
	}
	return string(s.word)
}

func isAlpha(ch rune) bool {
	return ch >= 'a' && ch <= 'z'
}

// Static word lists embedded in the KStemmer.
var kStemExceptionWords = []string{
	"aide", "bathe", "caste", "cute", "dame", "dime", "doge", "done", "dune",
	"envelope", "gage", "grille", "grippe", "lobe", "mane", "mare", "nape",
	"node", "pane", "pate", "plane", "pope", "programme", "quite", "ripe",
	"rote", "rune", "sage", "severe", "shoppe", "sine", "slime", "snipe",
	"steppe", "suite", "swinge", "tare", "tine", "tope", "tripe", "twine",
}

var kStemDirectConflations = [][2]string{
	{"aging", "age"}, {"going", "go"}, {"goes", "go"}, {"lying", "lie"},
	{"using", "use"}, {"owing", "owe"}, {"suing", "sue"}, {"dying", "die"},
	{"tying", "tie"}, {"vying", "vie"}, {"aged", "age"}, {"used", "use"},
	{"vied", "vie"}, {"cued", "cue"}, {"died", "die"}, {"eyed", "eye"},
	{"hued", "hue"}, {"iced", "ice"}, {"lied", "lie"}, {"owed", "owe"},
	{"sued", "sue"}, {"toed", "toe"}, {"tied", "tie"}, {"does", "do"},
	{"doing", "do"}, {"aeronautical", "aeronautics"}, {"mathematical", "mathematics"},
	{"political", "politics"}, {"metaphysical", "metaphysics"},
	{"cylindrical", "cylinder"}, {"nazism", "nazi"}, {"ambiguity", "ambiguous"},
	{"barbarity", "barbarous"}, {"credulity", "credulous"},
	{"generosity", "generous"}, {"spontaneity", "spontaneous"},
	{"unanimity", "unanimous"}, {"voracity", "voracious"},
	{"fled", "flee"}, {"miscarriage", "miscarry"},
}

var kStemCountryNationality = [][2]string{
	{"afghan", "afghanistan"}, {"african", "africa"}, {"albanian", "albania"},
	{"algerian", "algeria"}, {"american", "america"}, {"andorran", "andorra"},
	{"angolan", "angola"}, {"arabian", "arabia"}, {"argentine", "argentina"},
	{"armenian", "armenia"}, {"asian", "asia"}, {"australian", "australia"},
	{"austrian", "austria"}, {"azerbaijani", "azerbaijan"}, {"azeri", "azerbaijan"},
	{"bangladeshi", "bangladesh"}, {"belgian", "belgium"}, {"bermudan", "bermuda"},
	{"bolivian", "bolivia"}, {"bosnian", "bosnia"}, {"botswanan", "botswana"},
	{"brazilian", "brazil"}, {"british", "britain"}, {"bulgarian", "bulgaria"},
	{"burmese", "burma"}, {"californian", "california"}, {"cambodian", "cambodia"},
	{"canadian", "canada"}, {"chadian", "chad"}, {"chilean", "chile"},
	{"chinese", "china"}, {"colombian", "colombia"}, {"croat", "croatia"},
	{"croatian", "croatia"}, {"cuban", "cuba"}, {"cypriot", "cyprus"},
	{"czechoslovakian", "czechoslovakia"}, {"danish", "denmark"},
	{"egyptian", "egypt"}, {"equadorian", "equador"}, {"eritrean", "eritrea"},
	{"estonian", "estonia"}, {"ethiopian", "ethiopia"}, {"european", "europe"},
	{"fijian", "fiji"}, {"filipino", "philippines"}, {"finnish", "finland"},
	{"french", "france"}, {"gambian", "gambia"}, {"georgian", "georgia"},
	{"german", "germany"}, {"ghanian", "ghana"}, {"greek", "greece"},
	{"grenadan", "grenada"}, {"guamian", "guam"}, {"guatemalan", "guatemala"},
	{"guinean", "guinea"}, {"guyanan", "guyana"}, {"haitian", "haiti"},
	{"hawaiian", "hawaii"}, {"holland", "dutch"}, {"honduran", "honduras"},
	{"hungarian", "hungary"}, {"icelandic", "iceland"}, {"indonesian", "indonesia"},
	{"iranian", "iran"}, {"iraqi", "iraq"}, {"iraqui", "iraq"}, {"irish", "ireland"},
	{"israeli", "israel"}, {"italian", "italy"}, {"jamaican", "jamaica"},
	{"japanese", "japan"}, {"jordanian", "jordan"}, {"kampuchean", "cambodia"},
	{"kenyan", "kenya"}, {"korean", "korea"}, {"kuwaiti", "kuwait"},
	{"lankan", "lanka"}, {"laotian", "laos"}, {"latvian", "latvia"},
	{"lebanese", "lebanon"}, {"liberian", "liberia"}, {"libyan", "libya"},
	{"lithuanian", "lithuania"}, {"macedonian", "macedonia"},
	{"madagascan", "madagascar"}, {"malaysian", "malaysia"}, {"maltese", "malta"},
	{"mauritanian", "mauritania"}, {"mexican", "mexico"},
	{"micronesian", "micronesia"}, {"moldovan", "moldova"}, {"monacan", "monaco"},
	{"mongolian", "mongolia"}, {"montenegran", "montenegro"},
	{"moroccan", "morocco"}, {"myanmar", "burma"}, {"namibian", "namibia"},
	{"nepalese", "nepal"}, {"nicaraguan", "nicaragua"}, {"nigerian", "nigeria"},
	{"norwegian", "norway"}, {"omani", "oman"}, {"pakistani", "pakistan"},
	{"panamanian", "panama"}, {"papuan", "papua"}, {"paraguayan", "paraguay"},
	{"peruvian", "peru"}, {"portuguese", "portugal"}, {"romanian", "romania"},
	{"rumania", "romania"}, {"rumanian", "romania"}, {"russian", "russia"},
	{"rwandan", "rwanda"}, {"samoan", "samoa"}, {"scottish", "scotland"},
	{"serb", "serbia"}, {"serbian", "serbia"}, {"siam", "thailand"},
	{"siamese", "thailand"}, {"slovakia", "slovak"}, {"slovakian", "slovak"},
	{"slovenian", "slovenia"}, {"somali", "somalia"}, {"somalian", "somalia"},
	{"spanish", "spain"}, {"swedish", "sweden"}, {"swiss", "switzerland"},
	{"syrian", "syria"}, {"taiwanese", "taiwan"}, {"tanzanian", "tanzania"},
	{"texan", "texas"}, {"thai", "thailand"}, {"tunisian", "tunisia"},
	{"turkish", "turkey"}, {"ugandan", "uganda"}, {"ukrainian", "ukraine"},
	{"uruguayan", "uruguay"}, {"uzbek", "uzbekistan"}, {"venezuelan", "venezuela"},
	{"vietnamese", "viet"}, {"virginian", "virginia"}, {"yemeni", "yemen"},
	{"yugoslav", "yugoslavia"}, {"yugoslavian", "yugoslavia"},
	{"zambian", "zambia"}, {"zealander", "zealand"}, {"zimbabwean", "zimbabwe"},
}

var kStemSupplementDict = []string{
	"aids", "applicator", "capacitor", "digitize", "electromagnet",
	"ellipsoid", "exosphere", "extensible", "ferromagnet", "graphics",
	"hydromagnet", "polygraph", "toroid", "superconduct", "backscatter",
	"connectionism",
}

var kStemProperNouns = []string{
	"abrams", "achilles", "acropolis", "adams", "agnes", "aires", "alexander",
	"alexis", "alfred", "algiers", "alps", "amadeus", "ames", "amos", "andes",
	"angeles", "annapolis", "antilles", "aquarius", "archimedes", "arkansas",
	"asher", "ashly", "athens", "atkins", "atlantis", "avis", "bahamas",
	"bangor", "barbados", "barger", "bering", "brahms", "brandeis", "brussels",
	"bruxelles", "cairns", "camoros", "camus", "carlos", "celts", "chalker",
	"charles", "cheops", "ching", "christmas", "cocos", "collins", "columbus",
	"confucius", "conners", "connolly", "copernicus", "cramer", "cyclops",
	"cygnus", "cyprus", "dallas", "damascus", "daniels", "davies", "davis",
	"decker", "denning", "dennis", "descartes", "dickens", "doris", "douglas",
	"downs", "dreyfus", "dukakis", "dulles", "dumfries", "ecclesiastes",
	"edwards", "emily", "erasmus", "euphrates", "evans", "everglades",
	"fairbanks", "federales", "fisher", "fitzsimmons", "fleming", "forbes",
	"fowler", "france", "francis", "goering", "goodling", "goths", "grenadines",
	"guiness", "hades", "harding", "harris", "hastings", "hawkes", "hawking",
	"hayes", "heights", "hercules", "himalayas", "hippocrates", "hobbs",
	"holmes", "honduras", "hopkins", "hughes", "humphreys", "illinois",
	"indianapolis", "inverness", "iris", "iroquois", "irving", "isaacs",
	"italy", "james", "jarvis", "jeffreys", "jesus", "jones", "josephus",
	"judas", "julius", "kansas", "keynes", "kipling", "kiwanis", "lansing",
	"laos", "leeds", "levis", "leviticus", "lewis", "louis", "maccabees",
	"madras", "maimonides", "maldive", "massachusetts", "matthews", "mauritius",
	"memphis", "mercedes", "midas", "mingus", "minneapolis", "mohammed",
	"moines", "morris", "moses", "myers", "myknos", "nablus", "nanjing",
	"nantes", "naples", "neal", "netherlands", "nevis", "nostradamus",
	"oedipus", "olympus", "orleans", "orly", "papas", "paris", "parker",
	"pauling", "peking", "pershing", "peter", "peters", "philippines",
	"phineas", "pisces", "pryor", "pythagoras", "queens", "rabelais", "ramses",
	"reynolds", "rhesus", "rhodes", "richards", "robins", "rodgers", "rogers",
	"rubens", "sagittarius", "seychelles", "socrates", "texas", "thames",
	"thomas", "tiberias", "tunis", "venus", "vilnius", "wales", "warner",
	"wilkins", "williams", "wyoming", "xmas", "yonkers", "zeus", "frances",
	"aarhus", "adonis", "andrews", "angus", "antares", "aquinas", "arcturus",
	"ares", "artemis", "augustus", "ayers", "barnabas", "barnes", "becker",
	"bejing", "biggs", "billings", "boeing", "boris", "borroughs", "briggs",
	"buenos", "calais", "caracas", "cassius", "cerberus", "ceres", "cervantes",
	"chantilly", "chartres", "chester", "connally", "conner", "coors",
	"cummings", "curtis", "daedalus", "dionysus", "dobbs", "dolores", "edmonds",
}
