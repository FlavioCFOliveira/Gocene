// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "fmt"

// IndexSortSortedNumericDocValuesRangeQuery is a range query over a
// SortedNumeric doc-values field that exploits the segment's index sort when
// the leading sort field matches q.field. It rewrites itself into a more
// efficient form when the optimisation applies.
//
// Mirrors org.apache.lucene.search.IndexSortSortedNumericDocValuesRangeQuery.
type IndexSortSortedNumericDocValuesRangeQuery struct {
	BaseQuery
	field      string
	lowerValue int64
	upperValue int64
	fallback   Query
}

// NewIndexSortSortedNumericDocValuesRangeQuery wires the optimisation around
// a fallback Query (typically a NumericDocValuesRangeQuery or a
// PointRangeQuery) that handles the non-optimisable cases.
func NewIndexSortSortedNumericDocValuesRangeQuery(field string, lowerValue, upperValue int64, fallback Query) *IndexSortSortedNumericDocValuesRangeQuery {
	if field == "" {
		panic("IndexSortSortedNumericDocValuesRangeQuery: field is required")
	}
	if fallback == nil {
		panic("IndexSortSortedNumericDocValuesRangeQuery: fallback query is required")
	}
	return &IndexSortSortedNumericDocValuesRangeQuery{
		field:      field,
		lowerValue: lowerValue,
		upperValue: upperValue,
		fallback:   fallback,
	}
}

// GetField returns the field name.
func (q *IndexSortSortedNumericDocValuesRangeQuery) GetField() string { return q.field }

// LowerValue returns the inclusive lower bound.
func (q *IndexSortSortedNumericDocValuesRangeQuery) LowerValue() int64 { return q.lowerValue }

// UpperValue returns the inclusive upper bound.
func (q *IndexSortSortedNumericDocValuesRangeQuery) UpperValue() int64 { return q.upperValue }

// Fallback returns the underlying query used when the optimisation cannot be
// applied (e.g. the segment is not sorted on field).
func (q *IndexSortSortedNumericDocValuesRangeQuery) Fallback() Query { return q.fallback }

// String returns a debug representation.
func (q *IndexSortSortedNumericDocValuesRangeQuery) String() string {
	return fmt.Sprintf("IndexSortSortedNumericDocValuesRangeQuery(field=%s, [%d,%d])", q.field, q.lowerValue, q.upperValue)
}

// Equals checks structural equality.
func (q *IndexSortSortedNumericDocValuesRangeQuery) Equals(other Query) bool {
	o, ok := other.(*IndexSortSortedNumericDocValuesRangeQuery)
	if !ok {
		return false
	}
	return q.field == o.field && q.lowerValue == o.lowerValue && q.upperValue == o.upperValue && q.fallback.Equals(o.fallback)
}

// HashCode returns a stable hash.
func (q *IndexSortSortedNumericDocValuesRangeQuery) HashCode() int {
	h := 17
	for i := 0; i < len(q.field); i++ {
		h = 31*h + int(q.field[i])
	}
	h = 31*h + int(q.lowerValue^(q.lowerValue>>32))
	h = 31*h + int(q.upperValue^(q.upperValue>>32))
	h = 31*h + q.fallback.HashCode()
	return h
}

// Clone returns an independent copy.
func (q *IndexSortSortedNumericDocValuesRangeQuery) Clone() Query {
	return &IndexSortSortedNumericDocValuesRangeQuery{
		field:      q.field,
		lowerValue: q.lowerValue,
		upperValue: q.upperValue,
		fallback:   q.fallback.Clone(),
	}
}

// Rewrite rewrites the fallback query.
func (q *IndexSortSortedNumericDocValuesRangeQuery) Rewrite(reader IndexReader) (Query, error) {
	rw, err := q.fallback.Rewrite(reader)
	if err != nil {
		return nil, err
	}
	if rw == q.fallback {
		return q, nil
	}
	return &IndexSortSortedNumericDocValuesRangeQuery{
		field:      q.field,
		lowerValue: q.lowerValue,
		upperValue: q.upperValue,
		fallback:   rw,
	}, nil
}

// CreateWeight delegates to the fallback query weight. The full Lucene
// optimisation (segment-sort awareness producing a binary-search-driven
// scorer) requires SegmentReader leaf-sort metadata that is not yet wired in
// this package; the placeholder ensures correctness while keeping the API
// shape stable for future tuning.
func (q *IndexSortSortedNumericDocValuesRangeQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return q.fallback.CreateWeight(searcher, needsScores, boost)
}
