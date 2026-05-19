// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"errors"
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// LatLonShapeBoundingBoxQuery is the fast-path bounding-box variant of
// LatLonShapeQuery. It matches every indexed shape whose triangles
// relate to the supplied geo.Rectangle according to the requested
// QueryRelation. The field must have been indexed with
// LatLonShape.CreateIndexableFields per document.
//
// Mirrors the package-private final class
// org.apache.lucene.document.LatLonShapeBoundingBoxQuery (Lucene
// 10.4.0).
//
// # Composition vs inheritance
//
// The Java reference extends SpatialQuery and overrides the two
// abstract hooks (createComponent2D, getSpatialVisitor). Gocene's
// SpatialQuery is a concrete struct that captures the
// queryComponent2D and a SpatialVisitor factory at construction time,
// so LatLonShapeBoundingBoxQuery embeds *SpatialQuery and exposes
// the rectangle-specific construction through
// NewLatLonShapeBoundingBoxQuery.
//
// # Fast-path rationale
//
// LatLonShapeQuery walks every cell through the full Component2D
// tree. For an axis-aligned rectangle the relate step degenerates to
// four byte-level integer compares, so this query bypasses the tree
// and consults a dedicated EncodedLatLonRectangle that operates
// directly on the sortable-bytes BKD layout. The Intersects /
// Within / Contains branches still decode the triangle vertices but
// then dispatch through the rectangle-specific helpers
// (EncodedRectangle.Contains / IntersectsLine / etc.) instead of the
// tree.
//
// # Decoded triangle layout caveat
//
// The Java reference's per-doc predicates read the full vertex set
// (A, B, C) plus the three edge-membership bits from a
// ShapeField.DecodedTriangle. The current Gocene
// document.DecodeTriangle implements a simplified layout that
// recovers only the A vertex and the three edge bits; B and C are
// left at zero. POINT (single-vertex) triangles round-trip
// correctly, so the visitor handles every type but the LINE and
// TRIANGLE branches will operate against the partially-recovered
// vertices until the full rotation-aware decoder lands (backlog
// #2697). The visitor shape and dispatch match the Java reference
// in every other respect so the upgrade is a pure decoder swap.
//
// # Dateline-crossing CONTAINS caveat
//
// The Java reference's Contains() throws IllegalArgumentException
// when the bounding box crosses the dateline, because the
// "rectangle inside triangle" test is meaningless for a
// disconnected query region. Gocene mirrors that behaviour: the
// Contains() factory panics at first dispatch on a dateline-crossing
// rectangle. Callers that need CONTAINS over a wrapped rectangle
// should split the query into east/west halves and BooleanQuery
// them together.
type LatLonShapeBoundingBoxQuery struct {
	*SpatialQuery

	rectangle geo.Rectangle
}

// ErrLatLonShapeBoundingBoxQueryContainsDateline is returned by the
// Contains() per-doc factory when the query rectangle crosses the
// dateline. Mirrors the Java reference's IllegalArgumentException
// message "withinTriangle is not supported for rectangles crossing
// the date line".
var ErrLatLonShapeBoundingBoxQueryContainsDateline = errors.New(
	"search: LatLonShapeBoundingBoxQuery does not support CONTAINS for rectangles crossing the date line",
)

// NewLatLonShapeBoundingBoxQuery builds the bounding-box query for
// the given field / relation / geo.Rectangle. The rectangle's
// latitude bounds must satisfy minLat <= maxLat; longitude bounds
// may wrap the dateline (minLon > maxLon).
//
// The constructor builds a single-element Component2D tree (kept
// only for the parent's hashCode / equals / String identity — the
// Relate hook bypasses it) and wires the rectangle-specific
// SpatialVisitor factory.
//
// Errors:
//   - propagates geo.NewRectangle's invalid-bounds errors via the
//     caller (this constructor accepts an already-built Rectangle).
//   - propagates NewSpatialQuery's empty-field / nil-tree errors.
//
// Mirrors the Java constructor
// LatLonShapeBoundingBoxQuery(String, QueryRelation, Rectangle).
func NewLatLonShapeBoundingBoxQuery(
	field string,
	queryRelation document.QueryRelation,
	rectangle geo.Rectangle,
) (*LatLonShapeBoundingBoxQuery, error) {
	// Build the Component2D tree from the rectangle so the parent's
	// hashCode / equals / String identity match LatLonShapeQuery
	// when the geometry slice has a single element.
	tree, err := geo.CreateLatLonGeometry(rectangle)
	if err != nil {
		return nil, err
	}

	encoded := newEncodedLatLonRectangle(
		rectangle.MinLat(),
		rectangle.MaxLat(),
		rectangle.MinLon(),
		rectangle.MaxLon(),
	)

	factory := func() SpatialVisitor {
		return newLatLonShapeBoundingBoxSpatialVisitor(encoded)
	}

	parent, err := NewSpatialQuery(
		field,
		queryRelation,
		tree,
		factory,
		[]geo.Geometry{rectangle},
		WithSpatialQueryDisplayClassName("LatLonShapeBoundingBoxQuery"),
	)
	if err != nil {
		return nil, err
	}
	return &LatLonShapeBoundingBoxQuery{
		SpatialQuery: parent,
		rectangle:    rectangle,
	}, nil
}

