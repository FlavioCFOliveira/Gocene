// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "github.com/FlavioCFOliveira/Gocene/index"

// FuzzyTermsEnum is the canonical TermsEnum used by FuzzyQuery to enumerate
// terms within an edit-distance window of a base term.
//
// Mirrors org.apache.lucene.search.FuzzyTermsEnum. The contract surfaces the
// per-step automaton-driven enumeration; the concrete automaton is computed
// from the base term, the edit distance, and a shared prefix length.
type FuzzyTermsEnum interface {
	// Next returns the next matching term within the configured edit
	// distance, or nil when the enumeration is exhausted.
	Next() ([]byte, error)
	// Term returns the term of the current position.
	Term() []byte
	// DocFreq returns the document frequency of the current term.
	DocFreq() (int, error)
	// TotalTermFreq returns the total term frequency of the current term.
	TotalTermFreq() (int64, error)
	// Postings returns a PostingsEnum positioned on the current term.
	Postings() (index.PostingsEnum, error)
}

// FuzzyTermsEnumConfig captures the configuration parameters used to build a
// FuzzyTermsEnum. Concrete implementations live next to the terms-dictionary
// codec that exposes the underlying TermsEnum.
type FuzzyTermsEnumConfig struct {
	Term           []byte
	MaxEdits       int
	PrefixLength   int
	MaxExpansions  int
	Transpositions bool
	BoostAttribute MaxNonCompetitiveBoostAttribute
}

// NewFuzzyTermsEnumConfig returns a FuzzyTermsEnumConfig with the canonical
// Lucene defaults (2 edits, 0 prefix, transpositions enabled, 50 expansions).
func NewFuzzyTermsEnumConfig(term []byte) FuzzyTermsEnumConfig {
	return FuzzyTermsEnumConfig{
		Term:           term,
		MaxEdits:       2,
		PrefixLength:   0,
		MaxExpansions:  50,
		Transpositions: true,
	}
}
