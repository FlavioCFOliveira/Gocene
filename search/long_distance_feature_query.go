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

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// LongDistanceFeatureQuery is the Go port of Lucene 10.4.0
// org.apache.lucene.document.LongDistanceFeatureQuery (lucene/core/src/java/
// org/apache/lucene/document/LongDistanceFeatureQuery.java).
//
// The query scores documents by their proximity to an origin int64 value:
// the score is boost * pivotDistance / (pivotDistance + abs(value - origin)),
// using unsigned distance arithmetic that treats arithmetic underflow as
// the maximum representable distance. Documents that lack a value for the
// field are not matched.
//
// Multi-valued fields select the value closest to origin (unsigned
// distance), matching Lucene's selectValue logic.
//
// # Divergence from Lucene
//
// The Java reference lives in the document package (package-private) and
// is reached through the LongField.newDistanceFeatureQuery factory.
// In Gocene the type lives in the search package and is exported as
// LongDistanceFeatureQuery; the factory is exposed as
// LongFieldNewDistanceFeatureQuery in long_field_factories.go. This
// avoids a document→search import cycle (LongField in document needs
// to construct a query that lives in search).
//
// # Doc-values and point-values shapes
//
// The query consumes two narrow, package-local interfaces:
//
//   - [longDocValues] is the iterator-shaped view of the per-segment
//     numeric values for the field (advanceExact/longValue/...). This
//     matches the Java NumericDocValues iterator contract used by the
//     reference Weight; the existing index.NumericDocValues exposes a
//     random-access Get(docID) instead, so the per-segment wiring lives
//     in a small adapter on the production side.
//
//   - [longPointSource] is the visitor-driven point access surface used
//     for the dynamic skipping in setMinCompetitiveScore. It mirrors the
//     parts of org.apache.lucene.index.PointValues that LongDistanceFeatureQuery
//     actually uses (intersect + a coarse estimate). The full PointValues
//     port (PointTree, Relation, full IntersectVisitor) is being landed
//     in stages across the index/util layers; this query consumes a
//     focused surface so it does not block on that broader work.
type LongDistanceFeatureQuery struct {
	BaseQuery
	field         string
	origin        int64
	pivotDistance int64

	// testLeafLookup, when non-nil, overrides the production lookup
	// path used by CreateWeight to resolve per-segment doc-values and
	// point-values. It is set exclusively by tests via
	// installTestLeafLookup; production callers must leave it nil.
	testLeafLookup longDistanceLeafLookup
}

// Skip-interval constants control how aggressively the scorer samples
// setMinCompetitiveScore calls before recomputing the dynamic skip cell.
// Mirrors the MIN_SKIP_INTERVAL / MAX_SKIP_INTERVAL constants on the
// Java class; both must be powers of two so the (counter & (interval-1))
// sampling test in setMinCompetitiveScore is correct.
const (
	longDistanceFeatureMinSkipInterval = 32
	longDistanceFeatureMaxSkipInterval = 8192
)

// NewLongDistanceFeatureQuery constructs a LongDistanceFeatureQuery for
// the named field. origin is the reference value and pivotDistance is
// the distance at which the score is half of boost. pivotDistance must
// be strictly positive; the constructor returns an error otherwise,
// mirroring the Java IllegalArgumentException.
func NewLongDistanceFeatureQuery(field string, origin, pivotDistance int64) (*LongDistanceFeatureQuery, error) {
	if field == "" {
		return nil, errors.New("long distance feature query: field must not be empty")
	}
	if pivotDistance <= 0 {
		return nil, fmt.Errorf("long distance feature query: pivotDistance must be > 0, got %d", pivotDistance)
	}
	return &LongDistanceFeatureQuery{
		field:         field,
		origin:        origin,
		pivotDistance: pivotDistance,
	}, nil
}

// Field returns the field name targeted by this query.
func (q *LongDistanceFeatureQuery) Field() string { return q.field }

// Origin returns the origin int64 value.
func (q *LongDistanceFeatureQuery) Origin() int64 { return q.origin }

// PivotDistance returns the pivot distance.
func (q *LongDistanceFeatureQuery) PivotDistance() int64 { return q.pivotDistance }

// Rewrite returns this query unchanged; LongDistanceFeatureQuery does
// not rewrite to a simpler form.
func (q *LongDistanceFeatureQuery) Rewrite(_ IndexReader) (Query, error) { return q, nil }

// Clone returns a shallow copy of this query.
func (q *LongDistanceFeatureQuery) Clone() Query {
	c := *q
	return &c
}

// Equals returns true when other is a LongDistanceFeatureQuery with the
// same field, origin, and pivotDistance.
func (q *LongDistanceFeatureQuery) Equals(other Query) bool {
	o, ok := other.(*LongDistanceFeatureQuery)
	if !ok || o == nil {
		return false
	}
	return q.field == o.field && q.origin == o.origin && q.pivotDistance == o.pivotDistance
}

