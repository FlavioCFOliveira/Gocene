// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// FuzzyQuery matches documents containing terms similar to the given term.
// It uses the Damerau-Levenshtein algorithm to calculate edit distance.
type FuzzyQuery struct {
	*BaseQuery
	term          *index.Term
	maxEdits      int
	prefixLength  int
	maxExpansions int
	transpositions bool
}

// NewFuzzyQuery creates a new FuzzyQuery with default parameters.
func NewFuzzyQuery(term *index.Term) *FuzzyQuery {
	return &FuzzyQuery{
		BaseQuery:     &BaseQuery{},
		term:          term,
		maxEdits:      2,
		prefixLength:  0,
		maxExpansions: 50,
		transpositions: true,
	}
}

// NewFuzzyQueryWithParams creates a FuzzyQuery with custom parameters.
func NewFuzzyQueryWithParams(term *index.Term, maxEdits, prefixLength, maxExpansions int) *FuzzyQuery {
	return &FuzzyQuery{
		BaseQuery:     &BaseQuery{},
		term:          term,
		maxEdits:      maxEdits,
		prefixLength:  prefixLength,
		maxExpansions: maxExpansions,
		transpositions: true,
	}
}

// Term returns the fuzzy term.
func (q *FuzzyQuery) Term() *index.Term {
	return q.term
}

// MaxEdits returns the maximum edit distance.
func (q *FuzzyQuery) MaxEdits() int {
	return q.maxEdits
}

// PrefixLength returns the prefix length.
func (q *FuzzyQuery) PrefixLength() int {
	return q.prefixLength
}

// MaxExpansions returns the maximum number of expansions.
func (q *FuzzyQuery) MaxExpansions() int {
	return q.maxExpansions
}

// TranspositionsAllowed returns true if transpositions are allowed.
func (q *FuzzyQuery) TranspositionsAllowed() bool {
	return q.transpositions
}
