// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"errors"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
)

// XYShapeQuery finds all previously indexed cartesian shapes that
// comply with the supplied QueryRelation against an array of
// XYGeometry values. The field must have been indexed with
// XYShape.CreateIndexableFields per document.
//
// Mirrors the package-private final class
// org.apache.lucene.document.XYShapeQuery (Lucene 10.4.0).
//
// # Composition vs inheritance
//
// The Java reference extends SpatialQuery and overrides the two
// abstract hooks (createComponent2D, getSpatialVisitor). Gocene's
// SpatialQuery is a concrete struct that captures the
// queryComponent2D and a SpatialVisitor factory at construction
// time; XYShapeQuery therefore embeds *SpatialQuery and exposes
// the geometry-specific construction through NewXYShapeQuery.
// The factory function passed to NewSpatialQuery returns a fresh
// xyShapeSpatialVisitor on every call, replicating the Java
// reference's getSpatialVisitor() override.
//
// # Decoded triangle layout caveat
//
// Identical to the LatLonShapeQuery sibling: the current
// document.DecodeTriangle implements a simplified layout that
// recovers only the A vertex and the three edge bits; B and C are
// left at zero. POINT (single-vertex) triangles round-trip
// correctly, so the visitor handles every type but the LINE and
// TRIANGLE branches will operate against the partially-recovered
// vertices until the full rotation-aware decoder lands (backlog
// #2697). The visitor shape and dispatch match the Java reference
// in every other respect so the upgrade is a pure decoder swap.
type XYShapeQuery struct {
	*SpatialQuery
}

// ErrXYShapeQueryWithinLine is returned by NewXYShapeQuery when the
// caller asks for a WITHIN query over one or more XYLine
// geometries. Mirrors the Java reference's IllegalArgumentException
// message "XYShapeQuery does not support WITHIN queries with line
// geometries".
//
// Note: unlike the LatLon sibling, geo.XYLine's xyGeometry() marker
// is defined on the value receiver only, so *geo.XYLine is not
// assignable to geo.XYGeometry and the pointer-form guard is
// unnecessary.
var ErrXYShapeQueryWithinLine = errors.New(
	"search: XYShapeQuery does not support WITHIN queries with line geometries",
)

// NewXYShapeQuery builds an XYShapeQuery that matches every indexed
// shape whose relation to the supplied XYGeometry array equals
// queryRelation. The constructor validates the geometries, builds
// the Component2D tree, and wires the SpatialVisitor factory the
// parent SpatialQuery uses to drive the BKD tree walk.
//
// The constructor rejects the (WITHIN, XYLine) combination with
// ErrXYShapeQueryWithinLine, mirroring the Java reference. A nil
// or empty geometries slice surfaces as an error from
// geo.CreateXYGeometry; a nil element at index i is reported as
// "geometries[i] must not be null". Other constructor errors
// (empty field, nil tree) come from NewSpatialQuery.
//
// Mirrors the Java constructor
// XYShapeQuery(String, QueryRelation, XYGeometry...).
func NewXYShapeQuery(
	field string,
	queryRelation document.QueryRelation,
	geometries ...geo.XYGeometry,
) (*XYShapeQuery, error) {
	if err := validateXYShapeGeometries(queryRelation, geometries); err != nil {
		return nil, err
	}
	tree, err := geo.CreateXYGeometry(geometries...)
	if err != nil {
		return nil, err
	}
	// Promote the XYGeometry slice to the abstract geo.Geometry
	// slice so the parent's hashCode / equals semantics see the
	// same shape the Java reference would.
	geomShapes := make([]geo.Geometry, len(geometries))
	for i, g := range geometries {
		geomShapes[i] = g
	}

	factory := func() SpatialVisitor {
		return newXYShapeSpatialVisitor(tree)
	}

	parent, err := NewSpatialQuery(
		field,
		queryRelation,
		tree,
		factory,
		geomShapes,
		WithSpatialQueryDisplayClassName("XYShapeQuery"),
	)
	if err != nil {
		return nil, err
	}
	return &XYShapeQuery{SpatialQuery: parent}, nil
}

