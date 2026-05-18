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
	"strings"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// xyPointInGeometryQuery is the Go port of Apache Lucene 10.4.0
// org.apache.lucene.document.XYPointInGeometryQuery
// (lucene/core/src/java/org/apache/lucene/document/XYPointInGeometryQuery.java).
//
// The query matches every document whose indexed XY point (an
// [document.XYPointField]) lies inside any of the supplied XYGeometry
// shapes, scored as a constant boost.
//
// # Divergence from Lucene
//
//  1. Package: the Java type is a final, package-private class in
//     org.apache.lucene.document. In Gocene queries live in search/ to
//     avoid the search<->document import cycle (queries reference
//     search.Weight/Scorer; document is already imported by search).
//
//  2. Exposure: the Java class is package-private and reached through
//     the XYPointField static factories. In Gocene the type is unexported
//     (xyPointInGeometryQuery) but the factory
//     [NewXYPointInGeometryQuery] is exported so the XYPointField
//     factories (deferred — backlog #2697) and direct callers can
//     construct it. Same pattern as the FloatRangeSlowRangeQuery port
//     (GOC-3207).
//
//  3. Point-values plumbing: the Java reference calls
//     pointValues.intersect(visitor) and pointValues.estimateDocCount(visitor)
//     directly on org.apache.lucene.index.PointValues. The Gocene
//     index.PointValues interface does not yet expose those visitor-
//     driven methods (the BKD-walking surface lives in util/bkd and
//     codecs/). To stay forward compatible without forcing a wider
//     interface change here, this query consumes a narrow local
//     surface — [xyPointSource] / [xyPointVisitor] — mirroring the
//     subset of IntersectVisitor / Relation actually used. Once a
//     production LeafReader wires a real visitor-driven source it will
//     be picked up by [defaultXYPointInGeometryLeafLookup] without
//     changes here. Mirrors the strategy established by the
//     LongDistanceFeatureQuery port (GOC-3206).
type xyPointInGeometryQuery struct {
	BaseQuery

	field      string
	geometries []geo.XYGeometry

	// testLeafLookup, when non-nil, overrides the production lookup
	// path used by CreateWeight. Tests set it via
	// installTestXYPointInGeometryLeafLookup to inject in-memory point
	// sources; production callers must leave it nil.
	testLeafLookup xyPointInGeometryLeafLookup
}

// NewXYPointInGeometryQuery constructs an XYPointInGeometryQuery for
// the named field. At least one geometry is required, and no geometry
// may be nil. The geometries are defensively copied so the caller may
// mutate the slice it passed in without affecting the query.
//
// Returns an error for the validation cases that throw
// IllegalArgumentException on the Java reference: nil/empty field, nil
// geometries slice, empty geometries slice, or any nil entry within.
func NewXYPointInGeometryQuery(field string, geometries ...geo.XYGeometry) (Query, error) {
	if field == "" {
		return nil, errors.New("xy point in geometry query: field must not be empty")
	}
	if geometries == nil {
		return nil, errors.New("xy point in geometry query: geometries must not be nil")
	}
	if len(geometries) == 0 {
		return nil, errors.New("xy point in geometry query: geometries must not be empty")
	}
	for i, g := range geometries {
		if g == nil {
			return nil, fmt.Errorf("xy point in geometry query: geometry at index %d is nil", i)
		}
	}
	dup := make([]geo.XYGeometry, len(geometries))
	copy(dup, geometries)
	return &xyPointInGeometryQuery{
		field:      field,
		geometries: dup,
	}, nil
}

// Field returns the query field name.
func (q *xyPointInGeometryQuery) Field() string { return q.field }

// Geometries returns a defensive copy of the geometry list. Mirrors
// the Java getGeometries() accessor, which clones the array.
func (q *xyPointInGeometryQuery) Geometries() []geo.XYGeometry {
	out := make([]geo.XYGeometry, len(q.geometries))
	copy(out, q.geometries)
	return out
}

