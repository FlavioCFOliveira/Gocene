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
	"strings"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// latLonPointDistanceQuery is the Go port of Apache Lucene 10.4.0
// org.apache.lucene.document.LatLonPointDistanceQuery
// (lucene/core/src/java/org/apache/lucene/document/LatLonPointDistanceQuery.java).
//
// The query matches every document whose indexed [document.LatLonPoint]
// lies within radiusMeters of (latitude, longitude), scored as a
// constant boost. The radius is the great-circle distance computed on
// the WGS-84 ellipsoid via the SloppyMath haversine approximation,
// matching the Java reference.
//
// # Divergences from Lucene
//
//  1. Package: the Java type is final, package-private in
//     org.apache.lucene.document. In Gocene queries live in search/
//     to avoid the search<->document import cycle (queries reference
//     search.Weight/Scorer; document is already imported by search).
//     Same placement as the LatLonPointQuery and XYPointInGeometryQuery
//     ports.
//
//  2. Exposure: the Java class is package-private and reached through
//     the LatLonPoint.newDistanceQuery static factory. In Gocene the
//     type is unexported and the constructor [NewLatLonPointDistanceQuery]
//     is exported so the LatLonPoint factory (deferred — backlog #2697)
//     and direct callers can construct it.
//
//  3. Point-values plumbing: the Java reference calls
//     pointValues.intersect(visitor) and pointValues.estimateDocCount(visitor)
//     directly on org.apache.lucene.index.PointValues. The Gocene
//     index.PointValues surface only exposes metadata today; until a
//     visitor-driven Intersect surface lands at the index layer this
//     query consumes a narrow local source ([latLonDistancePointSource])
//     mirroring the subset of IntersectVisitor / Relation actually
//     used. Same strategy as XYPointInGeometryQuery (GOC-3208) and
//     LongDistanceFeatureQuery (GOC-3206); a single adapter site
//     ([defaultLatLonPointDistanceLeafLookup]) will be swapped once the
//     production wiring lands.
//
//  4. Inverse-visit fast path: the Java reference adds an inverse
//     visitor that clears non-matching docs from a pre-set FixedBitSet
//     when the cost estimate exceeds half the leaf size. Gocene does
//     not implement that fast path yet — the Get() entry always walks
//     the forward visitor and builds the matching DocIdSet through
//     [util.DocIdSetBuilder]. The behaviour is equivalent (every
//     matching doc surfaces with the same score); only the runtime
//     cost differs on dense segments. Backlog item #2697 will revisit
//     once a real visitor-driven PointValues source is wired and a
//     benchmark justifies the extra code path.
type latLonPointDistanceQuery struct {
	BaseQuery

	field        string
	latitude     float64
	longitude    float64
	radiusMeters float64

	// testLeafLookup, when non-nil, overrides the production lookup
	// path used by CreateWeight. Tests set it via
	// installTestLatLonPointDistanceLeafLookup to inject in-memory
	// point sources; production callers must leave it nil. Mirrors
	// the same hook on LongDistanceFeatureQuery and
	// XYPointInGeometryQuery.
	testLeafLookup latLonPointDistanceLeafLookup
}

// NewLatLonPointDistanceQuery constructs a LatLonPointDistanceQuery
// for the named field. Validates the same set of conditions the Java
// constructor throws IllegalArgumentException for: nil/empty field,
// non-finite or negative radius, out-of-range latitude or longitude.
//
// Returns an error in every Java IAE case so callers can decide
// whether the failure is fatal or recoverable.
func NewLatLonPointDistanceQuery(
	field string,
	latitude, longitude, radiusMeters float64,
) (Query, error) {
	if field == "" {
		return nil, errors.New("lat lon point distance query: field must not be empty")
	}
	if math.IsNaN(radiusMeters) || math.IsInf(radiusMeters, 0) || radiusMeters < 0 {
		return nil, fmt.Errorf(
			"lat lon point distance query: radiusMeters: '%v' is invalid", radiusMeters,
		)
	}
	if err := geo.CheckLatitude(latitude); err != nil {
		return nil, fmt.Errorf("lat lon point distance query: %w", err)
	}
	if err := geo.CheckLongitude(longitude); err != nil {
		return nil, fmt.Errorf("lat lon point distance query: %w", err)
	}
	return &latLonPointDistanceQuery{
		field:        field,
		latitude:     latitude,
		longitude:    longitude,
		radiusMeters: radiusMeters,
	}, nil
}

// Field returns the query field name. Mirrors the Java getField accessor.
func (q *latLonPointDistanceQuery) Field() string { return q.field }

// Latitude returns the query centre latitude. Mirrors the Java
// getLatitude accessor.
func (q *latLonPointDistanceQuery) Latitude() float64 { return q.latitude }

// Longitude returns the query centre longitude. Mirrors the Java
// getLongitude accessor.
func (q *latLonPointDistanceQuery) Longitude() float64 { return q.longitude }