// validateXYShapeGeometries rejects WITHIN+XYLine as the Java
// reference does. A nil or empty slice is left to the Component2D
// builder so the error message stays close to the Java reference's
// IllegalArgumentException text.
func validateXYShapeGeometries(
	queryRelation document.QueryRelation,
	geometries []geo.XYGeometry,
) error {
	if queryRelation != document.QueryRelationWithin {
		return nil
	}
	for _, g := range geometries {
		if _, isLine := g.(geo.XYLine); isLine {
			return ErrXYShapeQueryWithinLine
		}
	}
	return nil
}

// xyShapeSpatialVisitor implements SpatialVisitor for XYShapeQuery.
// It owns the queryComponent2D tree and decodes each visited
// triangle through document.DecodeTriangle and geo.XYDecode before
// dispatching to the matching Component2D hook.
//
// Mirrors the anonymous SpatialVisitor returned by
// XYShapeQuery.getSpatialVisitor(Component2D) on the Java
// reference.
type xyShapeSpatialVisitor struct {
	*BaseSpatialVisitor

	tree geo.Component2D
}

// newXYShapeSpatialVisitor wires the BaseSpatialVisitor backlink
// so GetInnerFunction / GetLeafPredicate dispatch through this
// type's Relate / Intersects / Within / Contains overrides.
func newXYShapeSpatialVisitor(tree geo.Component2D) *xyShapeSpatialVisitor {
	v := &xyShapeSpatialVisitor{tree: tree}
	v.BaseSpatialVisitor = NewBaseSpatialVisitor(v)
	return v
}

// Relate decodes the four corners of the cell described by
// minTriangle / maxTriangle and forwards to the tree. Mirrors the
// Java reference's anonymous relate(byte[], byte[]).
//
// Layout reminder: the BKD packs seven int32 dimensions per value
// (Y comes first because Lucene sorts by Y then X for the cartesian
// case, mirroring the LatLon (lat, lon) order):
//
//	dim 0 → minY              offset 0
//	dim 1 → minX              offset BYTES
//	dim 2 → maxY              offset 2*BYTES
//	dim 3 → maxX              offset 3*BYTES
//
// dims 4–6 carry edge data that is irrelevant for cell relate.
func (v *xyShapeSpatialVisitor) Relate(minTriangle, maxTriangle []byte) spatialRelation {
	if v.tree == nil {
		return spatialCellOutsideQuery
	}
	const stride = document.ShapeFieldBytes / 7 // 4 bytes per int32 dim
	if len(minTriangle) < 2*stride || len(maxTriangle) < 4*stride {
		return spatialCellOutsideQuery
	}
	minY := float64(geo.XYDecodeBytes(minTriangle, 0))
	minX := float64(geo.XYDecodeBytes(minTriangle, stride))
	maxY := float64(geo.XYDecodeBytes(maxTriangle, 2*stride))
	maxX := float64(geo.XYDecodeBytes(maxTriangle, 3*stride))
	return geoRelationToSpatial(v.tree.Relate(minX, maxX, minY, maxY))
}

