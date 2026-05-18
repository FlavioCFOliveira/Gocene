// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import "reflect"

// Indonesian stemmer flags. The Java reference uses a bit mask to
// remember which prefix was removed so that conflicting suffixes can
// be skipped during derivational stemming.
const (
	indonesianRemovedKe = 1 << iota
	indonesianRemovedPeng
	indonesianRemovedDi
	indonesianRemovedMeng
	indonesianRemovedTer
	indonesianRemovedBer
	indonesianRemovedPe
)

// IndonesianStemmer is the Go port of
// org.apache.lucene.analysis.id.IndonesianStemmer from Apache Lucene
// 10.4.0. The stemmer removes Indonesian particle, possessive
// pronoun, prefix, and suffix morphology.
type IndonesianStemmer struct {
	flags        int
	numSyllables int
}

// NewIndonesianStemmer returns a fresh stateless stemmer. The
// receiver holds per-call mutable state; do not share concurrently.
func NewIndonesianStemmer() *IndonesianStemmer {
	return &IndonesianStemmer{}
}

// Stem strips Indonesian morphology from runes[:length] and returns
// the new length. When stemDerivational is true the algorithm also
// attempts the derivational prefix/suffix layer.
func (s *IndonesianStemmer) Stem(runes []rune, length int, stemDerivational bool) int {
	s.flags = 0
	s.numSyllables = 0
	for i := 0; i < length; i++ {
		if indonesianIsVowel(runes[i]) {
			s.numSyllables++
		}
	}
	if s.numSyllables > 2 {
		length = s.removeParticle(runes, length)
	}
	if s.numSyllables > 2 {
		length = s.removePossessivePronoun(runes, length)
	}
	if stemDerivational {
		length = s.stemDerivational(runes, length)
	}
	return length
}

// StemString stems input using stemDerivational=true (the most
// commonly desired behaviour).
func (s *IndonesianStemmer) StemString(input string) string {
	if input == "" {
		return ""
	}
	runes := []rune(input)
	n := s.Stem(runes, len(runes), true)
	return string(runes[:n])
}

func (s *IndonesianStemmer) stemDerivational(runes []rune, length int) int {
	oldLen := length
	if s.numSyllables > 2 {
		length = s.removeFirstOrderPrefix(runes, length)
	}
	if oldLen != length {
		oldLen = length
		if s.numSyllables > 2 {
			length = s.removeSuffix(runes, length)
		}
		if oldLen != length && s.numSyllables > 2 {
			length = s.removeSecondOrderPrefix(runes, length)
		}
	} else {
		if s.numSyllables > 2 {
			length = s.removeSecondOrderPrefix(runes, length)
		}
		if s.numSyllables > 2 {
			length = s.removeSuffix(runes, length)
		}
	}
	return length
}

func indonesianIsVowel(r rune) bool {
	switch r {
	case 'a', 'e', 'i', 'o', 'u':
		return true
	}
	return false
}

func (s *IndonesianStemmer) removeParticle(runes []rune, length int) int {
	if runesEndWith(runes, length, "kah") || runesEndWith(runes, length, "lah") || runesEndWith(runes, length, "pun") {
		s.numSyllables--
		return length - 3
	}
	return length
}

func (s *IndonesianStemmer) removePossessivePronoun(runes []rune, length int) int {
	if runesEndWith(runes, length, "ku") || runesEndWith(runes, length, "mu") {
		s.numSyllables--
		return length - 2
	}
	if runesEndWith(runes, length, "nya") {
		s.numSyllables--
		return length - 3
	}
	return length
}

// startsWithRunes mirrors Lucene's StemmerUtil.startsWith on a rune
// slice.
func startsWithRunes(runes []rune, length int, prefix string) bool {
	pr := []rune(prefix)
	if len(pr) > length {
		return false
	}
	for i, r := range pr {
		if runes[i] != r {
			return false
		}
	}
	return true
}

