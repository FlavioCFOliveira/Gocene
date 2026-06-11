// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"
	"sort"
)

// SortedNumericDocValuesSetQuery is a data carrier for a set membership
// query over SortedNumericDocValues fields. It mirrors the
// package-private class
// org.apache.lucene.document.SortedNumericDocValuesSetQuery
// (Lucene 10.4.0).
//
// The actual Query / Weight / Scorer logic lives in the search package;
// this type holds the field name and the sorted set of long values that
// the search-layer implementation consumes.
type SortedNumericDocValuesSetQuery struct {
	field   string
	numbers []int64
}

// NewSortedNumericDocValuesSetQuery constructs a
// SortedNumericDocValuesSetQuery data carrier.
//
// field must be non-empty. The numbers slice is sorted and stored;
// duplicates are preserved (matching Lucene's behaviour).
func NewSortedNumericDocValuesSetQuery(field string, numbers []int64) (*SortedNumericDocValuesSetQuery, error) {
	if field == "" {
		return nil, fmt.Errorf("field must not be null")
	}
	if len(numbers) == 0 {
		return nil, fmt.Errorf("numbers must not be empty")
	}
	dup := make([]int64, len(numbers))
	copy(dup, numbers)
	sort.Slice(dup, func(i, j int) bool { return dup[i] < dup[j] })
	return &SortedNumericDocValuesSetQuery{
		field:   field,
		numbers: dup,
	}, nil
}

// Field returns the target field name.
func (q *SortedNumericDocValuesSetQuery) Field() string { return q.field }

// Numbers returns a defensive copy of the sorted set of values.
func (q *SortedNumericDocValuesSetQuery) Numbers() []int64 {
	out := make([]int64, len(q.numbers))
	copy(out, q.numbers)
	return out
}

// String returns a human-readable representation.
func (q *SortedNumericDocValuesSetQuery) String() string {
	return fmt.Sprintf("SortedNumericDocValuesSetQuery(field=%s, numbers=%v)", q.field, q.numbers)
}

// Equals reports whether two SortedNumericDocValuesSetQuery carriers
// are equal.
func (q *SortedNumericDocValuesSetQuery) Equals(other *SortedNumericDocValuesSetQuery) bool {
	if q == other {
		return true
	}
	if q == nil || other == nil {
		return false
	}
	if q.field != other.field || len(q.numbers) != len(other.numbers) {
		return false
	}
	for i := range q.numbers {
		if q.numbers[i] != other.numbers[i] {
			return false
		}
	}
	return true
}
