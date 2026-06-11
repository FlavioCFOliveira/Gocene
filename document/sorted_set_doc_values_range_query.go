// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// SortedSetDocValuesRangeQuery is a data carrier for a range query over
// SortedSetDocValues fields. It mirrors the package-private class
// org.apache.lucene.document.SortedSetDocValuesRangeQuery
// (Lucene 10.4.0).
//
// The actual Query / Weight / Scorer logic lives in the search package;
// this type holds the field name, the inclusive/exclusive lower and
// upper bounds in BytesRef form, and the inclusivity flags.
type SortedSetDocValuesRangeQuery struct {
	field          string
	lowerValue     *util.BytesRef
	upperValue     *util.BytesRef
	lowerInclusive bool
	upperInclusive bool
}

// NewSortedSetDocValuesRangeQuery constructs a
// SortedSetDocValuesRangeQuery data carrier.
//
// field must be non-empty. lowerValue and upperValue may be nil to
// represent unbounded ranges (matching Lucene's behaviour where a nil
// bound means "no bound"). lowerInclusive is only meaningful when
// lowerValue is non-nil; upperInclusive is only meaningful when
// upperValue is non-nil.
func NewSortedSetDocValuesRangeQuery(field string, lowerValue, upperValue *util.BytesRef, lowerInclusive, upperInclusive bool) (*SortedSetDocValuesRangeQuery, error) {
	if field == "" {
		return nil, fmt.Errorf("field must not be null")
	}
	// Copy bytes defensively if non-nil.
	var lowerDup *util.BytesRef
	if lowerValue != nil {
		b := make([]byte, len(lowerValue.Bytes))
		copy(b, lowerValue.Bytes)
		lowerDup = util.NewBytesRef(b)
	}
	var upperDup *util.BytesRef
	if upperValue != nil {
		b := make([]byte, len(upperValue.Bytes))
		copy(b, upperValue.Bytes)
		upperDup = util.NewBytesRef(b)
	}
	return &SortedSetDocValuesRangeQuery{
		field:          field,
		lowerValue:     lowerDup,
		upperValue:     upperDup,
		lowerInclusive: lowerValue != nil && lowerInclusive,
		upperInclusive: upperValue != nil && upperInclusive,
	}, nil
}

// Field returns the target field name.
func (q *SortedSetDocValuesRangeQuery) Field() string { return q.field }

// LowerValue returns the lower bound, or nil if unbounded.
func (q *SortedSetDocValuesRangeQuery) LowerValue() *util.BytesRef { return q.lowerValue }

// UpperValue returns the upper bound, or nil if unbounded.
func (q *SortedSetDocValuesRangeQuery) UpperValue() *util.BytesRef { return q.upperValue }

// LowerInclusive reports whether the lower bound is inclusive.
// Meaningful only when LowerValue is non-nil.
func (q *SortedSetDocValuesRangeQuery) LowerInclusive() bool { return q.lowerInclusive }

// UpperInclusive reports whether the upper bound is inclusive.
// Meaningful only when UpperValue is non-nil.
func (q *SortedSetDocValuesRangeQuery) UpperInclusive() bool { return q.upperInclusive }

// String returns a human-readable representation.
func (q *SortedSetDocValuesRangeQuery) String() string {
	lower := "*"
	if q.lowerValue != nil {
		lower = string(q.lowerValue.Bytes)
	}
	upper := "*"
	if q.upperValue != nil {
		upper = string(q.upperValue.Bytes)
	}
	open := "{"
	if q.lowerInclusive {
		open = "["
	}
	close := "}"
	if q.upperInclusive {
		close = "]"
	}
	return fmt.Sprintf("SortedSetDocValuesRangeQuery(field=%s, range=%s%s TO %s%s)", q.field, open, lower, upper, close)
}

// Equals reports whether two SortedSetDocValuesRangeQuery carriers are
// equal.
func (q *SortedSetDocValuesRangeQuery) Equals(other *SortedSetDocValuesRangeQuery) bool {
	if q == other {
		return true
	}
	if q == nil || other == nil {
		return false
	}
	if q.field != other.field || q.lowerInclusive != other.lowerInclusive || q.upperInclusive != other.upperInclusive {
		return false
	}
	if !bytesRefEqual(q.lowerValue, other.lowerValue) {
		return false
	}
	if !bytesRefEqual(q.upperValue, other.upperValue) {
		return false
	}
	return true
}

// bytesRefEqual reports whether two BytesRef values have equal content.
func bytesRefEqual(a, b *util.BytesRef) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.Length != b.Length {
		return false
	}
	for i := 0; i < a.Length; i++ {
		if a.Bytes[a.Offset+i] != b.Bytes[b.Offset+i] {
			return false
		}
	}
	return true
}