func (s *IndonesianStemmer) removeFirstOrderPrefix(runes []rune, length int) int {
	switch {
	case startsWithRunes(runes, length, "meng"):
		s.flags |= indonesianRemovedMeng
		s.numSyllables--
		return StemmerDeleteN(runes, 0, length, 4)
	case startsWithRunes(runes, length, "meny") && length > 4 && indonesianIsVowel(runes[4]):
		s.flags |= indonesianRemovedMeng
		runes[3] = 's'
		s.numSyllables--
		return StemmerDeleteN(runes, 0, length, 3)
	case startsWithRunes(runes, length, "men"):
		s.flags |= indonesianRemovedMeng
		s.numSyllables--
		return StemmerDeleteN(runes, 0, length, 3)
	case startsWithRunes(runes, length, "mem"):
		s.flags |= indonesianRemovedMeng
		s.numSyllables--
		return StemmerDeleteN(runes, 0, length, 3)
	case startsWithRunes(runes, length, "me"):
		s.flags |= indonesianRemovedMeng
		s.numSyllables--
		return StemmerDeleteN(runes, 0, length, 2)
	case startsWithRunes(runes, length, "peng"):
		s.flags |= indonesianRemovedPeng
		s.numSyllables--
		return StemmerDeleteN(runes, 0, length, 4)
	case startsWithRunes(runes, length, "peny") && length > 4 && indonesianIsVowel(runes[4]):
		s.flags |= indonesianRemovedPeng
		runes[3] = 's'
		s.numSyllables--
		return StemmerDeleteN(runes, 0, length, 3)
	case startsWithRunes(runes, length, "peny"):
		s.flags |= indonesianRemovedPeng
		s.numSyllables--
		return StemmerDeleteN(runes, 0, length, 4)
	case startsWithRunes(runes, length, "pen") && length > 3 && indonesianIsVowel(runes[3]):
		s.flags |= indonesianRemovedPeng
		runes[2] = 't'
		s.numSyllables--
		return StemmerDeleteN(runes, 0, length, 2)
	case startsWithRunes(runes, length, "pen"):
		s.flags |= indonesianRemovedPeng
		s.numSyllables--
		return StemmerDeleteN(runes, 0, length, 3)
	case startsWithRunes(runes, length, "pem"):
		s.flags |= indonesianRemovedPeng
		s.numSyllables--
		return StemmerDeleteN(runes, 0, length, 3)
	case startsWithRunes(runes, length, "di"):
		s.flags |= indonesianRemovedDi
		s.numSyllables--
		return StemmerDeleteN(runes, 0, length, 2)
	case startsWithRunes(runes, length, "ter"):
		s.flags |= indonesianRemovedTer
		s.numSyllables--
		return StemmerDeleteN(runes, 0, length, 3)
	case startsWithRunes(runes, length, "ke"):
		s.flags |= indonesianRemovedKe
		s.numSyllables--
		return StemmerDeleteN(runes, 0, length, 2)
	}
	return length
}

func (s *IndonesianStemmer) removeSecondOrderPrefix(runes []rune, length int) int {
	switch {
	case startsWithRunes(runes, length, "ber"):
		s.flags |= indonesianRemovedBer
		s.numSyllables--
		return StemmerDeleteN(runes, 0, length, 3)
	case length == 7 && startsWithRunes(runes, length, "belajar"):
		s.flags |= indonesianRemovedBer
		s.numSyllables--
		return StemmerDeleteN(runes, 0, length, 3)
	case startsWithRunes(runes, length, "be") && length > 4 && !indonesianIsVowel(runes[2]) && runes[3] == 'e' && runes[4] == 'r':
		s.flags |= indonesianRemovedBer
		s.numSyllables--
		return StemmerDeleteN(runes, 0, length, 2)
	case startsWithRunes(runes, length, "per"):
		s.numSyllables--
		return StemmerDeleteN(runes, 0, length, 3)
	case length == 7 && startsWithRunes(runes, length, "pelajar"):
		s.numSyllables--
		return StemmerDeleteN(runes, 0, length, 3)
	case startsWithRunes(runes, length, "pe"):
		s.flags |= indonesianRemovedPe
		s.numSyllables--
		return StemmerDeleteN(runes, 0, length, 2)
	}
	return length
}