// Visit dispatches to the QueryVisitor when the visitor accepts the
// target field, mirroring the two-step accept/visitLeaf protocol of
// the Java reference.
func (q *xyPointInGeometryQuery) Visit(visitor QueryVisitor) {
	if visitor.AcceptField(q.field) {
		visitor.VisitLeaf(q)
	}
}

// Rewrite returns the query unchanged. The Java reference inherits the
// no-op rewrite from Query.
func (q *xyPointInGeometryQuery) Rewrite(_ IndexReader) (Query, error) { return q, nil }

// Clone returns a shallow copy. The geometries slice is treated as
// immutable through the API surface, so a shared header is safe.
func (q *xyPointInGeometryQuery) Clone() Query {
	c := *q
	return &c
}

// Equals mirrors the Java reference: two queries are equal iff they
// share the same concrete type, field, and geometry sequence (in
// order). Element equality reduces to Go == on the interface value,
// which delegates to the underlying XYGeometry implementation's
// equality. Mirrors Arrays.equals on XYGeometry[].
func (q *xyPointInGeometryQuery) Equals(other Query) bool {
	o, ok := other.(*xyPointInGeometryQuery)
	if !ok {
		return false
	}
	if q == o {
		return true
	}
	if q.field != o.field {
		return false
	}
	if len(q.geometries) != len(o.geometries) {
		return false
	}
	for i := range q.geometries {
		if q.geometries[i] != o.geometries[i] {
			return false
		}
	}
	return true
}

// HashCode mirrors the Java hashCode: classHash + 31-poly over the
// field hash and the per-geometry hash. java.util.Arrays.hashCode on a
// reference array folds each element's hashCode; we approximate via the
// %v string representation, which is stable for the immutable geometry
// types in geo/.
func (q *xyPointInGeometryQuery) HashCode() int {
	h := classHashXYPointInGeometryQuery
	h = 31*h + stringHash(q.field)
	h = 31*h + xyGeometriesHash(q.geometries)
	return h
}

// String mirrors the Java toString(String field) formatting:
//
//	"xyPointInGeometryQuery: [g0, g1, ...]" when the rendered field
//	matches the query field, otherwise "...: field=<q.field>: [...]".
//
// The Go Query interface's String() does not take a field argument; we
// expose a no-arg helper used in test diagnostics and Lucene-compatible
// reporting.
func (q *xyPointInGeometryQuery) String() string {
	return q.toString("")
}

// StringField mirrors the Java toString(String field) signature directly
// for callers that need the field-aware rendering (e.g. parent query
// renderers).
func (q *xyPointInGeometryQuery) StringField(field string) string {
	return q.toString(field)
}

// toString is the shared formatter used by String/StringField. The
// "xyPointInGeometryQuery" prefix mirrors Java's getClass().getSimpleName().
func (q *xyPointInGeometryQuery) toString(field string) string {
	var b strings.Builder
	b.WriteString("xyPointInGeometryQuery:")
	if q.field != field {
		b.WriteString(" field=")
		b.WriteString(q.field)
		b.WriteByte(':')
	}
	b.WriteByte('[')
	for i, g := range q.geometries {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%v", g)
	}
	b.WriteByte(']')
	return b.String()
}

// CreateWeight builds the Component2D tree once (matching the Java
// reference, which calls XYGeometry.create in createWeight) and returns
// a [xyPointInGeometryWeight]. The supplier returns nil when the field
// is unknown to the leaf, matching Lucene's null-Scorer fast path.
func (q *xyPointInGeometryQuery) CreateWeight(searcher *IndexSearcher, _ bool, boost float32) (Weight, error) {
	tree, err := geo.CreateXYGeometry(q.geometries...)
	if err != nil {
		return nil, fmt.Errorf("xy point in geometry query: build component tree: %w", err)
	}
	w := &xyPointInGeometryWeight{
		query:      q,
		boost:      boost,
		tree:       tree,
		leafLookup: q.resolveLeafLookup(),
	}
	w.BaseWeight = NewBaseWeight(q)
	return w, nil
}