// HashCode mirrors the Java hashCode: 31-poly over the class hash, the
// field string, and the two int64 fields. The class hash uses the
// fully-qualified Java name so that two different Query types with the
// same field/origin/pivot do not collide.
func (q *LongDistanceFeatureQuery) HashCode() int {
	const classHash = 0x4c44_4651 // "LDFQ", arbitrary stable seed
	h := classHash
	for _, r := range q.field {
		h = 31*h + int(r)
	}
	h = 31*h + int(q.origin) + int(q.origin>>32)
	h = 31*h + int(q.pivotDistance) + int(q.pivotDistance>>32)
	return h
}

// String returns a Lucene-style textual representation of the query.
func (q *LongDistanceFeatureQuery) String() string {
	return fmt.Sprintf("LongDistanceFeatureQuery(field=%s,origin=%d,pivotDistance=%d)",
		q.field, q.origin, q.pivotDistance)
}

// Visit mirrors the Java Query.visit override: if the visitor accepts
// the field, dispatch a leaf visit for this query.
func (q *LongDistanceFeatureQuery) Visit(visitor QueryVisitor) {
	if visitor.AcceptField(q.field) {
		visitor.VisitLeaf(q)
	}
}

// CreateWeight returns a [longDistanceFeatureWeight] that resolves the
// field's per-segment doc-values and point-values lazily through the
// configured leafLookup. The default leafLookup uses the canonical
// index.LeafReaderInterface; tests can override it for in-memory
// fixtures via the package-internal [WithLongDistanceFeatureLeafLookup]
// helper.
func (q *LongDistanceFeatureQuery) CreateWeight(searcher *IndexSearcher, _ bool, boost float32) (Weight, error) {
	return &longDistanceFeatureWeight{
		query:      q,
		boost:      boost,
		leafLookup: q.resolveLeafLookup(searcher),
	}, nil
}

// resolveLeafLookup returns the leaf lookup configured for this query.
// When a test has installed a lookup, it is returned as-is. Otherwise
// the default lookup closes over the query's origin so that the
// SortedNumericDocValues adapter can apply the selectValue logic.
func (q *LongDistanceFeatureQuery) resolveLeafLookup(_ *IndexSearcher) longDistanceLeafLookup {
	if q.testLeafLookup != nil {
		return q.testLeafLookup
	}
	origin := q.origin
	return func(ctx *index.LeafReaderContext, field string) (longDocValues, longPointSource, error) {
		return defaultLongDistanceLeafLookup(ctx, field, origin)
	}
}

// installTestLeafLookup wires a test-only lookup. The package-private
// helper [WithLongDistanceFeatureLeafLookup] in the test build calls
// this to inject in-memory longDocValues / longPointSource without
// going through the (currently stub) production segment readers.
func (q *LongDistanceFeatureQuery) installTestLeafLookup(lookup longDistanceLeafLookup) {
	q.testLeafLookup = lookup
}

// Ensure LongDistanceFeatureQuery implements Query.
var _ Query = (*LongDistanceFeatureQuery)(nil)

// longDistanceLeafLookup resolves the per-segment numeric and point
// access surfaces for a given LeafReaderContext. The default lookup
// adapts the production index.LeafReaderInterface; tests inject an
// in-memory implementation.
type longDistanceLeafLookup func(ctx *index.LeafReaderContext, field string) (longDocValues, longPointSource, error)

// defaultLongDistanceLeafLookup resolves doc-values and point-values
// through the canonical LeafReaderContext path. While the existing
// production stubs return nil for both, this path remains forward
// compatible: once the segment readers wire real values up they will
// be picked up here without changing this query. origin is used by
// the multi-valued adapter to select the value closest to the query
// origin (Lucene's selectValue logic).
func defaultLongDistanceLeafLookup(ctx *index.LeafReaderContext, field string, origin int64) (longDocValues, longPointSource, error) {
	if ctx == nil {
		return nil, nil, nil
	}
	leaf := ctx.LeafReader()
	if leaf == nil {
		return nil, nil, nil
	}
	type docValuesReader interface {
		GetSortedNumericDocValues(field string) (index.SortedNumericDocValues, error)
	}
	type pointReader interface {
		GetPointValues(field string) (index.PointValues, error)
	}
	var dv longDocValues
	if r, ok := leaf.(docValuesReader); ok {
		sorted, err := r.GetSortedNumericDocValues(field)
		if err != nil {
			return nil, nil, err
		}
		if sorted != nil {
			dv = newAdapterFromSortedNumeric(sorted, origin)
		}
	}
	var pts longPointSource
	if r, ok := leaf.(pointReader); ok {
		raw, err := r.GetPointValues(field)
		if err != nil {
			return nil, nil, err
		}
		if raw != nil {
			pts = newAdapterFromIndexPointValues(raw)
		}
	}
	return dv, pts, nil
}

