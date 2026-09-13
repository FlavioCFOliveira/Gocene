// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/spans/SpanDisjunctionDISIApproximation.java

package spans

import "github.com/FlavioCFOliveira/Gocene/util"

// SpanDisjunctionDISIApproximation is a DocIdSetIterator which is a disjunction
// of the approximations of the provided iterators.
//
// Mirrors org.apache.lucene.queries.spans.SpanDisjunctionDISIApproximation (Lucene 10.4.0).
type SpanDisjunctionDISIApproximation struct {
	subIterators *SpanDisiPriorityQueue
	cost         int64
}

// NewSpanDisjunctionDISIApproximation builds a SpanDisjunctionDISIApproximation
// from the given priority queue.
func NewSpanDisjunctionDISIApproximation(subIterators *SpanDisiPriorityQueue) *SpanDisjunctionDISIApproximation {
	var cost int64
	for _, w := range subIterators.All() {
		cost += w.Cost
	}
	return &SpanDisjunctionDISIApproximation{
		subIterators: subIterators,
		cost:         cost,
	}
}

// Cost returns the estimated cost.
func (d *SpanDisjunctionDISIApproximation) Cost() int64 { return d.cost }

// DocID returns the current doc ID.
func (d *SpanDisjunctionDISIApproximation) DocID() int {
	return d.subIterators.Top().Doc
}

// DocIDRunEnd returns the end of the current run (single-doc, conservative).
func (d *SpanDisjunctionDISIApproximation) DocIDRunEnd() (int, error) {
	return d.DocID() + 1, nil
}

// NextDoc advances to the next document.
func (d *SpanDisjunctionDISIApproximation) NextDoc() (int, error) {
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

// Advance advances to at least the given target doc ID.
func (d *SpanDisjunctionDISIApproximation) Advance(target int) (int, error) {
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

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int) in Apache Lucene
// 10.5.0, which every subclass inherits unless it overrides it.
func (d *SpanDisjunctionDISIApproximation) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(d, upTo, bitSet, offset)
}
