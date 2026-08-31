// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import "fmt"

// LongRangeSlowRangeQuery is a data carrier for a slow range query over
// LongRange doc-values fields. It mirrors the package-private class
// org.apache.lucene.document.LongRangeSlowRangeQuery (Lucene 10.4.0).
//
// The actual Query / Weight / Scorer logic lives in the search package;
// this type holds the field name, the min/max arrays and the query type
// that the search-layer implementation consumes.
type LongRangeSlowRangeQuery struct {
	field     string
	min       []int64
	max       []int64
	queryType RangeFieldQueryType
}

// NewLongRangeSlowRangeQuery constructs a LongRangeSlowRangeQuery data
// carrier.
//
// field must be non-empty, min and max must have the same length (one
// entry per dimension), and each min[i] must be <= max[i].
// queryType should typically be RangeFieldQueryTypeIntersects.
func NewLongRangeSlowRangeQuery(field string, min, max []int64, queryType RangeFieldQueryType) (*LongRangeSlowRangeQuery, error) {
	if field == "" {
		return nil, fmt.Errorf("field must not be null")
	}
	if len(min) != len(max) {
		return nil, fmt.Errorf("min length %d != max length %d", len(min), len(max))
	}
	if len(min) == 0 {
		return nil, fmt.Errorf("min/max must contain at least one dimension")
	}
	for i := range min {
		if min[i] > max[i] {
			return nil, fmt.Errorf("dim %d: min %d > max %d", i, min[i], max[i])
		}
	}
	dupMin := make([]int64, len(min))
	copy(dupMin, min)
	dupMax := make([]int64, len(max))
	copy(dupMax, max)
	return &LongRangeSlowRangeQuery{
		field:     field,
		min:       dupMin,
		max:       dupMax,
		queryType: queryType,
	}, nil
}

// Field returns the target field name.
func (q *LongRangeSlowRangeQuery) Field() string { return q.field }

// Min returns a defensive copy of the minimum values per dimension.
func (q *LongRangeSlowRangeQuery) Min() []int64 {
	out := make([]int64, len(q.min))
	copy(out, q.min)
	return out
}

// Max returns a defensive copy of the maximum values per dimension.
func (q *LongRangeSlowRangeQuery) Max() []int64 {
	out := make([]int64, len(q.max))
	copy(out, q.max)
	return out
}

// QueryType returns the range query type.
func (q *LongRangeSlowRangeQuery) QueryType() RangeFieldQueryType { return q.queryType }

// String returns a human-readable representation.
func (q *LongRangeSlowRangeQuery) String() string {
	return fmt.Sprintf("LongRangeSlowRangeQuery(field=%s, min=%v, max=%v, type=%s)", q.field, q.min, q.max, q.queryType)
}

// Equals reports whether two LongRangeSlowRangeQuery carriers are equal.
func (q *LongRangeSlowRangeQuery) Equals(other *LongRangeSlowRangeQuery) bool {
	if q == other {
		return true
	}
	if q == nil || other == nil {
		return false
	}
	if q.field != other.field || q.queryType != other.queryType {
		return false
	}
	if len(q.min) != len(other.min) || len(q.max) != len(other.max) {
		return false
	}
	for i := range q.min {
		if q.min[i] != other.min[i] {
			return false
		}
	}
	for i := range q.max {
		if q.max[i] != other.max[i] {
			return false
		}
	}
	return true
}
