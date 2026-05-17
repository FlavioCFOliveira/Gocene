// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "fmt"

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
