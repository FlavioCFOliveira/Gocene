// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"github.com/FlavioCFOliveira/Gocene/spi"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// PointQuery is the base class for point-based queries.
// It provides common functionality for queries that operate on point fields
// indexed using the BKD tree structure.
//
// This is the Go port of Lucene's org.apache.lucene.search.PointQuery.
type PointQuery struct {
	field       string
	numDims     int
	bytesPerDim int
}

// NOTE: the constructor that stood here was named NewPointQuery, which is the
// port name of LatLonShape.newPointQuery (lat_lon_shape_query.go). There is no
// org.apache.lucene.search.PointQuery in Lucene 10.5.0, so neither this
// constructor nor the PointQuery type below corresponds to a Lucene artefact,
// and nothing in the package referenced them.

// Field returns the field name.
func (q *PointQuery) Field() string {
	return q.field
}

// NumDims returns the number of dimensions.
func (q *PointQuery) NumDims() int {
	return q.numDims
}

// BytesPerDim returns the number of bytes per dimension.
func (q *PointQuery) BytesPerDim() int {
	return q.bytesPerDim
}

// Rewrite rewrites this query to a more primitive form.
func (q *PointQuery) Rewrite(searcher *IndexSearcher) (Query, error) {
	return q, nil
}

// CreateWeight creates a Weight for this query.
//
// PointQuery is the abstract parent of PointInSetQuery and other
// point-based queries in Gocene; it does not match any document on its
// own.  Returning a ConstantScoreWeight whose ScorerSupplier always
// yields nil mirrors Lucene's "empty" Weight contract for abstract
// queries and lets the parent type be composed (e.g. as a base in
// PointInSetQuery) without triggering an error.
func (q *PointQuery) CreateWeight(searcher *IndexSearcher, scoreMode ScoreMode, boost float32) (Weight, error) {
	return NewConstantScoreWeight(q, boost,
		func(_ *index.LeafReaderContext) (ScorerSupplier, error) { return nil, nil },
		nil,
	), nil
}

// String returns a string representation of the query.
func (q *PointQuery) String(field string) string {
	if field == "" || field != q.field {
		return fmt.Sprintf("PointQuery(field=%s)", q.field)
	}
	return "PointQuery"
}

// Equals checks if this query equals another.
func (q *PointQuery) Equals(other spi.Query) bool {
	if other == nil {
		return false
	}
	if o, ok := other.(*PointQuery); ok {
		return q.field == o.field && q.numDims == o.numDims && q.bytesPerDim == o.bytesPerDim
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *PointQuery) HashCode() int {
	h := 17
	h = 31*h + len(q.field)
	for i := 0; i < len(q.field); i++ {
		h = 31*h + int(q.field[i])
	}
	h = 31*h + q.numDims
	h = 31*h + q.bytesPerDim
	return h
}

// Ensure PointQuery implements Query
var _ Query = (*PointQuery)(nil)
