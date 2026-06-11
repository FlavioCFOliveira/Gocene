// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import "fmt"

// IntRangeSlowRangeQuery is a data carrier for a slow range query over
// IntRange doc-values fields. It mirrors the package-private class
// org.apache.lucene.document.IntRangeSlowRangeQuery (Lucene 10.4.0).
//
// The actual Query / Weight / Scorer logic lives in the search package;
// this type holds the field name, the min/max arrays and the query type
// that the search-layer implementation consumes.
type IntRangeSlowRangeQuery struct {
	field     string
	min       []int32
	max       []int32
	queryType RangeFieldQueryType
}

// NewIntRangeSlowRangeQuery constructs an IntRangeSlowRangeQuery data
// carrier.
//
// field must be non-empty, min and max must have the same length (one
// entry per dimension), and each min[i] must be <= max[i].
// queryType should typically be RangeFieldQueryTypeIntersects.
func NewIntRangeSlowRangeQuery(field string, min, max []int32, queryType RangeFieldQueryType) (*IntRangeSlowRangeQuery, error) {
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
	dupMin := make([]int32, len(min))
	dupMax := make([]int32, len(max))
	copy(dupMin, min)
	copy(dupMax, max)
	return &IntRangeSlowRangeQuery{
		field:     field,
		min:       dupMin,
		max:       dupMax,
		queryType: queryType,
	}, nil
}

// Field returns the target field name.
func (q *IntRangeSlowRangeQuery) Field() string { return q.field }

// Min returns a defensive copy of the minimum values per dimension.
func (q *IntRangeSlowRangeQuery) Min() []int32 {
	out := make([]int32, len(q.min))
	copy(out, q.min)
	return out
}

// Max returns a defensive copy of the maximum values per dimension.
func (q *IntRangeSlowRangeQuery) Max() []int32 {
	out := make([]int32, len(q.max))
	copy(out, q.max)
	return out
}

// QueryType returns the range query type.
func (q *IntRangeSlowRangeQuery) QueryType() RangeFieldQueryType { return q.queryType }

// String returns a human-readable representation.
func (q *IntRangeSlowRangeQuery) String() string {
	return fmt.Sprintf("IntRangeSlowRangeQuery(field=%s, min=%v, max=%v, type=%s)", q.field, q.min, q.max, q.queryType)
}

// Equals reports whether two IntRangeSlowRangeQuery carriers are equal.
func (q *IntRangeSlowRangeQuery) Equals(other *IntRangeSlowRangeQuery) bool {
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
