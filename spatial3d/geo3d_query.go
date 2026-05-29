// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial3d

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spatial3d/geom"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ---------------------------------------------------------------------------
// PointInGeo3DShapeQuery
//
// Port of org.apache.lucene.spatial3d.PointInGeo3DShapeQuery.
// ---------------------------------------------------------------------------

// PointInGeo3DShapeQuery finds all previously indexed Geo3DPoint values that
// fall within the supplied GeoShape. The field must have been indexed with
// Geo3DPoint (3 dimensions × 4 bytes).
//
// The query is a ConstantScore query: every matching document receives the
// same score (the query boost). Matching is delegated to a BKD walk driven by
// PointInShapeIntersectVisitor; the authoritative per-document gate is
// geom.GeoShape.IsWithin, so the produced document set is exactly the set of
// docs whose decoded point satisfies IsWithin.
//
// Port of org.apache.lucene.spatial3d.PointInGeo3DShapeQuery.
type PointInGeo3DShapeQuery struct {
	field       string
	shape       geom.GeoShape
	planetModel *geom.PlanetModel
}

// NewPointInGeo3DShapeQuery constructs a PointInGeo3DShapeQuery over field for
// shape. The shape's PlanetModel (resolved through the geom.PlanetObject
// interface, falling back to geom.SPHERE) is used to decode indexed points.
//
// Port of org.apache.lucene.spatial3d.PointInGeo3DShapeQuery(String,GeoShape).
// Lucene also materialises an XYZBounds for the shape here
// (shape.getBounds(shapeBounds)); Gocene defers the bounds-driven BKD
// prefilter to rmp #4768 (the XYZBounds engine is still stubbed) and instead
// gates every visited point through IsWithin — see Compare below.
func NewPointInGeo3DShapeQuery(field string, shape geom.GeoShape) *PointInGeo3DShapeQuery {
	return &PointInGeo3DShapeQuery{
		field:       field,
		shape:       shape,
		planetModel: planetModelOf(shape),
	}
}

// planetModelOf returns the PlanetModel associated with shape, or geom.SPHERE
// when the shape does not expose one. All concrete geom shapes embed
// BasePlanetObject and therefore satisfy geom.PlanetObject.
func planetModelOf(shape geom.GeoShape) *geom.PlanetModel {
	if po, ok := shape.(geom.PlanetObject); ok {
		if pm := po.GetPlanetModel(); pm != nil {
			return pm
		}
	}
	return geom.SPHERE
}

// GetField returns the field name.
//
// Port of PointInGeo3DShapeQuery.getField.
func (q *PointInGeo3DShapeQuery) GetField() string { return q.field }

// GetShape returns the query shape.
//
// Port of PointInGeo3DShapeQuery.getShape.
func (q *PointInGeo3DShapeQuery) GetShape() geom.GeoShape { return q.shape }

// Rewrite returns the query unchanged.
//
// PointInGeo3DShapeQuery operates on an inverted (BKD) structure and never
// rewrites to a different query form, mirroring the Java reference which does
// not override rewrite.
func (q *PointInGeo3DShapeQuery) Rewrite(_ search.IndexReader) (search.Query, error) {
	return q, nil
}

// Clone creates a copy of this query.
func (q *PointInGeo3DShapeQuery) Clone() search.Query {
	return &PointInGeo3DShapeQuery{
		field:       q.field,
		shape:       q.shape,
		planetModel: q.planetModel,
	}
}

// Equals reports whether other is a PointInGeo3DShapeQuery with the same field
// and shape.
//
// Port of PointInGeo3DShapeQuery.equalsTo.
func (q *PointInGeo3DShapeQuery) Equals(other search.Query) bool {
	o, ok := other.(*PointInGeo3DShapeQuery)
	if !ok {
		return false
	}
	return q.field == o.field && q.shape == o.shape
}

// HashCode returns a hash code derived from the field and shape.
//
// Port of PointInGeo3DShapeQuery.hashCode (classHash folded with field and
// shape hashes).
func (q *PointInGeo3DShapeQuery) HashCode() int {
	h := classHashPointInGeo3DShapeQuery
	for _, c := range q.field {
		h = h*31 + int(c)
	}
	h = h*31 + stringHashGeo3D(fmt.Sprintf("%v", q.shape))
	return h
}