// RadiusMeters returns the query radius in metres. Mirrors the Java
// getRadiusMeters accessor.
func (q *latLonPointDistanceQuery) RadiusMeters() float64 { return q.radiusMeters }

// Visit dispatches to the QueryVisitor when the visitor accepts the
// target field, mirroring the two-step accept/visitLeaf protocol of
// the Java reference.
func (q *latLonPointDistanceQuery) Visit(visitor QueryVisitor) {
	if visitor.AcceptField(q.field) {
		visitor.VisitLeaf(q)
	}
}

// Rewrite returns the query unchanged. The Java reference inherits
// the no-op rewrite from Query.
func (q *latLonPointDistanceQuery) Rewrite(_ IndexReader) (Query, error) { return q, nil }

// Clone returns a shallow copy. The query holds only value-type
// fields, so a structural copy is safe.
func (q *latLonPointDistanceQuery) Clone() Query {
	c := *q
	return &c
}

// Equals mirrors the Java reference: two queries are equal iff they
// share the same concrete type, field, and (latitude, longitude,
// radiusMeters) triple compared by IEEE-754 long bits (so NaN ==
// NaN and +0 != -0, matching Double.doubleToLongBits).
func (q *latLonPointDistanceQuery) Equals(other Query) bool {
	o, ok := other.(*latLonPointDistanceQuery)
	if !ok {
		return false
	}
	if q == o {
		return true
	}
	if q.field != o.field {
		return false
	}
	if math.Float64bits(q.latitude) != math.Float64bits(o.latitude) {
		return false
	}
	if math.Float64bits(q.longitude) != math.Float64bits(o.longitude) {
		return false
	}
	if math.Float64bits(q.radiusMeters) != math.Float64bits(o.radiusMeters) {
		return false
	}
	return true
}

// HashCode mirrors the Java hashCode: classHash + 31-poly over the
// field hash and the three IEEE-754 long bits.
func (q *latLonPointDistanceQuery) HashCode() int {
	const prime = 31
	h := classHashLatLonPointDistanceQuery
	h = prime*h + stringHash(q.field)
	h = prime*h + foldLongBits(math.Float64bits(q.latitude))
	h = prime*h + foldLongBits(math.Float64bits(q.longitude))
	h = prime*h + foldLongBits(math.Float64bits(q.radiusMeters))
	return h
}

// String renders the query in the Lucene-canonical form. The Go
// Query interface's String() does not take a field argument; we
// expose a no-arg helper used in test diagnostics and Lucene-
// compatible reporting.
func (q *latLonPointDistanceQuery) String() string {
	return q.toString("")
}

// StringField mirrors the Java toString(String field) signature
// directly for callers that need the field-aware rendering (e.g.
// parent query renderers).
func (q *latLonPointDistanceQuery) StringField(field string) string {
	return q.toString(field)
}

// toString is the shared formatter used by String/StringField. The
// rendering matches the Java reference's
// "<field>:<lat>,<lon> +/- <radius> meters" form, with the "<field>:"
// prefix suppressed when the supplied field matches the query field.
func (q *latLonPointDistanceQuery) toString(field string) string {
	var b strings.Builder
	if q.field != field {
		b.WriteString(q.field)
		b.WriteByte(':')
	}
	fmt.Fprintf(&b, "%v,%v +/- %v meters", q.latitude, q.longitude, q.radiusMeters)
	return b.String()
}

// CreateWeight builds the dateline-aware encoded bounding box, the
// shared DistancePredicate, the sort key and axisLat parameters once
// (matching the Java reference, which derives them in createWeight)
// and returns a [latLonPointDistanceWeight]. The supplier returns nil
// when the field is unknown to the leaf, matching Lucene's null-
// Scorer fast path.
func (q *latLonPointDistanceQuery) CreateWeight(
	_ *IndexSearcher, _ bool, boost float32,
) (Weight, error) {
	box, err := geo.FromPointDistance(q.latitude, q.longitude, q.radiusMeters)
	if err != nil {
		return nil, fmt.Errorf("lat lon point distance query: build bounding box: %w", err)
	}
	bbox := encodeLatLonDistanceBBox(box)
	predicate := geo.CreateDistancePredicate(q.latitude, q.longitude, q.radiusMeters)
	sortKey := geo.DistanceQuerySortKey(q.radiusMeters)
	axisLat := geo.AxisLat(q.latitude, q.radiusMeters)
	w := &latLonPointDistanceWeight{
		query:      q,
		boost:      boost,
		bbox:       bbox,
		predicate:  predicate,
		sortKey:    sortKey,
		axisLat:    axisLat,
		leafLookup: q.resolveLeafLookup(),
	}
	w.BaseWeight = NewBaseWeight(q)
	return w, nil
}

// resolveLeafLookup returns the leaf lookup configured for this
// query. Mirrors the same hook on XYPointInGeometryQuery.
func (q *latLonPointDistanceQuery) resolveLeafLookup() latLonPointDistanceLeafLookup {
	if q.testLeafLookup != nil {
		return q.testLeafLookup
	}
	return defaultLatLonPointDistanceLeafLookup
}