// resolveLeafLookup returns the leaf lookup configured for this query.
func (q *xyPointInGeometryQuery) resolveLeafLookup() xyPointInGeometryLeafLookup {
	if q.testLeafLookup != nil {
		return q.testLeafLookup
	}
	return defaultXYPointInGeometryLeafLookup
}

// installTestLeafLookup wires a test-only lookup so the test build can
// inject in-memory point sources without depending on production
// segment readers. Mirrors the same hook on LongDistanceFeatureQuery.
func (q *xyPointInGeometryQuery) installTestLeafLookup(lookup xyPointInGeometryLeafLookup) {
	q.testLeafLookup = lookup
}

// Ensure xyPointInGeometryQuery implements Query.
var _ Query = (*xyPointInGeometryQuery)(nil)

// xyPointInGeometryLeafLookup resolves the per-segment point source,
// FieldInfo, and maxDoc for a given LeafReaderContext. Returns
// (nil, nil, 0, nil) when the field is unknown to the leaf, matching
// the Java reference's "no docs in this segment" fast path that
// returns a null Scorer.
//
// maxDoc is surfaced explicitly because the DocIdSetBuilder needs it
// to size the bitset, and the test build resolves it directly from
// the in-memory fixture rather than the (currently stub) production
// LeafReader.
type xyPointInGeometryLeafLookup func(ctx *index.LeafReaderContext, field string) (xyPointSource, *index.FieldInfo, int, error)

// defaultXYPointInGeometryLeafLookup resolves the visitor-driven point
// source from the production LeafReader path. The current production
// LeafReader API only exposes the metadata-only [index.PointValues]
// interface; until a visitor-driven Intersect surface lands at the
// index layer, this returns (nil, nil, 0, nil) so callers fall
// through to the null-Scorer fast path. Once the index layer is
// wired, this adapter is the single place to update.
func defaultXYPointInGeometryLeafLookup(ctx *index.LeafReaderContext, field string) (xyPointSource, *index.FieldInfo, int, error) {
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
		// Matching the Java reference's "no docs in this segment indexed
		// this field at all" fast path: return a null Scorer.
		return nil, nil, 0, nil
	}
	return newXYPointSourceFromIndexPointValues(raw), fi, leaf.MaxDoc(), nil
}

// xyPointSource is the narrow point-values surface consumed by the
// scorer. It mirrors the subset of org.apache.lucene.index.PointValues
// used by the Java reference: a visitor-driven Intersect and a coarse
// doc-count estimator. Kept here so the query does not block on the
// broader index.PointValues port (which currently exposes only the
// metadata surface).
type xyPointSource interface {
	// Intersect walks the BKD tree, invoking the visitor's per-doc and
	// per-(doc, packedValue) callbacks for matching docs and using
	// Compare to prune cells.
	Intersect(visitor xyPointVisitor) error
	// EstimateDocCount returns a coarse upper bound on the number of
	// documents the visitor would match. Used by the ScorerSupplier
	// cost() override to size pre-allocations. Negative results are
	// rejected by the Java reference's assertion; we surface them as
	// errors.
	EstimateDocCount(visitor xyPointVisitor) (int64, error)
}

// xyPointVisitor is the visitor contract the scorer hands to an
// xyPointSource. It mirrors the subset of
// org.apache.lucene.index.PointValues.IntersectVisitor used by the
// reference query: per-doc and per-(doc, packedValue) visits, an
// iterator-wide visit, a Grow size hint, and a Compare callback that
// returns the cell relation.
//
// The iterator parameters use util.DocIdSetIterator (not the
// search-package alias) because BulkAdder operates on the util shape
// and the source surface mirrors the structural BKD iteration shared
// by util/bkd and codecs/.
type xyPointVisitor interface {
	Visit(docID int) error
	VisitWithPackedValue(docID int, packedValue []byte) error
	VisitIterator(iter util.DocIdSetIterator) error
	VisitIteratorWithPackedValue(iter util.DocIdSetIterator, packedValue []byte) error
	Grow(count int)
	Compare(minPackedValue, maxPackedValue []byte) xyPointCellRelation
}