// classHashPointInGeo3DShapeQuery seeds HashCode with a type-stable constant so
// two distinct query types with the same field/shape do not collide. The value
// spells "g3dq" in ASCII.
const classHashPointInGeo3DShapeQuery = 0x6733_6471

// stringHashGeo3D mirrors java.lang.String.hashCode (h = 31*h + char) on a
// string. Used to fold the shape's representation into the query hash.
func stringHashGeo3D(s string) int {
	h := int32(0)
	for _, c := range s {
		h = 31*h + int32(c)
	}
	return int(h)
}

// CreateWeight builds a ConstantScore Weight for this query.
//
// Port of PointInGeo3DShapeQuery.createWeight: a ConstantScoreWeight whose
// scorerSupplier pulls the leaf's PointValues, walks the BKD tree with
// PointInShapeIntersectVisitor into a DocIdSetBuilder, and wraps the resulting
// iterator in a ConstantScoreScorer.
func (q *PointInGeo3DShapeQuery) CreateWeight(searcher *search.IndexSearcher, needsScores bool, boost float32) (search.Weight, error) {
	mode := search.COMPLETE
	if !needsScores {
		mode = search.COMPLETE_NO_SCORES
	}
	return &pointInGeo3DShapeWeight{
		BaseWeight: search.NewBaseWeight(q),
		query:      q,
		score:      boost,
		scoreMode:  mode,
	}, nil
}

// Visit dispatches this leaf query to the visitor when it accepts the field.
//
// Port of PointInGeo3DShapeQuery.visit.
func (q *PointInGeo3DShapeQuery) Visit(visitor search.QueryVisitor) {
	if visitor.AcceptField(q.field) {
		visitor.VisitLeaf(q)
	}
}

// String returns a human-readable representation.
//
// Port of PointInGeo3DShapeQuery.toString.
func (q *PointInGeo3DShapeQuery) String() string {
	return fmt.Sprintf("PointInGeo3DShapeQuery: field=%s: Shape: %v", q.field, q.shape)
}

// Ensure PointInGeo3DShapeQuery implements search.Query.
var _ search.Query = (*PointInGeo3DShapeQuery)(nil)

// ---------------------------------------------------------------------------
// pointInGeo3DShapeWeight
// ---------------------------------------------------------------------------

// pointInGeo3DShapeWeight is the Weight implementation for
// PointInGeo3DShapeQuery. It is a constant-score weight: ScorerSupplier walks
// the BKD tree once and emits the matching doc ids at the constant score.
type pointInGeo3DShapeWeight struct {
	*search.BaseWeight
	query     *PointInGeo3DShapeQuery
	score     float32
	scoreMode search.ScoreMode
}

// ScorerSupplier returns a lazy supplier that walks the leaf's BKD point tree.
//
// Port of the ConstantScoreWeight.scorerSupplier returned from
// PointInGeo3DShapeQuery.createWeight. Returns nil (no matches on this leaf)
// when the field exposes no point values, mirroring the Java reference's
// `if (values == null) return null`.
func (w *pointInGeo3DShapeWeight) ScorerSupplier(context *index.LeafReaderContext) (search.ScorerSupplier, error) {
	reader := context.LeafReader()
	if reader == nil {
		return nil, nil
	}
	pv, ok := getGeo3DPointValues(reader, w.query.field)
	if !ok || pv == nil {
		return nil, nil
	}
	return &pointInGeo3DShapeScorerSupplier{
		pv:        pv,
		query:     w.query,
		maxDoc:    reader.MaxDoc(),
		score:     w.score,
		scoreMode: w.scoreMode,
		estCost:   -1,
	}, nil
}

// Scorer delegates to ScorerSupplier.Get.
func (w *pointInGeo3DShapeWeight) Scorer(context *index.LeafReaderContext) (search.Scorer, error) {
	supplier, err := w.ScorerSupplier(context)
	if err != nil || supplier == nil {
		return nil, err
	}
	return supplier.Get(0)
}

