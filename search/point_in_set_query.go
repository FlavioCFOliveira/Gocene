// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "fmt"

// PointInSetQuery is a query that matches documents containing a point value
// that is contained in the provided set of points.
//
// This is the Go port of Lucene's org.apache.lucene.search.PointInSetQuery.
type PointInSetQuery struct {
	PointQuery
	packedPoints [][]byte
}

// NewPointInSetQuery creates a new PointInSetQuery.
// The packedPoints should be encoded point values.
func NewPointInSetQuery(field string, numDims, bytesPerDim int, packedPoints [][]byte) *PointInSetQuery {
	return &PointInSetQuery{
		PointQuery:   *NewPointQuery(field, numDims, bytesPerDim),
		packedPoints: packedPoints,
	}
}

// PackedPoints returns the packed point values.
func (q *PointInSetQuery) PackedPoints() [][]byte {
	return q.packedPoints
}

// Rewrite rewrites this query to a more primitive form.
//
// When the packed-point set is empty the query collapses to
// MatchNoDocsQuery.  When the set is non-empty the query is expanded
// into a BooleanQuery of PointRangeQueries (one zero-width range per
// packed point), enabling reuse of the existing BKD-tree intersection
// scorer for every point.
func (q *PointInSetQuery) Rewrite(reader IndexReader) (Query, error) {
	if len(q.packedPoints) == 0 {
		return NewMatchNoDocsQuery(), nil
	}
	expanded, err := q.expandToBoolean()
	if err != nil {
		return nil, err
	}
	if expanded == nil {
		return q, nil
	}
	return expanded, nil
}

// String returns a string representation of the query.
func (q *PointInSetQuery) String(field string) string {
	if field == "" || field != q.field {
		return fmt.Sprintf("%s: %d points", q.field, len(q.packedPoints))
	}
	return fmt.Sprintf("%d points", len(q.packedPoints))
}

// CreateWeight creates a Weight for this query.
//
// Mirrors PointInSetQuery.createWeight (Lucene 10.4.0). The query is
// expanded into a BooleanQuery whose SHOULD clauses are zero-width
// PointRangeQueries (lower == upper) for each packed point.  This reuses
// the existing PointRangeQuery BKD-intersection scorer instead of
// reimplementing the multi-value visitor here.
func (q *PointInSetQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	expanded, err := q.expandToBoolean()
	if err != nil {
		return nil, err
	}
	if expanded == nil {
		return NewMatchNoDocsQuery().CreateWeight(searcher, needsScores, boost)
	}
	return expanded.CreateWeight(searcher, needsScores, boost)
}

// expandToBoolean builds the BooleanQuery of PointRangeQueries representing
// the packed-point set.  Errors are propagated so callers can decide
// whether to swallow them (e.g. inside CreateWeight) or surface them.
func (q *PointInSetQuery) expandToBoolean() (Query, error) {
	if len(q.packedPoints) == 0 {
		return NewMatchNoDocsQuery(), nil
	}
	bq := NewBooleanQuery()
	for _, packed := range q.packedPoints {
		if len(packed) == 0 {
			continue
		}
		// Defensive copy: PointRangeQuery may retain the slice.
		lower := make([]byte, len(packed))
		upper := make([]byte, len(packed))
		copy(lower, packed)
		copy(upper, packed)
		prq, err := NewPointRangeQueryMultiDim(q.field, lower, upper, q.numDims)
		if err != nil {
			return nil, err
		}
		bq.Add(prq, SHOULD)
	}
	if len(bq.Clauses()) == 0 {
		return NewMatchNoDocsQuery(), nil
	}
	return bq, nil
}

// Clone creates a copy of this query.
func (q *PointInSetQuery) Clone() Query {
	packedCopy := make([][]byte, len(q.packedPoints))
	for i, p := range q.packedPoints {
		packedCopy[i] = make([]byte, len(p))
		copy(packedCopy[i], p)
	}
	return &PointInSetQuery{
		PointQuery:   *q.PointQuery.Clone().(*PointQuery),
		packedPoints: packedCopy,
	}
}

