// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

// Ported from Apache Lucene 10.4.0:
//   lucene/join/src/java/org/apache/lucene/search/join/DiversifyingChildrenByteKnnVectorQuery.java

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/search/knn"
)

// DiversifyingChildrenByteKnnVectorQuery is the byte-vector variant of the
// diversifying child-KNN query: at most one child per parent block contributes
// to the result so the top-K cannot collapse onto a single parent. Mirrors
// org.apache.lucene.search.join.DiversifyingChildrenByteKnnVectorQuery, which
// extends KnnByteVectorQuery.
//
// Like its float counterpart, both code paths run the faithful exact
// diversifying scan over the leaf's ByteVectorValues; the collector-driven HNSW
// approximate path is deferred to rmp #4770. See
// [DiversifyingChildrenFloatKnnVectorQuery] for the full divergence note.
type DiversifyingChildrenByteKnnVectorQuery struct {
	*search.BaseKnnVectorQuery

	Field         string
	Target        []byte
	K             int
	ChildFilter   search.Query
	ParentsFilter BitSetProducer
}

// NewDiversifyingChildrenByteKnnVectorQuery builds a runnable query.
func NewDiversifyingChildrenByteKnnVectorQuery(field string, target []byte, k int, childFilter search.Query, parents BitSetProducer) *DiversifyingChildrenByteKnnVectorQuery {
	clone := make([]byte, len(target))
	copy(clone, target)
	q := &DiversifyingChildrenByteKnnVectorQuery{
		Field:         field,
		Target:        clone,
		K:             k,
		ChildFilter:   childFilter,
		ParentsFilter: parents,
	}
	base := search.NewBaseKnnVectorQuery(q, field, k, childFilter, knn.DefaultHnsw)
	q.BaseKnnVectorQuery = &base
	return q
}

// ApproximateSearch performs the diversifying scan on one leaf (exact path; see
// the type doc for the rmp #4770 divergence).
//
// Mirrors DiversifyingChildrenByteKnnVectorQuery.approximateSearch.
func (q *DiversifyingChildrenByteKnnVectorQuery) ApproximateSearch(
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

// ExactSearch performs the exact diversifying search over acceptIterator.
//
// Mirrors DiversifyingChildrenByteKnnVectorQuery.exactSearch.
func (q *DiversifyingChildrenByteKnnVectorQuery) ExactSearch(
	ctx *index.LeafReaderContext,
	acceptIterator search.DocIdSetIterator,
	timeout index.QueryTimeout,
) (*search.TopDocs, error) {
	values, err := leafByteVectorValues(ctx, q.Field)
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
		return sim.CompareBytes(q.Target, vec), true, nil
	}
	return diversifyingExactSearch(acceptIterator, parentBitSet, q.K, timeout, score)
}

// CreateVectorScorer is unused (see the float counterpart): ExactSearch is
// reached first via the [search.ExactSearcher] hook.
func (q *DiversifyingChildrenByteKnnVectorQuery) CreateVectorScorer(
	_ *index.LeafReaderContext, _ *index.FieldInfo,
) (search.VectorScorer, error) {
	return nil, nil
}

// String returns a human-readable representation.
// Mirrors DiversifyingChildrenByteKnnVectorQuery.toString.
func (q *DiversifyingChildrenByteKnnVectorQuery) String() string {
	var sb strings.Builder
	sb.WriteString("DiversifyingChildrenByteKnnVectorQuery:")
	sb.WriteString(q.Field)
	if len(q.Target) > 0 {
		sb.WriteString(fmt.Sprintf("[%d,...][%d]", q.Target[0], q.K))
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
func (q *DiversifyingChildrenByteKnnVectorQuery) Equals(other search.Query) bool {
	o, ok := other.(*DiversifyingChildrenByteKnnVectorQuery)
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
func (q *DiversifyingChildrenByteKnnVectorQuery) HashCode() int {
	h := 17
	for _, b := range []byte(q.Field) {
		h = 31*h + int(b)
	}
	h = 31*h + q.K
	for _, b := range q.Target {
		h = 31*h + int(b)
	}
	return h
}

// Clone returns an independent copy.
func (q *DiversifyingChildrenByteKnnVectorQuery) Clone() search.Query {
	return NewDiversifyingChildrenByteKnnVectorQuery(q.Field, q.Target, q.K, q.ChildFilter, q.ParentsFilter)
}

// Compile-time guards.
var (
	_ search.KnnVectorQueryImpl = (*DiversifyingChildrenByteKnnVectorQuery)(nil)
	_ search.ExactSearcher      = (*DiversifyingChildrenByteKnnVectorQuery)(nil)
)
