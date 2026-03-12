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
	term           *index.Term
	maxEdits       int
	prefixLength   int
	maxExpansions  int
	transpositions bool
}

// NewFuzzyQuery creates a new FuzzyQuery with default parameters.
func NewFuzzyQuery(term *index.Term) *FuzzyQuery {
	return &FuzzyQuery{
		BaseQuery:      &BaseQuery{},
		term:           term,
		maxEdits:       2,
		prefixLength:   0,
		maxExpansions:  50,
		transpositions: true,
	}
}

// NewFuzzyQueryWithMaxEdits creates a FuzzyQuery with specified max edits.
func NewFuzzyQueryWithMaxEdits(term *index.Term, maxEdits int) *FuzzyQuery {
	return &FuzzyQuery{
		BaseQuery:      &BaseQuery{},
		term:           term,
		maxEdits:       maxEdits,
		prefixLength:   0,
		maxExpansions:  50,
		transpositions: true,
	}
}
func NewFuzzyQueryWithParams(term *index.Term, maxEdits, prefixLength, maxExpansions int) *FuzzyQuery {
	return &FuzzyQuery{
		BaseQuery:      &BaseQuery{},
		term:           term,
		maxEdits:       maxEdits,
		prefixLength:   prefixLength,
		maxExpansions:  maxExpansions,
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

// Clone creates a copy of this query.
func (q *FuzzyQuery) Clone() Query {
	if q.term == nil {
		return &FuzzyQuery{
			BaseQuery:      &BaseQuery{},
			term:           nil,
			maxEdits:       q.maxEdits,
			prefixLength:   q.prefixLength,
			maxExpansions:  q.maxExpansions,
			transpositions: q.transpositions,
		}
	}
	return &FuzzyQuery{
		BaseQuery:      &BaseQuery{},
		term:           q.term.Clone(),
		maxEdits:       q.maxEdits,
		prefixLength:   q.prefixLength,
		maxExpansions:  q.maxExpansions,
		transpositions: q.transpositions,
	}
}

// Equals checks if this query equals another.
func (q *FuzzyQuery) Equals(other Query) bool {
	if o, ok := other.(*FuzzyQuery); ok {
		if q.maxEdits != o.maxEdits || q.prefixLength != o.prefixLength ||
			q.maxExpansions != o.maxExpansions || q.transpositions != o.transpositions {
			return false
		}
		if q.term == nil || o.term == nil {
			return q.term == nil && o.term == nil
		}
		return q.term.Equals(o.term)
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *FuzzyQuery) HashCode() int {
	hash := 0
	if q.term != nil {
		hash = q.term.HashCode()
	}
	hash = hash*31 + q.maxEdits
	hash = hash*31 + q.prefixLength
	hash = hash*31 + q.maxExpansions
	if q.transpositions {
		hash = hash*31 + 1
	}
	return hash
}