// BulkScorer wraps the per-leaf scorer in the default bulk scorer.
func (w *pointInGeo3DShapeWeight) BulkScorer(context *index.LeafReaderContext) (search.BulkScorer, error) {
	scorer, err := w.Scorer(context)
	if err != nil || scorer == nil {
		return nil, err
	}
	return search.NewDefaultBulkScorer(scorer), nil
}

// Explain reports a constant-score match or a no-match for doc.
//
// Mirrors ConstantScoreWeight.explain: a hit yields the constant score with
// the query string as description; a miss yields "<query> doesn't match id N".
func (w *pointInGeo3DShapeWeight) Explain(context *index.LeafReaderContext, doc int) (search.Explanation, error) {
	scorer, err := w.Scorer(context)
	if err != nil {
		return nil, err
	}
	if scorer != nil {
		advanced, err := scorer.Advance(doc)
		if err != nil {
			return nil, err
		}
		if advanced == doc {
			return search.MatchExplanation(scorer.Score(), w.query.String()), nil
		}
	}
	return search.NoMatchExplanation(fmt.Sprintf("%s doesn't match id %d", w.query, doc)), nil
}

// IsCacheable returns true: the query result depends only on the field's
// indexed points, mirroring isCacheable returning true in the Java reference.
func (w *pointInGeo3DShapeWeight) IsCacheable(_ *index.LeafReaderContext) bool { return true }

// Ensure pointInGeo3DShapeWeight implements search.Weight.
var _ search.Weight = (*pointInGeo3DShapeWeight)(nil)

// ---------------------------------------------------------------------------
// pointInGeo3DShapeScorerSupplier
// ---------------------------------------------------------------------------

// pointInGeo3DShapeScorerSupplier lazily builds the per-leaf ConstantScoreScorer
// by walking the BKD tree with PointInShapeIntersectVisitor.
type pointInGeo3DShapeScorerSupplier struct {
	search.BaseScorerSupplier
	pv        geo3dPointValues
	query     *PointInGeo3DShapeQuery
	maxDoc    int
	score     float32
	scoreMode search.ScoreMode
	estCost   int64
}

// Get walks the BKD tree, gating each point through shape.IsWithin, and returns
// a ConstantScoreScorer over the matching documents.
func (s *pointInGeo3DShapeScorerSupplier) Get(_ int64) (search.Scorer, error) {
	builder := util.NewDocIdSetBuilder(s.maxDoc)
	visitor := NewPointInShapeIntersectVisitor(builder, s.query.shape, s.query.planetModel)
	if err := s.pv.Intersect(visitor); err != nil {
		return nil, err
	}
	docSet, err := builder.Build()
	if err != nil {
		return nil, err
	}
	if docSet == nil {
		return search.NewConstantScoreScorer(s.score, s.scoreMode, search.NewEmptyDocIdSetIterator()), nil
	}
	iter := docSet.Iterator()
	if iter == nil {
		return search.NewConstantScoreScorer(s.score, s.scoreMode, search.NewEmptyDocIdSetIterator()), nil
	}
	return search.NewConstantScoreScorer(s.score, s.scoreMode, newGeo3DUtilDISIAdapter(iter)), nil
}

// Cost returns a lazy, cached estimate of the matching-document count.
func (s *pointInGeo3DShapeScorerSupplier) Cost() int64 {
	if s.estCost < 0 {
		c := s.pv.EstimatePointCount(NewPointInShapeIntersectVisitor(nil, s.query.shape, s.query.planetModel))
		if c < 0 {
			c = 0
		}
		s.estCost = c
	}
	return s.estCost
}

// Ensure pointInGeo3DShapeScorerSupplier implements search.ScorerSupplier.
var _ search.ScorerSupplier = (*pointInGeo3DShapeScorerSupplier)(nil)

// ---------------------------------------------------------------------------
// PointInShapeIntersectVisitor
//
// Port of org.apache.lucene.spatial3d.PointInShapeIntersectVisitor.
// ---------------------------------------------------------------------------