// installTestLeafLookup wires a test-only lookup so the test build
// can inject in-memory point sources without depending on production
// segment readers. Mirrors the same hook on the sibling point queries.
func (q *latLonPointDistanceQuery) installTestLeafLookup(lookup latLonPointDistanceLeafLookup) {
	q.testLeafLookup = lookup
}

// Ensure latLonPointDistanceQuery implements Query.
var _ Query = (*latLonPointDistanceQuery)(nil)

// latLonPointDistanceLeafLookup resolves the per-segment point source,
// FieldInfo and maxDoc for a given LeafReaderContext. Returns
// (nil, nil, 0, nil) when the field is unknown to the leaf, matching
// the Java reference's "no docs in this segment" fast path that
// returns a null Scorer.
type latLonPointDistanceLeafLookup func(
	ctx *index.LeafReaderContext, field string,
) (latLonDistancePointSource, *index.FieldInfo, int, error)

// defaultLatLonPointDistanceLeafLookup resolves the visitor-driven
// point source from the production LeafReader path. The current
// production LeafReader API only exposes the metadata-only
// [index.PointValues] interface; until a visitor-driven Intersect
// surface lands at the index layer, this returns (nil, nil, 0, nil)
// so callers fall through to the null-Scorer fast path. Once the
// index layer is wired, this adapter is the single place to update.
func defaultLatLonPointDistanceLeafLookup(
	ctx *index.LeafReaderContext, field string,
) (latLonDistancePointSource, *index.FieldInfo, int, error) {
	if ctx == nil {
		return nil, nil, 0, nil
	}
	leaf := ctx.LeafReader()
	if leaf == nil {
		return nil, nil, 0, nil
	}
	type pointReader interface {
		GetPointValues(field string) (index.PointValues, error)
	}
	type fieldInfosProvider interface {
		GetFieldInfos() *index.FieldInfos
	}
	pr, ok := leaf.(pointReader)
	if !ok {
		return nil, nil, 0, nil
	}
	raw, err := pr.GetPointValues(field)
	if err != nil {
		return nil, nil, 0, err
	}
	if raw == nil {
		return nil, nil, 0, nil
	}
	var fi *index.FieldInfo
	if fip, ok := leaf.(fieldInfosProvider); ok {
		infos := fip.GetFieldInfos()
		if infos != nil {
			fi = infos.GetByName(field)
		}
	}
	if fi == nil {
		// Matching the Java reference's "no docs in this segment
		// indexed this field at all" fast path: return a null Scorer.
		return nil, nil, 0, nil
	}
	return newLatLonDistancePointSourceFromIndexPointValues(raw), fi, leaf.MaxDoc(), nil
}

// latLonDistancePointSource is the narrow point-values surface
// consumed by the scorer. It mirrors the subset of
// org.apache.lucene.index.PointValues used by the Java reference: a
// visitor-driven Intersect and a coarse doc-count estimator. Kept
// here so the query does not block on the broader index.PointValues
// port.
type latLonDistancePointSource interface {
	// Intersect walks the BKD tree, invoking the visitor's per-doc
	// and per-(doc, packedValue) callbacks for matching docs and
	// using Compare to prune cells.
	Intersect(visitor latLonDistancePointVisitor) error
	// EstimateDocCount returns a coarse upper bound on the number
	// of documents the visitor would match. Used by the
	// ScorerSupplier cost() override to size pre-allocations.
	EstimateDocCount(visitor latLonDistancePointVisitor) (int64, error)
}

// latLonDistancePointVisitor is the visitor contract the scorer
// hands to a latLonDistancePointSource. It mirrors the subset of
// org.apache.lucene.index.PointValues.IntersectVisitor used by the
// reference query: per-doc and per-(doc, packedValue) visits, an
// iterator-wide visit, a Grow size hint, and a Compare callback
// returning the cell relation.
type latLonDistancePointVisitor interface {
	Visit(docID int) error
	VisitWithPackedValue(docID int, packedValue []byte) error
	VisitIterator(iter util.DocIdSetIterator) error
	VisitIteratorWithPackedValue(iter util.DocIdSetIterator, packedValue []byte) error
	Grow(count int)
	Compare(minPackedValue, maxPackedValue []byte) latLonDistanceCellRelation
}

// latLonDistanceCellRelation classifies how a BKD cell intersects
// the disk, mirroring org.apache.lucene.index.PointValues.Relation.
type latLonDistanceCellRelation int

const (
	// latLonDistanceCellOutsideQuery indicates the cell lies fully outside the disk.
	latLonDistanceCellOutsideQuery latLonDistanceCellRelation = iota
	// latLonDistanceCellInsideQuery indicates the cell lies fully inside the disk.
	latLonDistanceCellInsideQuery
	// latLonDistanceCellCrossesQuery indicates the cell partially overlaps the disk.
	latLonDistanceCellCrossesQuery
)