// longDocValues is the iterator-shaped per-segment numeric doc-values
// surface consumed by [longDistanceFeatureScorer]. It mirrors the subset
// of org.apache.lucene.index.NumericDocValues that the reference query
// uses. The existing index.NumericDocValues exposes a random-access
// shape; the production wiring layer is responsible for adapting.
type longDocValues interface {
	// AdvanceExact positions the iterator at doc and returns true when
	// there is a value for that doc.
	AdvanceExact(doc int) (bool, error)
	// LongValue returns the value at the current position; only valid
	// after AdvanceExact returned true.
	LongValue() (int64, error)
	// DocID returns the current doc id, or -1 before iteration.
	DocID() int
	// NextDoc returns the next doc that has a value, or NO_MORE_DOCS.
	NextDoc() (int, error)
	// Advance jumps to the first doc >= target that has a value, or
	// NO_MORE_DOCS.
	Advance(target int) (int, error)
	// Cost returns an estimate of the number of docs this iterator will
	// emit.
	Cost() int64
}

// longPointSource is the narrow point-values surface consumed by the
// scorer. It mirrors the visitor-driven Intersect / EstimatePointCount
// pair used by the Java reference. The full PointValues port is being
// landed in stages; the query holds this focused surface so the impl
// does not block on broader interface work.
type longPointSource interface {
	// Intersect walks the BKD tree, invoking the visitor's Visit hooks
	// for matching docs and using Compare to prune cells.
	Intersect(visitor longPointVisitor) error
	// EstimatePointCountGreaterThanOrEqualTo returns true when the
	// number of points the visitor would receive is at least threshold.
	// Used by the scorer to decide whether a new skip range is worth
	// materializing.
	EstimatePointCountGreaterThanOrEqualTo(visitor longPointVisitor, threshold int64) bool
}

// longPointVisitor is the visitor contract the scorer hands to a
// longPointSource. It mirrors the subset of
// org.apache.lucene.index.PointValues.IntersectVisitor used by the
// reference query: per-doc and per-(doc, packedValue) visits, a Grow
// hint, and a Compare callback that returns the cell relation.
type longPointVisitor interface {
	Visit(docID int) error
	VisitWithPackedValue(docID int, packedValue []byte) error
	Grow(count int)
	Compare(minPackedValue, maxPackedValue []byte) longPointCellRelation
}

// longPointCellRelation classifies how a BKD cell intersects the query
// range, mirroring org.apache.lucene.index.PointValues.Relation.
type longPointCellRelation int

const (
	// longPointCellOutsideQuery means the cell lies fully outside the query.
	longPointCellOutsideQuery longPointCellRelation = iota
	// longPointCellInsideQuery means the cell lies fully inside the query.
	longPointCellInsideQuery
	// longPointCellCrossesQuery means the cell partially overlaps the query.
	longPointCellCrossesQuery
)

// longDistanceFeatureWeight is the Weight returned by
// [LongDistanceFeatureQuery.CreateWeight]. It produces a ScorerSupplier
// that lazily builds a [longDistanceFeatureScorer] per segment.
type longDistanceFeatureWeight struct {
	BaseWeight
	query      *LongDistanceFeatureQuery
	boost      float32
	leafLookup longDistanceLeafLookup
}

// GetQuery returns the parent LongDistanceFeatureQuery.
func (w *longDistanceFeatureWeight) GetQuery() Query { return w.query }

// IsCacheable returns false: the scorer rewrites its iterator
// dynamically as setMinCompetitiveScore tightens the range, so caching
// would defeat the purpose. Matches the Java override.
func (w *longDistanceFeatureWeight) IsCacheable(_ *index.LeafReaderContext) bool { return false }

// Count returns -1 to signal that no sub-linear count is available.
func (w *longDistanceFeatureWeight) Count(_ *index.LeafReaderContext) (int, error) { return -1, nil }

// Matches returns nil; this query does not produce match positions.
func (w *longDistanceFeatureWeight) Matches(_ *index.LeafReaderContext, _ int) (Matches, error) {
	return nil, nil
}

