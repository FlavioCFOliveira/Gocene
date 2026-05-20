// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

package search

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/DisjunctionDISIApproximation.java

import "slices"

// DisjunctionDISIApproximation is a DocIdSetIterator which is a disjunction
// of the approximations of the provided iterators.
//
// Mirrors org.apache.lucene.search.DisjunctionDISIApproximation
// (Lucene 10.4.0).
//
// Deviations from Java:
//   - Java extends AbstractDocIdSetIterator which tracks a protected `doc`
//     field; Go tracks doc explicitly.
//   - intoBitSet(int, FixedBitSet, int) is not on Gocene's DocIdSetIterator
//     interface; that method is omitted.
//   - DisiPriorityQueue.AddAll bulk-heapifies the lead entries as in
//     DisiPriorityQueueN.addAll.
//
// @lucene.internal
type DisjunctionDISIApproximation struct {
	// leadIterators is the min-doc heap of "lead" wrappers.
	leadIterators *DisiPriorityQueue
	// otherIterators is the linear-scan slice of remaining wrappers.
	otherIterators []*DisiWrapper
	cost           int64
	// leadTop caches leadIterators.Top() to avoid one method call per step.
	leadTop     *DisiWrapper
	minOtherDoc int
	doc         int
}

// NewDisjunctionDISIApproximation builds a DisjunctionDISIApproximation.
//
// The constructor splits wrappers into a priority-queue subset (lead) and
// a linear-scan subset (other) using the same heuristic as Java:
// wrappers are sorted by descending cost, and entries are placed in the
// priority queue as long as Σ min(cost, leadCost) ≤ 1.5 × leadCost.
// At least one wrapper is always placed in the priority queue.
//
// Mirrors DisjunctionDISIApproximation(Collection<DisiWrapper>, long).
func NewDisjunctionDISIApproximation(wrappers []*DisiWrapper, leadCost int64) *DisjunctionDISIApproximation {
	// Sort by descending cost.
	slices.SortFunc(wrappers, func(a, b *DisiWrapper) int {
		if a.cost > b.cost {
			return -1
		}
		if a.cost < b.cost {
			return 1
		}
		return 0
	})

	// reorderThreshold = leadCost + leadCost/2, clamped to avoid overflow.
	reorderThreshold := leadCost + (leadCost >> 1)
	if reorderThreshold < 0 { // overflow
		reorderThreshold = int64(^uint64(0) >> 1) // MaxInt64
	}

	var totalCost int64
	var reorderCost int64
	// lastIdx is the last index that stays out of the PQ (linear-scan slice).
	lastIdx := len(wrappers) - 1
	for ; lastIdx >= 0; lastIdx-- {
		w := wrappers[lastIdx]
		inc := w.cost
		if inc > leadCost {
			inc = leadCost
		}
		if reorderCost+inc < 0 || reorderCost+inc > reorderThreshold {
			break
		}
		reorderCost += inc
		totalCost += w.cost
	}

	// Guarantee at least one entry in the PQ.
	if lastIdx == len(wrappers)-1 {
		totalCost += wrappers[lastIdx].cost
		lastIdx--
	}

	pqLen := len(wrappers) - lastIdx - 1
	pq := NewDisiPriorityQueue(pqLen)
	pq.AddAll(wrappers, lastIdx+1, pqLen)

	otherSlice := wrappers[:lastIdx+1]
	minOther := NO_MORE_DOCS
	for _, w := range otherSlice {
		totalCost += w.cost
		if w.doc < minOther {
			minOther = w.doc
		}
	}

	d := &DisjunctionDISIApproximation{
		leadIterators:  pq,
		otherIterators: otherSlice,
		cost:           totalCost,
		minOtherDoc:    minOther,
		doc:            -1,
	}
	d.leadTop = pq.Top()
	return d
}

// DocID returns the current document ID.
func (it *DisjunctionDISIApproximation) DocID() int { return it.doc }

// Cost returns the estimated number of matching documents.
func (it *DisjunctionDISIApproximation) Cost() int64 { return it.cost }

// NextDoc advances to the next document.
//
// Mirrors DisjunctionDISIApproximation.nextDoc().
func (it *DisjunctionDISIApproximation) NextDoc() (int, error) {
	if it.leadTop.doc < it.minOtherDoc {
		curDoc := it.leadTop.doc
		for {
			var err error
			it.leadTop.doc, err = it.leadTop.approximation.NextDoc()
			if err != nil {
				return NO_MORE_DOCS, err
			}
			it.leadTop = it.leadIterators.UpdateTop()
			if it.leadTop.doc != curDoc {
				break
			}
		}
		if it.leadTop.doc < it.minOtherDoc {
			it.doc = it.leadTop.doc
		} else {
			it.doc = it.minOtherDoc
		}
		return it.doc, nil
	}
	return it.Advance(it.minOtherDoc + 1)
}

// Advance moves to the first document ≥ target.
//
// Mirrors DisjunctionDISIApproximation.advance(int).
func (it *DisjunctionDISIApproximation) Advance(target int) (int, error) {
	for it.leadTop.doc < target {
		var err error
		it.leadTop.doc, err = it.leadTop.approximation.Advance(target)
		if err != nil {
			return NO_MORE_DOCS, err
		}
		it.leadTop = it.leadIterators.UpdateTop()
	}

	it.minOtherDoc = NO_MORE_DOCS
	for _, w := range it.otherIterators {
		if w.doc < target {
			var err error
			w.doc, err = w.approximation.Advance(target)
			if err != nil {
				return NO_MORE_DOCS, err
			}
		}
		if w.doc < it.minOtherDoc {
			it.minOtherDoc = w.doc
		}
	}

	if it.leadTop.doc < it.minOtherDoc {
		it.doc = it.leadTop.doc
	} else {
		it.doc = it.minOtherDoc
	}
	return it.doc, nil
}

// DocIDRunEnd returns the end of the current contiguous run of matching
// document IDs.
//
// Mirrors DisjunctionDISIApproximation.docIDRunEnd().
func (it *DisjunctionDISIApproximation) DocIDRunEnd() int {
	max := it.doc + 1
	for w := it.topList(); w != nil; w = w.next {
		if end := w.approximation.DocIDRunEnd(); end > max {
			max = end
		}
	}
	return max
}

// TopList returns a linked list (via DisiWrapper.next) of all wrappers
// positioned at the current document. Callers must not retain the list
// across the next iteration step.
//
// Mirrors DisjunctionDISIApproximation.topList().
func (it *DisjunctionDISIApproximation) TopList() *DisiWrapper {
	return it.topList()
}

func (it *DisjunctionDISIApproximation) topList() *DisiWrapper {
	if it.leadTop.doc < it.minOtherDoc {
		return it.leadIterators.TopList()
	}
	return it.computeTopList()
}

func (it *DisjunctionDISIApproximation) computeTopList() *DisiWrapper {
	var list *DisiWrapper
	if it.leadTop.doc == it.minOtherDoc {
		list = it.leadIterators.TopList()
	}
	for _, w := range it.otherIterators {
		if w.doc == it.minOtherDoc {
			w.next = list
			list = w
		}
	}
	return list
}

// Compile-time check.
var _ DocIdSetIterator = (*DisjunctionDISIApproximation)(nil)
