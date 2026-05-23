// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/intervals/DisiWrapper.java

package intervals

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// DisiWrapper wraps an IntervalIterator for use in DisiPriorityQueue.
//
// Mirrors org.apache.lucene.queries.intervals.DisiWrapper (Lucene 10.4.0).
type DisiWrapper struct {
	// Iterator is the underlying DocIdSetIterator.
	Iterator search.DocIdSetIterator
	// Intervals is the original IntervalIterator.
	Intervals IntervalIterator
	// Cost is the estimated iteration cost.
	Cost int64
	// MatchCost is the per-document match cost.
	MatchCost float32
	// Doc is the current doc ID.
	Doc int
	// Next links wrappers sharing the same doc.
	Next *DisiWrapper
	// Approximation is the approximation iterator.
	Approximation search.DocIdSetIterator
}

// NewDisiWrapper constructs a DisiWrapper around the given IntervalIterator.
func NewDisiWrapper(iterator IntervalIterator) *DisiWrapper {
	return &DisiWrapper{
		Intervals:     iterator,
		Iterator:      iterator,
		Cost:          iterator.Cost(),
		Doc:           -1,
		Approximation: iterator,
		MatchCost:     iterator.MatchCost(),
	}
}
