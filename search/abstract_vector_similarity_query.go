// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

package search

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/AbstractVectorSimilarityQuery.java

import (
	"fmt"
	"hash/fnv"
	"math"
	"sort"

	hnswutil "github.com/FlavioCFOliveira/Gocene/util/hnsw"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search/knn"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// VectorSimilarityQueryImpl is the interface that concrete subtypes of
// AbstractVectorSimilarityQuery must satisfy.
//
// Mirrors the abstract methods of
// org.apache.lucene.search.AbstractVectorSimilarityQuery (Lucene 10.4.0).
type VectorSimilarityQueryImpl interface {
	Query

	// CreateVectorScorer returns a VectorScorer for the leaf segment, or nil
	// when the field is absent from that segment.
	//
	// Mirrors AbstractVectorSimilarityQuery.createVectorScorer.
	CreateVectorScorer(ctx *index.LeafReaderContext) (VectorScorer, error)

	// ApproximateSearch performs the HNSW approximate search for this query on
	// one leaf segment.
	//
	// Mirrors AbstractVectorSimilarityQuery.approximateSearch.
	ApproximateSearch(
		ctx *index.LeafReaderContext,
		acceptDocs AcceptDocs,
		visitLimit int,
		mgr knn.KnnCollectorManager,
	) (*TopDocs, error)
}

// BaseVectorSimilarityQuery holds the shared state and common logic for all
// vector similarity queries.  Concrete queries embed this struct and satisfy
// VectorSimilarityQueryImpl.
//
// Mirrors the protected fields and concrete methods of
// org.apache.lucene.search.AbstractVectorSimilarityQuery (Lucene 10.4.0).
//
// Deviations from Java:
//   - Java uses an abstract class; Go uses an interface+struct composition.
//   - IndexSearcher.getTimeout() is not yet ported; queryTimeout is always nil.
//   - Filter Weight construction is moved to Rewrite or CreateWeight callers
//     (no IndexSearcher.rewrite / IndexSearcher.createWeight available yet).
type BaseVectorSimilarityQuery struct {
	Field               string
	TraversalSimilarity float32
	ResultSimilarity    float32
	Filter              Query
}

// NewBaseVectorSimilarityQuery validates and builds shared query state.
//
// Mirrors AbstractVectorSimilarityQuery(String, float, float, Query).
func NewBaseVectorSimilarityQuery(
	field string,
	traversalSimilarity, resultSimilarity float32,
	filter Query,
) (*BaseVectorSimilarityQuery, error) {
	if traversalSimilarity > resultSimilarity {
		return nil, fmt.Errorf(
			"traversalSimilarity (%.4g) must be <= resultSimilarity (%.4g)",
			traversalSimilarity, resultSimilarity)
	}
	if field == "" {
		return nil, fmt.Errorf("field must not be empty")
	}
	return &BaseVectorSimilarityQuery{
		Field:               field,
		TraversalSimilarity: traversalSimilarity,
		ResultSimilarity:    resultSimilarity,
		Filter:              filter,
	}, nil
}

// getKnnCollectorManager returns a KnnCollectorManager that creates
// VectorSimilarityCollectors for the given thresholds.
//
// Mirrors AbstractVectorSimilarityQuery.getKnnCollectorManager().
func (q *BaseVectorSimilarityQuery) getKnnCollectorManager() knn.KnnCollectorManager {
	return &vSimilarityCollectorManager{
		traversalSimilarity: q.TraversalSimilarity,
		resultSimilarity:    q.ResultSimilarity,
	}
}

// vSimilarityCollectorManager is the KnnCollectorManager for
// AbstractVectorSimilarityQuery.
type vSimilarityCollectorManager struct {
	traversalSimilarity float32
	resultSimilarity    float32
}

func (m *vSimilarityCollectorManager) NewCollector(
	visitLimit int,
	_ knn.KnnSearchStrategy,
	_ *index.LeafReaderContext,
) (hnswutil.KnnCollector, error) {
	return newVectorSimilarityKnnAdapter(m.traversalSimilarity, m.resultSimilarity, int64(visitLimit)), nil
}

var _ knn.KnnCollectorManager = (*vSimilarityCollectorManager)(nil)

// vectorSimilarityKnnAdapter adapts *VectorSimilarityCollector to satisfy
// hnsw.KnnCollector.
//
// Two structural mismatches require explicit adaption:
//  1. search.KnnSearchStrategy (requires StrategyName()) vs
//     hnsw.KnnSearchStrategy (requires NextVectorsBlock()) are incompatible;
//     GetSearchStrategy returns nil.
//  2. TopDocs() must return *hnsw.TopDocs, not *search.TopDocs; the adapter
//     converts by field.
type vectorSimilarityKnnAdapter struct {
	*VectorSimilarityCollector
}

func newVectorSimilarityKnnAdapter(traversal, result float32, visitLimit int64) *vectorSimilarityKnnAdapter {
	return &vectorSimilarityKnnAdapter{
		VectorSimilarityCollector: NewVectorSimilarityCollector(traversal, result, visitLimit),
	}
}

// GetSearchStrategy returns nil; the hnsw.KnnSearchStrategy hook is not used
// by VectorSimilarityCollector.
func (a *vectorSimilarityKnnAdapter) GetSearchStrategy() hnswutil.KnnSearchStrategy { return nil }

// TopDocs converts *search.TopDocs to *hnsw.TopDocs by mapping fields.
func (a *vectorSimilarityKnnAdapter) TopDocs() *hnswutil.TopDocs {
	src := a.VectorSimilarityCollector.TopDocs()
	if src == nil {
		return hnswutil.NewTopDocs(hnswutil.NewTotalHits(0, hnswutil.EqualTo), nil)
	}
	scoreDocs := make([]*hnswutil.ScoreDoc, len(src.ScoreDocs))
	for i, sd := range src.ScoreDocs {
		scoreDocs[i] = hnswutil.NewScoreDoc(sd.Doc, sd.Score)
	}
	relation := hnswutil.EqualTo
	if src.TotalHits != nil && src.TotalHits.Relation != EQUAL_TO {
		relation = hnswutil.GreaterThanOrEqualTo
	}
	totalHits := hnswutil.NewTotalHits(
		func() int64 {
			if src.TotalHits != nil {
				return src.TotalHits.Value
			}
			return int64(len(scoreDocs))
		}(),
		relation,
	)
	return hnswutil.NewTopDocs(totalHits, scoreDocs)
}

var _ hnswutil.KnnCollector = (*vectorSimilarityKnnAdapter)(nil)

// CreateVectorSimilarityWeight creates the Weight for an
// AbstractVectorSimilarityQuery given a concrete impl.
//
// filterWeight should be pre-built by the caller (e.g. from Rewrite).
// Pass nil when there is no filter.
//
// Mirrors AbstractVectorSimilarityQuery.createWeight.
func CreateVectorSimilarityWeight(
	impl VectorSimilarityQueryImpl,
	base *BaseVectorSimilarityQuery,
	filterWeight Weight,
	boost float32,
) (Weight, error) {
	return &vectorSimilarityWeight{
		BaseWeight:          NewBaseWeight(impl),
		impl:                impl,
		base:                base,
		filterWeight:        filterWeight,
		boost:               boost,
		timeLimitingManager: newSearchTimeLimitingKnnCollectorManager(base.getKnnCollectorManager(), nil),
	}, nil
}

// vectorSimilarityWeight is the Weight for AbstractVectorSimilarityQuery.
type vectorSimilarityWeight struct {
	*BaseWeight
	impl                VectorSimilarityQueryImpl
	base                *BaseVectorSimilarityQuery
	filterWeight        Weight
	boost               float32
	timeLimitingManager knn.KnnCollectorManager
}

func (w *vectorSimilarityWeight) IsCacheable(_ *index.LeafReaderContext) bool { return true }

func (w *vectorSimilarityWeight) Explain(ctx *index.LeafReaderContext, doc int) (Explanation, error) {
	if w.filterWeight != nil {
		filterScorer, err := w.filterWeight.Scorer(ctx)
		if err != nil {
			return nil, err
		}
		if filterScorer == nil {
			return NoMatchExplanation("Doc does not match the filter"), nil
		}
		advanced, err := filterScorer.Advance(doc)
		if err != nil {
			return nil, err
		}
		if advanced > doc {
			return NoMatchExplanation("Doc does not match the filter"), nil
		}
	}

	scorer, err := w.impl.CreateVectorScorer(ctx)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return NoMatchExplanation("Not indexed as the correct vector field"), nil
	}

	it := scorer.Iterator()
	docID, err := it.Advance(doc)
	if err != nil {
		return nil, err
	}
	if docID == doc {
		score, err := scorer.Score()
		if err != nil {
			return nil, err
		}
		if score >= w.base.ResultSimilarity {
			return MatchExplanation(w.boost*score, "Score above threshold"), nil
		}
		return NoMatchExplanation("Score below threshold"), nil
	}
	return NoMatchExplanation("No vector found for doc"), nil
}