// Explain mirrors the Java Weight.explain: when the doc has a value,
// score is boost * pivotDistance / (pivotDistance + abs(value - origin))
// with unsigned-distance underflow capped at MaxInt64.
func (w *longDistanceFeatureWeight) Explain(ctx *index.LeafReaderContext, doc int) (Explanation, error) {
	dv, _, err := w.leafLookup(ctx, w.query.field)
	if err != nil {
		return nil, err
	}
	if dv == nil {
		return NoMatchExplanation(fmt.Sprintf("Document %d doesn't have a value for field %s", doc, w.query.field)), nil
	}
	ok, err := dv.AdvanceExact(doc)
	if err != nil {
		return nil, err
	}
	if !ok {
		return NoMatchExplanation(fmt.Sprintf("Document %d doesn't have a value for field %s", doc, w.query.field)), nil
	}
	value, err := dv.LongValue()
	if err != nil {
		return nil, err
	}
	distance := unsignedDistance(value, w.query.origin)
	score := computeLongDistanceScore(w.boost, w.query.pivotDistance, distance)

	root := NewExplanation(true, score,
		"Distance score, computed as weight * pivotDistance / (pivotDistance + abs(value - origin)) from:")
	root.AddDetail(MatchExplanation(w.boost, "weight"))
	root.AddDetail(MatchExplanation(float32(w.query.pivotDistance), "pivotDistance"))
	root.AddDetail(MatchExplanation(float32(w.query.origin), "origin"))
	root.AddDetail(MatchExplanation(float32(value), "current value"))
	return root, nil
}

// ScorerSupplier returns a supplier that lazily constructs the
// per-segment scorer. When the segment has no point values for the
// field the supplier is nil, matching the Java early-return.
func (w *longDistanceFeatureWeight) ScorerSupplier(ctx *index.LeafReaderContext) (ScorerSupplier, error) {
	dv, pts, err := w.leafLookup(ctx, w.query.field)
	if err != nil {
		return nil, err
	}
	if pts == nil {
		// No data on this segment; matches the Java early-return.
		return nil, nil
	}
	if dv == nil {
		// Without doc values we cannot score; treat as empty segment.
		return nil, nil
	}
	maxDoc := 0
	if leaf := ctx.LeafReader(); leaf != nil {
		maxDoc = leaf.MaxDoc()
	}
	return &longDistanceFeatureScorerSupplier{
		weight: w,
		ctx:    ctx,
		dv:     dv,
		pts:    pts,
		maxDoc: maxDoc,
	}, nil
}

// Scorer materializes the supplier with leadCost 0, matching the
// BaseWeight default semantics.
func (w *longDistanceFeatureWeight) Scorer(ctx *index.LeafReaderContext) (Scorer, error) {
	supplier, err := w.ScorerSupplier(ctx)
	if err != nil {
		return nil, err
	}
	if supplier == nil {
		return nil, nil
	}
	return supplier.Get(0)
}

// BulkScorer falls back to wrapping the per-doc Scorer.
func (w *longDistanceFeatureWeight) BulkScorer(ctx *index.LeafReaderContext) (BulkScorer, error) {
	scorer, err := w.Scorer(ctx)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return NewDefaultBulkScorer(scorer), nil
}

// Ensure longDistanceFeatureWeight implements Weight.
var _ Weight = (*longDistanceFeatureWeight)(nil)

// longDistanceFeatureScorerSupplier produces a per-segment scorer.
type longDistanceFeatureScorerSupplier struct {
	BaseScorerSupplier
	weight *longDistanceFeatureWeight
	ctx    *index.LeafReaderContext
	dv     longDocValues
	pts    longPointSource
	maxDoc int
}

// Get returns a [longDistanceFeatureScorer] configured for the given
// leadCost. Mirrors the Java ScorerSupplier.get(leadCost).
func (s *longDistanceFeatureScorerSupplier) Get(leadCost int64) (Scorer, error) {
	return newLongDistanceFeatureScorer(s.weight, s.dv, s.pts, s.maxDoc, leadCost), nil
}

// Cost returns the doc-values cost, mirroring the Java supplier.cost().
func (s *longDistanceFeatureScorerSupplier) Cost() int64 {
	if s.dv == nil {
		return 0
	}
	return s.dv.Cost()
}

// SetTopLevelScoringClause is a no-op.
func (s *longDistanceFeatureScorerSupplier) SetTopLevelScoringClause() {}

// Ensure longDistanceFeatureScorerSupplier implements ScorerSupplier.
var _ ScorerSupplier = (*longDistanceFeatureScorerSupplier)(nil)

// longDistanceFeatureScorer scores docs by their distance to origin and
// dynamically tightens the underlying iterator when setMinCompetitiveScore
// is called. Mirrors the Java DistanceScorer inner class.
type longDistanceFeatureScorer struct {
	weight        *longDistanceFeatureWeight
	dv            longDocValues
	pts           longPointSource
	boost         float32
	pivotDistance int64
	origin        int64
	maxDoc        int
	leadCost      int64

	// it is the current iterator the scorer hands out. It starts at
	// the doc-values iterator and may be replaced by a tighter one
	// after setMinCompetitiveScore rewrites the skip range.
	it  DocIdSetIterator
	doc int

	maxDistance                  int64
	currentSkipInterval          int
	tryUpdateFailCount           int
	setMinCompetitiveScoreCount  int
	scorerIteratorWrapperPointer *longDistanceFeatureIteratorWrapper
}

