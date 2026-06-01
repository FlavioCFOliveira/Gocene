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
	"math"

	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// LatLonPointDistanceFeatureQuery is the Go port of Lucene 10.4.0
// org.apache.lucene.document.LatLonPointDistanceFeatureQuery
// (lucene/core/src/java/org/apache/lucene/document/LatLonPointDistanceFeatureQuery.java).
//
// The query scores documents by their proximity to a geographic origin
// (originLat, originLon): the score is
//
//	boost * pivotDistance / (pivotDistance + haversineDistance(origin, doc))
//
// where the haversine distance is computed in meters. Documents that
// lack a value for the field are not matched.
//
// Multi-valued fields select the point closest to origin (Lucene's
// SortedNumericDocValues holds the encoded (lat, lon) tuple per value;
// the selectValue logic picks the smallest sort key).
//
// # Divergence from Lucene
//
// The Java reference lives in the document package (package-private)
// and is reached through LatLonPoint.newDistanceFeatureQuery. In
// Gocene the type lives in the search package and is exported as
// LatLonPointDistanceFeatureQuery; the eventual LatLonPoint factory
// will simply forward to NewLatLonPointDistanceFeatureQuery to avoid
// the document→search import cycle (LatLonPoint lives in document and
// the query family lives in search).
//
// # Doc-values and point-values shapes
//
// The query consumes two narrow, package-local interfaces:
//
//   - [latLonPointDocValues] is the iterator-shaped view of the
//     per-segment encoded (lat, lon) values for the field
//     (AdvanceExact/EncodedValue/NextDoc/...). The encoded value is
//     the 64-bit packed (latBits<<32)|lonBits tuple matching the
//     Lucene SortedNumericDocValues content.
//
//   - [latLonPointDistanceFeaturePointSource] is the visitor-driven
//     point access surface used for the dynamic skipping in
//     setMinCompetitiveScore. It mirrors the parts of
//     org.apache.lucene.index.PointValues that this query actually
//     uses (intersect + a coarse threshold estimator).
//
// Both interfaces are deliberately narrow so the implementation does
// not block on broader PointValues / DocValues plumbing. The
// production default lookup adapts the existing
// index.SortedNumericDocValues / index.PointValues surfaces; tests
// inject in-memory implementations.
type LatLonPointDistanceFeatureQuery struct {
	BaseQuery
	field         string
	originLat     float64
	originLon     float64
	pivotDistance float64

	// testLeafLookup, when non-nil, overrides the production lookup
	// path used by CreateWeight to resolve per-segment doc-values and
	// point-values. It is set exclusively by tests via
	// installTestLeafLookup; production callers must leave it nil.
	testLeafLookup latLonPointDistanceFeatureLeafLookup
}

// Skip-interval constants control how aggressively the scorer samples
// setMinCompetitiveScore calls before recomputing the dynamic skip
// cell. Mirrors the Java DistanceScorer sampling block; both values
// must be powers of two so the (counter & (interval-1)) sampling test
// remains correct.
const (
	latLonPointDistanceFeatureMinSkipInterval = 32
	latLonPointDistanceFeatureMaxSkipInterval = 8192
)

// latLonPointBytesPerDim (4) is reused from lat_lon_point_query.go;
// it mirrors Java's LatLonPoint.BYTES = Integer.BYTES used by the
// visitor's byte-offset math.

// NewLatLonPointDistanceFeatureQuery constructs a
// LatLonPointDistanceFeatureQuery for the named field. originLat /
// originLon are validated against the WGS-84 ranges and pivotDistance
// must be strictly positive (mirrors the Java
// IllegalArgumentException).
func NewLatLonPointDistanceFeatureQuery(field string, originLat, originLon, pivotDistance float64) (*LatLonPointDistanceFeatureQuery, error) {
	if field == "" {
		return nil, errors.New("lat lon point distance feature query: field must not be empty")
	}
	if err := geo.CheckLatitude(originLat); err != nil {
		return nil, fmt.Errorf("lat lon point distance feature query: %w", err)
	}
	if err := geo.CheckLongitude(originLon); err != nil {
		return nil, fmt.Errorf("lat lon point distance feature query: %w", err)
	}
	if pivotDistance <= 0 {
		return nil, fmt.Errorf("lat lon point distance feature query: pivotDistance must be > 0, got %v", pivotDistance)
	}
	return &LatLonPointDistanceFeatureQuery{
		field:         field,
		originLat:     originLat,
		originLon:     originLon,
		pivotDistance: pivotDistance,
	}, nil
}

// Field returns the field name targeted by this query.
func (q *LatLonPointDistanceFeatureQuery) Field() string { return q.field }

// OriginLat returns the origin latitude in decimal degrees.
func (q *LatLonPointDistanceFeatureQuery) OriginLat() float64 { return q.originLat }

// OriginLon returns the origin longitude in decimal degrees.
func (q *LatLonPointDistanceFeatureQuery) OriginLon() float64 { return q.originLon }

// PivotDistance returns the pivot distance in meters.
func (q *LatLonPointDistanceFeatureQuery) PivotDistance() float64 { return q.pivotDistance }

// Rewrite returns this query unchanged; LatLonPointDistanceFeatureQuery
// does not rewrite to a simpler form.
func (q *LatLonPointDistanceFeatureQuery) Rewrite(_ IndexReader) (Query, error) { return q, nil }