// latLonDistancePointTreeIntersect is the rich, visitor-driven read
// surface a BKD-backed PointValues exposes beyond the metadata-only
// index.PointValues. The on-disk reader returned by
// LeafReader.GetPointValues (the codec's *pointValues) satisfies it
// structurally; the parameter type is the index-package alias so the
// type assertion succeeds for the real codec reader (the same reason
// PointRangeQuery and XYPointInGeometryQuery alias
// index.PointTreeIntersectVisitor).
type latLonDistancePointTreeIntersect interface {
	Intersect(visitor index.PointTreeIntersectVisitor) error
	EstimatePointCount(visitor index.PointTreeIntersectVisitor) int64
}

// newLatLonDistancePointSourceFromIndexPointValues adapts a BKD-backed
// index.PointValues to the latLonDistancePointSource contract used by
// the scorer. When the concrete PointValues exposes the visitor-driven
// Intersect / EstimatePointCount surface (the codec reader does), it
// drives the real BKD walk; otherwise it falls back to a no-op source
// (zero matches), matching the Java reference's "field exists but yields
// no materialised data" behaviour. Mirrors the equivalent adapter in
// xy_point_in_geometry_query.go.
func newLatLonDistancePointSourceFromIndexPointValues(pv index.PointValues) latLonDistancePointSource {
	if rich, ok := pv.(latLonDistancePointTreeIntersect); ok {
		return &bkdLatLonDistancePointSource{pv: rich}
	}
	return noopLatLonDistancePointSource{}
}

// bkdLatLonDistancePointSource drives a BKD-backed PointValues,
// translating between the distance query's latLonDistancePointVisitor
// and the index.PointTreeIntersectVisitor the BKD reader expects.
type bkdLatLonDistancePointSource struct {
	pv latLonDistancePointTreeIntersect
}

func (s *bkdLatLonDistancePointSource) Intersect(visitor latLonDistancePointVisitor) error {
	return s.pv.Intersect(&latLonDistanceVisitorBridge{v: visitor})
}

func (s *bkdLatLonDistancePointSource) EstimateDocCount(visitor latLonDistancePointVisitor) (int64, error) {
	return s.pv.EstimatePointCount(&latLonDistanceVisitorBridge{v: visitor}), nil
}

// latLonDistanceVisitorBridge adapts a latLonDistancePointVisitor to the
// index.PointTreeIntersectVisitor surface the BKD reader invokes. The
// reader only drives Visit / VisitByPackedValue / Compare / Grow (the
// bulk-iterator methods on latLonDistancePointVisitor are not part of
// the BKD reader's intersect path).
type latLonDistanceVisitorBridge struct {
	v latLonDistancePointVisitor
}

func (b *latLonDistanceVisitorBridge) Visit(docID int) error { return b.v.Visit(docID) }

func (b *latLonDistanceVisitorBridge) VisitByPackedValue(docID int, packedValue []byte) error {
	return b.v.VisitWithPackedValue(docID, packedValue)
}

func (b *latLonDistanceVisitorBridge) Compare(minPackedValue, maxPackedValue []byte) int {
	return int(b.v.Compare(minPackedValue, maxPackedValue))
}

func (b *latLonDistanceVisitorBridge) Grow(count int) { b.v.Grow(count) }

var _ index.PointTreeIntersectVisitor = (*latLonDistanceVisitorBridge)(nil)

// noopLatLonDistancePointSource is the safe fallback when the
// PointValues does not expose the visitor-driven Intersect surface (e.g.
// an in-test metadata-only stub). It matches the Java reference's "field
// exists but yields zero matches" behaviour for unmaterialised data.
type noopLatLonDistancePointSource struct{}

func (noopLatLonDistancePointSource) Intersect(_ latLonDistancePointVisitor) error { return nil }
func (noopLatLonDistancePointSource) EstimateDocCount(_ latLonDistancePointVisitor) (int64, error) {
	return 0, nil
}

// latLonDistanceBBox holds the dateline-aware encoded bounding box
// the Weight derives from the disk's WGS-84 bounding rectangle.
// Mirrors the Java reference's (minLat, maxLat, minLon, maxLon,
// minLon2) tuple captured in createWeight.
//
// When the rectangle crosses the dateline the longitude range is
// split into two sub-ranges: [Integer.MIN_VALUE, maxLon] and
// [minLon2, Integer.MAX_VALUE]; minLon is set to Integer.MIN_VALUE
// and minLon2 to the encoded minLon. When the rectangle does not
// cross the dateline minLon2 is set to Integer.MAX_VALUE so the
// second-range fast-path test (lon < minLon2) reliably reports
// "outside" for every lon — matching the Java reference exactly.
type latLonDistanceBBox struct {
	minLat  int32
	maxLat  int32
	minLon  int32
	maxLon  int32
	minLon2 int32
}