// xyPointCellRelation classifies how a BKD cell intersects the query
// region, mirroring org.apache.lucene.index.PointValues.Relation.
type xyPointCellRelation int

const (
	// xyPointCellOutsideQuery indicates the cell lies fully outside the query.
	xyPointCellOutsideQuery xyPointCellRelation = iota
	// xyPointCellInsideQuery indicates the cell lies fully inside the query.
	xyPointCellInsideQuery
	// xyPointCellCrossesQuery indicates the cell partially overlaps the query.
	xyPointCellCrossesQuery
)

// newXYPointSourceFromIndexPointValues adapts the current
// metadata-only index.PointValues to the xyPointSource contract. The
// adapter is a no-op stub today (production segment readers do not
// expose visitor-driven Intersect yet); it exists so that once the
// index layer wires a real surface, this is the single place to swap
// the implementation. Matches the noop adapter pattern from
// LongDistanceFeatureQuery / longPointSource.
func newXYPointSourceFromIndexPointValues(_ index.PointValues) xyPointSource {
	return noopXYPointSource{}
}

// noopXYPointSource is the safe default for the production lookup
// while the visitor-driven PointValues surface is not yet wired. It
// matches the Java reference's "field exists but yields zero matches"
// behaviour for unmaterialised data.
type noopXYPointSource struct{}

func (noopXYPointSource) Intersect(_ xyPointVisitor) error { return nil }
func (noopXYPointSource) EstimateDocCount(_ xyPointVisitor) (int64, error) {
	return 0, nil
}

// xyPointInGeometryWeight is the per-Weight half of the query. It owns
// the pre-built Component2D tree (built once in CreateWeight) and the
// boost used as the constant per-doc score.
type xyPointInGeometryWeight struct {
	*BaseWeight

	query      *xyPointInGeometryQuery
	boost      float32
	tree       geo.Component2D
	leafLookup xyPointInGeometryLeafLookup
}

// GetQuery returns the parent query.
func (w *xyPointInGeometryWeight) GetQuery() Query { return w.query }

// IsCacheable mirrors the Java override, which returns true
// unconditionally for this query (the visitor result depends only on
// the indexed point values, which do not change for a frozen segment).
func (w *xyPointInGeometryWeight) IsCacheable(_ *index.LeafReaderContext) bool { return true }

// Count returns -1 to signal that no sub-linear count is available.
func (w *xyPointInGeometryWeight) Count(_ *index.LeafReaderContext) (int, error) { return -1, nil }

// Matches returns nil; this query does not produce per-match positions.
func (w *xyPointInGeometryWeight) Matches(_ *index.LeafReaderContext, _ int) (Matches, error) {
	return nil, nil
}

// ScorerSupplier resolves the per-leaf point source, validates the
// field shape via [document.CheckXYPointCompatible], and returns a
// ScorerSupplier whose Get builds a constant-score scorer walking the
// visitor. Returns (nil, nil) when the leaf has no source for the
// field — matching the Java fast path that returns a null Scorer.
func (w *xyPointInGeometryWeight) ScorerSupplier(ctx *index.LeafReaderContext) (ScorerSupplier, error) {
	source, fieldInfo, maxDoc, err := w.leafLookup(ctx, w.query.field)
	if err != nil {
		return nil, err
	}
	if source == nil || fieldInfo == nil {
		return nil, nil
	}
	if err := document.CheckXYPointCompatible(fieldInfo); err != nil {
		return nil, err
	}
	return &xyPointInGeometryScorerSupplier{
		weight: w,
		source: source,
		maxDoc: maxDoc,
		cost:   -1,
	}, nil
}

// Scorer is the convenience entry point that mirrors Java
// Weight.scorer(). It delegates to ScorerSupplier exactly as the
// Lucene Weight does.
func (w *xyPointInGeometryWeight) Scorer(ctx *index.LeafReaderContext) (Scorer, error) {
	supplier, err := w.ScorerSupplier(ctx)
	if err != nil {
		return nil, err
	}
	if supplier == nil {
		return nil, nil
	}
	return supplier.Get(0)
}