func (w *vectorSimilarityWeight) ScorerSupplier(ctx *index.LeafReaderContext) (ScorerSupplier, error) {
	r := ctx.Reader()
	var liveDocs util.Bits
	if lr, ok := r.(interface{ GetLiveDocs() util.Bits }); ok {
		liveDocs = lr.GetLiveDocs()
	}
	maxDoc := r.MaxDoc()

	if w.filterWeight == nil {
		// No filter — exhaustive approximate search.
		results, err := w.impl.ApproximateSearch(
			ctx,
			AcceptDocsFromLiveDocs(liveDocs, maxDoc),
			math.MaxInt32,
			w.timeLimitingManager,
		)
		if err != nil {
			return nil, err
		}
		return vSimilarityScorerSupplierFromScoreDocs(w.boost, results.ScoreDocs), nil
	}

	// With filter: build acceptDocs from filter scorer.
	acceptDocs := AcceptDocsFromIteratorSupplier(
		func() (DocIdSetIterator, error) {
			sc, err := w.filterWeight.Scorer(ctx)
			if err != nil {
				return nil, err
			}
			if sc == nil {
				return NewEmptyDocIdSetIterator(), nil
			}
			return sc, nil
		},
		liveDocs,
		maxDoc,
	)
	cardinality, err := acceptDocs.Cost()
	if err != nil {
		return nil, err
	}
	if cardinality == 0 {
		return nil, nil
	}

	results, err := w.impl.ApproximateSearch(ctx, acceptDocs, cardinality, w.timeLimitingManager)
	if err != nil {
		return nil, err
	}

	if results.TotalHits != nil && results.TotalHits.Relation == EQUAL_TO {
		return vSimilarityScorerSupplierFromScoreDocs(w.boost, results.ScoreDocs), nil
	}

	// Inexact result: lazy scoring against acceptDocs.
	vectorScorer, err := w.impl.CreateVectorScorer(ctx)
	if err != nil {
		return nil, err
	}
	acceptIt, err := acceptDocs.Iterator()
	if err != nil {
		return nil, err
	}
	return vSimilarityScorerSupplierFromAcceptDocs(
		w.boost, vectorScorer, acceptIt, w.base.ResultSimilarity,
	), nil
}

