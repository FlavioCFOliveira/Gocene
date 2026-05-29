// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

// Ported from Apache Lucene 10.4.0:
//   lucene/join/src/java/org/apache/lucene/search/join/DiversifyingChildrenFloatKnnVectorQuery.java

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/search/knn"
)

// DiversifyingChildrenFloatKnnVectorQuery is a kNN float-vector query that joins
// matching children vector documents with their parent doc id: at most one
// child per parent block contributes to the result, so the top-K cannot
// collapse onto a single parent. The documents returned are child doc ids with
// their similarity scores, to be wrapped by [ToParentBlockJoinQuery] with
// ScoreMode Max as documented on the Lucene class.
//
// Mirrors org.apache.lucene.search.join.DiversifyingChildrenFloatKnnVectorQuery,
// which extends KnnFloatVectorQuery and overrides exactSearch /
// approximateSearch / getKnnCollectorManager.
//
// # Divergence from Lucene (exact-only diversifying search)
//
// Lucene's approximate path passes a DiversifyingNearestChildrenKnnCollector to
// the codec's reader.searchNearestVectors so the HNSW graph traversal itself is
// diversified. Gocene's codec reader KNN API
// (SegmentReader.SearchNearestVectors) does not yet accept an external
// collector, so the collector-driven HNSW approximate path is unavailable.
// Both the "approximate" and "exact" code paths here therefore perform the
// faithful exact (brute-force) diversifying scan over the leaf's
// FloatVectorValues, which produces byte-exact (identical) results to Lucene's
// exact path. Wiring the collector into the HNSW traversal is tracked by
// rmp #4770.
type DiversifyingChildrenFloatKnnVectorQuery struct {
	*search.BaseKnnVectorQuery

	// Field, Target, K, ChildFilter and ParentsFilter are kept exported for
	// backward compatibility with the descriptor stub this type replaces and
	// for the table-driven tests that inspect them directly.
	Field         string
	Target        []float32
	K             int
	ChildFilter   search.Query
	ParentsFilter BitSetProducer
}

// NewDiversifyingChildrenFloatKnnVectorQuery builds a runnable query.
//
// childFilter is applied as the underlying KnnFloatVectorQuery pre-filter
// (AcceptDocs); parents identifies the parent documents of each block.
func NewDiversifyingChildrenFloatKnnVectorQuery(field string, target []float32, k int, childFilter search.Query, parents BitSetProducer) *DiversifyingChildrenFloatKnnVectorQuery {
	clone := make([]float32, len(target))
	copy(clone, target)
	q := &DiversifyingChildrenFloatKnnVectorQuery{
		Field:         field,
		Target:        clone,
		K:             k,
		ChildFilter:   childFilter,
		ParentsFilter: parents,
	}
	// impl must be q so the base query routes ApproximateSearch / ExactSearch
	// back to the diversifying overrides below (mirroring Java subclassing).
	// The DEFAULT Hnsw strategy matches the Lucene single-arg constructor.
	base := search.NewBaseKnnVectorQuery(q, field, k, childFilter, knn.DefaultHnsw)
	q.BaseKnnVectorQuery = &base
	return q
}

// ApproximateSearch performs the diversifying scan on one leaf. See the type
// doc for the exact-vs-HNSW divergence (rmp #4770): we run the faithful exact
// diversifying scan over the leaf's FloatVectorValues here too.
//
// Mirrors DiversifyingChildrenFloatKnnVectorQuery.approximateSearch.
func (q *DiversifyingChildrenFloatKnnVectorQuery) ApproximateSearch(
	ctx *index.LeafReaderContext,
	acceptDocs search.AcceptDocs,
	_ int,
	_ knn.KnnCollectorManager,
) (*search.TopDocs, error) {
	iter, err := acceptDocs.Iterator()
	if err != nil {
		return nil, err
	}
	return q.ExactSearch(ctx, iter, nil)
}

