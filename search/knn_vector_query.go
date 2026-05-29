// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/KnnFloatVectorQuery.java
//   lucene/core/src/java/org/apache/lucene/search/KnnByteVectorQuery.java

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search/knn"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// knnFloatLeafSearcher is the structural per-leaf search surface the float
// KNN query drives. *index.SegmentReader satisfies it (via the codec KNN
// reader wiring landed by rmp #4731).
type knnFloatLeafSearcher interface {
	SearchNearestVectors(field string, target []float32, k int, acceptDocs util.Bits) (index.TopDocs, error)
}

// knnByteLeafSearcher is the byte analogue of [knnFloatLeafSearcher].
type knnByteLeafSearcher interface {
	SearchNearestVectorsByte(field string, target []byte, k int, acceptDocs util.Bits) (index.TopDocs, error)
}

// KnnFloatVectorQuery searches for the k documents whose float vector for the
// given field is most similar to the target vector. An optional pre-filter
// and search-strategy hook mirror Lucene's KnnFloatVectorQuery.
//
// It is the Go port of org.apache.lucene.search.KnnFloatVectorQuery. The
// shared rewrite algorithm lives in the embedded [BaseKnnVectorQuery]
// (= AbstractKnnVectorQuery); this type supplies the two abstract methods
// ApproximateSearch and CreateVectorScorer that drive the codec-level HNSW
// search for float vectors.
type KnnFloatVectorQuery struct {
	*BaseKnnVectorQuery
	target []float32
}

// NewKnnFloatVectorQuery builds the simplest form (no filter, default strategy).
func NewKnnFloatVectorQuery(field string, target []float32, k int) *KnnFloatVectorQuery {
	return NewKnnFloatVectorQueryWithStrategy(field, target, k, nil, nil)
}

// NewKnnFloatVectorQueryWithFilter is the variant that pre-filters the
// candidate set with filter.
func NewKnnFloatVectorQueryWithFilter(field string, target []float32, k int, filter Query) *KnnFloatVectorQuery {
	return NewKnnFloatVectorQueryWithStrategy(field, target, k, filter, nil)
}

// NewKnnFloatVectorQueryWithStrategy is the variant accepting a custom
// [knn.KnnSearchStrategy].
func NewKnnFloatVectorQueryWithStrategy(field string, target []float32, k int, filter Query, strategy knn.KnnSearchStrategy) *KnnFloatVectorQuery {
	q := &KnnFloatVectorQuery{target: append([]float32(nil), target...)}
	base := NewBaseKnnVectorQuery(q, field, k, filter, strategy)
	q.BaseKnnVectorQuery = &base
	return q
}

// GetField returns the vector field name.
func (q *KnnFloatVectorQuery) GetField() string { return q.BaseKnnVectorQuery.GetField() }

// GetTargetCopy returns a defensive copy of the target vector.
func (q *KnnFloatVectorQuery) GetTargetCopy() []float32 {
	return append([]float32(nil), q.target...)
}

// K returns the requested number of neighbours.
func (q *KnnFloatVectorQuery) K() int { return q.GetK() }

// Filter returns the optional pre-filter query (may be nil).
func (q *KnnFloatVectorQuery) Filter() Query { return q.GetFilter() }

// Strategy returns the optional KnnSearchStrategy (may be nil).
func (q *KnnFloatVectorQuery) Strategy() knn.KnnSearchStrategy { return q.GetSearchStrategy() }

// ApproximateSearch runs the codec-level HNSW search on one leaf, returning
// the per-leaf top results. Mirrors KnnFloatVectorQuery.approximateSearch.
func (q *KnnFloatVectorQuery) ApproximateSearch(
	ctx *index.LeafReaderContext,
	acceptDocs AcceptDocs,
	visitedLimit int,
	collectorManager knn.KnnCollectorManager,
) (*TopDocs, error) {
	// The collector carries the per-leaf k (optimistic per-segment
	// rescaling happens inside the manager). Build it to learn that k.
	collector, err := collectorManager.NewCollector(visitedLimit, q.GetSearchStrategy(), ctx)
	if err != nil {
		return nil, err
	}
	perLeafK := collector.K()
	if perLeafK <= 0 {
		return emptyTopDocs(), nil
	}

	searcher, ok := ctx.Reader().(knnFloatLeafSearcher)
	if !ok {
		// Reader does not support vector search (e.g. a mock); no matches.
		return emptyTopDocs(), nil
	}

	bits, err := acceptDocs.Bits()
	if err != nil {
		return nil, err
	}

	td, err := searcher.SearchNearestVectors(q.GetField(), q.target, perLeafK, bits)
	if err != nil {
		return nil, err
	}
	return indexTopDocsToSearch(td), nil
}

