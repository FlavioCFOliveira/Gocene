// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package gl

import (
	"embed"
	"io/fs"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/analysis/pt"
)

//go:embed resources/galician.rslp
var galicianFS embed.FS

var (
	galicianStepsOnce sync.Once
	galicianSteps     map[string]*pt.Step
	galicianStepsErr  error
)

func loadGalicianSteps() (map[string]*pt.Step, error) {
	galicianStepsOnce.Do(func() {
		sub, err := fs.Sub(galicianFS, "resources")
		if err != nil {
			galicianStepsErr = err
			return
		}
		galicianSteps, galicianStepsErr = pt.ParseFS(sub, "galician.rslp")
	})
	return galicianSteps, galicianStepsErr
}

// ─── GalicianMinimalStemmer ───────────────────────────────────────────────────

// GalicianMinimalStemmer applies only the plural-reduction step of the
// Galician RSLP algorithm.
//
// This is the Go port of
// org.apache.lucene.analysis.gl.GalicianMinimalStemmer from
// Apache Lucene 10.4.0.
type GalicianMinimalStemmer struct {
	pluralStep *pt.Step
}

// NewGalicianMinimalStemmer constructs a GalicianMinimalStemmer.
// Returns an error if the galician.rslp resource cannot be parsed.
func NewGalicianMinimalStemmer() (*GalicianMinimalStemmer, error) {
	steps, err := loadGalicianSteps()
	if err != nil {
		return nil, err
	}
	return &GalicianMinimalStemmer{pluralStep: steps["Plural"]}, nil
}

// Stem applies the plural-reduction step to s[:length] and returns the
// new length. The buffer must be oversized by at least 1.
func (g *GalicianMinimalStemmer) Stem(s []rune, length int) int {
	return g.pluralStep.Apply(s, length)
}

// ─── GalicianStemmer ──────────────────────────────────────────────────────────

// GalicianStemmer implements the full "Regras do lematizador para o galego"
// algorithm, applying plural reduction, unification, adverb, augmentative,
// noun/verb, and vowel steps in sequence.
//
// This is the Go port of
// org.apache.lucene.analysis.gl.GalicianStemmer from Apache Lucene 10.4.0.
type GalicianStemmer struct {
	plural       *pt.Step
	unification  *pt.Step
	adverb       *pt.Step
	augmentative *pt.Step
	noun         *pt.Step
	verb         *pt.Step
	vowel        *pt.Step
}

// NewGalicianStemmer constructs a GalicianStemmer.
// Returns an error if the galician.rslp resource cannot be parsed.
func NewGalicianStemmer() (*GalicianStemmer, error) {
	steps, err := loadGalicianSteps()
	if err != nil {
		return nil, err
	}
	return &GalicianStemmer{
		plural:       steps["Plural"],
		unification:  steps["Unification"],
		adverb:       steps["Adverb"],
		augmentative: steps["Augmentative"],
		noun:         steps["Noun"],
		verb:         steps["Verb"],
		vowel:        steps["Vowel"],
	}, nil
}

// Stem applies the full Galician stemming algorithm to s[:length] and
// returns the new length. The buffer must be oversized by at least 1.
func (g *GalicianStemmer) Stem(s []rune, length int) int {
	length = g.plural.Apply(s, length)
	length = g.unification.Apply(s, length)
	length = g.adverb.Apply(s, length)

	for {
		prev := length
		length = g.augmentative.Apply(s, length)
		if length == prev {
			break
		}
	}

	prev := length
	length = g.noun.Apply(s, length)
	if length == prev {
		length = g.verb.Apply(s, length)
	}

	length = g.vowel.Apply(s, length)

	// RSLG accent removal
	for i := 0; i < length; i++ {
		switch s[i] {
		case 'á':
			s[i] = 'a'
		case 'é', 'ê':
			s[i] = 'e'
		case 'í':
			s[i] = 'i'
		case 'ó':
			s[i] = 'o'
		case 'ú':
			s[i] = 'u'
		}
	}

	return length
}
