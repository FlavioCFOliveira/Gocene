// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"
	"strconv"
	"strings"
)

// IntRangeSlowRangeQuery is a data carrier for a slow range query over
// IntRange doc-values fields. It mirrors the package-private class
// org.apache.lucene.document.IntRangeSlowRangeQuery (Lucene 10.5.0).
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
		return nil, fmt.Errorf("field name cannot be null")
	}
	if min == nil || max == nil || len(min) == 0 || len(max) == 0 {
		return nil, fmt.Errorf("min/max range values cannot be null or empty")
	}
	if len(min) != len(max) {
		return nil, fmt.Errorf("min/max ranges must agree")
	}
	if len(min) > 4 {
		return nil, fmt.Errorf("IntRange does not support greater than 4 dimensions")
	}
	for i := range min {
		if min[i] > max[i] {
			return nil, fmt.Errorf("min value (%d) is greater than max value (%d) at dim %d", min[i], max[i], i)
		}
	}

	dupMin := append([]int32(nil), min...)
	dupMax := append([]int32(nil), max...)

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
	out := append([]int32(nil), q.min...)
	return out
}

// Max returns a defensive copy of the maximum values per dimension.
func (q *IntRangeSlowRangeQuery) Max() []int32 {
	out := append([]int32(nil), q.max...)
	return out
}

// QueryType returns the range query type.
func (q *IntRangeSlowRangeQuery) QueryType() RangeFieldQueryType { return q.queryType }

// Equals reports whether two IntRangeSlowRangeQuery carriers are equal.
// Mirrors Lucene's implementation using field equality and Arrays.equals for min/max.
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

// HashCode mirrors Java's Objects/Arrays-based hash:
// a per-type constant rolled through (31*h + field-hash + Arrays.hashCode(min) + Arrays.hashCode(max)).
func (q *IntRangeSlowRangeQuery) HashCode() int {
	h := 0x6972_7372 // "irsr"
	h = 31*h + intStringHash(q.field)
	h = 31*h + int32SliceHash(q.min)
	h = 31*h + int32SliceHash(q.max)
	return h
}

// String mirrors Java's toString():
// "IntRangeSlowRangeQuery <field: [min0:max0] [min1:max1] ...>"
func (q *IntRangeSlowRangeQuery) String() string {
	var b strings.Builder
	b.WriteString("IntRangeSlowRangeQuery <")
	b.WriteString(q.field)
	b.WriteByte(':')
	for i := 0; i < len(q.min); i++ {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteByte('[')
		b.WriteString(strconv.FormatInt(int64(q.min[i]), 10))
		b.WriteString(" : ")
		b.WriteString(strconv.FormatInt(int64(q.max[i]), 10))
		b.WriteByte(']')
	}
	b.WriteByte('>')
	return b.String()
}

// ToString mirrors Java's toString(String field):
// "field:[min0, ...] TO [max0, ...]]" (if field differs) or "[min0, ...] TO [max0, ...]]" (if field matches).
func (q *IntRangeSlowRangeQuery) ToString(field string) string {
	var b strings.Builder
	if q.field != field {
		b.WriteString(q.field)
		b.WriteByte(':')
	}
	b.WriteByte('[')
	b.WriteString(formatInt32Slice(q.min))
	b.WriteString(" TO ")
	b.WriteString(formatInt32Slice(q.max))
	b.WriteByte(']')
	return b.String()
}

func intStringHash(s string) int {
	const (
		offset = 2166136261
		prime  = 16777619
	)
	var h uint32 = offset
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= prime
	}
	return int(int32(h))
}

func int32SliceHash(a []int32) int {
	h := int32(1)
	for _, v := range a {
		h = 31*h + v
	}
	return int(h)
}

func formatInt32Slice(a []int32) string {
	if len(a) == 0 {
		return "[]"
	}
	var b strings.Builder
	b.WriteByte('[')
	for i, v := range a {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(strconv.FormatInt(int64(v), 10))
	}
	b.WriteByte(']')
	return b.String()
}