// GetRectangle returns the query's bounding rectangle. Useful for
// tests; the Java reference exposes the field via a protected
// member.
func (q *LatLonShapeBoundingBoxQuery) GetRectangle() geo.Rectangle { return q.rectangle }

// Equals reports whether o is a LatLonShapeBoundingBoxQuery with
// the same field, relation, and rectangle. Mirrors the Java
// reference's equalsTo override (parent's equalsTo plus a Rectangle
// comparison).
func (q *LatLonShapeBoundingBoxQuery) Equals(other Query) bool {
	o, ok := other.(*LatLonShapeBoundingBoxQuery)
	if !ok {
		return false
	}
	if !q.SpatialQuery.Equals(o.SpatialQuery) {
		return false
	}
	return q.rectangle.Equals(o.rectangle)
}

// HashCode mirrors the Java reference: classHashSpatialQuery seed
// (inherited via the parent's HashCode call) folded through the
// rectangle's hashCode.
func (q *LatLonShapeBoundingBoxQuery) HashCode() int {
	h := q.SpatialQuery.HashCode()
	h = 31*h + int(q.rectangle.HashCode())
	return h
}

// String mirrors the Java reference's toString(String) override:
// "<ClassName>:" then optional "field=<field>:" then the rectangle's
// String() representation (no surrounding brackets — the rectangle's
// own String produces a complete "Rectangle(...)" form).
//
// This deviates from the parent's "[geom,]" layout because the Java
// reference deliberately uses the rectangle's toString directly to
// produce a more readable rendering for the single-geometry case.
func (q *LatLonShapeBoundingBoxQuery) String(field string) string {
	var sb strings.Builder
	sb.WriteString("LatLonShapeBoundingBoxQuery")
	sb.WriteByte(':')
	if q.GetField() != field {
		sb.WriteString(" field=")
		sb.WriteString(q.GetField())
		sb.WriteByte(':')
	}
	sb.WriteString(fmt.Sprintf("%v", q.rectangle))
	return sb.String()
}

// latLonShapeBoundingBoxSpatialVisitor implements SpatialVisitor for
// LatLonShapeBoundingBoxQuery. It owns an EncodedLatLonRectangle
// (the fast-path equivalent of the queryComponent2D tree) and
// dispatches every per-doc decision through it.
//
// Mirrors the anonymous SpatialVisitor returned by
// LatLonShapeBoundingBoxQuery.getSpatialVisitor() on the Java
// reference.
type latLonShapeBoundingBoxSpatialVisitor struct {
	*BaseSpatialVisitor

	rect *encodedLatLonRectangle
}

func newLatLonShapeBoundingBoxSpatialVisitor(rect *encodedLatLonRectangle) *latLonShapeBoundingBoxSpatialVisitor {
	v := &latLonShapeBoundingBoxSpatialVisitor{rect: rect}
	v.BaseSpatialVisitor = NewBaseSpatialVisitor(v)
	return v
}

