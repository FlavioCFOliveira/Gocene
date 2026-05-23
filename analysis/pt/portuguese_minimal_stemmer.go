// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package pt

// PortugueseMinimalStemmer implements the "RSLP-S" minimal stemmer for
// Portuguese: it applies only the Plural reduction step of the RSLP algorithm.
//
// Reference:
//   "A study on the Use of Stemming for Monolingual Ad-Hoc Portuguese
//   Information Retrieval" Orengo et al.
//
// Go port of org.apache.lucene.analysis.pt.PortugueseMinimalStemmer (Apache
// Lucene 10.4.0).
type PortugueseMinimalStemmer struct {
	pluralStep *Step
}

// NewPortugueseMinimalStemmer creates a PortugueseMinimalStemmer.
func NewPortugueseMinimalStemmer() *PortugueseMinimalStemmer {
	steps := getRSLPSteps()
	return &PortugueseMinimalStemmer{pluralStep: steps["Plural"]}
}

// Stem applies the Plural reduction step to s[:length] and returns the new
// length.
func (m *PortugueseMinimalStemmer) Stem(s []rune, length int) int {
	return m.pluralStep.Apply(s, length)
}