// newLongDistanceFeatureScorer builds a scorer with doc-values as the
// initial iterator, mirroring the Java constructor.
func newLongDistanceFeatureScorer(w *longDistanceFeatureWeight, dv longDocValues, pts longPointSource, maxDoc int, leadCost int64) *longDistanceFeatureScorer {
	s := &longDistanceFeatureScorer{
		weight:              w,
		dv:                  dv,
		pts:                 pts,
		boost:               w.boost,
		pivotDistance:       w.query.pivotDistance,
		origin:              w.query.origin,
		maxDoc:              maxDoc,
		leadCost:            leadCost,
		doc:                 -1,
		maxDistance:         maxInt64,
		currentSkipInterval: longDistanceFeatureMinSkipInterval,
		it:                  newLongDocValuesIteratorAdapter(dv),
	}
	s.scorerIteratorWrapperPointer = &longDistanceFeatureIteratorWrapper{owner: s}
	return s
}

// DocID returns the current document id.
func (s *longDistanceFeatureScorer) DocID() int { return s.doc }

// Score returns the per-doc score computed from the current value.
// Mirrors Java's score(): zero when the doc has no value, otherwise
// boost * pivot / (pivot + unsignedDistance(value, origin)).
func (s *longDistanceFeatureScorer) Score() float32 {
	ok, err := s.dv.AdvanceExact(s.doc)
	if err != nil || !ok {
		return 0
	}
	value, err := s.dv.LongValue()
	if err != nil {
		return 0
	}
	distance := unsignedDistance(value, s.origin)
	return computeLongDistanceScore(s.boost, s.pivotDistance, distance)
}

// GetMaxScore returns boost: the score reaches its maximum when
// distance is zero, which evaluates to boost * pivot / pivot = boost.
func (s *longDistanceFeatureScorer) GetMaxScore(_ int) float32 { return s.boost }

// Cost returns the cost of the current iterator.
func (s *longDistanceFeatureScorer) Cost() int64 { return s.it.Cost() }

// DocIDRunEnd defers to the underlying iterator.
func (s *longDistanceFeatureScorer) DocIDRunEnd() int { return s.it.DocIDRunEnd() }

// NextDoc and Advance go through the iterator wrapper so that the
// scorer always observes the latest skip iterator after
// setMinCompetitiveScore replaces it.
func (s *longDistanceFeatureScorer) NextDoc() (int, error) {
	return s.scorerIteratorWrapperPointer.NextDoc()
}

// Advance defers to the iterator wrapper.
func (s *longDistanceFeatureScorer) Advance(target int) (int, error) {
	return s.scorerIteratorWrapperPointer.Advance(target)
}

// Iterator returns the iterator wrapper so callers see iterator
// replacements made by setMinCompetitiveScore. Mirrors Java's iterator()
// override on DistanceScorer.
func (s *longDistanceFeatureScorer) Iterator() DocIdSetIterator {
	return s.scorerIteratorWrapperPointer
}

// SetMinCompetitiveScore implements the dynamic skip logic from the
// Java DistanceScorer.setMinCompetitiveScore. It samples invocations
// after the first 256 calls and recomputes maxDistance via binary
// search; when the new range is selective enough relative to leadCost,
// it intersects the point values to materialize a fresh iterator.
func (s *longDistanceFeatureScorer) SetMinCompetitiveScore(minScore float32) error {
	if minScore > s.boost {
		s.it = NewEmptyDocIdSetIterator()
		return nil
	}

	s.setMinCompetitiveScoreCount++
	if s.setMinCompetitiveScoreCount > 256 &&
		(s.setMinCompetitiveScoreCount&(s.currentSkipInterval-1)) != s.currentSkipInterval-1 {
		return nil
	}

	previousMaxDistance := s.maxDistance
	s.maxDistance = s.computeMaxDistance(minScore, s.maxDistance)
	if s.maxDistance == previousMaxDistance {
		return nil
	}

	minValue := s.origin - s.maxDistance
	if minValue > s.origin {
		minValue = minInt64
	}
	maxValue := s.origin + s.maxDistance
	if maxValue < s.origin {
		maxValue = maxInt64
	}

	if s.maxDoc <= 0 || s.pts == nil {
		// No point values to drive the new iterator; keep doc-values.
		s.updateSkipInterval(false)
		return nil
	}
	builder := util.NewDocIdSetBuilder(s.maxDoc)
	visitor := &longDistancePointVisitor{
		min:       minValue,
		max:       maxValue,
		result:    builder,
		alreadyAt: s.doc,
	}

	currentQueryCost := minInt64Cost(s.leadCost, s.it.Cost())
	// Java uses unsigned right shift here; cost is non-negative so a
	// plain shift produces the same value.
	threshold := currentQueryCost >> 3

	if s.pts.EstimatePointCountGreaterThanOrEqualTo(visitor, threshold) {
		// New range is not selective enough; do not pay for the intersect.
		s.updateSkipInterval(false)
		return nil
	}
	if err := s.pts.Intersect(visitor); err != nil {
		return err
	}
	set, err := builder.Build()
	if err != nil {
		return err
	}
	s.it = newUtilDocIdSetIteratorAdapter(set.Iterator())
	s.updateSkipInterval(true)
	return nil
}