// Relate forwards to the encoded rectangle's
// IntersectRangeBBox / RelateRangeBBox helpers depending on the
// caller relation. The dispatch is identical to the Java reference:
// INTERSECTS and DISJOINT use IntersectRangeBBox; WITHIN and
// CONTAINS use RelateRangeBBox.
//
// The implementation receives the parent's queryRelation through a
// thread-local-style closure: we re-derive the relation from the
// BaseSpatialVisitor's GetInnerFunction hook. However, GetInnerFunction
// already wraps Relate for DISJOINT transposition, so this Relate
// must always behave as the INTERSECTS-or-WITHIN baseline and let
// the base helper handle the DISJOINT flip.
//
// Java treats INTERSECTS and DISJOINT identically inside Relate
// (because the DISJOINT transposition happens in
// getInnerFunction). The branch boundary here is therefore:
//   - INTERSECTS / DISJOINT (treated identically here, transposed
//     later)  → IntersectRangeBBox
//   - WITHIN / CONTAINS                                → RelateRangeBBox
//
// Since the visitor cannot know the active relation at Relate-time
// without re-plumbing the SpatialVisitor contract, we use the
// stricter RelateRangeBBox: it returns CELL_INSIDE_QUERY only when
// the cell is fully inside, which matches the conservative semantics
// for both branches and never produces a false-positive match. The
// IntersectRangeBBox optimisation (which classifies a cell as
// CELL_INSIDE_QUERY when any of the cell's bounds clearly contains
// the query bbox) is therefore deferred — see backlog #2700.
//
// The deviation is benign: the per-cell decision is still correct;
// the only effect is that some cells that IntersectRangeBBox would
// classify as CELL_INSIDE_QUERY (allowing the full-subtree fast
// path) are classified as CELL_CROSSES_QUERY, forcing a per-leaf
// visit. The leaf predicate is then consulted, and the result is
// identical to the Java reference.
func (v *latLonShapeBoundingBoxSpatialVisitor) Relate(minTriangle, maxTriangle []byte) spatialRelation {
	if v.rect == nil {
		return spatialCellOutsideQuery
	}
	// Triangle byte layout: dim 0 = minY (offset 0), dim 1 = minX
	// (offset BYTES=4), dim 2 = maxY (offset 2*BYTES=8), dim 3 =
	// maxX (offset 3*BYTES=12). Dims 4..6 carry edge data and are
	// ignored by relate.
	rel := v.rect.relateRangeBBox(
		shapeFieldDimBytes, // minXOffset
		0,                  // minYOffset
		minTriangle,
		3*shapeFieldDimBytes, // maxXOffset
		2*shapeFieldDimBytes, // maxYOffset
		maxTriangle,
	)
	return relationToSpatial(rel)
}

// Intersects returns the per-doc predicate for INTERSECTS /
// DISJOINT queries. The triangle is decoded once per call, then
// dispatched on its kind.
func (v *latLonShapeBoundingBoxSpatialVisitor) Intersects() func(packed []byte) bool {
	return func(packed []byte) bool {
		t, err := document.DecodeTriangle(packed)
		if err != nil {
			return false
		}
		switch t.Kind {
		case document.DecodedTriangleTypePoint:
			return v.rect.Contains(t.AX, t.AY)
		case document.DecodedTriangleTypeLine:
			return v.rect.IntersectsLine(t.AX, t.AY, t.BX, t.BY)
		case document.DecodedTriangleTypeTriangle:
			return v.rect.IntersectsTriangle(t.AX, t.AY, t.BX, t.BY, t.CX, t.CY)
		default:
			return false
		}
	}
}

// Within returns the per-doc predicate for WITHIN queries.
func (v *latLonShapeBoundingBoxSpatialVisitor) Within() func(packed []byte) bool {
	return func(packed []byte) bool {
		t, err := document.DecodeTriangle(packed)
		if err != nil {
			return false
		}
		switch t.Kind {
		case document.DecodedTriangleTypePoint:
			return v.rect.Contains(t.AX, t.AY)
		case document.DecodedTriangleTypeLine:
			return v.rect.ContainsLine(t.AX, t.AY, t.BX, t.BY)
		case document.DecodedTriangleTypeTriangle:
			return v.rect.ContainsTriangle(t.AX, t.AY, t.BX, t.BY, t.CX, t.CY)
		default:
			return false
		}
	}
}

