// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"bytes"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search/knn"
)

// KnnByteVectorQuery uses KnnVectorsReader.Search to perform nearest
// neighbour search on byte vectors.
//
// This query also allows for performing a kNN search subject to a filter.
// In this case, it first executes the filter for each leaf, then chooses a
// strategy dynamically:
//
// - If the filter cost is less than k, just execute an exact search
// - Otherwise run a kNN search subject to the filter
// - If the kNN search visits too many vectors without completing, stop and run an exact search
//
// Ported from org.apache.lucene.search.KnnByteVectorQuery.
type KnnByteVectorQuery struct {
	BaseKnnVectorQuery
	target []byte
}

// NewKnnByteVectorQuery finds the k nearest documents to the target vector
// according to the vectors in the given field.
func NewKnnByteVectorQuery(field string, target []byte, k int) *KnnByteVectorQuery {
	return NewKnnByteVectorQueryWithFilter(field, target, k, nil)
}

// NewKnnByteVectorQueryWithFilter finds the k nearest documents to the target vector
// according to the vectors in the given field, subject to a filter applied before the vector search.
func NewKnnByteVectorQueryWithFilter(field string, target []byte, k int, filter Query) *KnnByteVectorQuery {
	q := &KnnByteVectorQuery{target: target}
	q.BaseKnnVectorQuery = NewBaseKnnVectorQuery(q, field, k, filter, knn.DefaultHnsw)
	return q
}

// NewKnnByteVectorQueryWithStrategy finds the k nearest documents to the target vector
// according to the vectors in the given field, subject to a filter applied before the vector search,
// using the provided search strategy.
func NewKnnByteVectorQueryWithStrategy(field string, target []byte, k int, filter Query, strategy knn.KnnSearchStrategy) *KnnByteVectorQuery {
	q := &KnnByteVectorQuery{target: target}
	q.BaseKnnVectorQuery = NewBaseKnnVectorQuery(q, field, k, filter, strategy)
	return q
}

// ApproximateSearch executes an approximate KNN search on one leaf.
//
// Mirrors KnnByteVectorQuery.approximateSearch.
func (q *KnnByteVectorQuery) ApproximateSearch(
	ctx *index.LeafReaderContext,
	acceptDocs AcceptDocs,
	visitedLimit int,
	collectorManager knn.KnnCollectorManager,
) (*TopDocs, error) {
	knnCollector, err := collectorManager.NewCollector(visitedLimit, q.strategy, ctx)
	if err != nil {
		return nil, err
	}
	reader := ctx.Reader()
	byteVectorValues := reader.GetByteVectorValues(q.field)
	if byteVectorValues == nil {
		if err := index.CheckField(reader, q.field); err != nil {
			return nil, err
		}
		return emptyTopDocs(), nil
	}

	if min(knnCollector.K(), byteVectorValues.Size()) == 0 {
		return emptyTopDocs(), nil
	}

	if err := reader.SearchNearestVectors(q.field, q.target, knnCollector, acceptDocs); err != nil {
		return nil, err
	}

	results := knnCollector.TopDocs()
	if results == nil {
		return emptyTopDocs(), nil
	}
	return results, nil
}

// CreateVectorScorer returns a VectorScorer for exact brute-force
// search on the leaf described by ctx and the given FieldInfo.
//
// Mirrors KnnByteVectorQuery.createVectorScorer.
func (q *KnnByteVectorQuery) CreateVectorScorer(ctx *index.LeafReaderContext, fi *index.FieldInfo) (VectorScorer, error) {
	reader := ctx.Reader()
	vectorValues := reader.GetByteVectorValues(q.field)
	if vectorValues == nil {
		if err := index.CheckField(reader, q.field); err != nil {
			return nil, err
		}
		return nil, nil
	}
	return vectorValues.Scorer(q.target)
}

// ToString returns a string representation of the query.
func (q *KnnByteVectorQuery) ToString(field string) string {
	targetFirst := "nil"
	if len(q.target) > 0 {
		targetFirst = fmt.Sprintf("%d", q.target[0])
	}

	res := fmt.Sprintf("KnnByteVectorQuery:%s[%s,...][%d]",
		field, targetFirst, q.k)
	if q.filter != nil {
		res += fmt.Sprintf("[%s]", q.filter.ToString())
	}
	return res
}

// Equals checks if this query equals another.
func (q *KnnByteVectorQuery) Equals(other Query) bool {
	if q == other {
		return true
	}
	if other == nil {
		return false
	}

	qOther, ok := other.(*KnnByteVectorQuery)
	if !ok {
		return false
	}

	if !q.EqualsBase(&qOther.BaseKnnVectorQuery) {
		return false
	}

	return bytes.Equal(q.target, qOther.target)
}

// HashCode returns a hash code for this query.
func (q *KnnByteVectorQuery) HashCode() int {
	h := q.HashCodeBase()

	targetHash := 0
	for _, b := range q.target {
		targetHash = 31*targetHash + int(b)
	}

	return 31*h + targetHash
}

// GetTargetCopy returns a copy of the target query vector.
func (q *KnnByteVectorQuery) GetTargetCopy() []byte {
	res := make([]byte, len(q.target))
	copy(res, q.target)
	return res
}
