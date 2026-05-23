// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/spans/SpanDisiWrapper.java

package spans

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// SpanDisiWrapper wraps a Spans for use in SpanDisiPriorityQueue.
//
// Mirrors org.apache.lucene.queries.spans.SpanDisiWrapper (Lucene 10.4.0).
type SpanDisiWrapper struct {
	// Iterator is the underlying DocIdSetIterator.
	Iterator search.DocIdSetIterator
	// Cost is the estimated iteration cost.
	Cost int64
	// MatchCost is the match cost for two-phase iterators, 0 otherwise.
	MatchCost float32
	// Doc is the current doc ID, used for comparison in the priority queue.
	Doc int
	// Next links wrappers sharing the same doc in a topList linked list.
	Next *SpanDisiWrapper

	// Approximation is an approximation of the iterator, or the iterator itself
	// if it does not support two-phase iteration.
	Approximation search.DocIdSetIterator
	// TwoPhaseView is the TwoPhaseIterator view, or nil if not supported.
	TwoPhaseView *search.TwoPhaseIterator

	// Spans is the original Spans this wrapper was built from.
	Spans Spans
	// LastApproxMatchDoc is the last doc of approximation that did match.
	LastApproxMatchDoc int
	// LastApproxNonMatchDoc is the last doc of approximation that did not match.
	LastApproxNonMatchDoc int
}

// NewSpanDisiWrapper constructs a SpanDisiWrapper around the given Spans.
func NewSpanDisiWrapper(spans Spans) *SpanDisiWrapper {
	w := &SpanDisiWrapper{
		Spans:                 spans,
		Iterator:              spans,
		Cost:                  spans.Cost(),
		Doc:                   -1,
		LastApproxNonMatchDoc: -2,
		LastApproxMatchDoc:    -2,
	}
	w.TwoPhaseView = spans.AsTwoPhaseIterator()
	if w.TwoPhaseView != nil {
		w.Approximation = w.TwoPhaseView.Approximation()
		w.MatchCost = w.TwoPhaseView.MatchCost()
	} else {
		w.Approximation = spans
		w.MatchCost = 0
	}
	return w
}