// Intersects returns the per-doc predicate the parent uses for
// INTERSECTS / DISJOINT queries. Mirrors the Java reference's
// anonymous intersects().
func (v *xyShapeSpatialVisitor) Intersects() func(packed []byte) bool {
	return func(packed []byte) bool {
		t, err := document.DecodeTriangle(packed)
		if err != nil {
			return false
		}
		switch t.Kind {
		case document.DecodedTriangleTypePoint:
			ay := float64(geo.XYDecode(t.AY))
			ax := float64(geo.XYDecode(t.AX))
			return v.tree.Contains(ax, ay)
		case document.DecodedTriangleTypeLine:
			ay := float64(geo.XYDecode(t.AY))
			ax := float64(geo.XYDecode(t.AX))
			by := float64(geo.XYDecode(t.BY))
			bx := float64(geo.XYDecode(t.BX))
			return geo.IntersectsLineDefault(v.tree, ax, ay, bx, by)
		case document.DecodedTriangleTypeTriangle:
			ay := float64(geo.XYDecode(t.AY))
			ax := float64(geo.XYDecode(t.AX))
			by := float64(geo.XYDecode(t.BY))
			bx := float64(geo.XYDecode(t.BX))
			cy := float64(geo.XYDecode(t.CY))
			cx := float64(geo.XYDecode(t.CX))
			return geo.IntersectsTriangleDefault(v.tree, ax, ay, bx, by, cx, cy)
		default:
			return false
		}
	}
}

// Within returns the per-doc predicate the parent uses for WITHIN
// queries. Mirrors the Java reference's anonymous within().
func (v *xyShapeSpatialVisitor) Within() func(packed []byte) bool {
	return func(packed []byte) bool {
		t, err := document.DecodeTriangle(packed)
		if err != nil {
			return false
		}
		switch t.Kind {
		case document.DecodedTriangleTypePoint:
			ay := float64(geo.XYDecode(t.AY))
			ax := float64(geo.XYDecode(t.AX))
			return v.tree.Contains(ax, ay)
		case document.DecodedTriangleTypeLine:
			ay := float64(geo.XYDecode(t.AY))
			ax := float64(geo.XYDecode(t.AX))
			by := float64(geo.XYDecode(t.BY))
			bx := float64(geo.XYDecode(t.BX))
			return geo.ContainsLineDefault(v.tree, ax, ay, bx, by)
		case document.DecodedTriangleTypeTriangle:
			ay := float64(geo.XYDecode(t.AY))
			ax := float64(geo.XYDecode(t.AX))
			by := float64(geo.XYDecode(t.BY))
			bx := float64(geo.XYDecode(t.BX))
			cy := float64(geo.XYDecode(t.CY))
			cx := float64(geo.XYDecode(t.CX))
			return geo.ContainsTriangleDefault(v.tree, ax, ay, bx, by, cx, cy)
		default:
			return false
		}
	}
}

// Contains returns the per-doc classifier the parent uses for
// CONTAINS queries. Mirrors the Java reference's anonymous
// contains().
func (v *xyShapeSpatialVisitor) Contains() func(packed []byte) geo.WithinRelation {
	return func(packed []byte) geo.WithinRelation {
		t, err := document.DecodeTriangle(packed)
		if err != nil {
			return geo.WithinDisjoint
		}
		switch t.Kind {
		case document.DecodedTriangleTypePoint:
			ay := float64(geo.XYDecode(t.AY))
			ax := float64(geo.XYDecode(t.AX))
			return v.tree.WithinPoint(ax, ay)
		case document.DecodedTriangleTypeLine:
			ay := float64(geo.XYDecode(t.AY))
			ax := float64(geo.XYDecode(t.AX))
			by := float64(geo.XYDecode(t.BY))
			bx := float64(geo.XYDecode(t.BX))
			return geo.WithinLineDefault(v.tree, ax, ay, t.AB, bx, by)
		case document.DecodedTriangleTypeTriangle:
			ay := float64(geo.XYDecode(t.AY))
			ax := float64(geo.XYDecode(t.AX))
			by := float64(geo.XYDecode(t.BY))
			bx := float64(geo.XYDecode(t.BX))
			cy := float64(geo.XYDecode(t.CY))
			cx := float64(geo.XYDecode(t.CX))
			return geo.WithinTriangleDefault(v.tree,
				ax, ay, t.AB,
				bx, by, t.BC,
				cx, cy, t.CA,
			)
		default:
			return geo.WithinDisjoint
		}
	}
}

// Compile-time guards: the visitor satisfies SpatialVisitor.
var _ SpatialVisitor = (*xyShapeSpatialVisitor)(nil)