// Cell-relation constants matching the order of codecs.Relation /
// index.PointValues.Relation. They are declared locally so this package does
// not import codecs (which would draw in the codecs → document → search
// dependency chain). Adapters between this enum and codecs.Relation are pure
// switches with no semantic difference.
const (
	geo3dCellInsideQuery  = 1 // CELL_INSIDE_QUERY
	geo3dCellOutsideQuery = 0 // CELL_OUTSIDE_QUERY
	geo3dCellCrossesQuery = 2 // CELL_CROSSES_QUERY
)

// PointInShapeIntersectVisitor walks BKD nodes, admitting each visited point to
// the DocIdSetBuilder iff the GeoShape contains the decoded XYZ coordinate.
//
// Port of org.apache.lucene.spatial3d.PointInShapeIntersectVisitor.
//
// Deviation (rmp #4768): Lucene's Compare extends the quantized cell bounds and
// consults an XYZSolid built from the shape's XYZBounds to prune sub-trees.
// Gocene's XYZBounds engine is stubbed, so Compare always returns
// CELL_CROSSES_QUERY: the BKD walk descends into every leaf and every point is
// gated by shape.IsWithin. This is a performance-only difference — the matched
// document set is identical to Lucene's because IsWithin is the same
// authoritative final gate Lucene applies in Visit.
type PointInShapeIntersectVisitor struct {
	hits        *util.DocIdSetBuilder
	shape       geom.GeoShape
	planetModel *geom.PlanetModel
	adder       util.BulkAdder
}

// NewPointInShapeIntersectVisitor constructs a visitor that adds matching docs
// to hits. The planetModel decodes the indexed dimensions; it must be the same
// model used at index time. hits may be nil for cost-estimation visitors that
// only need Compare.
//
// Port of the PointInShapeIntersectVisitor constructor. The bounds-derived
// rounded-XYZ fields are omitted because Compare does not use them under the
// rmp #4768 deviation.
func NewPointInShapeIntersectVisitor(hits *util.DocIdSetBuilder, shape geom.GeoShape, planetModel *geom.PlanetModel) *PointInShapeIntersectVisitor {
	if planetModel == nil {
		planetModel = planetModelOf(shape)
	}
	return &PointInShapeIntersectVisitor{
		hits:        hits,
		shape:       shape,
		planetModel: planetModel,
	}
}

// Grow reserves capacity for count more documents.
//
// Port of PointInShapeIntersectVisitor.grow.
func (v *PointInShapeIntersectVisitor) Grow(count int) {
	if v.hits != nil {
		v.adder = v.hits.Grow(count)
	}
}

// Visit admits docID unconditionally; it is only called for points whose
// enclosing cell Compare classified as CELL_INSIDE_QUERY. Under the rmp #4768
// deviation Compare never returns CELL_INSIDE_QUERY, so this path is not taken
// today, but it is implemented faithfully for when bounds pruning is restored.
//
// Port of PointInShapeIntersectVisitor.visit(int).
func (v *PointInShapeIntersectVisitor) Visit(docID int) error {
	if v.adder != nil {
		v.adder.Add(docID)
	}
	return nil
}

// VisitByPackedValue decodes the 12-byte (3 × 4) packed value and admits docID
// iff the shape contains the point.
//
// Port of PointInShapeIntersectVisitor.visit(int, byte[]). The bounding-box
// pre-check (x >= minimumX && ... ) is dropped because the bounds engine is
// stubbed (rmp #4768); shape.IsWithin alone is the authoritative gate and
// yields the identical document set.
func (v *PointInShapeIntersectVisitor) VisitByPackedValue(docID int, packedValue []byte) error {
	if len(packedValue) != 3*bytesPerDim {
		return fmt.Errorf("geo3d: PointInShapeIntersectVisitor: packed value length %d, want %d", len(packedValue), 3*bytesPerDim)
	}
	x := DecodeDimension(v.planetModel, packedValue, 0)
	y := DecodeDimension(v.planetModel, packedValue, bytesPerDim)
	z := DecodeDimension(v.planetModel, packedValue, 2*bytesPerDim)
	if membership, ok := v.shape.(geom.Membership); ok && membership.IsWithin(x, y, z) {
		if v.adder != nil {
			v.adder.Add(docID)
		}
	}
	return nil
}