// Clone returns a shallow copy of this query.
func (q *LatLonPointDistanceFeatureQuery) Clone() Query {
	c := *q
	return &c
}

// Equals returns true when other is a LatLonPointDistanceFeatureQuery
// with the same field, origin, and pivot. Mirrors the Java equalsTo
// helper exactly: float comparisons by bit-pattern equality (==).
func (q *LatLonPointDistanceFeatureQuery) Equals(other Query) bool {
	o, ok := other.(*LatLonPointDistanceFeatureQuery)
	if !ok || o == nil {
		return false
	}
	return q.field == o.field &&
		q.originLat == o.originLat &&
		q.originLon == o.originLon &&
		q.pivotDistance == o.pivotDistance
}

// HashCode mirrors the Java hashCode: 31-poly over the class hash,
// the field string, and the three float64 fields. Float fields are
// hashed by their IEEE-754 bit pattern (Double.hashCode in Java).
func (q *LatLonPointDistanceFeatureQuery) HashCode() int {
	h := classHashLatLonPointDistanceFeatureQuery
	for _, r := range q.field {
		h = 31*h + int(r)
	}
	h = 31*h + floatHash(q.originLat)
	h = 31*h + floatHash(q.originLon)
	h = 31*h + floatHash(q.pivotDistance)
	return h
}

// String returns a Lucene-style textual representation of the query.
func (q *LatLonPointDistanceFeatureQuery) String() string {
	return fmt.Sprintf(
		"LatLonPointDistanceFeatureQuery(field=%s,originLat=%v,originLon=%v,pivotDistance=%v)",
		q.field, q.originLat, q.originLon, q.pivotDistance,
	)
}

// Visit mirrors the Java Query.visit override: if the visitor accepts
// the field, dispatch a leaf visit for this query.
func (q *LatLonPointDistanceFeatureQuery) Visit(visitor QueryVisitor) {
	if visitor.AcceptField(q.field) {
		visitor.VisitLeaf(q)
	}
}

// CreateWeight returns a [latLonPointDistanceFeatureWeight] that
// resolves the field's per-segment doc-values and point-values lazily
// through the configured leaf lookup. The default lookup uses the
// canonical index.LeafReaderInterface; tests can override it for
// in-memory fixtures via the package-internal installTestLeafLookup
// helper.
func (q *LatLonPointDistanceFeatureQuery) CreateWeight(searcher *IndexSearcher, _ bool, boost float32) (Weight, error) {
	return &latLonPointDistanceFeatureWeight{
		query:      q,
		boost:      boost,
		leafLookup: q.resolveLeafLookup(searcher),
	}, nil
}

// resolveLeafLookup returns the leaf lookup configured for this query.
// When a test has installed a lookup, it is returned as-is. Otherwise
// the default lookup closes over the query's origin so the
// SortedNumericDocValues adapter can apply the selectValue logic.
func (q *LatLonPointDistanceFeatureQuery) resolveLeafLookup(_ *IndexSearcher) latLonPointDistanceFeatureLeafLookup {
	if q.testLeafLookup != nil {
		return q.testLeafLookup
	}
	originLat := q.originLat
	originLon := q.originLon
	return func(ctx *index.LeafReaderContext, field string) (latLonPointDocValues, latLonPointDistanceFeaturePointSource, error) {
		return defaultLatLonPointDistanceFeatureLeafLookup(ctx, field, originLat, originLon)
	}
}

// installTestLeafLookup wires a test-only lookup. Tests call this to
// inject in-memory latLonPointDocValues /
// latLonPointDistanceFeaturePointSource without going through the
// production segment readers.
func (q *LatLonPointDistanceFeatureQuery) installTestLeafLookup(lookup latLonPointDistanceFeatureLeafLookup) {
	q.testLeafLookup = lookup
}

// Ensure LatLonPointDistanceFeatureQuery implements Query.
var _ Query = (*LatLonPointDistanceFeatureQuery)(nil)

// classHashLatLonPointDistanceFeatureQuery is the stable seed used by
// HashCode so two different Query types with identical field/origin/
// pivot triplets do not collide.
const classHashLatLonPointDistanceFeatureQuery = 0x4c4c_4446 // "LLDF"

// floatHash returns the Java Double.hashCode of v: long bits XOR'd
// with their high half, narrowed back to int.
func floatHash(v float64) int {
	bits := math.Float64bits(v)
	return int(int32(bits ^ (bits >> 32)))
}

// latLonPointDistanceFeatureLeafLookup resolves the per-segment
// numeric and point access surfaces for a given LeafReaderContext.
// The default lookup adapts the production
// index.LeafReaderInterface; tests inject an in-memory implementation.
type latLonPointDistanceFeatureLeafLookup func(ctx *index.LeafReaderContext, field string) (latLonPointDocValues, latLonPointDistanceFeaturePointSource, error)

