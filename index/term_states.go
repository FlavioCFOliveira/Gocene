// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// TermStates aggregates per-segment TermState instances for a single Term
// across a composite reader. Mirrors org.apache.lucene.index.TermStates from
// Apache Lucene 10.4.0.
//
// Gocene skeleton: the public surface (Add, Get, DocFreq, TotalTermFreq,
// HasOnlyRealTerms, BuildFromIndex helpers) is present so callers can wire
// TermStates parameters; the full traversal logic that builds a TermStates
// from a top-level IndexReader+Term is deferred to backlog #2709.
type TermStates struct {
	// owner is the cache key owner.
	owner *CacheKey

	// states is indexed by per-leaf ord; nil entries indicate the term is
	// absent from that leaf.
	states []TermState

	// docFreqs[i] is the document frequency contributed by leaf i.
	docFreqs []int

	// totalTermFreqs[i] is the total term frequency contributed by leaf i.
	totalTermFreqs []int64
}

// NewTermStates allocates an empty TermStates sized for leafCount leaves.
func NewTermStates(owner *CacheKey, leafCount int) *TermStates {
	return &TermStates{
		owner:          owner,
		states:         make([]TermState, leafCount),
		docFreqs:       make([]int, leafCount),
		totalTermFreqs: make([]int64, leafCount),
	}
}

// Register records a TermState contribution for the given leaf ord.
func (ts *TermStates) Register(ord int, state TermState, docFreq int, totalTermFreq int64) {
	ts.states[ord] = state
	ts.docFreqs[ord] = docFreq
	ts.totalTermFreqs[ord] = totalTermFreq
}

// Get returns the TermState for the given leaf ord, or nil if the term is
// absent from that leaf.
func (ts *TermStates) Get(ord int) TermState { return ts.states[ord] }

// DocFreq returns the aggregate docFreq across all leaves.
func (ts *TermStates) DocFreq() int {
	sum := 0
	for _, d := range ts.docFreqs {
		sum += d
	}
	return sum
}

// TotalTermFreq returns the aggregate total term frequency across all leaves.
func (ts *TermStates) TotalTermFreq() int64 {
	var sum int64
	for _, t := range ts.totalTermFreqs {
		sum += t
	}
	return sum
}
