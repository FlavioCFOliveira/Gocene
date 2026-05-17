// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// IndriAndScorer scores documents under the IndriAndQuery model: it iterates
// over documents matching at least one clause and combines clause scores
// (including smoothing scores for missing clauses) into a single per-doc
// score.
//
// Mirrors org.apache.lucene.search.IndriAndScorer.
type IndriAndScorer struct {
	IndriScorer
}

// NewIndriAndScorer constructs an IndriAndScorer.
func NewIndriAndScorer(weight Weight, boost float32, subs []Scorer) *IndriAndScorer {
	return &IndriAndScorer{IndriScorer: *NewIndriScorer(weight, boost, subs)}
}
