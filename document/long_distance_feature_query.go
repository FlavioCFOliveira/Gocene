// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import "fmt"

// LongDistanceFeatureQuery is a data carrier for a query that scores
// documents by their distance from a long-valued origin. It mirrors
// the package-private class
// org.apache.lucene.document.LongDistanceFeatureQuery (Lucene 10.4.0).
//
// The score is computed as:
//
//	boost * pivotDistance / (pivotDistance + abs(value - origin))
//
// The actual Query / Weight / Scorer logic lives in the search package;
// this type holds the field name, the origin value and the pivot
// distance that the search-layer implementation consumes.
type LongDistanceFeatureQuery struct {
	field         string
	origin        int64
	pivotDistance int64
}

// NewLongDistanceFeatureQuery constructs a LongDistanceFeatureQuery
// data carrier. field must be non-empty and pivotDistance must be > 0.
func NewLongDistanceFeatureQuery(field string, origin int64, pivotDistance int64) (*LongDistanceFeatureQuery, error) {
	if field == "" {
		return nil, fmt.Errorf("field must not be null")
	}
	if pivotDistance <= 0 {
		return nil, fmt.Errorf("pivotDistance must be > 0, got %d", pivotDistance)
	}
	return &LongDistanceFeatureQuery{
		field:         field,
		origin:        origin,
		pivotDistance: pivotDistance,
	}, nil
}

// Field returns the target field name.
func (q *LongDistanceFeatureQuery) Field() string { return q.field }

// Origin returns the origin value used for distance computation.
func (q *LongDistanceFeatureQuery) Origin() int64 { return q.origin }

// PivotDistance returns the pivot distance used in scoring.
func (q *LongDistanceFeatureQuery) PivotDistance() int64 { return q.pivotDistance }

// String returns a human-readable representation.
func (q *LongDistanceFeatureQuery) String() string {
	return fmt.Sprintf("LongDistanceFeatureQuery(field=%s, origin=%d, pivotDistance=%d)", q.field, q.origin, q.pivotDistance)
}

// Equals reports whether two LongDistanceFeatureQuery carriers are
// equal.
func (q *LongDistanceFeatureQuery) Equals(other *LongDistanceFeatureQuery) bool {
	if q == other {
		return true
	}
	if q == nil || other == nil {
		return false
	}
	return q.field == other.field && q.origin == other.origin && q.pivotDistance == other.pivotDistance
}