// Ensure xyPointInGeometryWeight implements Weight.
var _ Weight = (*xyPointInGeometryWeight)(nil)

// xyPointInGeometryScorerSupplier mirrors the inner ScorerSupplier of
// the Java reference. cost is computed lazily on the first Cost() call
// because EstimateDocCount can be expensive.
type xyPointInGeometryScorerSupplier struct {
	weight *xyPointInGeometryWeight
	source xyPointSource
	maxDoc int

	cost int64
}

// Get materializes the matching DocIdSet and wraps it in a constant-
// score scorer. Mirrors the Java reference, which builds a
// DocIdSetBuilder fresh per Get call and wraps it in
// ConstantScoreScorer(score(), scoreMode, iterator).
//
// NOTE: the canonical ConstantScoreScorer surface lives only as a
// test-file helper in search/disjunction_disi_approximation_test.go,
// so this query inlines a minimal scorer (same shape as the
// binaryRangeFieldRangeScorer pattern from GOC-3207). Once the
// production ConstantScoreScorer/ConstantScoreWeight surface is
// promoted out of the test build, this scorer is the single
// replacement site.
func (s *xyPointInGeometryScorerSupplier) Get(_ int64) (Scorer, error) {
	builder := util.NewDocIdSetBuilder(s.maxDoc)
	visitor := newXYPointInGeometryVisitor(builder, s.weight.tree)
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
			disi = newUtilToSearchDISIAdapter(utilIter)
		}
	}
	return &xyPointInGeometryScorer{
		BaseScorer: NewBaseScorer(s.weight),
		weight:     s.weight,
		iter:       disi,
		score:      s.weight.boost,
	}, nil
}

// Cost returns the upper-bound doc count, computed lazily.
func (s *xyPointInGeometryScorerSupplier) Cost() int64 {
	if s.cost == -1 {
		visitor := newXYPointInGeometryVisitor(util.NewDocIdSetBuilder(s.maxDoc), s.weight.tree)
		cost, err := s.source.EstimateDocCount(visitor)
		if err != nil || cost < 0 {
			cost = 0
		}
		s.cost = cost
	}
	return s.cost
}

// SetTopLevelScoringClause is a no-op for this constant-score supplier.
func (s *xyPointInGeometryScorerSupplier) SetTopLevelScoringClause() {}

// Ensure xyPointInGeometryScorerSupplier implements ScorerSupplier.
var _ ScorerSupplier = (*xyPointInGeometryScorerSupplier)(nil)

// xyPointInGeometryVisitor is the visitor handed to the xyPointSource.
// It mirrors the inner IntersectVisitor of the Java reference: per-doc
// adds bypass the geometry check (the cell is fully inside the query
// region), per-(doc, packedValue) adds decode the X/Y dims and run
// Component2D.Contains, and the Compare hook delegates to
// Component2D.Relate.
type xyPointInGeometryVisitor struct {
	result *util.DocIdSetBuilder
	tree   geo.Component2D
	adder  util.BulkAdder
}

// newXYPointInGeometryVisitor wires the builder/tree pair into a
// visitor instance. The adder is captured lazily on the first Grow
// call so the visitor is safe to construct cheaply for cost-only paths.
func newXYPointInGeometryVisitor(result *util.DocIdSetBuilder, tree geo.Component2D) *xyPointInGeometryVisitor {
	return &xyPointInGeometryVisitor{result: result, tree: tree}
}

// Grow asks the underlying builder for an adder sized for count docs.
// Mirrors the Java visitor's grow(int) override.
func (v *xyPointInGeometryVisitor) Grow(count int) {
	v.adder = v.result.Grow(count)
}

// ensureAdder lazily obtains an adder if the visitor was never grown.
// The Java reference always calls grow before the first add; this
// belt-and-braces fallback handles tests that bypass that contract.
func (v *xyPointInGeometryVisitor) ensureAdder() {
	if v.adder == nil {
		v.adder = v.result.Grow(0)
	}
}

