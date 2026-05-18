// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
)

// LatLonShapeQuery finds all previously indexed geo shapes that
// comply with the supplied QueryRelation against an array of
// LatLonGeometry values. The field must have been indexed with
// LatLonShape.CreateIndexableFields per document.
//
// Mirrors the package-private final class
// org.apache.lucene.document.LatLonShapeQuery (Lucene 10.4.0).
//
// # Composition vs inheritance
//
// The Java reference extends SpatialQuery and overrides the two
// abstract hooks (createComponent2D, getSpatialVisitor). Gocene's
// SpatialQuery is a concrete struct that captures the
// queryComponent2D and a SpatialVisitor factory at construction
// time; LatLonShapeQuery therefore embeds *SpatialQuery and exposes
// the geometry-specific construction through NewLatLonShapeQuery.
// The factory function passed to NewSpatialQuery returns a fresh
// latLonShapeSpatialVisitor on every call, replicating the Java
// reference's getSpatialVisitor() override.
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
type LatLonShapeQuery struct {
	*SpatialQuery
}

// ErrLatLonShapeQueryWithinLine is returned by NewLatLonShapeQuery
// when the caller asks for a WITHIN query over one or more Line
// geometries. Mirrors the Java reference's
// IllegalArgumentException message
// "LatLonShapeQuery does not support WITHIN queries with line
// geometries".
var ErrLatLonShapeQueryWithinLine = errors.New(
	"search: LatLonShapeQuery does not support WITHIN queries with line geometries",
)

// NewLatLonShapeQuery builds a LatLonShapeQuery that matches every
// indexed shape whose relation to the supplied LatLonGeometry array
// equals queryRelation. The constructor validates the geometries,
// builds the Component2D tree, and wires the SpatialVisitor factory
// the parent SpatialQuery uses to drive the BKD tree walk.
//
// The constructor rejects the (WITHIN, Line) combination with
// ErrLatLonShapeQueryWithinLine, mirroring the Java reference. A
// nil or empty geometries slice surfaces as an error from
// geo.CreateLatLonGeometry; a nil element at index i is reported as
// "geometries[i] must not be null". Other constructor errors
// (empty field, nil tree) come from NewSpatialQuery.
//
// Mirrors the Java constructor
// LatLonShapeQuery(String, QueryRelation, LatLonGeometry...).
func NewLatLonShapeQuery(
	field string,
	queryRelation document.QueryRelation,
	geometries ...geo.LatLonGeometry,
) (*LatLonShapeQuery, error) {
	if err := validateLatLonShapeGeometries(queryRelation, geometries); err != nil {
		return nil, err
	}
	tree, err := geo.CreateLatLonGeometry(geometries...)
	if err != nil {
		return nil, err
	}
	// Promote the LatLonGeometry slice to the abstract geo.Geometry
	// slice so the parent's hashCode / equals semantics see the
	// same shape the Java reference would.
	geomShapes := make([]geo.Geometry, len(geometries))
	for i, g := range geometries {
		geomShapes[i] = g
	}

	factory := func() SpatialVisitor {
		return newLatLonShapeSpatialVisitor(tree)
	}

	parent, err := NewSpatialQuery(
		field,
		queryRelation,
		tree,
		factory,
		geomShapes,
		WithSpatialQueryDisplayClassName("LatLonShapeQuery"),
	)
	if err != nil {
		return nil, err
	}
	return &LatLonShapeQuery{SpatialQuery: parent}, nil
}

// validateLatLonShapeGeometries rejects WITHIN+Line as the Java
// reference does. A nil or empty slice is left to the
// Component2D builder so the error message stays close to the
// Java reference's IllegalArgumentException text.
func validateLatLonShapeGeometries(
	queryRelation document.QueryRelation,
	geometries []geo.LatLonGeometry,
) error {
	if queryRelation != document.QueryRelationWithin {
		return nil
	}
	for _, g := range geometries {
		if _, isLine := g.(geo.Line); isLine {
			return ErrLatLonShapeQueryWithinLine
		}
		if _, isLinePtr := g.(*geo.Line); isLinePtr {
			return ErrLatLonShapeQueryWithinLine
		}
	}
	return nil
}