func (s *IndonesianStemmer) removeSuffix(runes []rune, length int) int {
	if runesEndWith(runes, length, "kan") &&
		s.flags&indonesianRemovedKe == 0 &&
		s.flags&indonesianRemovedPeng == 0 &&
		s.flags&indonesianRemovedPe == 0 {
		s.numSyllables--
		return length - 3
	}
	if runesEndWith(runes, length, "an") &&
		s.flags&indonesianRemovedDi == 0 &&
		s.flags&indonesianRemovedMeng == 0 &&
		s.flags&indonesianRemovedTer == 0 {
		s.numSyllables--
		return length - 2
	}
	if runesEndWith(runes, length, "i") && !runesEndWith(runes, length, "si") &&
		s.flags&indonesianRemovedBer == 0 &&
		s.flags&indonesianRemovedKe == 0 &&
		s.flags&indonesianRemovedPeng == 0 {
		s.numSyllables--
		return length - 1
	}
	return length
}

// IndonesianStemFilter wraps a TokenStream with IndonesianStemmer.
//
// This is the Go port of
// org.apache.lucene.analysis.id.IndonesianStemFilter from Apache
// Lucene 10.4.0.
type IndonesianStemFilter struct {
	*BaseTokenFilter

	stemDerivational bool
	termAttr         CharTermAttribute
	keywordAttr      KeywordAttribute
}

// NewIndonesianStemFilter wraps input with derivational stemming
// enabled (the upstream default).
func NewIndonesianStemFilter(input TokenStream) *IndonesianStemFilter {
	return NewIndonesianStemFilterWithDerivational(input, true)
}

// NewIndonesianStemFilterWithDerivational wraps input with explicit
// derivational flag.
func NewIndonesianStemFilterWithDerivational(input TokenStream, stemDerivational bool) *IndonesianStemFilter {
	f := &IndonesianStemFilter{
		BaseTokenFilter:  NewBaseTokenFilter(input),
		stemDerivational: stemDerivational,
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttributeByType(reflect.TypeOf(&charTermAttribute{})); a != nil {
			f.termAttr = a.(CharTermAttribute)
		}
		if a := src.GetAttributeByType(KeywordAttributeType); a != nil {
			f.keywordAttr = a.(KeywordAttribute)
		}
	}
	return f
}

// IncrementToken stems the current token unless it is a keyword.
func (f *IndonesianStemFilter) IncrementToken() (bool, error) {
	ok, err := f.input.IncrementToken()
	if err != nil || !ok {
		return ok, err
	}
	if f.termAttr == nil {
		return true, nil
	}
	if f.keywordAttr != nil && f.keywordAttr.IsKeywordToken() {
		return true, nil
	}
	// Allocate a fresh stemmer per token so concurrent filter chains
	// remain safe (the receiver carries mutable state).
	stemmer := NewIndonesianStemmer()
	runes := []rune(f.termAttr.String())
	n := stemmer.Stem(runes, len(runes), f.stemDerivational)
	res := string(runes[:n])
	if res != f.termAttr.String() {
		f.termAttr.SetEmpty()
		f.termAttr.AppendString(res)
	}
	return true, nil
}

// Ensure IndonesianStemFilter implements TokenFilter.
var _ TokenFilter = (*IndonesianStemFilter)(nil)

// IndonesianStemFilterFactory creates IndonesianStemFilter instances.
type IndonesianStemFilterFactory struct {
	stemDerivational bool
}

// NewIndonesianStemFilterFactory returns a factory using the
// derivational default.
func NewIndonesianStemFilterFactory() *IndonesianStemFilterFactory {
	return &IndonesianStemFilterFactory{stemDerivational: true}
}

// NewIndonesianStemFilterFactoryWithDerivational returns a configured
// factory.
func NewIndonesianStemFilterFactoryWithDerivational(stemDerivational bool) *IndonesianStemFilterFactory {
	return &IndonesianStemFilterFactory{stemDerivational: stemDerivational}
}

// Create wraps input.
func (f *IndonesianStemFilterFactory) Create(input TokenStream) TokenFilter {
	return NewIndonesianStemFilterWithDerivational(input, f.stemDerivational)
}

// Ensure factory satisfies TokenFilterFactory.
var _ TokenFilterFactory = (*IndonesianStemFilterFactory)(nil)
