// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import "fmt"

// SortedNumericDocValuesRangeQuery is a data carrier for a range query
// over SortedNumericDocValues fields. It mirrors the package-private
// class org.apache.lucene.document.SortedNumericDocValuesRangeQuery
// (Lucene 10.4.0).
//
// The actual Query / Weight / Scorer logic lives in the search package;
// this type holds the field name and the inclusive lower/upper bounds
// that the search-layer implementation consumes.
type SortedNumericDocValuesRangeQuery struct {
	field      string
	lowerValue int64
	upperValue int64
}

// NewSortedNumericDocValuesRangeQuery constructs a
// SortedNumericDocValuesRangeQuery data carrier.
// field must be non-empty.
func NewSortedNumericDocValuesRangeQuery(field string, lowerValue, upperValue int64) (*SortedNumericDocValuesRangeQuery, error) {
	if field == "" {
		return nil, fmt.Errorf("field must not be null")
	}
	return &SortedNumericDocValuesRangeQuery{
		field:      field,
		lowerValue: lowerValue,
		upperValue: upperValue,
	}, nil
}

// Field returns the target field name.
func (q *SortedNumericDocValuesRangeQuery) Field() string { return q.field }

// LowerValue returns the inclusive lower bound.
func (q *SortedNumericDocValuesRangeQuery) LowerValue() int64 { return q.lowerValue }

// UpperValue returns the inclusive upper bound.
func (q *SortedNumericDocValuesRangeQuery) UpperValue() int64 { return q.upperValue }

// String returns a human-readable representation.
func (q *SortedNumericDocValuesRangeQuery) String() string {
	return fmt.Sprintf("SortedNumericDocValuesRangeQuery(field=%s, lower=%d, upper=%d)", q.field, q.lowerValue, q.upperValue)
}

// Equals reports whether two SortedNumericDocValuesRangeQuery carriers
// are equal.
func (q *SortedNumericDocValuesRangeQuery) Equals(other *SortedNumericDocValuesRangeQuery) bool {
	if q == other {
		return true
	}
	if q == nil || other == nil {
		return false
	}
	return q.field == other.field && q.lowerValue == other.lowerValue && q.upperValue == other.upperValue
}