// Contains returns the per-doc classifier for CONTAINS queries.
// Panics on first dispatch if the bounding box crosses the dateline,
// mirroring the Java reference's IllegalArgumentException — see
// ErrLatLonShapeBoundingBoxQueryContainsDateline.
func (v *latLonShapeBoundingBoxSpatialVisitor) Contains() func(packed []byte) geo.WithinRelation {
	if v.rect.crossesDateline() {
		// Mirrors the Java reference's eager IllegalArgumentException.
		// The error is surfaced as a panic because the SpatialVisitor
		// contract has no error return; callers should validate the
		// rectangle ahead of CONTAINS dispatch using the constructor
		// or GetRectangle().CrossesDateline().
		panic(ErrLatLonShapeBoundingBoxQueryContainsDateline)
	}
	return func(packed []byte) geo.WithinRelation {
		t, err := document.DecodeTriangle(packed)
		if err != nil {
			return geo.WithinDisjoint
		}
		switch t.Kind {
		case document.DecodedTriangleTypePoint:
			if v.rect.Contains(t.AX, t.AY) {
				return geo.WithinNotWithin
			}
			return geo.WithinDisjoint
		case document.DecodedTriangleTypeLine:
			return v.rect.WithinLine(t.AX, t.AY, t.AB, t.BX, t.BY)
		case document.DecodedTriangleTypeTriangle:
			return v.rect.WithinTriangle(
				t.AX, t.AY, t.AB,
				t.BX, t.BY, t.BC,
				t.CX, t.CY, t.CA,
			)
		default:
			return geo.WithinDisjoint
		}
	}
}

// encodedLatLonRectangle is the bounding-box-specific extension of
// EncodedRectangle. It mirrors the private nested class
// LatLonShapeBoundingBoxQuery.EncodedLatLonRectangle on the Java
// reference.
//
// It owns two sortable-bytes payloads:
//   - bbox: the canonical (or eastern half when wrapping) bounding
//     box, packed as [minY, minX, maxY, maxX] using
//     util.IntToSortableBytes.
//   - west: when the rectangle wraps the dateline, the western half
//     of the split is packed here so the relate / intersect helpers
//     can fall back to it when the eastern half misses.
type encodedLatLonRectangle struct {
	*EncodedRectangle

	bbox []byte // 16 bytes = 4 dims × 4 sortable bytes
	west []byte // 16 bytes when wrapping; nil otherwise
}

// newEncodedLatLonRectangle builds the encoded form of a lat/lon
// rectangle. The minLon == 180 special case mirrors the Java
// reference's validateMinLon: a "single point on the antimeridian"
// rectangle is treated as a wrap-around rectangle from -180.
func newEncodedLatLonRectangle(minLat, maxLat, minLon, maxLon float64) *encodedLatLonRectangle {
	resolvedMinLon := validateBoundingBoxMinLon(minLon, maxLon)
	wraps := resolvedMinLon > maxLon

	base := NewEncodedRectangle(
		geo.EncodeLongitudeCeil(resolvedMinLon),
		geo.EncodeLongitude(maxLon),
		geo.EncodeLatitudeCeil(minLat),
		geo.EncodeLatitude(maxLat),
		wraps,
	)

	r := &encodedLatLonRectangle{
		EncodedRectangle: base,
		bbox:             make([]byte, 4*shapeFieldDimBytes),
	}

	if wraps {
		r.west = make([]byte, 4*shapeFieldDimBytes)
		encodeBoundingBoxBytes(geo.MinLonEncoded, base.MaxX(), base.MinY(), base.MaxY(), r.west)
		encodeBoundingBoxBytes(base.MinX(), geo.MaxLonEncoded, base.MinY(), base.MaxY(), r.bbox)
	} else {
		encodeBoundingBoxBytes(base.MinX(), base.MaxX(), base.MinY(), base.MaxY(), r.bbox)
	}
	return r
}

// validateBoundingBoxMinLon mirrors the Java reference's
// validateMinLon: a rectangle whose minLon is exactly +180 and
// whose minLon > maxLon collapses into a wrap-around rectangle
// starting at -180. Every other case is returned as-is.
func validateBoundingBoxMinLon(minLon, maxLon float64) float64 {
	if minLon == 180.0 && minLon > maxLon {
		return -180.0
	}
	return minLon
}

// encodeBoundingBoxBytes packs the four int32 bounds into the
// sortable-bytes layout the BKD tree uses. The layout is
// [minY, minX, maxY, maxX] — matching the Java reference's encode()
// in EncodedLatLonRectangle, which is in turn the same dim order
// the BKD tree indexes the four-dim shape bbox with.
func encodeBoundingBoxBytes(minX, maxX, minY, maxY int32, b []byte) {
	util.IntToSortableBytes(minY, b, 0)
	util.IntToSortableBytes(minX, b, shapeFieldDimBytes)
	util.IntToSortableBytes(maxY, b, 2*shapeFieldDimBytes)
	util.IntToSortableBytes(maxX, b, 3*shapeFieldDimBytes)
}