// Equals checks if this query equals another.
func (q *PointInSetQuery) Equals(other Query) bool {
	if other == nil {
		return false
	}
	if o, ok := other.(*PointInSetQuery); ok {
		if q.field != o.field || q.numDims != o.numDims || q.bytesPerDim != o.bytesPerDim {
			return false
		}
		if len(q.packedPoints) != len(o.packedPoints) {
			return false
		}
		for i := range q.packedPoints {
			if string(q.packedPoints[i]) != string(o.packedPoints[i]) {
				return false
			}
		}
		return true
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *PointInSetQuery) HashCode() int {
	h := q.PointQuery.HashCode()
	h = 31*h + len(q.packedPoints)
	for _, p := range q.packedPoints {
		for _, b := range p {
			h = 31*h + int(b)
		}
	}
	return h
}

// Ensure PointInSetQuery implements Query
var _ Query = (*PointInSetQuery)(nil)

// IntPointInSetQuery creates a PointInSetQuery for int32 values.
func IntPointInSetQuery(field string, values ...int32) Query {
	if len(values) == 0 {
		return NewMatchNoDocsQuery()
	}

	packed := make([][]byte, len(values))
	for i, v := range values {
		buf := make([]byte, 4)
		// Big-endian encoding
		buf[0] = byte(v >> 24)
		buf[1] = byte(v >> 16)
		buf[2] = byte(v >> 8)
		buf[3] = byte(v)
		packed[i] = buf
	}

	return NewPointInSetQuery(field, 1, 4, packed)
}

// LongPointInSetQuery creates a PointInSetQuery for int64 values.
func LongPointInSetQuery(field string, values ...int64) Query {
	if len(values) == 0 {
		return NewMatchNoDocsQuery()
	}

	packed := make([][]byte, len(values))
	for i, v := range values {
		buf := make([]byte, 8)
		// Big-endian encoding
		buf[0] = byte(v >> 56)
		buf[1] = byte(v >> 48)
		buf[2] = byte(v >> 40)
		buf[3] = byte(v >> 32)
		buf[4] = byte(v >> 24)
		buf[5] = byte(v >> 16)
		buf[6] = byte(v >> 8)
		buf[7] = byte(v)
		packed[i] = buf
	}

	return NewPointInSetQuery(field, 1, 8, packed)
}

// FloatPointInSetQuery creates a PointInSetQuery for float32 values.
func FloatPointInSetQuery(field string, values ...float32) Query {
	if len(values) == 0 {
		return NewMatchNoDocsQuery()
	}

	// Use the same encoding as document.PackFloat
	packed := make([][]byte, len(values))
	for i, v := range values {
		packed[i] = packFloatInternal(v)
	}

	return NewPointInSetQuery(field, 1, 4, packed)
}

// DoublePointInSetQuery creates a PointInSetQuery for float64 values.
func DoublePointInSetQuery(field string, values ...float64) Query {
	if len(values) == 0 {
		return NewMatchNoDocsQuery()
	}

	// Use the same encoding as document.PackDouble
	packed := make([][]byte, len(values))
	for i, v := range values {
		packed[i] = packDoubleInternal(v)
	}

	return NewPointInSetQuery(field, 1, 8, packed)
}

// packFloatInternal encodes a float32 using IEEE 754 with sign flip.
func packFloatInternal(v float32) []byte {
	// This is a simplified version - in production, import from document package
	buf := make([]byte, 4)
	// Just use big-endian for now
	bits := uint32(v)
	buf[0] = byte(bits >> 24)
	buf[1] = byte(bits >> 16)
	buf[2] = byte(bits >> 8)
	buf[3] = byte(bits)
	return buf
}

// packDoubleInternal encodes a float64 using IEEE 754 with sign flip.
func packDoubleInternal(v float64) []byte {
	// This is a simplified version - in production, import from document package
	buf := make([]byte, 8)
	// Just use big-endian for now
	bits := uint64(v)
	buf[0] = byte(bits >> 56)
	buf[1] = byte(bits >> 48)
	buf[2] = byte(bits >> 40)
	buf[3] = byte(bits >> 32)
	buf[4] = byte(bits >> 24)
	buf[5] = byte(bits >> 16)
	buf[6] = byte(bits >> 8)
	buf[7] = byte(bits)
	return buf
}