// encodeLatLonDistanceBBox materializes the dateline-aware encoded
// bounding box from the disk's WGS-84 bounding rectangle. Mirrors
// the inline block at the head of the Java createWeight: split the
// longitude range on dateline crossings, disable the second range
// otherwise.
func encodeLatLonDistanceBBox(box geo.Rectangle) latLonDistanceBBox {
	out := latLonDistanceBBox{
		minLat: geo.EncodeLatitude(box.MinLat()),
		maxLat: geo.EncodeLatitude(box.MaxLat()),
	}
	if box.CrossesDateline() {
		// box1: longitude range [MIN_INT, encodedMaxLon]
		out.minLon = math.MinInt32
		out.maxLon = geo.EncodeLongitude(box.MaxLon())
		// box2: longitude range [encodedMinLon, MAX_INT]
		out.minLon2 = geo.EncodeLongitude(box.MinLon())
	} else {
		out.minLon = geo.EncodeLongitude(box.MinLon())
		out.maxLon = geo.EncodeLongitude(box.MaxLon())
		// disabling sentinel — the "lon < minLon2" guard reliably
		// reports "outside" for every encoded longitude.
		out.minLon2 = math.MaxInt32
	}
	return out
}

// latLonPointDistanceWeight is the per-Weight half of the query. It
// owns the encoded bounding box, the shared DistancePredicate, the
// sort key and axisLat parameters built once in CreateWeight, plus
// the boost used as the constant per-doc score.
type latLonPointDistanceWeight struct {
	*BaseWeight

	query     *latLonPointDistanceQuery
	boost     float32
	bbox      latLonDistanceBBox
	predicate geo.DistancePredicate
	sortKey   float64
	axisLat   float64

	leafLookup latLonPointDistanceLeafLookup
}

// GetQuery returns the parent query.
func (w *latLonPointDistanceWeight) GetQuery() Query { return w.query }

// IsCacheable mirrors the Java override, which returns true
// unconditionally for this query (the visitor result depends only on
// the indexed point values, which do not change for a frozen segment).
func (w *latLonPointDistanceWeight) IsCacheable(_ *index.LeafReaderContext) bool { return true }

// Count returns -1 to signal that no sub-linear count is available.
func (w *latLonPointDistanceWeight) Count(_ *index.LeafReaderContext) (int, error) { return -1, nil }

// Matches returns nil; this query does not produce per-match positions.
func (w *latLonPointDistanceWeight) Matches(_ *index.LeafReaderContext, _ int) (Matches, error) {
	return nil, nil
}

// ScorerSupplier resolves the per-leaf point source, validates the
// field shape, and returns a ScorerSupplier whose Get builds a
// constant-score scorer walking the visitor. Returns (nil, nil) when
// the leaf has no source for the field — matching the Java fast path
// that returns a null Scorer.
func (w *latLonPointDistanceWeight) ScorerSupplier(
	ctx *index.LeafReaderContext,
) (ScorerSupplier, error) {
	source, fieldInfo, maxDoc, err := w.leafLookup(ctx, w.query.field)
	if err != nil {
		return nil, err
	}
	if source == nil || fieldInfo == nil {
		return nil, nil
	}
	if err := checkLatLonPointCompatible(fieldInfo); err != nil {
		return nil, err
	}
	return &latLonPointDistanceScorerSupplier{
		weight: w,
		source: source,
		maxDoc: maxDoc,
		cost:   -1,
	}, nil
}

// Scorer is the convenience entry point that mirrors Java
// Weight.scorer(). It delegates to ScorerSupplier exactly as the
// Lucene Weight does.
func (w *latLonPointDistanceWeight) Scorer(ctx *index.LeafReaderContext) (Scorer, error) {
	supplier, err := w.ScorerSupplier(ctx)
	if err != nil {
		return nil, err
	}
	if supplier == nil {
		return nil, nil
	}
	return supplier.Get(0)
}

// Ensure latLonPointDistanceWeight implements Weight.
var _ Weight = (*latLonPointDistanceWeight)(nil)

// latLonPointDistanceScorerSupplier mirrors the inner ScorerSupplier
// of the Java reference. cost is computed lazily on the first Cost()
// call because EstimateDocCount can be expensive.
type latLonPointDistanceScorerSupplier struct {
	weight *latLonPointDistanceWeight
	source latLonDistancePointSource
	maxDoc int

	cost int64
}