// Compare always reports CELL_CROSSES_QUERY so the BKD walk visits every leaf
// and every point is gated by VisitByPackedValue → shape.IsWithin.
//
// Deviation (rmp #4768): Lucene prunes sub-trees here using the shape's
// XYZBounds via an XYZSolid relationship test. Gocene's bounds engine is
// stubbed, so returning CELL_CROSSES_QUERY unconditionally trades pruning for
// exact correctness — see the type doc.
//
// Port of PointInShapeIntersectVisitor.compare (degenerate form).
func (v *PointInShapeIntersectVisitor) Compare(_, _ []byte) int {
	return geo3dCellCrossesQuery
}

// ---------------------------------------------------------------------------
// PointValues bridge
// ---------------------------------------------------------------------------

// geo3dPointValues is the narrow point-source contract PointInGeo3DShapeQuery
// needs from a leaf. It is declared locally (mirroring pointRangePointValues in
// search/point_range_query.go) because the canonical index.PointValues port
// does not yet expose Intersect / EstimatePointCount, and importing codecs
// would re-introduce a dependency cycle.
type geo3dPointValues interface {
	Intersect(visitor geo3dIntersectVisitor) error
	EstimatePointCount(visitor geo3dIntersectVisitor) int64
}

// geo3dIntersectVisitor is the BKD visitor surface geo3dPointValues drives. It
// is an alias of index.PointTreeIntersectVisitor (rmp #4769) so the on-disk
// BKD-backed PointValues returned by LeafReader.GetPointValues — whose
// Intersect method takes index.PointTreeIntersectVisitor — satisfies
// geo3dPointValues. The three hooks (Visit, VisitByPackedValue, Compare
// returning an int) plus Grow match the codecs.IntersectVisitor surface so the
// BKD walk drives this visitor without further adaptation.
type geo3dIntersectVisitor = index.PointTreeIntersectVisitor

// getGeo3DPointValues type-asserts the leaf reader to expose BKD point values
// for field, then narrows them to geo3dPointValues. Returns (nil, false) when
// the reader does not expose point values or the field is absent — interpreted
// by the Weight as "no matches on this leaf".
//
// Until LeafReader.GetPointValues serves on-disk Geo3DPoint values (rmp #4769),
// the only types satisfying geo3dPointValues are in-memory test stubs.
func getGeo3DPointValues(reader index.LeafReaderInterface, field string) (geo3dPointValues, bool) {
	type pvProvider interface {
		GetPointValues(field string) (index.PointValues, error)
	}
	pvp, ok := reader.(pvProvider)
	if !ok {
		return nil, false
	}
	raw, err := pvp.GetPointValues(field)
	if err != nil || raw == nil {
		return nil, false
	}
	pv, ok := raw.(geo3dPointValues)
	return pv, ok
}

// ---------------------------------------------------------------------------
// DocIdSetIterator bridge
// ---------------------------------------------------------------------------

// newGeo3DUtilDISIAdapter bridges a util.DocIdSetIterator to the
// search.DocIdSetIterator contract. Both interfaces are structurally identical
// (DocID/NextDoc/Advance/Cost/DocIDRunEnd); only the package differs, so the
// adapter is a thin forwarder. Mirrors newUtilToSearchDISIAdapter in
// search/xy_point_in_geometry_query.go, kept local because that one is
// package-private to search.
func newGeo3DUtilDISIAdapter(inner util.DocIdSetIterator) search.DocIdSetIterator {
	return &geo3dUtilDISIAdapter{inner: inner}
}

type geo3dUtilDISIAdapter struct {
	inner util.DocIdSetIterator
}

func (a *geo3dUtilDISIAdapter) DocID() int                      { return a.inner.DocID() }
func (a *geo3dUtilDISIAdapter) NextDoc() (int, error)           { return a.inner.NextDoc() }
func (a *geo3dUtilDISIAdapter) Advance(target int) (int, error) { return a.inner.Advance(target) }
func (a *geo3dUtilDISIAdapter) Cost() int64                     { return a.inner.Cost() }
func (a *geo3dUtilDISIAdapter) DocIDRunEnd() int                { return a.inner.DocIDRunEnd() }

var _ search.DocIdSetIterator = (*geo3dUtilDISIAdapter)(nil)