// latLonShapeSpatialVisitor implements SpatialVisitor for
// LatLonShapeQuery. It owns the queryComponent2D tree and decodes
// each visited triangle through document.DecodeTriangle and
// geo.DecodeLatitude / DecodeLongitude before dispatching to the
// matching Component2D hook.
//
// Mirrors the anonymous SpatialVisitor returned by
// LatLonShapeQuery.getSpatialVisitor(Component2D) on the Java
// reference.
type latLonShapeSpatialVisitor struct {
	*BaseSpatialVisitor

	tree geo.Component2D
}

// newLatLonShapeSpatialVisitor wires the BaseSpatialVisitor backlink
// so GetInnerFunction / GetLeafPredicate dispatch through this
// type's Relate / Intersects / Within / Contains overrides.
func newLatLonShapeSpatialVisitor(tree geo.Component2D) *latLonShapeSpatialVisitor {
	v := &latLonShapeSpatialVisitor{tree: tree}
	v.BaseSpatialVisitor = NewBaseSpatialVisitor(v)
	return v
}

// Relate decodes the four corners of the cell described by
// minTriangle / maxTriangle and forwards to the tree. Mirrors the
// Java reference's anonymous relate(byte[], byte[]).
//
// Layout reminder: the BKD packs seven int32 dimensions per value:
//
//	dim 0 → minY (lat)         offset 0
//	dim 1 → minX (lon)         offset BYTES
//	dim 2 → maxY (lat)         offset 2*BYTES
//	dim 3 → maxX (lon)         offset 3*BYTES
//
// dims 4–6 carry edge data that is irrelevant for cell relate.
func (v *latLonShapeSpatialVisitor) Relate(minTriangle, maxTriangle []byte) spatialRelation {
	if v.tree == nil {
		return spatialCellOutsideQuery
	}
	const stride = document.ShapeFieldBytes / 7 // 4 bytes per int32 dim
	if len(minTriangle) < 2*stride || len(maxTriangle) < 4*stride {
		return spatialCellOutsideQuery
	}
	minLat := geo.DecodeLatitudeBytes(minTriangle, 0)
	minLon := geo.DecodeLongitudeBytes(minTriangle, stride)
	maxLat := geo.DecodeLatitudeBytes(maxTriangle, 2*stride)
	maxLon := geo.DecodeLongitudeBytes(maxTriangle, 3*stride)
	return geoRelationToSpatial(v.tree.Relate(minLon, maxLon, minLat, maxLat))
}

// Intersects returns the per-doc predicate the parent uses for
// INTERSECTS / DISJOINT queries. Mirrors the Java reference's
// anonymous intersects().
func (v *latLonShapeSpatialVisitor) Intersects() func(packed []byte) bool {
	return func(packed []byte) bool {
		t, err := document.DecodeTriangle(packed)
		if err != nil {
			return false
		}
		switch t.Kind {
		case document.DecodedTriangleTypePoint:
			alat := geo.DecodeLatitude(t.AY)
			alon := geo.DecodeLongitude(t.AX)
			return v.tree.Contains(alon, alat)
		case document.DecodedTriangleTypeLine:
			alat := geo.DecodeLatitude(t.AY)
			alon := geo.DecodeLongitude(t.AX)
			blat := geo.DecodeLatitude(t.BY)
			blon := geo.DecodeLongitude(t.BX)
			return geo.IntersectsLineDefault(v.tree, alon, alat, blon, blat)
		case document.DecodedTriangleTypeTriangle:
			alat := geo.DecodeLatitude(t.AY)
			alon := geo.DecodeLongitude(t.AX)
			blat := geo.DecodeLatitude(t.BY)
			blon := geo.DecodeLongitude(t.BX)
			clat := geo.DecodeLatitude(t.CY)
			clon := geo.DecodeLongitude(t.CX)
			return geo.IntersectsTriangleDefault(v.tree, alon, alat, blon, blat, clon, clat)
		default:
			return false
		}
	}
}

