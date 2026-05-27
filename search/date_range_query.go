// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"encoding/binary"
	"fmt"
	"time"
)

// DateRangeQuery is a query that matches documents with date/time values
// within a specified range.
//
// This is the Go port of Lucene's org.apache.lucene.search.DateRangeQuery.
type DateRangeQuery struct {
	field          string
	lower          time.Time
	upper          time.Time
	lowerInclusive bool
	upperInclusive bool
}

// NewDateRangeQuery creates a new DateRangeQuery.
func NewDateRangeQuery(field string, lower, upper time.Time) *DateRangeQuery {
	return &DateRangeQuery{
		field:          field,
		lower:          lower,
		upper:          upper,
		lowerInclusive: true,
		upperInclusive: true,
	}
}

// SetInclusive sets whether the range bounds are inclusive.
func (q *DateRangeQuery) SetInclusive(lower, upper bool) {
	q.lowerInclusive = lower
	q.upperInclusive = upper
}

// Field returns the field name.
func (q *DateRangeQuery) Field() string {
	return q.field
}

// Lower returns the lower bound.
func (q *DateRangeQuery) Lower() time.Time {
	return q.lower
}

// Upper returns the upper bound.
func (q *DateRangeQuery) Upper() time.Time {
	return q.upper
}

// Rewrite rewrites this query into a PointRangeQuery whose packed values
// are big-endian sortable representations of the millisecond timestamps
// (matching how DateField persists indexed Date points in Lucene 10.4.0).
//
// When the lower bound is after the upper bound the query collapses to
// MatchNoDocsQuery, matching the empty-range semantics from Lucene.
func (q *DateRangeQuery) Rewrite(reader IndexReader) (Query, error) {
	if q.lower.After(q.upper) {
		return NewMatchNoDocsQuery(), nil
	}
	lower, upper := q.packedBounds()
	return NewPointRangeQuery(q.field, lower, upper)
}

// CreateWeight creates a Weight for this query by delegating to the
// PointRangeQuery produced by Rewrite.  The BKD-tree intersection
// scorer in PointRangeWeight handles the actual leaf walk.
func (q *DateRangeQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	rewritten, err := q.Rewrite(nil)
	if err != nil {
		return nil, err
	}
	if rewritten == nil {
		return nil, fmt.Errorf("date_range_query: nil rewrite")
	}
	return rewritten.CreateWeight(searcher, needsScores, boost)
}

// packedBounds materialises the lower/upper time bounds as big-endian
// int64 millisecond packs.  Exclusive bounds are adjusted by ±1ms to
// mirror Lucene's inclusive-range BKD semantics.
func (q *DateRangeQuery) packedBounds() ([]byte, []byte) {
	lo := q.lower.UnixMilli()
	hi := q.upper.UnixMilli()
	if !q.lowerInclusive {
		lo++
	}
	if !q.upperInclusive {
		hi--
	}
	lower := make([]byte, 8)
	upper := make([]byte, 8)
	// Sortable-long encoding: flip the sign bit so big-endian byte order
	// matches numeric order across positive and negative timestamps.
	binary.BigEndian.PutUint64(lower, uint64(lo)^(1<<63))
	binary.BigEndian.PutUint64(upper, uint64(hi)^(1<<63))
	return lower, upper
}

// Clone creates a copy of this query.
func (q *DateRangeQuery) Clone() Query {
	return &DateRangeQuery{
		field:          q.field,
		lower:          q.lower,
		upper:          q.upper,
		lowerInclusive: q.lowerInclusive,
		upperInclusive: q.upperInclusive,
	}
}

// Equals checks if this query equals another.
func (q *DateRangeQuery) Equals(other Query) bool {
	if other == nil {
		return false
	}
	if o, ok := other.(*DateRangeQuery); ok {
		return q.field == o.field &&
			q.lower.Equal(o.lower) &&
			q.upper.Equal(o.upper) &&
			q.lowerInclusive == o.lowerInclusive &&
			q.upperInclusive == o.upperInclusive
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *DateRangeQuery) HashCode() int {
	h := 17
	h = 31*h + len(q.field)
	for i := 0; i < len(q.field); i++ {
		h = 31*h + int(q.field[i])
	}
	h = 31*h + int(q.lower.UnixMilli())
	h = 31*h + int(q.upper.UnixMilli())
	return h
}

// String returns a string representation of the query.
func (q *DateRangeQuery) String(field string) string {
	if field == "" || field != q.field {
		return fmt.Sprintf("DateRangeQuery(field=%s, lower=%v, upper=%v)", q.field, q.lower, q.upper)
	}
	return fmt.Sprintf("DateRangeQuery(lower=%v, upper=%v)", q.lower, q.upper)
}

// Ensure DateRangeQuery implements Query
var _ Query = (*DateRangeQuery)(nil)