// ExactSearch performs the exact diversifying search over acceptIterator,
// keeping the single best-scoring child per parent block and collecting the
// global top-K of those.
//
// Mirrors DiversifyingChildrenFloatKnnVectorQuery.exactSearch and the inner
// DiversifyingChildrenVectorScorer.
func (q *DiversifyingChildrenFloatKnnVectorQuery) ExactSearch(
	ctx *index.LeafReaderContext,
	acceptIterator search.DocIdSetIterator,
	timeout index.QueryTimeout,
) (*search.TopDocs, error) {
	values, err := leafFloatVectorValues(ctx, q.Field)
	if err != nil {
		return nil, err
	}
	if values == nil {
		return search.NewTopDocs(search.NewTotalHits(0, search.EQUAL_TO), nil), nil
	}
	parentBitSet, err := q.ParentsFilter.GetBitSet(ctx)
	if err != nil {
		return nil, err
	}
	if parentBitSet == nil {
		return search.NewTopDocs(search.NewTotalHits(0, search.EQUAL_TO), nil), nil
	}
	sim := leafVectorSimilarity(ctx, q.Field)
	score := func(docID int) (float32, bool, error) {
		vec, err := values.Get(docID)
		if err != nil {
			return 0, false, err
		}
		if vec == nil {
			return 0, false, nil
		}
		return sim.Compare(q.Target, vec), true, nil
	}
	return diversifyingExactSearch(acceptIterator, parentBitSet, q.K, timeout, score)
}

// CreateVectorScorer is unused: the diversifying query overrides ExactSearch and
// never falls back to the base linear-scan VectorScorer path. Returning nil is
// the "no exact-search support" sentinel for the base query, but ExactSearch is
// reached first via the [search.ExactSearcher] hook.
func (q *DiversifyingChildrenFloatKnnVectorQuery) CreateVectorScorer(
	_ *index.LeafReaderContext, _ *index.FieldInfo,
) (search.VectorScorer, error) {
	return nil, nil
}

// String returns a human-readable representation.
// Mirrors DiversifyingChildrenFloatKnnVectorQuery.toString.
func (q *DiversifyingChildrenFloatKnnVectorQuery) String() string {
	var sb strings.Builder
	sb.WriteString("DiversifyingChildrenFloatKnnVectorQuery:")
	sb.WriteString(q.Field)
	if len(q.Target) > 0 {
		sb.WriteString(fmt.Sprintf("[%g,...][%d]", q.Target[0], q.K))
	} else {
		sb.WriteString(fmt.Sprintf("[][%d]", q.K))
	}
	if q.ChildFilter != nil {
		sb.WriteString("[")
		sb.WriteString(fmt.Sprintf("%v", q.ChildFilter))
		sb.WriteString("]")
	}
	return sb.String()
}

// Equals reports structural equality.
func (q *DiversifyingChildrenFloatKnnVectorQuery) Equals(other search.Query) bool {
	o, ok := other.(*DiversifyingChildrenFloatKnnVectorQuery)
	if !ok {
		return false
	}
	if q.Field != o.Field || q.K != o.K || len(q.Target) != len(o.Target) {
		return false
	}
	for i := range q.Target {
		if q.Target[i] != o.Target[i] {
			return false
		}
	}
	return queriesEqual(q.ChildFilter, o.ChildFilter)
}

// HashCode hashes the field, k, and vector contents.
func (q *DiversifyingChildrenFloatKnnVectorQuery) HashCode() int {
	h := 17
	for _, b := range []byte(q.Field) {
		h = 31*h + int(b)
	}
	h = 31*h + q.K
	for _, v := range q.Target {
		h = 31*h + int(v)
	}
	return h
}

// Clone returns an independent copy.
func (q *DiversifyingChildrenFloatKnnVectorQuery) Clone() search.Query {
	return NewDiversifyingChildrenFloatKnnVectorQuery(q.Field, q.Target, q.K, q.ChildFilter, q.ParentsFilter)
}

// Compile-time guards.
var (
	_ search.KnnVectorQueryImpl = (*DiversifyingChildrenFloatKnnVectorQuery)(nil)
	_ search.ExactSearcher      = (*DiversifyingChildrenFloatKnnVectorQuery)(nil)
)
