// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package pt

import (
	"embed"
	"sync"
)

//go:embed portuguese.rslp
var rslpFS embed.FS

var (
	rslpOnce  sync.Once
	rslpSteps map[string]*Step
)

func getRSLPSteps() map[string]*Step {
	rslpOnce.Do(func() {
		steps, err := ParseFS(rslpFS, "portuguese.rslp")
		if err != nil {
			panic("pt: failed to load portuguese.rslp: " + err.Error())
		}
		rslpSteps = steps
	})
	return rslpSteps
}

// PortugueseStemmer implements the RSLP (Removedor de Sufixos da Lingua
// Portuguesa) stemming algorithm for Portuguese.
//
// Go port of org.apache.lucene.analysis.pt.PortugueseStemmer (Apache Lucene
// 10.4.0).
type PortugueseStemmer struct {
	plural      *Step
	feminine    *Step
	adverb      *Step
	augmentative *Step
	noun        *Step
	verb        *Step
	vowel       *Step
}

// NewPortugueseStemmer creates a PortugueseStemmer backed by the embedded
// portuguese.rslp rule file.
func NewPortugueseStemmer() *PortugueseStemmer {
	steps := getRSLPSteps()
	return &PortugueseStemmer{
		plural:       steps["Plural"],
		feminine:     steps["Feminine"],
		adverb:       steps["Adverb"],
		augmentative: steps["Augmentative"],
		noun:         steps["Noun"],
		verb:         steps["Verb"],
		vowel:        steps["Vowel"],
	}
}

// Stem applies the RSLP algorithm to s[:length] in-place and returns the new
// length. The slice must be oversized by at least 1 to accommodate the worst-
// case suffix expansion ('ã' → 'ão').
func (st *PortugueseStemmer) Stem(s []rune, length int) int {
	length = st.plural.Apply(s, length)
	length = st.adverb.Apply(s, length)
	length = st.feminine.Apply(s, length)
	length = st.augmentative.Apply(s, length)

	oldLen := length
	length = st.noun.Apply(s, length)
	if length == oldLen {
		oldLen = length
		length = st.verb.Apply(s, length)
		if length == oldLen {
			length = st.vowel.Apply(s, length)
		}
	}

	// RSLP accent removal.
	for i := 0; i < length; i++ {
		switch s[i] {
		case 'à', 'á', 'â', 'ã', 'ä', 'å':
			s[i] = 'a'
		case 'ç':
			s[i] = 'c'
		case 'è', 'é', 'ê', 'ë':
			s[i] = 'e'
		case 'ì', 'í', 'î', 'ï':
			s[i] = 'i'
		case 'ñ':
			s[i] = 'n'
		case 'ò', 'ó', 'ô', 'õ', 'ö':
			s[i] = 'o'
		case 'ù', 'ú', 'û', 'ü':
			s[i] = 'u'
		case 'ý', 'ÿ':
			s[i] = 'y'
		}
	}
	return length
}
