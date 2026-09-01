// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search/knn"
)

// FloatVectorSimilarityQuery is a query that finds documents with vectors
// similar to the provided float vector.
//
// Mirrors org.apache.lucene.search.FloatVectorSimilarityQuery.
type FloatVectorSimilarityQuery struct {
	*BaseVectorSimilarityQuery
	vector []float32
}

// NewFloatVectorSimilarityQuery creates a new FloatVectorSimilarityQuery.
//
// Mirrors FloatVectorSimilarityQuery(String, float, float, float[], Query).
func NewFloatVectorSimilarityQuery(
	field string,
	traversalSimilarity, resultSimilarity float32,
	vector []float32,
	filter Query,
) (*FloatVectorSimilarityQuery, error) {
	if len(vector) == 0 {
		return nil, fmt.Errorf("vector must not be empty")
	}

	base, err := NewBaseVectorSimilarityQuery(field, traversalSimilarity, resultSimilarity, filter)
	if err != nil {
		return nil, err
	}

	return &FloatVectorSimilarityQuery{
		BaseVectorSimilarityQuery: base,
		vector:                   vector,
	}, nil
}

// CreateVectorScorer returns a VectorScorer for the leaf segment.
//
// Mirrors AbstractVectorSimilarityQuery.createVectorScorer.
func (q *FloatVectorSimilarityQuery) CreateVectorScorer(ctx *index.LeafReaderContext) (VectorScorer, error) {
	// The actual vector scorer creation is typically handled by the VectorField's reader.
	return ctx.Reader().GetVectorScorerFloat(q.Field, q.vector)
}

// ApproximateSearch performs the HNSW approximate search on one leaf segment.
//
// Mirrors AbstractVectorSimilarityQuery.approximateSearch.
func (q *FloatVectorSimilarityQuery) ApproximateSearch(
	ctx *index.LeafReaderContext,
	acceptDocs AcceptDocs,
	visitLimit int,
	mgr knn.KnnCollectorManager,
) (*TopDocs, error) {
	// HNSW approximate search is implemented in the VectorField's reader.
	return ctx.Reader().ApproximateVectorSearchFloat(q.Field, q.vector, acceptDocs, visitLimit, mgr)
}

// Vector represents the query vector.
func (q *FloatVectorSimilarityQuery) Vector() []float32 {
	return q.vector
}

var _ VectorSimilarityQueryImpl = (*FloatVectorSimilarityQuery)(nil)
var _ Query = (*FloatVectorSimilarityQuery)(nil)