// computeMaxDistance binary-searches for the largest distance whose
// score is still >= minScore. Mirrors the Java helper of the same name.
func (s *longDistanceFeatureScorer) computeMaxDistance(minScore float32, previousMaxDistance int64) int64 {
	if computeLongDistanceScore(s.boost, s.pivotDistance, previousMaxDistance) >= minScore {
		return previousMaxDistance
	}
	min := int64(0)
	max := previousMaxDistance
	// Invariant: score(min) >= minScore && score(max) < minScore.
	for max-min > 1 {
		mid := int64(uint64(min+max) >> 1)
		if computeLongDistanceScore(s.boost, s.pivotDistance, mid) >= minScore {
			min = mid
		} else {
			max = mid
		}
	}
	return min
}

// updateSkipInterval grows/shrinks the sampling interval based on
// whether the last setMinCompetitiveScore call produced a tighter
// iterator. Mirrors the Java updateSkipInterval helper.
func (s *longDistanceFeatureScorer) updateSkipInterval(success bool) {
	if s.setMinCompetitiveScoreCount <= 256 {
		return
	}
	if success {
		if s.currentSkipInterval/2 > longDistanceFeatureMinSkipInterval {
			s.currentSkipInterval /= 2
		} else {
			s.currentSkipInterval = longDistanceFeatureMinSkipInterval
		}
		s.tryUpdateFailCount = 0
		return
	}
	if s.tryUpdateFailCount >= 3 {
		if s.currentSkipInterval*2 < longDistanceFeatureMaxSkipInterval {
			s.currentSkipInterval *= 2
		} else {
			s.currentSkipInterval = longDistanceFeatureMaxSkipInterval
		}
		s.tryUpdateFailCount = 0
		return
	}
	s.tryUpdateFailCount++
}

// SmoothingScore returns 0; this scorer does not produce smoothing.
func (s *longDistanceFeatureScorer) SmoothingScore(_ int) (float32, error) { return 0, nil }

// GetChildren returns no children.
func (s *longDistanceFeatureScorer) GetChildren() ([]ChildScorable, error) { return nil, nil }

// Ensure longDistanceFeatureScorer implements Scorer.
var _ Scorer = (*longDistanceFeatureScorer)(nil)

// longDistanceFeatureIteratorWrapper is the indirection the scorer
// returns from Iterator(): it always reads s.it, so iterator
// replacements driven by setMinCompetitiveScore are visible.
//
// Mirrors the anonymous DocIdSetIterator returned by the Java scorer.
type longDistanceFeatureIteratorWrapper struct {
	owner *longDistanceFeatureScorer
}

// NextDoc advances the wrapped iterator and stores the new doc id on
// the owning scorer.
func (w *longDistanceFeatureIteratorWrapper) NextDoc() (int, error) {
	doc, err := w.owner.it.NextDoc()
	if err != nil {
		return 0, err
	}
	w.owner.doc = doc
	return doc, nil
}

// DocID returns the owner's current doc id.
func (w *longDistanceFeatureIteratorWrapper) DocID() int { return w.owner.doc }

// Advance jumps to the given target on the wrapped iterator.
func (w *longDistanceFeatureIteratorWrapper) Advance(target int) (int, error) {
	doc, err := w.owner.it.Advance(target)
	if err != nil {
		return 0, err
	}
	w.owner.doc = doc
	return doc, nil
}

// Cost defers to the wrapped iterator.
func (w *longDistanceFeatureIteratorWrapper) Cost() int64 { return w.owner.it.Cost() }

// DocIDRunEnd defers to the wrapped iterator.
func (w *longDistanceFeatureIteratorWrapper) DocIDRunEnd() int { return w.owner.it.DocIDRunEnd() }

// Ensure longDistanceFeatureIteratorWrapper implements DocIdSetIterator.
var _ DocIdSetIterator = (*longDistanceFeatureIteratorWrapper)(nil)

// longDistancePointVisitor is the visitor used to materialize a new
// skip range. It mirrors the inner IntersectVisitor of the Java
// DistanceScorer: per-doc and per-(doc, packedValue) visits, a Grow
// hint, and a Compare callback that classifies cells against the
// [min, max] range.
type longDistancePointVisitor struct {
	min       int64
	max       int64
	result    *util.DocIdSetBuilder
	adder     util.BulkAdder
	alreadyAt int
}