// Visit adds a single docID unconditionally. Used when the source
// already knows the cell is fully inside the query.
func (v *xyPointInGeometryVisitor) Visit(docID int) error {
	v.ensureAdder()
	v.adder.Add(docID)
	return nil
}

// VisitWithPackedValue decodes the 8-byte XY payload and admits the
// doc only when the point lies inside the query tree. Mirrors the
// per-(doc, packedValue) visit on the Java reference, which decodes
// via XYEncodingUtils.decode at offsets {0, Integer.BYTES}.
func (v *xyPointInGeometryVisitor) VisitWithPackedValue(docID int, packedValue []byte) error {
	if len(packedValue) < 2*xyPointBytesPerDim {
		return fmt.Errorf("xy point visitor: packed value too short: %d", len(packedValue))
	}
	x := float64(geo.XYDecodeBytes(packedValue, 0))
	y := float64(geo.XYDecodeBytes(packedValue, xyPointBytesPerDim))
	if v.tree.Contains(x, y) {
		return v.Visit(docID)
	}
	return nil
}

// VisitIterator drains an iterator of docs that are all fully inside
// the query. Mirrors visit(DocIdSetIterator) on the Java reference.
func (v *xyPointInGeometryVisitor) VisitIterator(iter util.DocIdSetIterator) error {
	v.ensureAdder()
	return v.adder.AddIterator(iter)
}

// VisitIteratorWithPackedValue is the iterator-shaped variant that
// gates on the shared packedValue (every doc in iter has the same
// point coordinates). Mirrors visit(DocIdSetIterator, byte[]) on the
// Java reference.
func (v *xyPointInGeometryVisitor) VisitIteratorWithPackedValue(iter util.DocIdSetIterator, packedValue []byte) error {
	if len(packedValue) < 2*xyPointBytesPerDim {
		return fmt.Errorf("xy point visitor: packed value too short: %d", len(packedValue))
	}
	x := float64(geo.XYDecodeBytes(packedValue, 0))
	y := float64(geo.XYDecodeBytes(packedValue, xyPointBytesPerDim))
	if v.tree.Contains(x, y) {
		return v.VisitIterator(iter)
	}
	return nil
}

// Compare decodes the min/max packed values as (x, y) corners and asks
// the Component2D tree to relate the bounding cell. Mirrors
// compare(byte[], byte[]) on the Java reference.
func (v *xyPointInGeometryVisitor) Compare(minPackedValue, maxPackedValue []byte) xyPointCellRelation {
	if len(minPackedValue) < 2*xyPointBytesPerDim || len(maxPackedValue) < 2*xyPointBytesPerDim {
		// Mirrors Java's array-bounds failure: a malformed cell payload
		// is a programmer error. The safe answer here is "crosses"
		// (force the source to recurse and surface the bug downstream)
		// rather than silently dropping the cell.
		return xyPointCellCrossesQuery
	}
	cellMinX := float64(geo.XYDecodeBytes(minPackedValue, 0))
	cellMinY := float64(geo.XYDecodeBytes(minPackedValue, xyPointBytesPerDim))
	cellMaxX := float64(geo.XYDecodeBytes(maxPackedValue, 0))
	cellMaxY := float64(geo.XYDecodeBytes(maxPackedValue, xyPointBytesPerDim))
	return xyRelationFromGeo(v.tree.Relate(cellMinX, cellMaxX, cellMinY, cellMaxY))
}

// xyPointBytesPerDim mirrors Integer.BYTES (4): the byte-width of a
// single XY dimension in the packed payload. The dimension layout is
// 4-byte X followed by 4-byte Y, matching XYPointField.
const xyPointBytesPerDim = 4