func (w *vectorSimilarityWeight) Scorer(ctx *index.LeafReaderContext) (Scorer, error) {
	ss, err := w.ScorerSupplier(ctx)
	if err != nil || ss == nil {
		return nil, err
	}
	return ss.Get(math.MaxInt64)
}

func (w *vectorSimilarityWeight) BulkScorer(_ *index.LeafReaderContext) (BulkScorer, error) {
	return nil, nil
}

func (w *vectorSimilarityWeight) Count(_ *index.LeafReaderContext) (int, error) { return -1, nil }
func (w *vectorSimilarityWeight) Matches(_ *index.LeafReaderContext, _ int) (Matches, error) {
	return nil, nil
}

// ─── VectorSimilarityScorerSupplier ──────────────────────────────────────────

// vSimilarityScorerSupplier is the ScorerSupplier for AbstractVectorSimilarityQuery.
//
// Mirrors the private inner VectorSimilarityScorerSupplier (Lucene 10.4.0).
type vSimilarityScorerSupplier struct {
	BaseScorerSupplier
	iterator    DocIdSetIterator
	cachedScore *float32
}

// vSimilarityScorerSupplierFromScoreDocs builds a ScorerSupplier from a
// pre-computed ScoreDoc slice (sorted ascending by doc after this call).
//
// Mirrors VectorSimilarityScorerSupplier.fromScoreDocs.
func vSimilarityScorerSupplierFromScoreDocs(boost float32, scoreDocs []*ScoreDoc) ScorerSupplier {
	if len(scoreDocs) == 0 {
		return nil
	}
	sorted := make([]*ScoreDoc, len(scoreDocs))
	copy(sorted, scoreDocs)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Doc < sorted[j].Doc })

	score := float32(0)
	it := &scoreDocDISI{docs: sorted, boost: boost, cached: &score, index: -1}
	it.doc = -1
	return &vSimilarityScorerSupplier{
		BaseScorerSupplier: BaseScorerSupplier{cost: int64(len(sorted))},
		iterator:           it,
		cachedScore:        &score,
	}
}