// Within returns the per-doc predicate the parent uses for WITHIN
// queries. Mirrors the Java reference's anonymous within().
func (v *latLonShapeSpatialVisitor) Within() func(packed []byte) bool {
	return func(packed []byte) bool {
		t, err := document.DecodeTriangle(packed)
		if err != nil {
			return false
		}
		switch t.Kind {
		case document.DecodedTriangleTypePoint:
			alat := geo.DecodeLatitude(t.AY)
			alon := geo.DecodeLongitude(t.AX)
			return v.tree.Contains(alon, alat)
		case document.DecodedTriangleTypeLine:
			alat := geo.DecodeLatitude(t.AY)
			alon := geo.DecodeLongitude(t.AX)
			blat := geo.DecodeLatitude(t.BY)
			blon := geo.DecodeLongitude(t.BX)
			return geo.ContainsLineDefault(v.tree, alon, alat, blon, blat)
		case document.DecodedTriangleTypeTriangle:
			alat := geo.DecodeLatitude(t.AY)
			alon := geo.DecodeLongitude(t.AX)
			blat := geo.DecodeLatitude(t.BY)
			blon := geo.DecodeLongitude(t.BX)
			clat := geo.DecodeLatitude(t.CY)
			clon := geo.DecodeLongitude(t.CX)
			return geo.ContainsTriangleDefault(v.tree, alon, alat, blon, blat, clon, clat)
		default:
			return false
		}
	}
}

// Contains returns the per-doc classifier the parent uses for
// CONTAINS queries. Mirrors the Java reference's anonymous
// contains().
func (v *latLonShapeSpatialVisitor) Contains() func(packed []byte) geo.WithinRelation {
	return func(packed []byte) geo.WithinRelation {
		t, err := document.DecodeTriangle(packed)
		if err != nil {
			return geo.WithinDisjoint
		}
		switch t.Kind {
		case document.DecodedTriangleTypePoint:
			alat := geo.DecodeLatitude(t.AY)
			alon := geo.DecodeLongitude(t.AX)
			return v.tree.WithinPoint(alon, alat)
		case document.DecodedTriangleTypeLine:
			alat := geo.DecodeLatitude(t.AY)
			alon := geo.DecodeLongitude(t.AX)
			blat := geo.DecodeLatitude(t.BY)
			blon := geo.DecodeLongitude(t.BX)
			return geo.WithinLineDefault(v.tree, alon, alat, t.AB, blon, blat)
		case document.DecodedTriangleTypeTriangle:
			alat := geo.DecodeLatitude(t.AY)
			alon := geo.DecodeLongitude(t.AX)
			blat := geo.DecodeLatitude(t.BY)
			blon := geo.DecodeLongitude(t.BX)
			clat := geo.DecodeLatitude(t.CY)
			clon := geo.DecodeLongitude(t.CX)
			return geo.WithinTriangleDefault(v.tree,
				alon, alat, t.AB,
				blon, blat, t.BC,
				clon, clat, t.CA,
			)
		default:
			return geo.WithinDisjoint
		}
	}
}

// geoRelationToSpatial converts a geo.Relation (returned by
// Component2D.Relate) to the internal spatialRelation the
// SpatialQuery pipeline uses. The three values are stable across
// both enums; the switch is exhaustive.
func geoRelationToSpatial(r geo.Relation) spatialRelation {
	switch r {
	case geo.CellInsideQuery:
		return spatialCellInsideQuery
	case geo.CellOutsideQuery:
		return spatialCellOutsideQuery
	case geo.CellCrossesQuery:
		return spatialCellCrossesQuery
	default:
		panic(fmt.Sprintf("search: unknown geo.Relation %v", r))
	}
}

// Compile-time guards: the visitor satisfies SpatialVisitor.
var _ SpatialVisitor = (*latLonShapeSpatialVisitor)(nil)