// xyRelationFromGeo maps geo.Relation onto the local
// xyPointCellRelation enum. The two enums carry identical semantics;
// the local enum exists so the query surface stays decoupled from the
// geo package (which a future PointValues port may not want to depend
// on transitively).
func xyRelationFromGeo(r geo.Relation) xyPointCellRelation {
	switch r {
	case geo.CellInsideQuery:
		return xyPointCellInsideQuery
	case geo.CellCrossesQuery:
		return xyPointCellCrossesQuery
	default:
		return xyPointCellOutsideQuery
	}
}

// xyPointInGeometryScorer is the constant-score scorer returned by
// the supplier. It mirrors the inner ConstantScoreScorer wrapping on
// the Java reference: score() returns the boost, and the iterator
// forwards every position/cost call to the materialized DocIdSet's
// iterator.
type xyPointInGeometryScorer struct {
	*BaseScorer

	weight *xyPointInGeometryWeight
	iter   DocIdSetIterator
	score  float32
}

// DocID forwards to the underlying iterator.
func (s *xyPointInGeometryScorer) DocID() int { return s.iter.DocID() }

// NextDoc advances the scorer to the next matching document.
func (s *xyPointInGeometryScorer) NextDoc() (int, error) { return s.iter.NextDoc() }

// Advance moves the scorer to the first matching document at or beyond target.
func (s *xyPointInGeometryScorer) Advance(target int) (int, error) {
	return s.iter.Advance(target)
}

// Cost returns the underlying iterator's cost estimate.
func (s *xyPointInGeometryScorer) Cost() int64 { return s.iter.Cost() }

// DocIDRunEnd returns the end of the current run.
func (s *xyPointInGeometryScorer) DocIDRunEnd() int { return s.iter.DocIDRunEnd() }

// Score returns the constant boost score.
func (s *xyPointInGeometryScorer) Score() float32 { return s.score }

// GetMaxScore returns the constant boost score (no per-doc variability).
func (s *xyPointInGeometryScorer) GetMaxScore(_ int) float32 { return s.score }

// Ensure xyPointInGeometryScorer implements Scorer.
var _ Scorer = (*xyPointInGeometryScorer)(nil)

// newUtilToSearchDISIAdapter bridges a util.DocIdSetIterator to the
// search.DocIdSetIterator contract. Both iterators are structurally
// identical (DocID/NextDoc/Advance/Cost/DocIDRunEnd); only the package
// differs, so the adapter is a thin forwarder. Mirrors the same
// adapter pattern in long_distance_feature_query.go but kept local so
// the two query files have no hidden coupling.
func newUtilToSearchDISIAdapter(inner util.DocIdSetIterator) DocIdSetIterator {
	return &xyUtilDISIAdapter{inner: inner}
}

type xyUtilDISIAdapter struct {
	inner util.DocIdSetIterator
}

func (a *xyUtilDISIAdapter) DocID() int                      { return a.inner.DocID() }
func (a *xyUtilDISIAdapter) NextDoc() (int, error)           { return a.inner.NextDoc() }
func (a *xyUtilDISIAdapter) Advance(target int) (int, error) { return a.inner.Advance(target) }
func (a *xyUtilDISIAdapter) Cost() int64                     { return a.inner.Cost() }
func (a *xyUtilDISIAdapter) DocIDRunEnd() int                { return a.inner.DocIDRunEnd() }

var _ DocIdSetIterator = (*xyUtilDISIAdapter)(nil)

// xyGeometriesHash mirrors java.util.Arrays.hashCode on a reference
// array: seed at 1, fold each element via 31*h + element.hashCode().
// Element hashCode is approximated by hashing the %v representation —
// stable for the immutable XYGeometry types in geo/.
func xyGeometriesHash(geoms []geo.XYGeometry) int {
	h := int32(1)
	for _, g := range geoms {
		gh := int32(0)
		if g != nil {
			gh = int32(stringHash(fmt.Sprintf("%v", g)))
		}
		h = 31*h + gh
	}
	return int(h)
}

// classHashXYPointInGeometryQuery seeds the hash with a type-stable
// constant. Distinct from other query class hashes so two different
// query types with the same field/payload do not collide.
const classHashXYPointInGeometryQuery = 0x7879_7069 // "xypi"
