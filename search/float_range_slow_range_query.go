// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"math"
	"strings"
)

// FloatRangeSlowRangeQuery is a range query over FloatRange doc-values fields.
//
// This is a faithful port of Lucene's org.apache.lucene.document.FloatRangeSlowRangeQuery.
type FloatRangeSlowRangeQuery struct {
	RangeFieldQuery
	field string
	min   []float32
	max   []float32
}

// NewFloatRangeSlowRangeQuery constructs a FloatRangeSlowRangeQuery.
//
// This mirrors the constructor of Lucene's FloatRangeSlowRangeQuery.
func NewFloatRangeSlowRangeQuery(field string, min, max []float32, queryType RangeFieldQueryType) (*FloatRangeSlowRangeQuery, error) {
	if field == "" {
		return nil, fmt.Errorf("field must not be null")
	}
	if len(min) == 0 || len(max) == 0 {
		return nil, fmt.Errorf("min/max range values cannot be null or empty")
	}
	if len(min) != len(max) {
		return nil, fmt.Errorf("min/max ranges must agree")
	}
	if len(min) > 4 {
		return nil, fmt.Errorf("FloatRange does not support greater than 4 dimensions")
	}
	for i := range min {
		if math.IsNaN(float64(min[i])) {
			return nil, fmt.Errorf("invalid min value (%f) in FloatRange", min[i])
		}
		if math.IsNaN(float64(max[i])) {
			return nil, fmt.Errorf("invalid max value (%f) in FloatRange", max[i])
		}
		if min[i] > max[i] {
			return nil, fmt.Errorf("min value (%f) is greater than max value (%f)", min[i], max[i])
		}
	}

	encoded, err := Encode(min, max)
	if err != nil {
		return nil, err
	}

	rfq, err := NewRangeFieldQuery(field, encoded, len(min), queryType)
	if err != nil {
		return nil, err
	}

	dupMin := make([]float32, len(min))
	copy(dupMin, min)
	dupMax := make([]float32, len(max))
	copy(dupMax, max)

	return &FloatRangeSlowRangeQuery{
		RangeFieldQuery: *rfq,
		field:           field,
		min:             dupMin,
		max:             dupMax,
	}, nil
}

// Equals reports whether two FloatRangeSlowRangeQuery instances are equal.
//
// This mirrors the equals method in Lucene's FloatRangeSlowRangeQuery.
func (q *FloatRangeSlowRangeQuery) Equals(other Query) bool {
	if q == other {
		return true
	}
	that, ok := other.(*FloatRangeSlowRangeQuery)
	if !ok {
		return false
	}
	if q.field != that.field {
		return false
	}
	if len(q.min) != len(that.min) || len(q.max) != len(that.max) {
		return false
	}
	for i := range q.min {
		if q.min[i] != that.min[i] {
			return false
		}
	}
	for i := range q.max {
		if q.max[i] != that.max[i] {
			return false
		}
	}
	return true
}

func (q *FloatRangeSlowRangeQuery) Clone() Query {
	dupMin := make([]float32, len(q.min))
	copy(dupMin, q.min)
	dupMax := make([]float32, len(q.max))
	copy(dupMax, q.max)

	return &FloatRangeSlowRangeQuery{
		RangeFieldQuery: q.RangeFieldQuery,
		field:           q.field,
		min:             dupMin,
		max:             dupMax,
	}
}

// CreateWeight is a stub for the weight implementation.
func (q *FloatRangeSlowRangeQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return nil, fmt.Errorf("CreateWeight is not yet implemented for FloatRangeSlowRangeQuery")
}

// HashCode returns a hash code for the query.
//
// This mirrors the hashCode method in Lucene's FloatRangeSlowRangeQuery.
func (q *FloatRangeSlowRangeQuery) HashCode() int {
	h := 1 // Simple class hash seed
	h = 31*h + hashString(q.field)
	h = 31*h + hashFloatSlice(q.min)
	h = 31*h + hashFloatSlice(q.max)
	return h
}

func hashString(s string) int {
	h := 0
	for i := 0; i < len(s); i++ {
		h = 31*h + int(s[i])
	}
	return h
}

func hashFloatSlice(slice []float32) int {
	h := 0
	for _, v := range slice {
		// In Java, Arrays.hashCode(float[]) uses Float.floatToIntBits
		h = 31*h + int(math.Float32bits(v))
	}
	return h
}

// Visit allows a QueryVisitor to visit this query.
func (q *FloatRangeSlowRangeQuery) Visit(visitor QueryVisitor) {
	if visitor.AcceptField(q.field) {
		visitor.VisitLeaf(q)
	}
}

// ToString returns a string representation of the query.
//
// This mirrors the toString(String field) method in Lucene's FloatRangeSlowRangeQuery.
func (q *FloatRangeSlowRangeQuery) ToString(field string) string {
	var b strings.Builder
	if q.field != field {
		b.WriteString(q.field)
		b.WriteString(":")
	}
	b.WriteByte('[')
	for i, v := range q.min {
		if i > 0 {
			b.WriteByte(' ')
		}
		fmt.Fprintf(&b, "%f", v)
	}
	b.WriteString(" TO ")
	for i, v := range q.max {
		if i > 0 {
			b.WriteByte(' ')
		}
		fmt.Fprintf(&b, "%f", v)
	}
	b.WriteByte(']')
	return b.String()
}

// Rewrite returns a rewritten version of the query.
func (q *FloatRangeSlowRangeQuery) Rewrite(indexSearcher IndexSearcher) (Query, error) {
	return q, nil
}
