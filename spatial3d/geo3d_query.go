// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial3d

import (
	"fmt"
	"math"

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
// Pruning capability (rmp #4768): Lucene's Compare consults an XYZSolid built
// from the shape's XYZBounds to relate each BKD cell to the shape, and Visit
// pre-filters points against the rounded XYZBounds box. Gocene enables this
// path ONLY for shapes whose XYZBounds is a proven complete superset (see
// shapeBoundsAreCompleteSuperset) — currently circles and degenerate points,
// whose getBounds uses only Plane.recordBounds (ported in #4768) and addPoint.
// For those shapes:
//   - Compare returns CELL_OUTSIDE_QUERY when the shape's (rounded) XYZBounds
//     box and the cell box are axis-aligned disjoint, otherwise
//     CELL_CROSSES_QUERY. CELL_INSIDE_QUERY is intentionally never returned —
//     answering CROSSES instead only costs an IsWithin call per point and never
//     changes the document set.
//   - VisitByPackedValue first rejects points outside the rounded XYZBounds box
//     (a safe early-out, since the box is a superset of every in-shape point),
//     then applies shape.IsWithin as the authoritative gate.
//
// For all other shapes (bbox/polygon, whose bounds need the still-deferred
// two-plane Plane.recordBounds intersection variant), pruning is disabled:
// Compare always returns CELL_CROSSES_QUERY and VisitByPackedValue gates purely
// on shape.IsWithin. The matched document set is identical either way because
// IsWithin is the same authoritative final gate.
type PointInShapeIntersectVisitor struct {
	hits        *util.DocIdSetBuilder
	shape       geom.GeoShape
	planetModel *geom.PlanetModel
	adder       util.BulkAdder

	// pruneCapable is true when the shape's XYZBounds is a proven complete
	// superset, enabling the rounded-box pre-check and Compare pruning below.
	pruneCapable bool
	// Rounded XYZBounds box (only meaningful when pruneCapable). These mirror
	// the minimumX..maximumZ fields the Java visitor computes from
	// DocValueEncoder.roundDownX/roundUpX.
	minimumX, maximumX float64
	minimumY, maximumY float64
	minimumZ, maximumZ float64
}

// NewPointInShapeIntersectVisitor constructs a visitor that adds matching docs
// to hits. The planetModel decodes the indexed dimensions; it must be the same
// model used at index time. hits may be nil for cost-estimation visitors that
// only need Compare.
//
// Port of the PointInShapeIntersectVisitor constructor. When the shape's
// XYZBounds is a proven complete superset, the rounded-XYZ box is materialised
// for the Compare/Visit prune path; otherwise the visitor runs in full-scan
// mode (Compare always CROSSES).
func NewPointInShapeIntersectVisitor(hits *util.DocIdSetBuilder, shape geom.GeoShape, planetModel *geom.PlanetModel) *PointInShapeIntersectVisitor {
	if planetModel == nil {
		planetModel = planetModelOf(shape)
	}
	v := &PointInShapeIntersectVisitor{
		hits:        hits,
		shape:       shape,
		planetModel: planetModel,
	}
	if shapeBoundsAreCompleteSuperset(shape) {
		bounds := geom.NewXYZBounds()
		shape.GetBounds(bounds)
		// Require a fully-populated box; otherwise fall back to full scan.
		if bounds.HasX() && bounds.HasY() && bounds.HasZ() {
			v.pruneCapable = true
			step := docValueStep(planetModel)
			v.minimumX = bounds.MinimumX - step
			v.maximumX = bounds.MaximumX + step
			v.minimumY = bounds.MinimumY - step
			v.maximumY = bounds.MaximumY + step
			v.minimumZ = bounds.MinimumZ - step
			v.maximumZ = bounds.MaximumZ + step
		}
	}
	return v
}

// shapeBoundsAreCompleteSuperset reports whether shape.GetBounds produces an
// XYZBounds that is a proven complete superset of the shape, making it safe to
// use as a BKD prefilter.
//
// This holds exactly for shapes whose getBounds uses only the single-plane
// Plane.recordBounds variant (ported in rmp #4768) and addPoint, with no
// Membership-bounded planes and no addIntersection. Today those are
// GeoStandardCircle and GeoDegeneratePoint. Shapes that need the deferred
// two-plane intersection variant (GeoRectangle / bbox, GeoConvexPolygon,
// GeoConcavePolygon) are excluded: their bounds can under-approximate, so they
// must keep the full-scan behaviour.
func shapeBoundsAreCompleteSuperset(shape geom.GeoShape) bool {
	switch shape.(type) {
	case *geom.GeoStandardCircle, *geom.GeoDegeneratePoint:
		return true
	default:
		return false
	}
}

// docValueStep returns the per-axis rounding step used to widen the shape's
// XYZBounds into the visitor's pre-filter box. It mirrors Lucene's
// DocValueEncoder.roundDownX/roundUpX, which add/subtract
// inverseFactor*STEP_FUDGE where inverseFactor = (max-min)/0x1FFFFF and
// STEP_FUDGE = 10. The three axes share the same span (the planet box is
// symmetric per axis up to the xy/z scaling), so a single step suffices for the
// superset widening; using the larger of the two spans keeps the box a superset
// on every axis.
func docValueStep(pm *geom.PlanetModel) float64 {
	const inverseMaxValue = 1.0 / float64(0x1FFFFF)
	const stepFudge = 10.0
	xySpan := pm.GetMaximumXValue() - pm.GetMinimumXValue()
	zSpan := pm.GetMaximumZValue() - pm.GetMinimumZValue()
	span := xySpan
	if zSpan > span {
		span = zSpan
	}
	return span * inverseMaxValue * stepFudge
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
// Port of PointInShapeIntersectVisitor.visit(int, byte[]). For prune-capable
// shapes the rounded-XYZBounds pre-check (x >= minimumX && ...) runs first — a
// safe early-out because the box is a proven superset of every in-shape point,
// so it can only reject points the shape would also reject. shape.IsWithin is
// the authoritative gate and yields the identical document set in both modes.
func (v *PointInShapeIntersectVisitor) VisitByPackedValue(docID int, packedValue []byte) error {
	if len(packedValue) != 3*bytesPerDim {
		return fmt.Errorf("geo3d: PointInShapeIntersectVisitor: packed value length %d, want %d", len(packedValue), 3*bytesPerDim)
	}
	x := DecodeDimension(v.planetModel, packedValue, 0)
	y := DecodeDimension(v.planetModel, packedValue, bytesPerDim)
	z := DecodeDimension(v.planetModel, packedValue, 2*bytesPerDim)
	if v.pruneCapable {
		if x < v.minimumX || x > v.maximumX ||
			y < v.minimumY || y > v.maximumY ||
			z < v.minimumZ || z > v.maximumZ {
			return nil
		}
	}
	if membership, ok := v.shape.(geom.Membership); ok && membership.IsWithin(x, y, z) {
		if v.adder != nil {
			v.adder.Add(docID)
		}
	}
	return nil
}

// Compare relates a BKD cell to the query shape.
//
// For prune-capable shapes (see the type doc), it decodes the cell box to the
// largest un-quantized range that could round into the packed bounds and
// returns CELL_OUTSIDE_QUERY when that box is axis-aligned disjoint from the
// shape's rounded XYZBounds box, otherwise CELL_CROSSES_QUERY. CELL_INSIDE_QUERY
// is never returned: answering CROSSES instead only costs an IsWithin call per
// point and never changes the document set. For non-prune-capable shapes it
// always returns CELL_CROSSES_QUERY (full scan).
//
// Port of PointInShapeIntersectVisitor.compare, scoped to the disjoint/cross
// cases (rmp #4768); the XYZSolid.getRelationship CONTAINS/WITHIN cases are
// deferred with the two-plane recordBounds variant.
func (v *PointInShapeIntersectVisitor) Compare(minPackedValue, maxPackedValue []byte) int {
	if !v.pruneCapable {
		return geo3dCellCrossesQuery
	}
	if len(minPackedValue) != 3*bytesPerDim || len(maxPackedValue) != 3*bytesPerDim {
		// Malformed cell: never prune.
		return geo3dCellCrossesQuery
	}
	xMin := decodeValueFloor(v.planetModel, minPackedValue, 0)
	xMax := decodeValueCeil(v.planetModel, maxPackedValue, 0)
	yMin := decodeValueFloor(v.planetModel, minPackedValue, bytesPerDim)
	yMax := decodeValueCeil(v.planetModel, maxPackedValue, bytesPerDim)
	zMin := decodeValueFloor(v.planetModel, minPackedValue, 2*bytesPerDim)
	zMax := decodeValueCeil(v.planetModel, maxPackedValue, 2*bytesPerDim)

	// Axis-aligned disjointness of the cell box and the shape's bounds box.
	if v.maximumX < xMin || v.minimumX > xMax ||
		v.maximumY < yMin || v.minimumY > yMax ||
		v.maximumZ < zMin || v.minimumZ > zMax {
		return geo3dCellOutsideQuery
	}
	return geo3dCellCrossesQuery
}

// decodeValueFloor returns the smallest coordinate that quantizes to the int
// encoded at offset in packed, extending the inclusive cell lower bound to the
// largest un-quantized value range.
//
// Port of org.apache.lucene.spatial3d.Geo3DUtil.decodeValueFloor.
func decodeValueFloor(pm *geom.PlanetModel, packed []byte, offset int) float64 {
	x := util.SortableBytesToInt(packed, offset)
	if x == pm.MinEncodedValue {
		return -pm.MaxValue
	}
	return float64(x) * pm.Decode
}

// decodeValueCeil returns the largest coordinate that quantizes to the int
// encoded at offset in packed, extending the inclusive cell upper bound.
//
// Port of org.apache.lucene.spatial3d.Geo3DUtil.decodeValueCeil.
func decodeValueCeil(pm *geom.PlanetModel, packed []byte, offset int) float64 {
	x := util.SortableBytesToInt(packed, offset)
	if x == pm.MaxEncodedValue {
		return pm.MaxValue
	}
	return math.Nextafter(float64(x+1)*pm.Decode, math.Inf(-1))
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
