// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/MultiTermQueryConstantScoreBlendedWrapper.java

import "github.com/FlavioCFOliveira/Gocene/index"

// multiTermQueryConstantScoreBlendedWrapper provides the functionality
// behind MultiTermQuery.CONSTANT_SCORE_BLENDED_REWRITE.  It maintains
// a boolean-query-like approach over a limited number of the most costly
// terms while rewriting the remaining terms into a filter bitset.
//
// The Java original is a final, package-private class; the Go port
// follows the same visibility model (unexported).
//
// Full scoring logic requires AbstractMultiTermQueryConstantScoreWrapper
// (task 3311), RewritingWeight, WeightOrDocIdSetIterator, TermAndState,
// TermStates, and a wired TermsEnum/PostingsEnum path — none of which are
// yet ported.  The weight returned by CreateWeight delegates per-leaf
// scoring to an empty iterator until those dependencies land.
//
// Ported from org.apache.lucene.search.MultiTermQueryConstantScoreBlendedWrapper.
type multiTermQueryConstantScoreBlendedWrapper struct {
	query *MultiTermQuery
}

// postingsPreProcessThreshold is the maximum doc-frequency below which a
// term's postings list is always pre-processed into the bitset rather than
// placed in the priority queue of high-frequency terms.
//
// Mirrors MultiTermQueryConstantScoreBlendedWrapper.POSTINGS_PRE_PROCESS_THRESHOLD.
const postingsPreProcessThreshold = 512

// newMultiTermQueryConstantScoreBlendedWrapper creates a blended constant-
// score wrapper for the given MultiTermQuery.
func newMultiTermQueryConstantScoreBlendedWrapper(q *MultiTermQuery) *multiTermQueryConstantScoreBlendedWrapper {
	return &multiTermQueryConstantScoreBlendedWrapper{query: q}
}

// GetQuery returns the wrapped MultiTermQuery.
func (w *multiTermQueryConstantScoreBlendedWrapper) GetQuery() *MultiTermQuery { return w.query }

// GetField returns the field name of the wrapped query.
func (w *multiTermQueryConstantScoreBlendedWrapper) GetField() string {
	return w.query.GetField()
}

// String returns a human-readable representation, delegating to the wrapped query.
func (w *multiTermQueryConstantScoreBlendedWrapper) String() string {
	return w.query.String(w.query.GetField())
}

// Rewrite returns the receiver unchanged; the weight's scorerSupplier is
// responsible for the per-segment rewriting logic.
func (w *multiTermQueryConstantScoreBlendedWrapper) Rewrite(_ IndexReader) (Query, error) {
	return w, nil
}

// Clone returns a shallow copy of the wrapper.
func (w *multiTermQueryConstantScoreBlendedWrapper) Clone() Query {
	return &multiTermQueryConstantScoreBlendedWrapper{query: w.query}
}

// Equals reports whether other is a multiTermQueryConstantScoreBlendedWrapper
// over the same underlying MultiTermQuery.
func (w *multiTermQueryConstantScoreBlendedWrapper) Equals(other Query) bool {
	if other == nil {
		return false
	}
	o, ok := other.(*multiTermQueryConstantScoreBlendedWrapper)
	if !ok {
		return false
	}
	return w.query.Equals(o.query)
}

// HashCode returns a hash code for the wrapper.
func (w *multiTermQueryConstantScoreBlendedWrapper) HashCode() int {
	return 31*classHashBlended + w.query.HashCode()
}

// classHashBlended is a stable class-level hash discriminator analogous to
// Java's Object.getClass().hashCode(), computed from the type name.
// Using a fixed prime keeps the value stable across runs.
const classHashBlended = 0x_6d74_7163 // 'mtqc'

// CreateWeight returns a ConstantScoreWeight for this wrapper.
//
// Full blended-rewrite logic (per-leaf TermsEnum enumeration, splitting
// low-/high-frequency terms across DocIdSetBuilder and PriorityQueue,
// and building a DisjunctionDISIApproximation) requires
// AbstractMultiTermQueryConstantScoreWrapper (task 3311) and its
// RewritingWeight/WeightOrDocIdSetIterator helpers.  Until those land,
// the per-leaf supplier yields an empty DocIdSetIterator so the query
// compiles and integrates cleanly with the rest of the search pipeline.
func (w *multiTermQueryConstantScoreBlendedWrapper) CreateWeight(
	searcher *IndexSearcher,
	needsScores bool,
	boost float32,
) (Weight, error) {
	return NewConstantScoreWeight(
		w,
		boost,
		func(_ *index.LeafReaderContext) (ScorerSupplier, error) {
			return NewConstantScoreScorerSupplierFromIterator(
				boost,
				COMPLETE_NO_SCORES,
				NewEmptyDocIdSetIterator(),
			), nil
		},
		nil,
	), nil
}

// Ensure multiTermQueryConstantScoreBlendedWrapper implements Query.
var _ Query = (*multiTermQueryConstantScoreBlendedWrapper)(nil)
