// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "fmt"

// SeededKnnVectorQuery seeds an HNSW-driven KNN search with an initial set of
// candidate ordinals coming from a seed query.
//
// Mirrors org.apache.lucene.search.SeededKnnVectorQuery.
type SeededKnnVectorQuery struct {
	BaseQuery
	field  string
	inner  Query // typically a KnnFloatVectorQuery or KnnByteVectorQuery
	seed   Query // produces the candidate ordinals
	maxK   int
}

// NewSeededKnnVectorQuery wraps inner with seed providing initial candidates.
func NewSeededKnnVectorQuery(field string, inner, seed Query, maxK int) *SeededKnnVectorQuery {
	if inner == nil || seed == nil {
		panic("SeededKnnVectorQuery: both inner and seed are required")
	}
	return &SeededKnnVectorQuery{field: field, inner: inner, seed: seed, maxK: maxK}
}

// GetField returns the vector field name.
func (q *SeededKnnVectorQuery) GetField() string { return q.field }

// Inner returns the wrapped KNN query.
func (q *SeededKnnVectorQuery) Inner() Query { return q.inner }

// Seed returns the seed query that produces the initial candidate ordinals.
func (q *SeededKnnVectorQuery) Seed() Query { return q.seed }

// MaxK returns the requested number of neighbours.
func (q *SeededKnnVectorQuery) MaxK() int { return q.maxK }

// String returns a debug representation.
func (q *SeededKnnVectorQuery) String() string {
	return fmt.Sprintf("SeededKnnVectorQuery(field=%s, maxK=%d)", q.field, q.maxK)
}

// Equals checks structural equality.
func (q *SeededKnnVectorQuery) Equals(other Query) bool {
	o, ok := other.(*SeededKnnVectorQuery)
	if !ok {
		return false
	}
	return q.field == o.field && q.maxK == o.maxK && q.inner.Equals(o.inner) && q.seed.Equals(o.seed)
}

// HashCode hashes the field, maxK and component queries.
func (q *SeededKnnVectorQuery) HashCode() int {
	h := 17
	for i := 0; i < len(q.field); i++ {
		h = 31*h + int(q.field[i])
	}
	h = 31*h + q.maxK
	h = 31*h + q.inner.HashCode()
	h = 31*h + q.seed.HashCode()
	return h
}

// Clone returns an independent copy.
func (q *SeededKnnVectorQuery) Clone() Query {
	return &SeededKnnVectorQuery{
		field: q.field, inner: q.inner.Clone(), seed: q.seed.Clone(), maxK: q.maxK,
	}
}

// CreateWeight delegates to the inner KNN query. The seed wiring is applied at
// scorer-construction time by the strategy carried by KnnSearchStrategy.
func (q *SeededKnnVectorQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return q.inner.CreateWeight(searcher, needsScores, boost)
}
