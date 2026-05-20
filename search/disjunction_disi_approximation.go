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

// DisjunctionDISIApproximation is a DocIdSetIterator over the union
// (disjunction) of a set of DocIdSetIterators, using a priority queue
// on the lead iterators and a linear scan over the rest.
//
// Mirrors org.apache.lucene.search.DisjunctionDISIApproximation
// (Lucene 10.4.0).
//
// Deviations from Java:
//   - The Java implementation separates "lead" iterators (those whose
//     cost is ≤ leadCost) from "other" iterators. This port replicates
//     that heuristic: if a wrapper's cost ≤ leadCost it goes into the
//     priority queue, otherwise into the linear-scan slice.
//   - All iterators are placed in the lead queue when the total count
//     is small (≤ 2), matching Lucene's minimum-heap path.
type DisjunctionDISIApproximation struct {
	lead        *DisiPriorityQueue
	others      []*DisiWrapper
	cost        int64
	minOtherDoc int
	doc         int
}

// NewDisjunctionDISIApproximation builds a DisjunctionDISIApproximation
// from the given wrappers. leadCost is the cost budget: wrappers with
// cost ≤ leadCost are placed in the priority queue; the rest in the
// linear-scan slice.
func NewDisjunctionDISIApproximation(wrappers []*DisiWrapper, leadCost int64) *DisjunctionDISIApproximation {
	var total int64
	for _, w := range wrappers {
		total += w.cost
	}

	pq := NewDisiPriorityQueue(len(wrappers))
	var others []*DisiWrapper
	for _, w := range wrappers {
		if w.cost <= leadCost {
			w.doc = w.approximation.DocID()
			pq.Add(w)
		} else {
			w.doc = w.approximation.DocID()
			others = append(others, w)
		}
	}
	// Guarantee at least one entry in the priority queue.
	if pq.Size() == 0 && len(others) > 0 {
		w := others[len(others)-1]
		others = others[:len(others)-1]
		pq.Add(w)
	}

	minOther := NO_MORE_DOCS
	for _, w := range others {
		if w.doc < minOther {
			minOther = w.doc
		}
	}

	return &DisjunctionDISIApproximation{
		lead:        pq,
		others:      others,
		cost:        total,
		minOtherDoc: minOther,
		doc:         -1,
	}
}

// DocID returns the current document ID.
func (it *DisjunctionDISIApproximation) DocID() int { return it.doc }

// Cost returns the estimated number of matching documents.
func (it *DisjunctionDISIApproximation) Cost() int64 { return it.cost }

// NextDoc advances to the next document.
func (it *DisjunctionDISIApproximation) NextDoc() (int, error) {
	top := it.lead.Top()
	if top == nil {
		it.doc = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	if top.doc < it.minOtherDoc {
		curDoc := top.doc
		for {
			var err error
			top.doc, err = top.approximation.NextDoc()
			if err != nil {
				return NO_MORE_DOCS, err
			}
			top = it.lead.UpdateTop()
			if top.doc != curDoc {
				break
			}
		}
		if top.doc < it.minOtherDoc {
			it.doc = top.doc
		} else {
			it.doc = it.minOtherDoc
		}
		return it.doc, nil
	}
	return it.Advance(it.minOtherDoc + 1)
}

// Advance moves to the first document ≥ target.
func (it *DisjunctionDISIApproximation) Advance(target int) (int, error) {
	top := it.lead.Top()
	for top != nil && top.doc < target {
		var err error
		top.doc, err = top.approximation.Advance(target)
		if err != nil {
			return NO_MORE_DOCS, err
		}
		top = it.lead.UpdateTop()
	}

	it.minOtherDoc = NO_MORE_DOCS
	for _, w := range it.others {
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

	if top != nil && top.doc < it.minOtherDoc {
		it.doc = top.doc
	} else {
		it.doc = it.minOtherDoc
	}
	return it.doc, nil
}

// DocIDRunEnd returns the end of the current contiguous run of matching
// document IDs, used by DISI bulk-advance optimizations.
func (it *DisjunctionDISIApproximation) DocIDRunEnd() int {
	max := it.doc + 1
	for w := it.topList(); w != nil; w = w.next {
		if end := w.approximation.DocIDRunEnd(); end > max {
			max = end
		}
	}
	return max
}

// topList returns a linked list (via DisiWrapper.next) of all wrappers
// positioned at the current document. Callers must not retain the list
// across the next iteration step.
func (it *DisjunctionDISIApproximation) topList() *DisiWrapper {
	top := it.lead.Top()
	if top == nil {
		return nil
	}
	if top.doc < it.minOtherDoc {
		return it.lead.TopList()
	}
	return it.computeTopList()
}

func (it *DisjunctionDISIApproximation) computeTopList() *DisiWrapper {
	var list *DisiWrapper
	top := it.lead.Top()
	if top != nil && top.doc == it.minOtherDoc {
		list = it.lead.TopList()
	}
	for _, w := range it.others {
		if w.doc == it.minOtherDoc {
			w.next = list
			list = w
		}
	}
	return list
}

// Compile-time check.
var _ DocIdSetIterator = (*DisjunctionDISIApproximation)(nil)