// crossesDateline reports whether the rectangle's X interval wraps
// the dateline. Mirrors the package-private accessor on the Java
// reference.
func (r *encodedLatLonRectangle) crossesDateline() bool {
	return r.WrapsCoordinateSystem()
}

// relateRangeBBox compares the rectangle to the (min, max) cell
// bbox of a triangle range and returns the matching pointRelation.
// When the rectangle wraps the dateline and the eastern half misses,
// the western half is consulted before returning OUTSIDE.
//
// Mirrors EncodedLatLonRectangle.relateRangeBBox.
func (r *encodedLatLonRectangle) relateRangeBBox(
	minXOffset, minYOffset int,
	minTriangle []byte,
	maxXOffset, maxYOffset int,
	maxTriangle []byte,
) pointRelation {
	east := compareBBoxToRangeBBox(
		r.bbox, minXOffset, minYOffset, minTriangle, maxXOffset, maxYOffset, maxTriangle,
	)
	if r.crossesDateline() && east == pointCellOutsideQuery {
		return compareBBoxToRangeBBox(
			r.west, minXOffset, minYOffset, minTriangle, maxXOffset, maxYOffset, maxTriangle,
		)
	}
	return east
}

// intersectRangeBBox computes the looser INTERSECTS relation between
// the rectangle and a triangle range cell. The looser semantics
// classify a cell as CELL_INSIDE_QUERY in two extra cases the
// stricter compareBBoxToRangeBBox would miss; the relax matches the
// Java reference's intersectBBoxWithRangeBBox.
//
// Currently unused by the visitor (which conservatively routes
// through relateRangeBBox; see backlog #2700) but kept exported as
// a package-private helper so the eventual upgrade is a one-line
// dispatch swap.
func (r *encodedLatLonRectangle) intersectRangeBBox(
	minXOffset, minYOffset int,
	minTriangle []byte,
	maxXOffset, maxYOffset int,
	maxTriangle []byte,
) pointRelation {
	east := intersectBBoxWithRangeBBox(
		r.bbox, minXOffset, minYOffset, minTriangle, maxXOffset, maxYOffset, maxTriangle,
	)
	if r.crossesDateline() && east == pointCellOutsideQuery {
		return intersectBBoxWithRangeBBox(
			r.west, minXOffset, minYOffset, minTriangle, maxXOffset, maxYOffset, maxTriangle,
		)
	}
	return east
}

// pointRelation is a private alias for the three-valued
// PointValues.Relation enum the Java reference uses. It is
// deliberately a separate type from spatialRelation so the helpers
// can be tested directly with the same semantics as
// org.apache.lucene.index.PointValues.Relation.
type pointRelation int

const (
	pointCellInsideQuery pointRelation = iota
	pointCellOutsideQuery
	pointCellCrossesQuery
)

// relationToSpatial converts a pointRelation into the
// spatialRelation the SpatialQuery pipeline consumes. The three
// values are stable across both enums; the switch is exhaustive.
func relationToSpatial(r pointRelation) spatialRelation {
	switch r {
	case pointCellInsideQuery:
		return spatialCellInsideQuery
	case pointCellOutsideQuery:
		return spatialCellOutsideQuery
	default:
		return spatialCellCrossesQuery
	}
}

// compareBBoxToRangeBBox classifies a triangle-range cell against a
// packed bbox. The bbox layout is [minY, minX, maxY, maxX]; the
// caller supplies the matching offsets into minTriangle / maxTriangle
// (which carry seven dims, of which only the four bbox dims are
// consulted here).
//
// Mirrors EncodedLatLonRectangle.compareBBoxToRangeBBox.
func compareBBoxToRangeBBox(
	bbox []byte,
	minXOffset, minYOffset int,
	minTriangle []byte,
	maxXOffset, maxYOffset int,
	maxTriangle []byte,
) pointRelation {
	if rangeBBoxDisjoint(bbox, minXOffset, minYOffset, minTriangle, maxXOffset, maxYOffset, maxTriangle) {
		return pointCellOutsideQuery
	}
	// Cell-inside-query when every cell dim falls within the bbox
	// dim. The X and Y bounds use the bbox layout offsets (X at
	// BYTES and 3*BYTES; Y at 0 and 2*BYTES).
	if util.CompareUnsigned4(minTriangle, minXOffset, bbox, shapeFieldDimBytes) >= 0 &&
		util.CompareUnsigned4(maxTriangle, maxXOffset, bbox, 3*shapeFieldDimBytes) <= 0 &&
		util.CompareUnsigned4(minTriangle, minYOffset, bbox, 0) >= 0 &&
		util.CompareUnsigned4(maxTriangle, maxYOffset, bbox, 2*shapeFieldDimBytes) <= 0 {
		return pointCellInsideQuery
	}
	return pointCellCrossesQuery
}