// Get materializes the matching DocIdSet and wraps it in a constant-
// score scorer. Mirrors the Java reference's forward-visitor branch,
// which builds a DocIdSetBuilder fresh per Get call and wraps it in
// ConstantScoreScorer(score(), scoreMode, iterator). The inverse-
// visit fast path is not implemented; see the type-level divergence
// note.
func (s *latLonPointDistanceScorerSupplier) Get(_ int64) (Scorer, error) {
	builder := util.NewDocIdSetBuilder(s.maxDoc)
	visitor := newLatLonPointDistanceVisitor(builder, &s.weight.bbox, &s.weight.predicate,
		s.weight.query.latitude, s.weight.query.longitude, s.weight.sortKey, s.weight.axisLat)
	if err := s.source.Intersect(visitor); err != nil {
		return nil, err
	}
	set, err := builder.Build()
	if err != nil {
		return nil, err
	}
	var disi DocIdSetIterator
	if set == nil {
		disi = NewEmptyDocIdSetIterator()
	} else {
		utilIter := set.Iterator()
		if utilIter == nil {
			disi = NewEmptyDocIdSetIterator()
		} else {
			disi = newLatLonDistanceUtilDISIAdapter(utilIter)
		}
	}
	return &latLonPointDistanceScorer{
		BaseScorer: NewBaseScorer(s.weight),
		weight:     s.weight,
		iter:       disi,
		score:      s.weight.boost,
	}, nil
}

// Cost returns the upper-bound doc count, computed lazily.
func (s *latLonPointDistanceScorerSupplier) Cost() int64 {
	if s.cost == -1 {
		visitor := newLatLonPointDistanceVisitor(util.NewDocIdSetBuilder(s.maxDoc),
			&s.weight.bbox, &s.weight.predicate,
			s.weight.query.latitude, s.weight.query.longitude,
			s.weight.sortKey, s.weight.axisLat)
		cost, err := s.source.EstimateDocCount(visitor)
		if err != nil || cost < 0 {
			cost = 0
		}
		s.cost = cost
	}
	return s.cost
}

// SetTopLevelScoringClause is a no-op for this constant-score supplier.
func (s *latLonPointDistanceScorerSupplier) SetTopLevelScoringClause() {}

// Ensure latLonPointDistanceScorerSupplier implements ScorerSupplier.
var _ ScorerSupplier = (*latLonPointDistanceScorerSupplier)(nil)

// latLonPointDistanceVisitor is the visitor handed to the
// latLonDistancePointSource. It mirrors the inner forward
// IntersectVisitor of the Java reference: per-doc adds bypass the
// distance check (the cell is fully inside the disk), per-(doc,
// packedValue) adds first run the encoded bounding-box gate then the
// DistancePredicate, and the Compare hook delegates to a relate
// helper that mirrors the Java relate(byte[], byte[]) shape.
type latLonPointDistanceVisitor struct {
	result    *util.DocIdSetBuilder
	bbox      *latLonDistanceBBox
	predicate *geo.DistancePredicate
	lat       float64
	lon       float64
	sortKey   float64
	axisLat   float64
	adder     util.BulkAdder
}

// newLatLonPointDistanceVisitor wires the builder + parameters into
// a visitor instance. The adder is captured lazily on the first
// Grow call so the visitor is safe to construct cheaply for cost-
// only paths.
func newLatLonPointDistanceVisitor(
	result *util.DocIdSetBuilder,
	bbox *latLonDistanceBBox,
	predicate *geo.DistancePredicate,
	lat, lon, sortKey, axisLat float64,
) *latLonPointDistanceVisitor {
	return &latLonPointDistanceVisitor{
		result:    result,
		bbox:      bbox,
		predicate: predicate,
		lat:       lat,
		lon:       lon,
		sortKey:   sortKey,
		axisLat:   axisLat,
	}
}

// Grow asks the underlying builder for an adder sized for count
// docs. Mirrors the Java visitor's grow(int) override.
func (v *latLonPointDistanceVisitor) Grow(count int) {
	v.adder = v.result.Grow(count)
}

// ensureAdder lazily obtains an adder if the visitor was never
// grown. The Java reference always calls grow before the first add;
// this belt-and-braces fallback handles tests that bypass that
// contract.
func (v *latLonPointDistanceVisitor) ensureAdder() {
	if v.adder == nil {
		v.adder = v.result.Grow(0)
	}
}

// Visit adds a single docID unconditionally. Used when the source
// already knows the cell is fully inside the disk.
func (v *latLonPointDistanceVisitor) Visit(docID int) error {
	v.ensureAdder()
	v.adder.Add(docID)
	return nil
}

// VisitWithPackedValue decodes the 8-byte (lat, lon) payload and
// admits the doc only when the point falls within the disk. Mirrors
// the per-(doc, packedValue) visit on the Java reference, which
// runs the encoded bounding-box gate first and then the
// DistancePredicate test.
func (v *latLonPointDistanceVisitor) VisitWithPackedValue(docID int, packedValue []byte) error {
	if len(packedValue) < 2*latLonPointBytesPerDim {
		return fmt.Errorf(
			"lat lon point distance visitor: packed value too short: %d", len(packedValue),
		)
	}
	if v.matches(packedValue) {
		return v.Visit(docID)
	}
	return nil
}

// VisitIterator drains an iterator of docs that are all fully
// inside the disk. Mirrors visit(DocIdSetIterator) on the Java
// reference.
func (v *latLonPointDistanceVisitor) VisitIterator(iter util.DocIdSetIterator) error {
	v.ensureAdder()
	return v.adder.AddIterator(iter)
}