// CreateVectorScorer builds a VectorScorer for exact brute-force search on
// the leaf. Mirrors KnnFloatVectorQuery.createVectorScorer.
//
// Exact scoring requires a per-doc VectorScorer over the leaf's float
// vectors. Gocene exposes the vectors but not yet the search.VectorScorer
// bridge over them, so this returns nil; the BaseKnnVectorQuery treats a nil
// scorer as "no exact-search support" and relies on the approximate path.
// Tracked alongside the seeded-search wiring (backlog).
func (q *KnnFloatVectorQuery) CreateVectorScorer(
	_ *index.LeafReaderContext, _ *index.FieldInfo,
) (VectorScorer, error) {
	return nil, nil
}

// String returns a debug representation.
func (q *KnnFloatVectorQuery) String() string {
	return fmt.Sprintf("KnnFloatVectorQuery(field=%s, k=%d, dim=%d)", q.GetField(), q.GetK(), len(q.target))
}

// Equals checks structural equality.
func (q *KnnFloatVectorQuery) Equals(other Query) bool {
	o, ok := other.(*KnnFloatVectorQuery)
	if !ok || len(q.target) != len(o.target) {
		return false
	}
	for i := range q.target {
		if q.target[i] != o.target[i] {
			return false
		}
	}
	return q.EqualsBase(o.BaseKnnVectorQuery)
}

// HashCode hashes the field, k, filter, and vector contents.
func (q *KnnFloatVectorQuery) HashCode() int {
	h := q.HashCodeBase()
	for _, v := range q.target {
		h = 31*h + int(v)
	}
	return h
}

// Clone returns an independent copy.
func (q *KnnFloatVectorQuery) Clone() Query {
	return NewKnnFloatVectorQueryWithStrategy(
		q.GetField(), q.target, q.GetK(), cloneFilter(q.GetFilter()), q.GetSearchStrategy(),
	)
}

// KnnByteVectorQuery is the byte-vector analogue of KnnFloatVectorQuery.
//
// Go port of org.apache.lucene.search.KnnByteVectorQuery.
type KnnByteVectorQuery struct {
	*BaseKnnVectorQuery
	target []byte
}

// NewKnnByteVectorQuery is the simplest form.
func NewKnnByteVectorQuery(field string, target []byte, k int) *KnnByteVectorQuery {
	return NewKnnByteVectorQueryWithStrategy(field, target, k, nil, nil)
}

// NewKnnByteVectorQueryWithFilter is the filter variant.
func NewKnnByteVectorQueryWithFilter(field string, target []byte, k int, filter Query) *KnnByteVectorQuery {
	return NewKnnByteVectorQueryWithStrategy(field, target, k, filter, nil)
}

// NewKnnByteVectorQueryWithStrategy is the variant taking a custom strategy.
func NewKnnByteVectorQueryWithStrategy(field string, target []byte, k int, filter Query, strategy knn.KnnSearchStrategy) *KnnByteVectorQuery {
	q := &KnnByteVectorQuery{target: append([]byte(nil), target...)}
	base := NewBaseKnnVectorQuery(q, field, k, filter, strategy)
	q.BaseKnnVectorQuery = &base
	return q
}

// GetField returns the field name.
func (q *KnnByteVectorQuery) GetField() string { return q.BaseKnnVectorQuery.GetField() }

// GetTargetCopy returns a defensive copy.
func (q *KnnByteVectorQuery) GetTargetCopy() []byte { return append([]byte(nil), q.target...) }

