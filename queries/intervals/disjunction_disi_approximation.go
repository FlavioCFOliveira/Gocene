// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/intervals/DisjunctionDISIApproximation.java

package intervals

// DisjunctionDISIApproximation is a DocIdSetIterator which is the disjunction
// of the approximations of the provided interval iterators.
//
// Mirrors org.apache.lucene.queries.intervals.DisjunctionDISIApproximation (Lucene 10.4.0).
type DisjunctionDISIApproximation struct {
	subIterators *DisiPriorityQueue
	cost         int64
}

// NewDisjunctionDISIApproximation builds a DisjunctionDISIApproximation.
func NewDisjunctionDISIApproximation(subIterators *DisiPriorityQueue) *DisjunctionDISIApproximation {
	var cost int64
	for _, w := range subIterators.All() {
		cost += w.Cost
	}
	return &DisjunctionDISIApproximation{subIterators: subIterators, cost: cost}
}

// Cost returns the estimated cost.
func (d *DisjunctionDISIApproximation) Cost() int64 { return d.cost }

// DocID returns the current doc ID.
func (d *DisjunctionDISIApproximation) DocID() int { return d.subIterators.Top().Doc }

// DocIDRunEnd returns a conservative upper bound.
func (d *DisjunctionDISIApproximation) DocIDRunEnd() int { return d.DocID() + 1 }

// NextDoc advances to the next document.
func (d *DisjunctionDISIApproximation) NextDoc() (int, error) {
	top := d.subIterators.Top()
	doc := top.Doc
	for {
		nextDoc, err := top.Approximation.NextDoc()
		if err != nil {
			return 0, err
		}
		top.Doc = nextDoc
		top = d.subIterators.UpdateTop()
		if top.Doc != doc {
			break
		}
	}
	return top.Doc, nil
}

// Advance advances to at least the given target.
func (d *DisjunctionDISIApproximation) Advance(target int) (int, error) {
	top := d.subIterators.Top()
	for {
		nextDoc, err := top.Approximation.Advance(target)
		if err != nil {
			return 0, err
		}
		top.Doc = nextDoc
		top = d.subIterators.UpdateTop()
		if top.Doc >= target {
			break
		}
	}
	return top.Doc, nil
}
