// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// NumericDocValuesRangeQuery is an abstract Query over a range of long values
// stored as numeric doc-values for a field. Concrete subclasses provide the
// CreateWeight implementation that actually walks the doc-values.
//
// Mirrors org.apache.lucene.search.NumericDocValuesRangeQuery.
type NumericDocValuesRangeQuery struct {
	BaseQuery
	field      string
	lowerValue int64
	upperValue int64
}

// NewNumericDocValuesRangeQuery constructs a NumericDocValuesRangeQuery with
// inclusive lower and upper bounds. field must be non-empty.
func NewNumericDocValuesRangeQuery(field string, lowerValue, upperValue int64) *NumericDocValuesRangeQuery {
	if field == "" {
		panic("NumericDocValuesRangeQuery: field is required")
	}
	return &NumericDocValuesRangeQuery{
		field:      field,
		lowerValue: lowerValue,
		upperValue: upperValue,
	}
}

// GetField returns the field name.
func (q *NumericDocValuesRangeQuery) GetField() string { return q.field }

// LowerValue returns the inclusive lower bound.
func (q *NumericDocValuesRangeQuery) LowerValue() int64 { return q.lowerValue }

// UpperValue returns the inclusive upper bound.
func (q *NumericDocValuesRangeQuery) UpperValue() int64 { return q.upperValue }

// String returns a debug representation.
func (q *NumericDocValuesRangeQuery) String() string {
	return fmt.Sprintf("NumericDocValuesRangeQuery(field=%s, [%d,%d])", q.field, q.lowerValue, q.upperValue)
}

// Equals checks structural equality.
func (q *NumericDocValuesRangeQuery) Equals(other Query) bool {
	o, ok := other.(*NumericDocValuesRangeQuery)
	if !ok {
		return false
	}
	return q.field == o.field && q.lowerValue == o.lowerValue && q.upperValue == o.upperValue
}

// HashCode returns a stable hash.
func (q *NumericDocValuesRangeQuery) HashCode() int {
	h := 17
	h = 31*h + len(q.field)
	for i := 0; i < len(q.field); i++ {
		h = 31*h + int(q.field[i])
	}
	h = 31*h + int(q.lowerValue^(q.lowerValue>>32))
	h = 31*h + int(q.upperValue^(q.upperValue>>32))
	return h
}

// Clone returns an independent copy.
func (q *NumericDocValuesRangeQuery) Clone() Query {
	return &NumericDocValuesRangeQuery{field: q.field, lowerValue: q.lowerValue, upperValue: q.upperValue}
}

// Rewrite returns the query unchanged. Mirrors the Java base class which
// also relies on subclasses for any concrete simplification.
func (q *NumericDocValuesRangeQuery) Rewrite(_ IndexReader) (Query, error) { return q, nil }

// CreateWeight returns a ConstantScoreWeight that walks the per-leaf
// NumericDocValues iterator and emits a doc-id set containing every
// document whose value falls within [lowerValue, upperValue].
//
// Mirrors the leaf scanning that NumericDocValuesRangeQuery's subclasses
// share in Lucene 10.4.0; Gocene folds the abstract+concrete pair into a
// single concrete CreateWeight so the query can be used directly by the
// DoubleValuesSource / LongValuesSource ranges without a separate
// subclass.
func (q *NumericDocValuesRangeQuery) CreateWeight(_ *IndexSearcher, _ bool, boost float32) (Weight, error) {
	lower, upper, field := q.lowerValue, q.upperValue, q.field

	supplier := func(ctx *index.LeafReaderContext) (ScorerSupplier, error) {
		if ctx == nil {
			return nil, nil
		}
		reader := ctx.LeafReader()
		if reader == nil {
			return nil, nil
		}
		type numericProvider interface {
			GetNumericDocValues(field string) (index.NumericDocValues, error)
		}
		np, ok := interface{}(reader).(numericProvider)
		if !ok {
			return nil, nil
		}
		dv, err := np.GetNumericDocValues(field)
		if err != nil {
			return nil, err
		}
		if dv == nil {
			return nil, nil
		}
		maxDoc := reader.MaxDoc()

		// Collect all matching doc ids up front. This is the same eager
		// strategy used by the sortedNumericDocValuesRangeQuery for the
		// non-singleton path — a doc-values range scan is O(maxDoc).
		//
		// Migrated to the Lucene-faithful iterator surface (rmp #4709):
		// NextDoc positions the iterator, LongValue reads the value at
		// the current position. Monotonic — strictly forward.
		matches := make([]int, 0, 16)
		for {
			doc, err := dv.NextDoc()
			if err != nil {
				return nil, err
			}
			if doc == NO_MORE_DOCS {
				break
			}
			v, err := dv.LongValue()
			if err != nil {
				return nil, err
			}
			if v >= lower && v <= upper {
				matches = append(matches, doc)
			}
		}
		if len(matches) == 0 {
			return nil, nil
		}
		iter := newSortedDocIdSetIterator(matches, maxDoc)
		return NewScorerSupplierAdapter(NewConstantScoreScorer(boost, COMPLETE, iter)), nil
	}
	return NewConstantScoreWeight(q, boost, supplier, nil), nil
}

// sortedDocIdSetIterator is a tiny DocIdSetIterator over a pre-sorted,
// ascending slice of doc ids. It mirrors the DocIdSetBuilder.build path
// used by other docvalues range queries when the match set is materialised
// up front rather than streamed.
type sortedDocIdSetIterator struct {
	docs   []int
	idx    int
	doc    int
	maxDoc int
}

func newSortedDocIdSetIterator(docs []int, maxDoc int) DocIdSetIterator {
	return &sortedDocIdSetIterator{docs: docs, idx: -1, doc: -1, maxDoc: maxDoc}
}

func (it *sortedDocIdSetIterator) DocID() int { return it.doc }

func (it *sortedDocIdSetIterator) NextDoc() (int, error) {
	it.idx++
	if it.idx >= len(it.docs) {
		it.doc = NO_MORE_DOCS
		return it.doc, nil
	}
	it.doc = it.docs[it.idx]
	return it.doc, nil
}

func (it *sortedDocIdSetIterator) Advance(target int) (int, error) {
	for {
		d, err := it.NextDoc()
		if err != nil {
			return d, err
		}
		if d >= target {
			return d, nil
		}
	}
}

func (it *sortedDocIdSetIterator) Cost() int64 { return int64(len(it.docs)) }

func (it *sortedDocIdSetIterator) DocIDRunEnd() int {
	if it.doc < 0 || it.doc == NO_MORE_DOCS {
		return it.doc + 1
	}
	end := it.doc + 1
	for i := it.idx + 1; i < len(it.docs); i++ {
		if it.docs[i] != end {
			break
		}
		end = it.docs[i] + 1
	}
	return end
}