// Grow installs a BulkAdder sized for the upcoming batch.
func (v *longDistancePointVisitor) Grow(count int) {
	v.adder = v.result.Grow(count)
}

// Visit accepts a docID-only match. Docs <= alreadyAt are dropped to
// mirror Java's "already visited or skipped" guard.
func (v *longDistancePointVisitor) Visit(docID int) error {
	if docID <= v.alreadyAt {
		return nil
	}
	if v.adder == nil {
		v.adder = v.result.Grow(1)
	}
	v.adder.Add(docID)
	return nil
}

// VisitWithPackedValue decodes the value and accepts the doc only when
// its value is inside [min, max]. Mirrors the Java visit(int, byte[]).
func (v *longDistancePointVisitor) VisitWithPackedValue(docID int, packedValue []byte) error {
	if docID <= v.alreadyAt {
		return nil
	}
	docValue := util.SortableBytesToLong(packedValue, 0)
	if docValue < v.min || docValue > v.max {
		return nil
	}
	if v.adder == nil {
		v.adder = v.result.Grow(1)
	}
	v.adder.Add(docID)
	return nil
}

// Compare classifies how the cell [minPackedValue, maxPackedValue]
// intersects [min, max]. Mirrors the Java compare(byte[], byte[]).
func (v *longDistancePointVisitor) Compare(minPackedValue, maxPackedValue []byte) longPointCellRelation {
	minDocValue := util.SortableBytesToLong(minPackedValue, 0)
	maxDocValue := util.SortableBytesToLong(maxPackedValue, 0)
	if minDocValue > v.max || maxDocValue < v.min {
		return longPointCellOutsideQuery
	}
	if minDocValue < v.min || maxDocValue > v.max {
		return longPointCellCrossesQuery
	}
	return longPointCellInsideQuery
}

// Ensure longDistancePointVisitor implements longPointVisitor.
var _ longPointVisitor = (*longDistancePointVisitor)(nil)

// Helpers

// unsignedDistance returns abs(a - b) treating the result as unsigned.
// Mirrors Java's "long distance = Math.max(v, origin) - Math.min(v, origin)"
// with the post-condition "if (distance < 0) distance = Long.MAX_VALUE".
func unsignedDistance(a, b int64) int64 {
	if a >= b {
		d := a - b
		if d < 0 {
			return maxInt64
		}
		return d
	}
	d := b - a
	if d < 0 {
		return maxInt64
	}
	return d
}

// computeLongDistanceScore returns boost * pivot / (pivot + distance)
// using float64 division so the rounding semantics mirror the Java
// (float) cast applied to the same expression.
func computeLongDistanceScore(boost float32, pivot, distance int64) float32 {
	if distance < 0 {
		distance = maxInt64
	}
	return float32(float64(boost) * (float64(pivot) / (float64(pivot) + float64(distance))))
}