// VisitIteratorWithPackedValue is the iterator-shaped variant that
// gates on the shared packedValue (every doc in iter has the same
// point coordinates). Mirrors visit(DocIdSetIterator, byte[]) on
// the Java reference.
func (v *latLonPointDistanceVisitor) VisitIteratorWithPackedValue(
	iter util.DocIdSetIterator, packedValue []byte,
) error {
	if len(packedValue) < 2*latLonPointBytesPerDim {
		return fmt.Errorf(
			"lat lon point distance visitor: packed value too short: %d", len(packedValue),
		)
	}
	if v.matches(packedValue) {
		return v.VisitIterator(iter)
	}
	return nil
}

// Compare decodes the min/max packed values and asks the relate
// helper to classify the cell. Mirrors compare(byte[], byte[]) on
// the Java reference.
func (v *latLonPointDistanceVisitor) Compare(
	minPackedValue, maxPackedValue []byte,
) latLonDistanceCellRelation {
	if len(minPackedValue) < 2*latLonPointBytesPerDim ||
		len(maxPackedValue) < 2*latLonPointBytesPerDim {
		// Mirrors Java's array-bounds failure: a malformed cell
		// payload is a programmer error. The safe answer here is
		// "crosses" (force the source to recurse and surface the
		// bug downstream) rather than silently dropping the cell.
		return latLonDistanceCellCrossesQuery
	}
	return v.relate(minPackedValue, maxPackedValue)
}

// matches mirrors the inner matches(byte[]) helper of the Java
// reference: first the encoded bounding-box gate (including the
// dateline-aware second range), then the DistancePredicate test.
func (v *latLonPointDistanceVisitor) matches(packedValue []byte) bool {
	lat := util.SortableBytesToInt(packedValue, 0)
	if lat > v.bbox.maxLat || lat < v.bbox.minLat {
		// Latitude out of bounding-box range.
		return false
	}
	lon := util.SortableBytesToInt(packedValue, latLonPointBytesPerDim)
	if (lon > v.bbox.maxLon || lon < v.bbox.minLon) && lon < v.bbox.minLon2 {
		// Longitude out of bounding-box range (with dateline second
		// range disabled by the MAX_INT sentinel for non-crossing
		// queries).
		return false
	}
	return v.predicate.Test(lat, lon)
}

// relate mirrors the inner relate(byte[], byte[]) helper of the
// Java reference: encoded bounding-box reject first, then decoded-
// degree call into geo.Relate for the disk-vs-cell classification.
func (v *latLonPointDistanceVisitor) relate(
	minPackedValue, maxPackedValue []byte,
) latLonDistanceCellRelation {
	latLowerBound := util.SortableBytesToInt(minPackedValue, 0)
	latUpperBound := util.SortableBytesToInt(maxPackedValue, 0)
	if latLowerBound > v.bbox.maxLat || latUpperBound < v.bbox.minLat {
		// Latitude out of bounding-box range.
		return latLonDistanceCellOutsideQuery
	}
	lonLowerBound := util.SortableBytesToInt(minPackedValue, latLonPointBytesPerDim)
	lonUpperBound := util.SortableBytesToInt(maxPackedValue, latLonPointBytesPerDim)
	if (lonLowerBound > v.bbox.maxLon || lonUpperBound < v.bbox.minLon) &&
		lonUpperBound < v.bbox.minLon2 {
		// Longitude out of bounding-box range.
		return latLonDistanceCellOutsideQuery
	}
	latMin := geo.DecodeLatitude(latLowerBound)
	lonMin := geo.DecodeLongitude(lonLowerBound)
	latMax := geo.DecodeLatitude(latUpperBound)
	lonMax := geo.DecodeLongitude(lonUpperBound)
	return latLonDistanceRelationFromGeo(
		geo.Relate(latMin, latMax, lonMin, lonMax,
			v.lat, v.lon, v.sortKey, v.axisLat),
	)
}

// latLonDistanceRelationFromGeo maps geo.Relation onto the local
// latLonDistanceCellRelation enum. The two enums carry identical
// semantics; the local enum exists so the query surface stays
// decoupled from the geo package (a future PointValues port may not
// want to depend on it transitively).
func latLonDistanceRelationFromGeo(r geo.Relation) latLonDistanceCellRelation {
	switch r {
	case geo.CellInsideQuery:
		return latLonDistanceCellInsideQuery
	case geo.CellCrossesQuery:
		return latLonDistanceCellCrossesQuery
	default:
		return latLonDistanceCellOutsideQuery
	}
}

// latLonPointDistanceScorer is the constant-score scorer returned by
// the supplier. It mirrors the inner ConstantScoreScorer wrapping on
// the Java reference: score() returns the boost, and the iterator
// forwards every position/cost call to the materialized DocIdSet's
// iterator.
type latLonPointDistanceScorer struct {
	*BaseScorer

	weight *latLonPointDistanceWeight
	iter   DocIdSetIterator
	score  float32
}

