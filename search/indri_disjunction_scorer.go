// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// IndriDisjunctionScorer is the abstract base for Indri disjunction-style
// scorers. It iterates over the union of clause posting lists and computes a
// per-doc score that includes a smoothing contribution from clauses that did
// not match the current document.
//
// Mirrors org.apache.lucene.search.IndriDisjunctionScorer.
type IndriDisjunctionScorer struct {
	IndriScorer
}

// NewIndriDisjunctionScorer constructs an IndriDisjunctionScorer.
func NewIndriDisjunctionScorer(weight Weight, boost float32, subs []Scorer) *IndriDisjunctionScorer {
	return &IndriDisjunctionScorer{IndriScorer: *NewIndriScorer(weight, boost, subs)}
}