// defaultLatLonPointDistanceFeatureLeafLookup resolves doc-values and
// point-values through the canonical LeafReaderContext path. While
// the production stubs return nil for both, this path remains forward
// compatible: once the segment readers wire real values up they will
// be picked up here without changes to this query.
func defaultLatLonPointDistanceFeatureLeafLookup(ctx *index.LeafReaderContext, field string, originLat, originLon float64) (latLonPointDocValues, latLonPointDistanceFeaturePointSource, error) {
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
	var dv latLonPointDocValues
	if r, ok := leaf.(docValuesReader); ok {
		sorted, err := r.GetSortedNumericDocValues(field)
		if err != nil {
			return nil, nil, err
		}
		if sorted != nil {
			dv = newLatLonPointSortedNumericAdapter(sorted, originLat, originLon)
		}
	}
	var pts latLonPointDistanceFeaturePointSource
	if r, ok := leaf.(pointReader); ok {
		raw, err := r.GetPointValues(field)
		if err != nil {
			return nil, nil, err
		}
		if raw != nil {
			pts = newLatLonPointDistanceFeaturePointSourceFromIndexPointValues(raw)
		}
	}
	return dv, pts, nil
}

// latLonPointDocValues is the iterator-shaped per-segment numeric
// doc-values surface consumed by [latLonPointDistanceFeatureScorer].
// EncodedValue returns the 64-bit packed (latBits<<32)|lonBits tuple
// matching the Lucene SortedNumericDocValues content; the helper
// methods decodeLat/decodeLon convert this back to degrees.
type latLonPointDocValues interface {
	AdvanceExact(doc int) (bool, error)
	EncodedValue() (int64, error)
	DocID() int
	NextDoc() (int, error)
	Advance(target int) (int, error)
	Cost() int64
}

// latLonPointDistanceFeaturePointSource is the narrow point-values
// surface consumed by the scorer. It mirrors the visitor-driven
// Intersect / EstimatePointCountGreaterThanOrEqualTo pair used by the
// Java reference.
type latLonPointDistanceFeaturePointSource interface {
	Intersect(visitor latLonPointDistanceFeaturePointVisitor) error
	EstimatePointCountGreaterThanOrEqualTo(visitor latLonPointDistanceFeaturePointVisitor, threshold int64) bool
}

// latLonPointDistanceFeaturePointVisitor is the visitor contract the
// scorer hands to a latLonPointDistanceFeaturePointSource. It mirrors
// the subset of org.apache.lucene.index.PointValues.IntersectVisitor
// used by the reference query: per-doc and per-(doc, packedValue)
// visits, a Grow hint, and a Compare callback returning the cell
// relation.
type latLonPointDistanceFeaturePointVisitor interface {
	Visit(docID int) error
	VisitWithPackedValue(docID int, packedValue []byte) error
	Grow(count int)
	Compare(minPackedValue, maxPackedValue []byte) latLonPointDistanceFeatureCellRelation
}

// latLonPointDistanceFeatureCellRelation classifies how a BKD cell
// intersects the query range, mirroring
// org.apache.lucene.index.PointValues.Relation.
type latLonPointDistanceFeatureCellRelation int

const (
	// latLonPointDistanceFeatureCellOutsideQuery means the cell lies fully outside the query.
	latLonPointDistanceFeatureCellOutsideQuery latLonPointDistanceFeatureCellRelation = iota
	// latLonPointDistanceFeatureCellInsideQuery means the cell lies fully inside the query.
	latLonPointDistanceFeatureCellInsideQuery
	// latLonPointDistanceFeatureCellCrossesQuery means the cell partially overlaps the query.
	latLonPointDistanceFeatureCellCrossesQuery
)

// latLonPointDistanceFeatureWeight is the Weight returned by
// [LatLonPointDistanceFeatureQuery.CreateWeight]. It produces a
// ScorerSupplier that lazily builds a [latLonPointDistanceFeatureScorer]
// per segment.
type latLonPointDistanceFeatureWeight struct {
	BaseWeight
	query      *LatLonPointDistanceFeatureQuery
	boost      float32
	leafLookup latLonPointDistanceFeatureLeafLookup
}

// GetQuery returns the parent LatLonPointDistanceFeatureQuery.
func (w *latLonPointDistanceFeatureWeight) GetQuery() Query { return w.query }

// IsCacheable returns false: the scorer rewrites its iterator
// dynamically as setMinCompetitiveScore tightens the range, so caching
// would defeat the purpose. Matches the Java override.
func (w *latLonPointDistanceFeatureWeight) IsCacheable(_ *index.LeafReaderContext) bool { return false }

// Count returns -1 to signal that no sub-linear count is available.
func (w *latLonPointDistanceFeatureWeight) Count(_ *index.LeafReaderContext) (int, error) {
	return -1, nil
}

// Matches returns nil; this query does not produce match positions.
func (w *latLonPointDistanceFeatureWeight) Matches(_ *index.LeafReaderContext, _ int) (Matches, error) {
	return nil, nil
}

// Explain mirrors the Java Weight.explain: when the doc has a value
// the score is boost * pivot / (pivot + haversine(origin, doc))).
func (w *latLonPointDistanceFeatureWeight) Explain(ctx *index.LeafReaderContext, doc int) (Explanation, error) {
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
	encoded, err := dv.EncodedValue()
	if err != nil {
		return nil, err
	}
	lat := geo.DecodeLatitude(int32(encoded >> 32))
	lon := geo.DecodeLongitude(int32(encoded & 0xFFFFFFFF))
	distance := util.HaversinMeters(w.query.originLat, w.query.originLon, lat, lon)
	score := computeLatLonDistanceFeatureScore(w.boost, w.query.pivotDistance, distance)

	root := NewExplanation(true, score,
		"Distance score, computed as weight * pivotDistance / (pivotDistance + abs(distance)) from:")
	root.AddDetail(MatchExplanation(w.boost, "weight"))
	root.AddDetail(MatchExplanation(float32(w.query.pivotDistance), "pivotDistance"))
	root.AddDetail(MatchExplanation(float32(w.query.originLat), "originLat"))
	root.AddDetail(MatchExplanation(float32(w.query.originLon), "originLon"))
	root.AddDetail(MatchExplanation(float32(lat), "current lat"))
	root.AddDetail(MatchExplanation(float32(lon), "current lon"))
	root.AddDetail(MatchExplanation(float32(distance), "distance"))
	return root, nil
}