// vSimilarityScorerSupplierFromAcceptDocs builds a ScorerSupplier that lazily
// evaluates similarity for each accepted document.
//
// Mirrors VectorSimilarityScorerSupplier.fromAcceptDocs.
func vSimilarityScorerSupplierFromAcceptDocs(
	boost float32,
	scorer VectorScorer,
	acceptDocs DocIdSetIterator,
	threshold float32,
) ScorerSupplier {
	if scorer == nil {
		return nil
	}
	score := float32(0)
	vectorIt := scorer.Iterator()
	conj := newConjunctionDISI([]DocIdSetIterator{vectorIt, acceptDocs})
	it := &filteredVectorDISI{
		inner:  conj,
		scorer: scorer,
		boost:  boost,
		cached: &score,
		thresh: threshold,
		doc:    -1,
	}
	return &vSimilarityScorerSupplier{
		BaseScorerSupplier: BaseScorerSupplier{cost: vectorIt.Cost()},
		iterator:           it,
		cachedScore:        &score,
	}
}

func (s *vSimilarityScorerSupplier) Get(_ int64) (Scorer, error) {
	return &vectorSimilarityScorer{iterator: s.iterator, cachedScore: s.cachedScore}, nil
}

// vectorSimilarityScorer is the Scorer returned by vSimilarityScorerSupplier.
type vectorSimilarityScorer struct {
	BaseDocIdSetIterator
	BaseScorer
	iterator    DocIdSetIterator
	cachedScore *float32
}

func (s *vectorSimilarityScorer) DocID() int                 { return s.iterator.DocID() }
func (s *vectorSimilarityScorer) NextDoc() (int, error)      { return s.iterator.NextDoc() }
func (s *vectorSimilarityScorer) Advance(t int) (int, error) { return s.iterator.Advance(t) }
func (s *vectorSimilarityScorer) Cost() int64                { return s.iterator.Cost() }
func (s *vectorSimilarityScorer) DocIDRunEnd() int {
	d := s.iterator.DocID()
	if d >= NO_MORE_DOCS {
		return NO_MORE_DOCS
	}
	return d + 1
}
func (s *vectorSimilarityScorer) Score() float32            { return *s.cachedScore }
func (s *vectorSimilarityScorer) GetMaxScore(_ int) float32 { return float32(math.Inf(1)) }

var _ Scorer = (*vectorSimilarityScorer)(nil)

// scoreDocDISI iterates over a sorted []ScoreDoc.
//
// Mirrors the anonymous DocIdSetIterator in
// VectorSimilarityScorerSupplier.fromScoreDocs.
type scoreDocDISI struct {
	BaseDocIdSetIterator
	docs   []*ScoreDoc
	index  int
	doc    int
	boost  float32
	cached *float32
}

func (it *scoreDocDISI) DocID() int {
	if it.index < 0 {
		return -1
	}
	if it.index >= len(it.docs) {
		return NO_MORE_DOCS
	}
	*it.cached = it.boost * it.docs[it.index].Score
	return it.docs[it.index].Doc
}

func (it *scoreDocDISI) NextDoc() (int, error) {
	it.index++
	return it.DocID(), nil
}