// K returns the requested number of neighbours.
func (q *KnnByteVectorQuery) K() int { return q.GetK() }

// Filter returns the pre-filter query (may be nil).
func (q *KnnByteVectorQuery) Filter() Query { return q.GetFilter() }

// Strategy returns the configured KnnSearchStrategy.
func (q *KnnByteVectorQuery) Strategy() knn.KnnSearchStrategy { return q.GetSearchStrategy() }

// ApproximateSearch runs the codec-level HNSW search on one leaf for byte
// vectors. Mirrors KnnByteVectorQuery.approximateSearch.
func (q *KnnByteVectorQuery) ApproximateSearch(
	ctx *index.LeafReaderContext,
	acceptDocs AcceptDocs,
	visitedLimit int,
	collectorManager knn.KnnCollectorManager,
) (*TopDocs, error) {
	collector, err := collectorManager.NewCollector(visitedLimit, q.GetSearchStrategy(), ctx)
	if err != nil {
		return nil, err
	}
	perLeafK := collector.K()
	if perLeafK <= 0 {
		return emptyTopDocs(), nil
	}

	searcher, ok := ctx.Reader().(knnByteLeafSearcher)
	if !ok {
		return emptyTopDocs(), nil
	}

	bits, err := acceptDocs.Bits()
	if err != nil {
		return nil, err
	}

	td, err := searcher.SearchNearestVectorsByte(q.GetField(), q.target, perLeafK, bits)
	if err != nil {
		return nil, err
	}
	return indexTopDocsToSearch(td), nil
}

// CreateVectorScorer returns nil; exact byte-vector scoring is not yet
// bridged (see the float counterpart).
func (q *KnnByteVectorQuery) CreateVectorScorer(
	_ *index.LeafReaderContext, _ *index.FieldInfo,
) (VectorScorer, error) {
	return nil, nil
}

// String returns a debug representation.
func (q *KnnByteVectorQuery) String() string {
	return fmt.Sprintf("KnnByteVectorQuery(field=%s, k=%d, dim=%d)", q.GetField(), q.GetK(), len(q.target))
}

// Equals checks structural equality.
func (q *KnnByteVectorQuery) Equals(other Query) bool {
	o, ok := other.(*KnnByteVectorQuery)
	if !ok || !bytesEqual(q.target, o.target) {
		return false
	}
	return q.EqualsBase(o.BaseKnnVectorQuery)
}

// HashCode hashes the field, k, filter, and vector contents.
func (q *KnnByteVectorQuery) HashCode() int {
	h := q.HashCodeBase()
	for _, b := range q.target {
		h = 31*h + int(b)
	}
	return h
}

// Clone returns an independent copy.
func (q *KnnByteVectorQuery) Clone() Query {
	return NewKnnByteVectorQueryWithStrategy(
		q.GetField(), q.target, q.GetK(), cloneFilter(q.GetFilter()), q.GetSearchStrategy(),
	)
}

// cloneFilter returns a clone of filter, or nil when filter is nil.
func cloneFilter(filter Query) Query {
	if filter == nil {
		return nil
	}
	return filter.Clone()
}

// indexTopDocsToSearch converts the index-package TopDocs returned by a leaf
// reader's vector search into a search-package *TopDocs. Per-leaf vector
// results are exact (EQUAL_TO) for the result count reported.
func indexTopDocsToSearch(td index.TopDocs) *TopDocs {
	scoreDocs := make([]*ScoreDoc, len(td.ScoreDocs))
	for i, sd := range td.ScoreDocs {
		scoreDocs[i] = &ScoreDoc{Doc: sd.Doc, Score: sd.Score}
	}
	return NewTopDocs(NewTotalHits(int64(len(scoreDocs)), EQUAL_TO), scoreDocs)
}

// Compile-time guards that the queries satisfy the KnnVectorQueryImpl
// contract (and therefore Query).
var (
	_ KnnVectorQueryImpl = (*KnnFloatVectorQuery)(nil)
	_ KnnVectorQueryImpl = (*KnnByteVectorQuery)(nil)
)