// intersectBBoxWithRangeBBox is the looser INTERSECTS variant of
// compareBBoxToRangeBBox. Three extra checks expand the
// CELL_INSIDE_QUERY classification to catch a few more cells the
// stricter helper would mark as CELL_CROSSES_QUERY.
//
// Mirrors EncodedLatLonRectangle.intersectBBoxWithRangeBBox.
func intersectBBoxWithRangeBBox(
	bbox []byte,
	minXOffset, minYOffset int,
	minTriangle []byte,
	maxXOffset, maxYOffset int,
	maxTriangle []byte,
) pointRelation {
	if rangeBBoxDisjoint(bbox, minXOffset, minYOffset, minTriangle, maxXOffset, maxYOffset, maxTriangle) {
		return pointCellOutsideQuery
	}

	if util.CompareUnsigned4(minTriangle, minXOffset, bbox, shapeFieldDimBytes) >= 0 &&
		util.CompareUnsigned4(minTriangle, minYOffset, bbox, 0) >= 0 {
		if util.CompareUnsigned4(maxTriangle, minXOffset, bbox, 3*shapeFieldDimBytes) <= 0 &&
			util.CompareUnsigned4(maxTriangle, maxYOffset, bbox, 2*shapeFieldDimBytes) <= 0 {
			return pointCellInsideQuery
		}
		if util.CompareUnsigned4(maxTriangle, maxXOffset, bbox, 3*shapeFieldDimBytes) <= 0 &&
			util.CompareUnsigned4(maxTriangle, minYOffset, bbox, 2*shapeFieldDimBytes) <= 0 {
			return pointCellInsideQuery
		}
	}

	if util.CompareUnsigned4(maxTriangle, maxXOffset, bbox, 3*shapeFieldDimBytes) <= 0 &&
		util.CompareUnsigned4(maxTriangle, maxYOffset, bbox, 2*shapeFieldDimBytes) <= 0 {
		if util.CompareUnsigned4(minTriangle, minXOffset, bbox, shapeFieldDimBytes) >= 0 &&
			util.CompareUnsigned4(minTriangle, maxYOffset, bbox, 0) >= 0 {
			return pointCellInsideQuery
		}
		if util.CompareUnsigned4(minTriangle, maxXOffset, bbox, shapeFieldDimBytes) >= 0 &&
			util.CompareUnsigned4(minTriangle, minYOffset, bbox, 0) >= 0 {
			return pointCellInsideQuery
		}
	}

	return pointCellCrossesQuery
}

// rangeBBoxDisjoint reports whether the triangle range cell is
// strictly outside the bbox along at least one axis. Mirrors the
// Java reference's private disjoint helper.
func rangeBBoxDisjoint(
	bbox []byte,
	minXOffset, minYOffset int,
	minTriangle []byte,
	maxXOffset, maxYOffset int,
	maxTriangle []byte,
) bool {
	return util.CompareUnsigned4(minTriangle, minXOffset, bbox, 3*shapeFieldDimBytes) > 0 ||
		util.CompareUnsigned4(maxTriangle, maxXOffset, bbox, shapeFieldDimBytes) < 0 ||
		util.CompareUnsigned4(minTriangle, minYOffset, bbox, 2*shapeFieldDimBytes) > 0 ||
		util.CompareUnsigned4(maxTriangle, maxYOffset, bbox, 0) < 0
}

// shapeFieldDimBytes is the per-dimension byte width inside a packed
// ShapeField value: four bytes per int32 dim, matching Java's
// ShapeField.BYTES = Integer.BYTES = 4. Kept private to keep the
// constant arithmetic local; tests assert the dispatch matches.
const shapeFieldDimBytes = 4

// Compile-time guards.
var _ SpatialVisitor = (*latLonShapeBoundingBoxSpatialVisitor)(nil)
var _ Query = (*LatLonShapeBoundingBoxQuery)(nil)