func (it *scoreDocDISI) Advance(target int) (int, error) {
	lo, hi := it.index+1, len(it.docs)
	if lo < 0 {
		lo = 0
	}
	for lo < hi {
		mid := (lo + hi) >> 1
		if it.docs[mid].Doc < target {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	it.index = lo
	return it.DocID(), nil
}

func (it *scoreDocDISI) Cost() int64 { return int64(len(it.docs)) }

func (it *scoreDocDISI) DocIDRunEnd() int {
	id := it.DocID()
	if id >= NO_MORE_DOCS {
		return NO_MORE_DOCS
	}
	return id + 1
}

// filteredVectorDISI iterates over a conjunction, filtering by score threshold.
//
// Mirrors the anonymous FilteredDocIdSetIterator in
// VectorSimilarityScorerSupplier.fromAcceptDocs.
type filteredVectorDISI struct {
	BaseDocIdSetIterator
	inner  DocIdSetIterator
	scorer VectorScorer
	boost  float32
	cached *float32
	thresh float32
	doc    int
}

func (it *filteredVectorDISI) DocID() int { return it.doc }

func (it *filteredVectorDISI) DocIDRunEnd() int {
	if it.doc >= NO_MORE_DOCS {
		return NO_MORE_DOCS
	}
	return it.doc + 1
}

func (it *filteredVectorDISI) NextDoc() (int, error) {
	for {
		d, err := it.inner.NextDoc()
		if err != nil {
			return 0, err
		}
		if d == NO_MORE_DOCS {
			it.doc = NO_MORE_DOCS
			return NO_MORE_DOCS, nil
		}
		score, err := it.scorer.Score()
		if err != nil {
			return 0, err
		}
		*it.cached = score * it.boost
		if score >= it.thresh {
			it.doc = d
			return d, nil
		}
	}
}

func (it *filteredVectorDISI) Advance(target int) (int, error) {
	d, err := it.inner.Advance(target)
	if err != nil {
		return 0, err
	}
	if d == NO_MORE_DOCS {
		it.doc = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	score, err := it.scorer.Score()
	if err != nil {
		return 0, err
	}
	*it.cached = score * it.boost
	if score >= it.thresh {
		it.doc = d
		return d, nil
	}
	return it.NextDoc()
}

func (it *filteredVectorDISI) Cost() int64 { return it.inner.Cost() }

// ─── Query helpers ────────────────────────────────────────────────────────────

// VisitVectorSimilarityQuery implements the visit pattern for concrete queries.
//
// Mirrors AbstractVectorSimilarityQuery.visit(QueryVisitor).
func VisitVectorSimilarityQuery(q Query, field string, visitor QueryVisitor) {
	if visitor.AcceptField(field) {
		visitor.VisitLeaf(q)
	}
}

// VectorSimilarityQueryEquals reports whether the shared fields of two
// concrete query instances are equal.
//
// Mirrors AbstractVectorSimilarityQuery.equals(Object).
func VectorSimilarityQueryEquals(a, b *BaseVectorSimilarityQuery) bool {
	if a == b {
		return true
	}
	return a.Field == b.Field &&
		math.Float32bits(a.TraversalSimilarity) == math.Float32bits(b.TraversalSimilarity) &&
		math.Float32bits(a.ResultSimilarity) == math.Float32bits(b.ResultSimilarity) &&
		filterString(a.Filter) == filterString(b.Filter)
}

// VectorSimilarityQueryHashCode computes a hash for the shared fields.
//
// Mirrors AbstractVectorSimilarityQuery.hashCode().
func VectorSimilarityQueryHashCode(q *BaseVectorSimilarityQuery) uint64 {
	h := fnv.New64a()
	_, _ = fmt.Fprint(h, q.Field)
	_, _ = h.Write(vsqFloat32Bytes(q.TraversalSimilarity))
	_, _ = h.Write(vsqFloat32Bytes(q.ResultSimilarity))
	_, _ = fmt.Fprint(h, filterString(q.Filter))
	return h.Sum64()
}

func vsqFloat32Bytes(v float32) []byte {
	b := math.Float32bits(v)
	return []byte{byte(b), byte(b >> 8), byte(b >> 16), byte(b >> 24)}
}

func filterString(q Query) string {
	if q == nil {
		return "<nil>"
	}
	if s, ok := q.(interface{ String() string }); ok {
		return s.String()
	}
	return fmt.Sprintf("%v", q)
}