// minInt64Cost returns min(a, b) on non-negative int64 costs.
func minInt64Cost(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

const (
	maxInt64 int64 = 1<<63 - 1
	minInt64 int64 = -1 << 63
)

// Adapters

// newLongDocValuesIteratorAdapter wraps a longDocValues so that it
// satisfies the search.DocIdSetIterator contract used by the initial
// scorer iterator. The adapter just forwards: longDocValues already
// exposes NextDoc/Advance/DocID/Cost.
func newLongDocValuesIteratorAdapter(dv longDocValues) DocIdSetIterator {
	return &longDocValuesIteratorAdapter{dv: dv}
}

type longDocValuesIteratorAdapter struct {
	dv longDocValues
}

func (a *longDocValuesIteratorAdapter) DocID() int            { return a.dv.DocID() }
func (a *longDocValuesIteratorAdapter) NextDoc() (int, error) { return a.dv.NextDoc() }
func (a *longDocValuesIteratorAdapter) Advance(target int) (int, error) {
	return a.dv.Advance(target)
}
func (a *longDocValuesIteratorAdapter) Cost() int64      { return a.dv.Cost() }
func (a *longDocValuesIteratorAdapter) DocIDRunEnd() int { return a.dv.DocID() + 1 }

var _ DocIdSetIterator = (*longDocValuesIteratorAdapter)(nil)

// newUtilDocIdSetIteratorAdapter bridges a util.DocIdSetIterator to the
// search.DocIdSetIterator contract. Both iterators are structurally
// identical (NextDoc/Advance/DocID/Cost/DocIDRunEnd); only the package
// differs, so the adapter is a thin forwarder.
func newUtilDocIdSetIteratorAdapter(inner util.DocIdSetIterator) DocIdSetIterator {
	return &utilDocIdSetIteratorAdapter{inner: inner}
}

type utilDocIdSetIteratorAdapter struct {
	inner util.DocIdSetIterator
}

func (a *utilDocIdSetIteratorAdapter) DocID() int            { return a.inner.DocID() }
func (a *utilDocIdSetIteratorAdapter) NextDoc() (int, error) { return a.inner.NextDoc() }
func (a *utilDocIdSetIteratorAdapter) Advance(target int) (int, error) {
	return a.inner.Advance(target)
}
func (a *utilDocIdSetIteratorAdapter) Cost() int64      { return a.inner.Cost() }
func (a *utilDocIdSetIteratorAdapter) DocIDRunEnd() int { return a.inner.DocIDRunEnd() }

var _ DocIdSetIterator = (*utilDocIdSetIteratorAdapter)(nil)

// newAdapterFromSortedNumeric wraps an index.SortedNumericDocValues so
// it satisfies longDocValues. The existing index.SortedNumericDocValues
// shape is random-access (Get(docID) returns the full value slice for
// a doc), so this adapter materializes the value list per
// AdvanceExact call and applies the Java selectValue logic to pick
// the value closest to origin.
func newAdapterFromSortedNumeric(sorted index.SortedNumericDocValues, origin int64) longDocValues {
	return &sortedNumericLongAdapter{sorted: sorted, doc: -1, origin: origin}
}

type sortedNumericLongAdapter struct {
	sorted   index.SortedNumericDocValues
	doc      int
	value    int64
	hasValue bool
	origin   int64
}

func (a *sortedNumericLongAdapter) AdvanceExact(doc int) (bool, error) {
	values, err := a.sorted.Get(doc)
	if err != nil {
		return false, err
	}
	a.doc = doc
	if len(values) == 0 {
		a.hasValue = false
		return false, nil
	}
	a.value = selectClosestValue(values, a.origin)
	a.hasValue = true
	return true, nil
}

func (a *sortedNumericLongAdapter) LongValue() (int64, error) {
	if !a.hasValue {
		return 0, errors.New("long distance feature query: LongValue() called without AdvanceExact match")
	}
	return a.value, nil
}

func (a *sortedNumericLongAdapter) DocID() int { return a.doc }

func (a *sortedNumericLongAdapter) NextDoc() (int, error) {
	doc, err := a.sorted.NextDoc()
	if err != nil {
		return 0, err
	}
	a.doc = doc
	if doc != NO_MORE_DOCS {
		values, err := a.sorted.Get(doc)
		if err != nil {
			return 0, err
		}
		if len(values) > 0 {
			a.value = selectClosestValue(values, a.origin)
			a.hasValue = true
		} else {
			a.hasValue = false
		}
	}
	return doc, nil
}

func (a *sortedNumericLongAdapter) Advance(target int) (int, error) {
	doc, err := a.sorted.Advance(target)
	if err != nil {
		return 0, err
	}
	a.doc = doc
	if doc != NO_MORE_DOCS {
		values, err := a.sorted.Get(doc)
		if err != nil {
			return 0, err
		}
		if len(values) > 0 {
			a.value = selectClosestValue(values, a.origin)
			a.hasValue = true
		} else {
			a.hasValue = false
		}
	}
	return doc, nil
}

func (a *sortedNumericLongAdapter) Cost() int64 {
	// SortedNumericDocValues does not currently expose a cost estimate;
	// return a coarse value derived from the doc id space.
	return 0
}

// selectClosestValue returns the value in vs whose unsigned distance to
// origin is the smallest, mirroring the Java selectValue helper of the
// DistanceScorer. The input slice must be sorted (Lucene's
// SortedNumericDocValues invariant).
func selectClosestValue(vs []int64, origin int64) int64 {
	if len(vs) == 1 || vs[0] >= origin {
		return vs[0]
	}
	prev := vs[0]
	for i := 1; i < len(vs); i++ {
		next := vs[i]
		if next >= origin {
			// Unsigned comparison because of potential underflow.
			if uint64(origin-prev) < uint64(next-origin) {
				return prev
			}
			return next
		}
		prev = next
	}
	return prev
}

// newAdapterFromIndexPointValues wraps an index.PointValues into a
// longPointSource. The existing index.PointValues interface only
// exposes meta accessors and does not provide visitor-driven
// intersection, so the adapter returns a source whose Intersect is a
// no-op and whose estimator always reports zero. This keeps the
// production path linkable; the test path injects an in-memory source.
func newAdapterFromIndexPointValues(_ index.PointValues) longPointSource {
	return noopLongPointSource{}
}

type noopLongPointSource struct{}

func (noopLongPointSource) Intersect(_ longPointVisitor) error { return nil }
func (noopLongPointSource) EstimatePointCountGreaterThanOrEqualTo(_ longPointVisitor, _ int64) bool {
	return false
}
