// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package surround

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// MatchingTermVisitor collects terms that match a SimpleTerm's criteria.
// Mirrors the logic used in Lucene's SimpleTermRewriteQuery expansion.
type MatchingTermVisitor struct {
	terms []index.Term
}

// NewMatchingTermVisitor creates a new MatchingTermVisitor.
func NewMatchingTermVisitor() *MatchingTermVisitor {
	return &MatchingTermVisitor{
		terms: make([]index.Term, 0),
	}
}

// AddTerm adds a matching term to the collection.
func (v *MatchingTermVisitor) AddTerm(term index.Term) {
	v.terms = append(v.terms, term)
}

// Terms returns the list of collected terms.
func (v *MatchingTermVisitor) Terms() []index.Term {
	return v.terms
}