// ScorerSupplier returns a supplier that lazily constructs the
// per-segment scorer. When the segment has no point values for the
// field the supplier is nil, matching the Java early-return.
func (w *latLonPointDistanceFeatureWeight) ScorerSupplier(ctx *index.LeafReaderContext) (ScorerSupplier, error) {
	dv, pts, err := w.leafLookup(ctx, w.query.field)
	if err != nil {
		return nil, err
	}
	if pts == nil {
		// No data on this segment; matches the Java early-return.
		return nil, nil
	}
	if dv == nil {
		return nil, nil
	}
	maxDoc := 0
	if leaf := ctx.LeafReader(); leaf != nil {
		maxDoc = leaf.MaxDoc()
	}
	return &latLonPointDistanceFeatureScorerSupplier{
		weight: w,
		ctx:    ctx,
		dv:     dv,
		pts:    pts,
		maxDoc: maxDoc,
	}, nil
}

// Scorer materializes the supplier with leadCost 0.
func (w *latLonPointDistanceFeatureWeight) Scorer(ctx *index.LeafReaderContext) (Scorer, error) {
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
func (w *latLonPointDistanceFeatureWeight) BulkScorer(ctx *index.LeafReaderContext) (BulkScorer, error) {
	scorer, err := w.Scorer(ctx)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return NewDefaultBulkScorer(scorer), nil
}

// Ensure latLonPointDistanceFeatureWeight implements Weight.
var _ Weight = (*latLonPointDistanceFeatureWeight)(nil)

// latLonPointDistanceFeatureScorerSupplier produces a per-segment scorer.
type latLonPointDistanceFeatureScorerSupplier struct {
	BaseScorerSupplier
	weight *latLonPointDistanceFeatureWeight
	ctx    *index.LeafReaderContext
	dv     latLonPointDocValues
	pts    latLonPointDistanceFeaturePointSource
	maxDoc int
}

// Get returns a [latLonPointDistanceFeatureScorer] configured for the
// given leadCost.
func (s *latLonPointDistanceFeatureScorerSupplier) Get(leadCost int64) (Scorer, error) {
	return newLatLonPointDistanceFeatureScorer(s.weight, s.dv, s.pts, s.maxDoc, leadCost), nil
}

// Cost returns the doc-values cost, mirroring the Java supplier.cost().
func (s *latLonPointDistanceFeatureScorerSupplier) Cost() int64 {
	if s.dv == nil {
		return 0
	}
	return s.dv.Cost()
}

// SetTopLevelScoringClause is a no-op.
func (s *latLonPointDistanceFeatureScorerSupplier) SetTopLevelScoringClause() {}

// Ensure latLonPointDistanceFeatureScorerSupplier implements ScorerSupplier.
var _ ScorerSupplier = (*latLonPointDistanceFeatureScorerSupplier)(nil)

// latLonPointDistanceFeatureScorer scores docs by their haversine
// distance to origin and dynamically tightens the underlying iterator
// when setMinCompetitiveScore is called. Mirrors the Java
// DistanceScorer inner class.
type latLonPointDistanceFeatureScorer struct {
	weight        *latLonPointDistanceFeatureWeight
	dv            latLonPointDocValues
	pts           latLonPointDistanceFeaturePointSource
	boost         float32
	pivotDistance float64
	originLat     float64
	originLon     float64
	maxDoc        int
	leadCost      int64

	// it is the current iterator the scorer hands out. It starts at
	// the doc-values iterator and may be replaced by a tighter one
	// after setMinCompetitiveScore rewrites the skip range.
	it  DocIdSetIterator
	doc int

	// maxDistance is the current upper bound (in meters) on the
	// distance of a competitive document. Initialised at the Java
	// reference cap (EARTH_MEAN_RADIUS * pi).
	maxDistance                  float64
	currentSkipInterval          int
	tryUpdateFailCount           int
	setMinCompetitiveScoreCount  int
	scorerIteratorWrapperPointer *latLonPointDistanceFeatureIteratorWrapper
}

// newLatLonPointDistanceFeatureScorer builds a scorer with doc-values
// as the initial iterator, mirroring the Java constructor.
func newLatLonPointDistanceFeatureScorer(w *latLonPointDistanceFeatureWeight, dv latLonPointDocValues, pts latLonPointDistanceFeaturePointSource, maxDoc int, leadCost int64) *latLonPointDistanceFeatureScorer {
	s := &latLonPointDistanceFeatureScorer{
		weight:              w,
		dv:                  dv,
		pts:                 pts,
		boost:               w.boost,
		pivotDistance:       w.query.pivotDistance,
		originLat:           w.query.originLat,
		originLon:           w.query.originLon,
		maxDoc:              maxDoc,
		leadCost:            leadCost,
		doc:                 -1,
		maxDistance:         geo.EarthMeanRadiusMeters * math.Pi,
		currentSkipInterval: latLonPointDistanceFeatureMinSkipInterval,
		it:                  newLatLonPointDocValuesIteratorAdapter(dv),
	}
	s.scorerIteratorWrapperPointer = &latLonPointDistanceFeatureIteratorWrapper{owner: s}
	return s
}

// DocID returns the current document id.
func (s *latLonPointDistanceFeatureScorer) DocID() int { return s.doc }

// Score returns the per-doc score computed from the current value.
// Mirrors Java's score(): zero when the doc has no value, otherwise
// boost * pivot / (pivot + haversine(origin, lat, lon)).
func (s *latLonPointDistanceFeatureScorer) Score() float32 {
	ok, err := s.dv.AdvanceExact(s.doc)
	if err != nil || !ok {
		return 0
	}
	encoded, err := s.dv.EncodedValue()
	if err != nil {
		return 0
	}
	lat := geo.DecodeLatitude(int32(encoded >> 32))
	lon := geo.DecodeLongitude(int32(encoded & 0xFFFFFFFF))
	distance := util.HaversinMeters(s.originLat, s.originLon, lat, lon)
	return computeLatLonDistanceFeatureScore(s.boost, s.pivotDistance, distance)
}

// GetMaxScore returns boost: the score reaches its maximum when the
// distance is zero, which evaluates to boost * pivot / pivot = boost.
func (s *latLonPointDistanceFeatureScorer) GetMaxScore(_ int) float32 { return s.boost }

// AdvanceShallow returns NO_MORE_DOCS, the default defined by
// org.apache.lucene.search.Scorer#advanceShallow. Lucene's distance-feature
// scorer does not override advanceShallow either, so the whole remaining
// postings list is treated as a single block.
func (s *latLonPointDistanceFeatureScorer) AdvanceShallow(target int) (int, error) {
	return NO_MORE_DOCS, nil
}

// Cost returns the cost of the current iterator.
func (s *latLonPointDistanceFeatureScorer) Cost() int64 { return s.it.Cost() }

// DocIDRunEnd defers to the underlying iterator.
func (s *latLonPointDistanceFeatureScorer) DocIDRunEnd() int { return s.it.DocIDRunEnd() }

// NextDoc and Advance go through the iterator wrapper so the scorer
// always observes the latest skip iterator after setMinCompetitiveScore
// replaces it.
func (s *latLonPointDistanceFeatureScorer) NextDoc() (int, error) {
	return s.scorerIteratorWrapperPointer.NextDoc()
}

// Advance defers to the iterator wrapper.
func (s *latLonPointDistanceFeatureScorer) Advance(target int) (int, error) {
	return s.scorerIteratorWrapperPointer.Advance(target)
}

// Iterator returns the iterator wrapper so callers see iterator
// replacements made by setMinCompetitiveScore. Mirrors Java's
// iterator() override on DistanceScorer.
func (s *latLonPointDistanceFeatureScorer) Iterator() DocIdSetIterator {
	return s.scorerIteratorWrapperPointer
}

// SetMinCompetitiveScore implements the dynamic skip logic from the
// Java DistanceScorer.setMinCompetitiveScore. It samples invocations
// after the first 256 calls and recomputes maxDistance via binary
// search; when the new range is selective enough relative to leadCost,
// it intersects the point values to materialize a fresh iterator.
func (s *latLonPointDistanceFeatureScorer) SetMinCompetitiveScore(minScore float32) error {
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

	if s.maxDoc <= 0 || s.pts == nil {
		s.updateSkipInterval(false)
		return nil
	}

	// Approximate the disk with the WGS-84 bounding rectangle (Java
	// uses Rectangle.fromPointDistance for the same reason: a true
	// distance query is too expensive to materialize on every skip).
	box, err := geo.FromPointDistance(s.originLat, s.originLon, s.maxDistance)
	if err != nil {
		// Could not derive a tighter range; keep doc-values.
		s.updateSkipInterval(false)
		return nil
	}
	minLat := geo.EncodeLatitude(box.MinLat())
	maxLat := geo.EncodeLatitude(box.MaxLat())
	minLon := geo.EncodeLongitude(box.MinLon())
	maxLon := geo.EncodeLongitude(box.MaxLon())
	crossDateLine := box.CrossesDateline()

	builder := util.NewDocIdSetBuilder(s.maxDoc)
	visitor := &latLonPointDistanceFeaturePointVisitorImpl{
		minLat:        minLat,
		maxLat:        maxLat,
		minLon:        minLon,
		maxLon:        maxLon,
		crossDateLine: crossDateLine,
		result:        builder,
		alreadyAt:     s.doc,
	}

	currentQueryCost := latLonPointDistanceFeatureMinCost(s.leadCost, s.it.Cost())
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
	s.it = newLatLonPointDocIdSetIteratorAdapter(set.Iterator())
	s.updateSkipInterval(true)
	return nil
}

// computeMaxDistance binary-searches for the largest distance whose
// score is still >= minScore. Mirrors the Java helper of the same
// name: the binary search converges to within 1 meter (the Java limit).
func (s *latLonPointDistanceFeatureScorer) computeMaxDistance(minScore float32, previousMaxDistance float64) float64 {
	if computeLatLonDistanceFeatureScore(s.boost, s.pivotDistance, previousMaxDistance) >= minScore {
		return previousMaxDistance
	}
	minD := 0.0
	maxD := previousMaxDistance
	for maxD-minD > 1 {
		mid := (minD + maxD) / 2
		if computeLatLonDistanceFeatureScore(s.boost, s.pivotDistance, mid) >= minScore {
			minD = mid
		} else {
			maxD = mid
		}
	}
	return minD
}

// updateSkipInterval grows/shrinks the sampling interval based on
// whether the last setMinCompetitiveScore call produced a tighter
// iterator. Mirrors the Java updateSkipInterval helper.
func (s *latLonPointDistanceFeatureScorer) updateSkipInterval(success bool) {
	if s.setMinCompetitiveScoreCount <= 256 {
		return
	}
	if success {
		if s.currentSkipInterval/2 > latLonPointDistanceFeatureMinSkipInterval {
			s.currentSkipInterval /= 2
		} else {
			s.currentSkipInterval = latLonPointDistanceFeatureMinSkipInterval
		}
		s.tryUpdateFailCount = 0
		return
	}
	if s.tryUpdateFailCount >= 3 {
		if s.currentSkipInterval*2 < latLonPointDistanceFeatureMaxSkipInterval {
			s.currentSkipInterval *= 2
		} else {
			s.currentSkipInterval = latLonPointDistanceFeatureMaxSkipInterval
		}
		s.tryUpdateFailCount = 0
		return
	}
	s.tryUpdateFailCount++
}

// SmoothingScore returns 0; this scorer does not produce smoothing.
func (s *latLonPointDistanceFeatureScorer) SmoothingScore(_ int) (float32, error) { return 0, nil }

// GetChildren returns no children.
func (s *latLonPointDistanceFeatureScorer) GetChildren() ([]ChildScorable, error) { return nil, nil }

// Ensure latLonPointDistanceFeatureScorer implements Scorer.
var _ Scorer = (*latLonPointDistanceFeatureScorer)(nil)

// latLonPointDistanceFeatureIteratorWrapper is the indirection the
// scorer returns from Iterator(): it always reads s.it, so iterator
// replacements driven by setMinCompetitiveScore are visible.
type latLonPointDistanceFeatureIteratorWrapper struct {
	owner *latLonPointDistanceFeatureScorer
}

// NextDoc advances the wrapped iterator and stores the new doc id on
// the owning scorer.
func (w *latLonPointDistanceFeatureIteratorWrapper) NextDoc() (int, error) {
	doc, err := w.owner.it.NextDoc()
	if err != nil {
		return 0, err
	}
	w.owner.doc = doc
	return doc, nil
}

// DocID returns the owner's current doc id.
func (w *latLonPointDistanceFeatureIteratorWrapper) DocID() int { return w.owner.doc }

// Advance jumps to the given target on the wrapped iterator.
func (w *latLonPointDistanceFeatureIteratorWrapper) Advance(target int) (int, error) {
	doc, err := w.owner.it.Advance(target)
	if err != nil {
		return 0, err
	}
	w.owner.doc = doc
	return doc, nil
}

// Cost defers to the wrapped iterator.
func (w *latLonPointDistanceFeatureIteratorWrapper) Cost() int64 { return w.owner.it.Cost() }

// DocIDRunEnd defers to the wrapped iterator.
func (w *latLonPointDistanceFeatureIteratorWrapper) DocIDRunEnd() int {
	return w.owner.it.DocIDRunEnd()
}

// Ensure latLonPointDistanceFeatureIteratorWrapper implements DocIdSetIterator.
var _ DocIdSetIterator = (*latLonPointDistanceFeatureIteratorWrapper)(nil)

// latLonPointDistanceFeaturePointVisitorImpl is the visitor used to
// materialize a new skip range. It mirrors the inner IntersectVisitor
// of the Java DistanceScorer: per-doc and per-(doc, packedValue)
// visits, a Grow hint, and a Compare callback that classifies cells
// against the [minLat, maxLat] x [minLon, maxLon] rectangle (with the
// dateline split when crossDateLine is true).
type latLonPointDistanceFeaturePointVisitorImpl struct {
	minLat        int32
	maxLat        int32
	minLon        int32
	maxLon        int32
	crossDateLine bool
	result        *util.DocIdSetBuilder
	adder         util.BulkAdder
	alreadyAt     int
}

// Grow installs a BulkAdder sized for the upcoming batch.
func (v *latLonPointDistanceFeaturePointVisitorImpl) Grow(count int) {
	v.adder = v.result.Grow(count)
}

// Visit accepts a docID-only match. Docs <= alreadyAt are dropped to
// mirror Java's "already visited or skipped" guard.
func (v *latLonPointDistanceFeaturePointVisitorImpl) Visit(docID int) error {
	if docID <= v.alreadyAt {
		return nil
	}
	if v.adder == nil {
		v.adder = v.result.Grow(1)
	}
	v.adder.Add(docID)
	return nil
}

// VisitWithPackedValue decodes the (lat, lon) bytes and accepts the
// doc only when the point is inside the rectangle. Mirrors the Java
// visit(int, byte[]).
func (v *latLonPointDistanceFeaturePointVisitorImpl) VisitWithPackedValue(docID int, packedValue []byte) error {
	if docID <= v.alreadyAt {
		return nil
	}
	lat := util.SortableBytesToInt(packedValue, 0)
	if lat > v.maxLat || lat < v.minLat {
		return nil
	}
	lon := util.SortableBytesToInt(packedValue, latLonPointBytesPerDim)
	if v.crossDateLine {
		// On a dateline-crossing query the in-range region is
		// (lon >= minLon) OR (lon <= maxLon); the Java guard rejects
		// the gap (lon < minLon && lon > maxLon).
		if lon < v.minLon && lon > v.maxLon {
			return nil
		}
	} else {
		if lon > v.maxLon || lon < v.minLon {
			return nil
		}
	}
	if v.adder == nil {
		v.adder = v.result.Grow(1)
	}
	v.adder.Add(docID)
	return nil
}

// Compare classifies how the cell intersects the bounding rectangle.
// Mirrors the Java compare(byte[], byte[]) word-for-word, including
// the dateline-aware fast paths.
func (v *latLonPointDistanceFeaturePointVisitorImpl) Compare(minPackedValue, maxPackedValue []byte) latLonPointDistanceFeatureCellRelation {
	latLowerBound := util.SortableBytesToInt(minPackedValue, 0)
	latUpperBound := util.SortableBytesToInt(maxPackedValue, 0)
	if latLowerBound > v.maxLat || latUpperBound < v.minLat {
		return latLonPointDistanceFeatureCellOutsideQuery
	}
	crosses := latLowerBound < v.minLat || latUpperBound > v.maxLat
	lonLowerBound := util.SortableBytesToInt(minPackedValue, latLonPointBytesPerDim)
	lonUpperBound := util.SortableBytesToInt(maxPackedValue, latLonPointBytesPerDim)
	if v.crossDateLine {
		if lonLowerBound > v.maxLon && lonUpperBound < v.minLon {
			return latLonPointDistanceFeatureCellOutsideQuery
		}
		crosses = crosses || lonLowerBound < v.maxLon || lonUpperBound > v.minLon
	} else {
		if lonLowerBound > v.maxLon || lonUpperBound < v.minLon {
			return latLonPointDistanceFeatureCellOutsideQuery
		}
		crosses = crosses || lonLowerBound < v.minLon || lonUpperBound > v.maxLon
	}
	if crosses {
		return latLonPointDistanceFeatureCellCrossesQuery
	}
	return latLonPointDistanceFeatureCellInsideQuery
}

// Ensure latLonPointDistanceFeaturePointVisitorImpl implements latLonPointDistanceFeaturePointVisitor.
var _ latLonPointDistanceFeaturePointVisitor = (*latLonPointDistanceFeaturePointVisitorImpl)(nil)

// Helpers

// computeLatLonDistanceFeatureScore returns boost * pivot / (pivot +
// distance) using float64 division so the rounding semantics mirror
// the Java (float) cast applied to the same expression.
func computeLatLonDistanceFeatureScore(boost float32, pivot, distance float64) float32 {
	if math.IsNaN(distance) || distance < 0 {
		return 0
	}
	return float32(float64(boost) * (pivot / (pivot + distance)))
}

// latLonPointDistanceFeatureMinCost returns min(a, b) on non-negative
// int64 costs.
func latLonPointDistanceFeatureMinCost(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// Adapters

// newLatLonPointDocValuesIteratorAdapter wraps a latLonPointDocValues
// so that it satisfies the search.DocIdSetIterator contract used by
// the initial scorer iterator.
func newLatLonPointDocValuesIteratorAdapter(dv latLonPointDocValues) DocIdSetIterator {
	return &latLonPointDocValuesIteratorAdapter{dv: dv}
}

type latLonPointDocValuesIteratorAdapter struct {
	dv latLonPointDocValues
}

func (a *latLonPointDocValuesIteratorAdapter) DocID() int            { return a.dv.DocID() }
func (a *latLonPointDocValuesIteratorAdapter) NextDoc() (int, error) { return a.dv.NextDoc() }
func (a *latLonPointDocValuesIteratorAdapter) Advance(target int) (int, error) {
	return a.dv.Advance(target)
}
func (a *latLonPointDocValuesIteratorAdapter) Cost() int64      { return a.dv.Cost() }
func (a *latLonPointDocValuesIteratorAdapter) DocIDRunEnd() int { return a.dv.DocID() + 1 }

var _ DocIdSetIterator = (*latLonPointDocValuesIteratorAdapter)(nil)

// newLatLonPointDocIdSetIteratorAdapter bridges a util.DocIdSetIterator
// to the search.DocIdSetIterator contract.
func newLatLonPointDocIdSetIteratorAdapter(inner util.DocIdSetIterator) DocIdSetIterator {
	return &latLonPointDocIdSetIteratorAdapter{inner: inner}
}

type latLonPointDocIdSetIteratorAdapter struct {
	inner util.DocIdSetIterator
}

func (a *latLonPointDocIdSetIteratorAdapter) DocID() int            { return a.inner.DocID() }
func (a *latLonPointDocIdSetIteratorAdapter) NextDoc() (int, error) { return a.inner.NextDoc() }
func (a *latLonPointDocIdSetIteratorAdapter) Advance(target int) (int, error) {
	return a.inner.Advance(target)
}
func (a *latLonPointDocIdSetIteratorAdapter) Cost() int64      { return a.inner.Cost() }
func (a *latLonPointDocIdSetIteratorAdapter) DocIDRunEnd() int { return a.inner.DocIDRunEnd() }

var _ DocIdSetIterator = (*latLonPointDocIdSetIteratorAdapter)(nil)

// newLatLonPointSortedNumericAdapter wraps an
// index.SortedNumericDocValues so it satisfies latLonPointDocValues.
// The existing index.SortedNumericDocValues shape is random-access
// (Get(docID) returns the value slice for a doc), so this adapter
// materialises the value list per AdvanceExact call and applies the
// Java selectValue logic to pick the point with the smallest sort key
// (the haversine sort key shares its ordering with haversine meters,
// avoiding the asin() in the per-doc fast path).
func newLatLonPointSortedNumericAdapter(sorted index.SortedNumericDocValues, originLat, originLon float64) latLonPointDocValues {
	return &latLonPointSortedNumericAdapter{
		sorted:    sorted,
		doc:       -1,
		originLat: originLat,
		originLon: originLon,
	}
}

type latLonPointSortedNumericAdapter struct {
	sorted    index.SortedNumericDocValues
	doc       int
	value     int64
	hasValue  bool
	originLat float64
	originLon float64
}

func (a *latLonPointSortedNumericAdapter) AdvanceExact(doc int) (bool, error) {
	values, err := index.DrainSortedNumeric(a.sorted, doc)
	if err != nil {
		return false, err
	}
	a.doc = doc
	if len(values) == 0 {
		a.hasValue = false
		return false, nil
	}
	a.value = selectClosestLatLonValue(values, a.originLat, a.originLon)
	a.hasValue = true
	return true, nil
}

func (a *latLonPointSortedNumericAdapter) EncodedValue() (int64, error) {
	if !a.hasValue {
		return 0, errors.New("lat lon point distance feature query: EncodedValue() called without AdvanceExact match")
	}
	return a.value, nil
}

func (a *latLonPointSortedNumericAdapter) DocID() int { return a.doc }

func (a *latLonPointSortedNumericAdapter) NextDoc() (int, error) {
	doc, err := a.sorted.NextDoc()
	if err != nil {
		return 0, err
	}
	a.doc = doc
	if doc != NO_MORE_DOCS {
		// a.sorted is positioned on doc; CollectSortedNumericValues drains
		// the per-doc values via DocValueCount + NextValue.
		values, err := index.CollectSortedNumericValues(a.sorted)
		if err != nil {
			return 0, err
		}
		if len(values) > 0 {
			a.value = selectClosestLatLonValue(values, a.originLat, a.originLon)
			a.hasValue = true
		} else {
			a.hasValue = false
		}
	}
	return doc, nil
}

func (a *latLonPointSortedNumericAdapter) Advance(target int) (int, error) {
	doc, err := a.sorted.Advance(target)
	if err != nil {
		return 0, err
	}
	a.doc = doc
	if doc != NO_MORE_DOCS {
		values, err := index.CollectSortedNumericValues(a.sorted)
		if err != nil {
			return 0, err
		}
		if len(values) > 0 {
			a.value = selectClosestLatLonValue(values, a.originLat, a.originLon)
			a.hasValue = true
		} else {
			a.hasValue = false
		}
	}
	return doc, nil
}

func (a *latLonPointSortedNumericAdapter) Cost() int64 { return 0 }

// selectClosestLatLonValue mirrors the Java selectValue helper on
// DistanceScorer: it returns the encoded (lat, lon) tuple with the
// smallest haversine sort key (a strictly increasing transform of
// haversine distance, so picking by sort key picks the closest
// point).
func selectClosestLatLonValue(vs []int64, originLat, originLon float64) int64 {
	value := vs[0]
	if len(vs) == 1 {
		return value
	}
	bestKey := haversinSortKeyFromEncoded(value, originLat, originLon)
	for i := 1; i < len(vs); i++ {
		next := vs[i]
		k := haversinSortKeyFromEncoded(next, originLat, originLon)
		if k < bestKey {
			bestKey = k
			value = next
		}
	}
	return value
}

// haversinSortKeyFromEncoded decodes the (lat, lon) tuple from the
// 64-bit packed encoding and returns the haversine sort key against
// origin.
func haversinSortKeyFromEncoded(encoded int64, originLat, originLon float64) float64 {
	lat := geo.DecodeLatitude(int32(encoded >> 32))
	lon := geo.DecodeLongitude(int32(encoded & 0xFFFFFFFF))
	return util.HaversinSortKey(originLat, originLon, lat, lon)
}

// newLatLonPointDistanceFeaturePointSourceFromIndexPointValues wraps
// an index.PointValues into a
// latLonPointDistanceFeaturePointSource. The existing
// index.PointValues interface only exposes meta accessors and does
// not provide visitor-driven intersection, so the adapter returns a
// source whose Intersect is a no-op and whose estimator always
// reports false. This keeps the production path linkable; the test
// path injects an in-memory source.
func newLatLonPointDistanceFeaturePointSourceFromIndexPointValues(_ index.PointValues) latLonPointDistanceFeaturePointSource {
	return noopLatLonPointDistanceFeaturePointSource{}
}

type noopLatLonPointDistanceFeaturePointSource struct{}

func (noopLatLonPointDistanceFeaturePointSource) Intersect(_ latLonPointDistanceFeaturePointVisitor) error {
	return nil
}

func (noopLatLonPointDistanceFeaturePointSource) EstimatePointCountGreaterThanOrEqualTo(_ latLonPointDistanceFeaturePointVisitor, _ int64) bool {
	return false
}
