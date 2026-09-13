// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
	"fmt"
	"math"
	"slices"

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
		vector:                    vector,
	}, nil
}

// CreateVectorScorer returns a VectorScorer for the leaf segment.
//
// Mirrors FloatVectorSimilarityQuery.createVectorScorer(LeafReaderContext).
func (q *FloatVectorSimilarityQuery) CreateVectorScorer(ctx *index.LeafReaderContext) (VectorScorer, error) {
	vectorValues, err := ctx.LeafReader().GetFloatVectorValues(q.Field)
	if err != nil {
		return nil, err
	}
	if vectorValues == nil {
		return nil, nil
	}
	return vectorValues.Scorer(q.vector)
}

// ApproximateSearch performs the HNSW approximate search on one leaf segment.
//
// Mirrors FloatVectorSimilarityQuery.approximateSearch(LeafReaderContext,
// AcceptDocs, int, KnnCollectorManager).
//
// PORT NOTE. Java passes getSearchStrategy() to newCollector. Gocene's
// BaseVectorSimilarityQuery does not yet carry the searchStrategy field that
// Lucene 10.5.0's AbstractVectorSimilarityQuery holds, so nil is passed; the
// collector manager this query supplies ignores the argument.
func (q *FloatVectorSimilarityQuery) ApproximateSearch(
	ctx *index.LeafReaderContext,
	acceptDocs AcceptDocs,
	visitLimit int,
	mgr knn.KnnCollectorManager,
) (*TopDocs, error) {
	collector, err := mgr.NewCollector(visitLimit, nil, ctx)
	if err != nil {
		return nil, err
	}
	bits, err := acceptDocs.Bits()
	if err != nil {
		return nil, err
	}
	if err := ctx.LeafReader().SearchNearestVectorsCollector(q.Field, q.vector, collector, bits); err != nil {
		return nil, err
	}
	return collector.TopDocs(), nil
}

// Vector represents the query vector.
func (q *FloatVectorSimilarityQuery) Vector() []float32 {
	return q.vector
}

// CreateWeight mirrors AbstractVectorSimilarityQuery.createWeight(IndexSearcher,
// ScoreMode, float) (Lucene 10.5.0,
// lucene/core/src/java/org/apache/lucene/search/AbstractVectorSimilarityQuery.java),
// whose anonymous Weight opens with:
//
//	final Weight filterWeight =
//	    filter == null
//	        ? null
//	        : searcher.createWeight(searcher.rewrite(filter), ScoreMode.COMPLETE_NO_SCORES, 1);
//
// Java resolves `this` to the concrete subclass, so the method is declared on
// FloatVectorSimilarityQuery rather than on the shared
// BaseVectorSimilarityQuery, which cannot reach its embedder.
func (q *FloatVectorSimilarityQuery) CreateWeight(searcher *IndexSearcher, scoreMode ScoreMode, boost float32) (Weight, error) {
	var filterWeight Weight
	if q.Filter != nil {
		rewritten, err := searcher.Rewrite(q.Filter)
		if err != nil {
			return nil, err
		}
		filterWeight, err = searcher.CreateWeight(rewritten, COMPLETE_NO_SCORES, 1)
		if err != nil {
			return nil, err
		}
	}
	return CreateVectorSimilarityWeight(q, q.BaseVectorSimilarityQuery, filterWeight, boost)
}

// Equals mirrors FloatVectorSimilarityQuery.equals(Object) (Lucene 10.5.0):
//
//	return sameClassAs(o)
//	    && super.equals(o)
//	    && Arrays.equals(target, ((FloatVectorSimilarityQuery) o).target);
//
// super.equals(o) is AbstractVectorSimilarityQuery.equals, which compares the
// shared query state. NOTE: Lucene 10.5.0's AbstractVectorSimilarityQuery
// carries {field, resultSimilarity, decay, filter, searchStrategy}, whereas
// this port's BaseVectorSimilarityQuery still carries the 10.4 field set
// {field, traversalSimilarity, resultSimilarity, filter}; the comparison
// therefore runs over the fields the port actually declares.
func (q *FloatVectorSimilarityQuery) Equals(other spi.Query) bool {
	o, ok := other.(*FloatVectorSimilarityQuery)
	if !ok {
		return false
	}
	if q.Field != o.Field ||
		q.TraversalSimilarity != o.TraversalSimilarity ||
		q.ResultSimilarity != o.ResultSimilarity {
		return false
	}
	if (q.Filter == nil) != (o.Filter == nil) {
		return false
	}
	if q.Filter != nil && !q.Filter.Equals(o.Filter) {
		return false
	}
	return slices.Equal(q.vector, o.vector)
}

// HashCode mirrors FloatVectorSimilarityQuery.hashCode() (Lucene 10.5.0):
//
//	int result = super.hashCode();
//	result = 31 * result + Arrays.hashCode(target);
//	return result;
//
// super.hashCode() is AbstractVectorSimilarityQuery.hashCode, i.e.
// Objects.hash over the shared query state; see the note on Equals about the
// field set this port declares. Arrays.hashCode is reproduced exactly: seed 1,
// then 31*h + element.
func (q *FloatVectorSimilarityQuery) HashCode() int {
	result := 1
	result = 31*result + stringHash(q.Field)
	result = 31*result + int(q.TraversalSimilarity*1000)
	result = 31*result + int(q.ResultSimilarity*1000)
	if q.Filter != nil {
		result = 31*result + q.Filter.HashCode()
	} else {
		result = 31 * result
	}
	target := 1
	for _, v := range q.vector {
		target = 31*target + int(math.Float32bits(v))
	}
	return 31*result + target
}

// Rewrite mirrors Query.rewrite(IndexSearcher) (Lucene 10.5.0,
// Query.java), whose default body is `return this;`. Neither
// AbstractVectorSimilarityQuery nor FloatVectorSimilarityQuery overrides it.
func (q *FloatVectorSimilarityQuery) Rewrite(_ *IndexSearcher) (Query, error) {
	return q, nil
}

var _ VectorSimilarityQueryImpl = (*FloatVectorSimilarityQuery)(nil)
var _ Query = (*FloatVectorSimilarityQuery)(nil)

// Visit mirrors AbstractVectorSimilarityQuery.visit(QueryVisitor) of Apache
// Lucene 10.5.0 (AbstractVectorSimilarityQuery.java:260). FloatVectorSimilarityQuery.java
// declares no override and inherits that body; the body is written here rather
// than on [BaseVectorSimilarityQuery] because Java's
// AbstractVectorSimilarityQuery.createWeight is concrete (line 146) while this
// port pushed CreateWeight down to each concrete query, so the base type does
// not satisfy [Query] and cannot be handed to VisitLeaf. Java's `this` is the
// concrete query at run time, so the visitor sees the same leaf either way.
func (q *FloatVectorSimilarityQuery) Visit(visitor QueryVisitor) {
	if visitor.AcceptField(q.Field) {
		visitor.VisitLeaf(q)
	}
}
