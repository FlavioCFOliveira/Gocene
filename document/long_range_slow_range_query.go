// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"
	"strconv"
	"strings"
)

const classHashLongRangeSlowRangeQuery = 0x6c72_7372 // "lrsr"

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

// HashCode mirrors Java's Objects/Arrays-based hash: a per-type constant
// rolled through (31*h + field-hash + Arrays.hashCode(min) + Arrays.hashCode(max)).
func (q *LongRangeSlowRangeQuery) HashCode() int {
	h := int32(classHashLongRangeSlowRangeQuery)
	h = 31*h + int32(stringHash(q.field))
	h = 31*h + int32(int64SliceHash(q.min))
	h = 31*h + int32(int64SliceHash(q.max))
	return int(h)
}

// ToString mirrors the Java reference: optional "field:" prefix when
// rendered out of context, followed by "[ [min0, min1, ...] TO [max0, max1, ...] ]".
func (q *LongRangeSlowRangeQuery) ToString(field string) string {
	var b strings.Builder
	if q.field != field {
		b.WriteString(q.field)
		b.WriteByte(':')
	}
	b.WriteByte('[')
	b.WriteString(formatInt64Slice(q.min))
	b.WriteString(" TO ")
	b.WriteString(formatInt64Slice(q.max))
	b.WriteByte(']')
	return b.String()
}

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

func stringHash(s string) int32 {
	var h int32
	for i := 0; i < len(s); i++ {
		h = 31*h + int32(s[i])
	}
	return h
}

func int64SliceHash(a []int64) int32 {
	h := int32(1)
	for _, v := range a {
		uv := uint64(v)
		mix := int32(uv ^ (uv >> 32))
		h = 31*h + mix
	}
	return h
}

func formatInt64Slice(a []int64) string {
	if len(a) == 0 {
		return "[]"
	}
	var b strings.Builder
	b.WriteByte('[')
	for i, v := range a {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(strconv.FormatInt(v, 10))
	}
	b.WriteByte(']')
	return b.String()
}
