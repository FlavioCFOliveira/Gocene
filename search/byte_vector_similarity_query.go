// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search/knn"
)

// ByteVectorSimilarityQuery is a query that finds documents with vectors
// similar to the provided byte vector.
//
// Mirrors org.apache.lucene.search.ByteVectorSimilarityQuery.
type ByteVectorSimilarityQuery struct {
	*BaseVectorSimilarityQuery
	vector []byte
}

// NewByteVectorSimilarityQuery creates a new ByteVectorSimilarityQuery.
//
// Mirrors ByteVectorSimilarityQuery(String, float, float, byte[], Query).
func NewByteVectorSimilarityQuery(
	field string,
	traversalSimilarity, resultSimilarity float32,
	vector []byte,
	filter Query,
) (*ByteVectorSimilarityQuery, error) {
	if len(vector) == 0 {
		return nil, fmt.Errorf("vector must not be empty")
	}

	base, err := NewBaseVectorSimilarityQuery(field, traversalSimilarity, resultSimilarity, filter)
	if err != nil {
		return nil, err
	}

	return &ByteVectorSimilarityQuery{
		BaseVectorSimilarityQuery: base,
		vector:                   vector,
	}, nil
}

// CreateVectorScorer returns a VectorScorer for the leaf segment.
//
// Mirrors AbstractVectorSimilarityQuery.createVectorScorer.
func (q *ByteVectorSimilarityQuery) CreateVectorScorer(ctx *index.LeafReaderContext) (VectorScorer, error) {
	// The actual vector scorer creation is typically handled by the VectorField's reader.
	// We assume the context's reader can provide a scorer for the given vector.
	return ctx.Reader().GetVectorScorer(q.Field, q.vector)
}

// ApproximateSearch performs the HNSW approximate search on one leaf segment.
//
// Mirrors AbstractVectorSimilarityQuery.approximateSearch.
func (q *ByteVectorSimilarityQuery) ApproximateSearch(
	ctx *index.LeafReaderContext,
	acceptDocs AcceptDocs,
	visitLimit int,
	mgr knn.KnnCollectorManager,
) (*TopDocs, error) {
	// HNSW approximate search is implemented in the VectorField's reader.
	return ctx.Reader().ApproximateVectorSearch(q.Field, q.vector, acceptDocs, visitLimit, mgr)
}

// Vector represents the query vector.
func (q *ByteVectorSimilarityQuery) Vector() []byte {
	return q.vector
}

var _ VectorSimilarityQueryImpl = (*ByteVectorSimilarityQuery)(nil)
var _ Query = (*ByteVectorSimilarityQuery)(nil)