// DocID forwards to the underlying iterator.
func (s *latLonPointDistanceScorer) DocID() int { return s.iter.DocID() }

// NextDoc advances the scorer to the next matching document.
func (s *latLonPointDistanceScorer) NextDoc() (int, error) { return s.iter.NextDoc() }

// Advance moves the scorer to the first matching document at or beyond target.
func (s *latLonPointDistanceScorer) Advance(target int) (int, error) {
	return s.iter.Advance(target)
}

// Cost returns the underlying iterator's cost estimate.
func (s *latLonPointDistanceScorer) Cost() int64 { return s.iter.Cost() }

// DocIDRunEnd returns the end of the current run.
func (s *latLonPointDistanceScorer) DocIDRunEnd() int { return s.iter.DocIDRunEnd() }

// Score returns the constant boost score.
func (s *latLonPointDistanceScorer) Score() float32 { return s.score }

// GetMaxScore returns the constant boost score (no per-doc variability).
func (s *latLonPointDistanceScorer) GetMaxScore(_ int) float32 { return s.score }

// Ensure latLonPointDistanceScorer implements Scorer.
var _ Scorer = (*latLonPointDistanceScorer)(nil)

// newLatLonDistanceUtilDISIAdapter bridges a util.DocIdSetIterator
// to the search.DocIdSetIterator contract. Both iterators are
// structurally identical (DocID/NextDoc/Advance/Cost/DocIDRunEnd);
// only the package differs, so the adapter is a thin forwarder.
// Mirrors the same adapter pattern in xy_point_in_geometry_query.go
// but kept local so the two query files have no hidden coupling.
func newLatLonDistanceUtilDISIAdapter(inner util.DocIdSetIterator) DocIdSetIterator {
	return &latLonDistanceUtilDISIAdapter{inner: inner}
}

type latLonDistanceUtilDISIAdapter struct {
	inner util.DocIdSetIterator
}

func (a *latLonDistanceUtilDISIAdapter) DocID() int            { return a.inner.DocID() }
func (a *latLonDistanceUtilDISIAdapter) NextDoc() (int, error) { return a.inner.NextDoc() }
func (a *latLonDistanceUtilDISIAdapter) Advance(target int) (int, error) {
	return a.inner.Advance(target)
}
func (a *latLonDistanceUtilDISIAdapter) Cost() int64      { return a.inner.Cost() }
func (a *latLonDistanceUtilDISIAdapter) DocIDRunEnd() int { return a.inner.DocIDRunEnd() }

var _ DocIdSetIterator = (*latLonDistanceUtilDISIAdapter)(nil)

// checkLatLonPointCompatible mirrors the package-private
// LatLonPoint.checkCompatible(FieldInfo) helper in Lucene 10.4.0. It
// returns an error when the supplied FieldInfo was indexed with a
// point shape that does not match LatLonPointType (dimensionCount=2,
// numBytes=4). Per the Java reference, an "unset" (zero) shape is
// tolerated because the field could have been written by another
// field type with the same name in the same segment.
//
// Kept local to this query because the document package has not
// promoted a LatLonPoint-specific helper yet; once it does this
// helper collapses to a single call site.
func checkLatLonPointCompatible(fi *index.FieldInfo) error {
	if fi == nil {
		return errors.New("field info must not be nil")
	}
	dims := fi.PointDimensionCount()
	wantDims := document.LatLonPointType.PointDimensionCount()
	if dims != 0 && dims != wantDims {
		return fmt.Errorf(
			"field=%q was indexed with numDims=%d but LatLonPoint expects numDims=%d, "+
				"is the field really a LatLonPoint?",
			fi.Name(), dims, wantDims,
		)
	}
	bytesPerDim := fi.PointNumBytes()
	wantBytesPerDim := document.LatLonPointType.PointNumBytes()
	if bytesPerDim != 0 && bytesPerDim != wantBytesPerDim {
		return fmt.Errorf(
			"field=%q was indexed with bytesPerDim=%d but LatLonPoint expects "+
				"bytesPerDim=%d, is the field really a LatLonPoint?",
			fi.Name(), bytesPerDim, wantBytesPerDim,
		)
	}
	return nil
}

// foldLongBits collapses a 64-bit IEEE-754 bit pattern into a Java-
// style int hash via (int)(bits ^ (bits >>> 32)). Mirrors the inner
// helper used in the Java hashCode override and kept local to this
// file so the formula is auditable at the call site.
func foldLongBits(bits uint64) int {
	return int(int32(bits ^ (bits >> 32)))
}

// classHashLatLonPointDistanceQuery seeds the hash with a type-stable
// constant. Distinct from other query class hashes so two different
// query types with the same field/payload do not collide.
const classHashLatLonPointDistanceQuery = 0x4c4c_5044 // "LLPD"
