// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "fmt"

// PatienceKnnVectorQuery wraps a KNN query with a patience-based early-termination
// strategy that stops the search once additional iterations are unlikely to
// yield better candidates.
//
// Mirrors org.apache.lucene.search.PatienceKnnVectorQuery. The concrete
// patience logic lives in the search/knn package via HnswQueueSaturationCollector;
// this query carries the canonical public surface.
type PatienceKnnVectorQuery struct {
	BaseQuery
	inner    Query
	patience int
}

// NewPatienceKnnVectorQuery wires inner with a patience threshold.
// inner must be a KnnFloatVectorQuery or KnnByteVectorQuery.
func NewPatienceKnnVectorQuery(inner Query, patience int) *PatienceKnnVectorQuery {
	if inner == nil {
		panic("PatienceKnnVectorQuery: inner is required")
	}
	if patience <= 0 {
		panic("PatienceKnnVectorQuery: patience must be > 0")
	}
	return &PatienceKnnVectorQuery{inner: inner, patience: patience}
}

// Inner returns the wrapped KNN query.
func (q *PatienceKnnVectorQuery) Inner() Query { return q.inner }

// Patience returns the configured patience threshold.
func (q *PatienceKnnVectorQuery) Patience() int { return q.patience }

// String returns a debug representation.
func (q *PatienceKnnVectorQuery) String() string {
	return fmt.Sprintf("PatienceKnnVectorQuery(patience=%d, inner=%v)", q.patience, sprintQuery(q.inner))
}

// Equals checks structural equality.
func (q *PatienceKnnVectorQuery) Equals(other Query) bool {
	o, ok := other.(*PatienceKnnVectorQuery)
	if !ok {
		return false
	}
	return q.patience == o.patience && q.inner.Equals(o.inner)
}

// HashCode hashes the inner query and patience.
func (q *PatienceKnnVectorQuery) HashCode() int {
	h := 17
	h = 31*h + q.inner.HashCode()
	h = 31*h + q.patience
	return h
}

// Clone returns an independent copy.
func (q *PatienceKnnVectorQuery) Clone() Query {
	return &PatienceKnnVectorQuery{inner: q.inner.Clone(), patience: q.patience}
}

// Rewrite delegates to the inner KNN query's rewrite, which runs the full
// AbstractKnnVectorQuery search across all segments and returns a
// DocAndScoreQuery (or MatchNoDocsQuery).
//
// Mirrors PatienceKnnVectorQuery.rewrite, which wraps the delegate's per-leaf
// collectors in an HnswQueueSaturationCollector to stop early once the result
// queue saturates, then delegates to the underlying KNN search. Gocene's
// patience early-termination collector is not yet wired through the leaf-level
// codec search, so this falls back to the underlying (non-early-terminated)
// search — producing the same final top-K, just without the saturation
// short-circuit. Without this override the embedded BaseQuery.Rewrite would
// return the bare BaseQuery receiver, erasing the KNN algorithm and silently
// matching zero documents.
func (q *PatienceKnnVectorQuery) Rewrite(reader IndexReader) (Query, error) {
	return q.inner.Rewrite(reader)
}

// CreateWeight delegates to the inner query.
func (q *PatienceKnnVectorQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return q.inner.CreateWeight(searcher, needsScores, boost)
}
