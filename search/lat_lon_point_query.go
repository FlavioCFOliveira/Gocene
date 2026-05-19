// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"errors"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// LatLonPointQuery finds every previously indexed geo point whose
// (latitude, longitude) value complies with the supplied
// [document.QueryRelation] against an array of [geo.LatLonGeometry]
// values. The field must have been indexed with [document.LatLonPoint]
// per document.
//
// Mirrors the package-private final class
// org.apache.lucene.document.LatLonPointQuery (Lucene 10.4.0).
//
// # Composition vs inheritance
//
// The Java reference extends SpatialQuery and overrides the two
// abstract hooks (createComponent2D, getSpatialVisitor). Gocene's
// SpatialQuery is a concrete struct that captures the
// queryComponent2D and a SpatialVisitor factory at construction
// time; LatLonPointQuery therefore embeds *SpatialQuery and exposes
// the geometry-specific construction through NewLatLonPointQuery.
// The factory function passed to NewSpatialQuery returns a fresh
// latLonPointSpatialVisitor on every call, replicating the Java
// reference's getSpatialVisitor() override. Same shape as the
// sibling LatLonShapeQuery port.
//
// # Wire-format
//
// Each indexed value is the 8-byte packed payload produced by
// [document.EncodeLatLon]: 4 bytes of sortable-bytes latitude
// followed by 4 bytes of sortable-bytes longitude. The visitor
// decodes both dimensions through [util.SortableBytesToInt] /
// [geo.DecodeLatitude] / [geo.DecodeLongitude] before dispatching
// to the [geo.Component2D] tree, exactly as the Java reference does
// in its anonymous SpatialVisitor.
type LatLonPointQuery struct {
	*SpatialQuery
}

// ErrLatLonPointQueryWithinLine is returned by NewLatLonPointQuery
// when the caller asks for a WITHIN query over one or more Line
// geometries. Mirrors the Java reference's IllegalArgumentException
// message "LatLonPointQuery does not support WITHIN queries with
// line geometries".
var ErrLatLonPointQueryWithinLine = errors.New(
	"search: LatLonPointQuery does not support WITHIN queries with line geometries",
)

// ErrLatLonPointQueryContainsNonPoint is returned by
// NewLatLonPointQuery when the caller asks for a CONTAINS query
// over a non-Point geometry. Mirrors the Java reference's
// IllegalArgumentException message "LatLonPointQuery does not
// support CONTAINS queries with non-points geometries".
var ErrLatLonPointQueryContainsNonPoint = errors.New(
	"search: LatLonPointQuery does not support CONTAINS queries with non-points geometries",
)

// NewLatLonPointQuery builds a LatLonPointQuery that matches every
// indexed point whose relation to the supplied LatLonGeometry array
// equals queryRelation. The constructor validates the geometry
// shape against the relation, builds the Component2D tree, and
// wires the SpatialVisitor factory the parent SpatialQuery uses to
// drive the BKD tree walk.
//
// Validation mirrors LatLonPointQuery.validateGeometry on the Java
// reference: WITHIN + Line is rejected with
// ErrLatLonPointQueryWithinLine, CONTAINS + non-Point is rejected
// with ErrLatLonPointQueryContainsNonPoint. A nil or empty
// geometries slice surfaces as an error from
// geo.CreateLatLonGeometry; a nil element is reported by the same
// helper. Other constructor errors (empty field, nil tree) come
// from NewSpatialQuery.
//
// Mirrors the Java constructor
// LatLonPointQuery(String, QueryRelation, LatLonGeometry...).
func NewLatLonPointQuery(
	field string,
	queryRelation document.QueryRelation,
	geometries ...geo.LatLonGeometry,
) (*LatLonPointQuery, error) {
	if err := validateLatLonPointGeometries(queryRelation, geometries); err != nil {
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
		return newLatLonPointSpatialVisitor(tree)
	}

	parent, err := NewSpatialQuery(
		field,
		queryRelation,
		tree,
		factory,
		geomShapes,
		WithSpatialQueryDisplayClassName("LatLonPointQuery"),
	)
	if err != nil {
		return nil, err
	}
	return &LatLonPointQuery{SpatialQuery: parent}, nil
}

// validateLatLonPointGeometries rejects the two invalid (relation,
// geometry-kind) pairs the Java reference explicitly forbids:
// WITHIN + Line and CONTAINS + non-Point. The validation walks the
// supplied slice; a nil or empty slice is left to
// geo.CreateLatLonGeometry so the error message stays close to the
// Java reference's text.
func validateLatLonPointGeometries(
	queryRelation document.QueryRelation,
	geometries []geo.LatLonGeometry,
) error {
	if queryRelation == document.QueryRelationWithin {
		for _, g := range geometries {
			if _, isLine := g.(geo.Line); isLine {
				return ErrLatLonPointQueryWithinLine
			}
			if _, isLinePtr := g.(*geo.Line); isLinePtr {
				return ErrLatLonPointQueryWithinLine
			}
		}
	}
	if queryRelation == document.QueryRelationContains {
		for _, g := range geometries {
			if isLatLonPoint(g) {
				continue
			}
			return ErrLatLonPointQueryContainsNonPoint
		}
	}
	return nil
}

// isLatLonPoint reports whether g is a geo.Point — covering both
// value and pointer forms because LatLonGeometry implementations
// are exposed through both shapes across the geo/ package.
func isLatLonPoint(g geo.LatLonGeometry) bool {
	if _, ok := g.(geo.Point); ok {
		return true
	}
	if _, ok := g.(*geo.Point); ok {
		return true
	}
	return false
}

// latLonPointSpatialVisitor implements SpatialVisitor for
// LatLonPointQuery. It owns the queryComponent2D tree, the four
// encoded bbox bounds used by the Relate fast path, and a
// Component2DPredicate built once for the Intersects / Within hot
// paths. Per-doc decoding goes through util.SortableBytesToInt /
// geo.DecodeLatitude / geo.DecodeLongitude exactly as the Java
// reference does via NumericUtils.sortableBytesToInt and
// GeoEncodingUtils.decode{Latitude,Longitude}.
//
// Mirrors the anonymous SpatialVisitor returned by
// LatLonPointQuery.getSpatialVisitor() on the Java reference.
type latLonPointSpatialVisitor struct {
	*BaseSpatialVisitor

	tree      geo.Component2D
	predicate geo.Component2DPredicate

	// Encoded global bounding box derived from the queryComponent2D.
	// Mirrors the Java reference's getMinY / getMaxY / getMinX /
	// getMaxX captured once at visitor build time so the per-cell
	// Relate hook can short-circuit on cells that lie outside the
	// bounding box without consulting the heavier Component2D tree.
	minLat int32
	maxLat int32
	minLon int32
	maxLon int32
}

// newLatLonPointSpatialVisitor wires the BaseSpatialVisitor backlink
// so GetInnerFunction / GetLeafPredicate dispatch through this
// type's Relate / Intersects / Within / Contains overrides. The
// Component2DPredicate and encoded bbox bounds are computed once
// at construction so the per-cell / per-doc paths stay tight.
func newLatLonPointSpatialVisitor(tree geo.Component2D) *latLonPointSpatialVisitor {
	v := &latLonPointSpatialVisitor{
		tree:      tree,
		predicate: geo.CreateComponentPredicate(tree),
		minLat:    geo.EncodeLatitude(tree.MinY()),
		maxLat:    geo.EncodeLatitude(tree.MaxY()),
		minLon:    geo.EncodeLongitude(tree.MinX()),
		maxLon:    geo.EncodeLongitude(tree.MaxX()),
	}
	v.BaseSpatialVisitor = NewBaseSpatialVisitor(v)
	return v
}

// Relate decodes the lat/lon corners of the cell described by
// minPackedValue / maxPackedValue, applies the encoded bounding-box
// short-circuit, and forwards to the underlying Component2D tree.
//
// Mirrors the Java reference's anonymous relate(byte[], byte[])
// override: it first compares the encoded bounds (which avoids a
// double-encode round-trip on the hot path) and only decodes to
// floating-point degrees if the cell survives the bbox test.
//
// Layout reminder: a single packed value is 8 bytes laid out as
//
//	[0..4) → sortable-bytes int32 latitude   (Y)
//	[4..8) → sortable-bytes int32 longitude  (X)
//
// matching [document.EncodeLatLon].
func (v *latLonPointSpatialVisitor) Relate(minPackedValue, maxPackedValue []byte) spatialRelation {
	if v.tree == nil {
		return spatialCellOutsideQuery
	}
	if len(minPackedValue) < 2*latLonPointBytesPerDim ||
		len(maxPackedValue) < 2*latLonPointBytesPerDim {
		return spatialCellOutsideQuery
	}

	latLowerBound := util.SortableBytesToInt(minPackedValue, 0)
	latUpperBound := util.SortableBytesToInt(maxPackedValue, 0)
	if latLowerBound > v.maxLat || latUpperBound < v.minLat {
		// Outside of the global bounding box range.
		return spatialCellOutsideQuery
	}

	lonLowerBound := util.SortableBytesToInt(minPackedValue, latLonPointBytesPerDim)
	lonUpperBound := util.SortableBytesToInt(maxPackedValue, latLonPointBytesPerDim)
	if lonLowerBound > v.maxLon || lonUpperBound < v.minLon {
		// Outside of the global bounding box range.
		return spatialCellOutsideQuery
	}

	cellMinLat := geo.DecodeLatitude(latLowerBound)
	cellMinLon := geo.DecodeLongitude(lonLowerBound)
	cellMaxLat := geo.DecodeLatitude(latUpperBound)
	cellMaxLon := geo.DecodeLongitude(lonUpperBound)

	return geoRelationToSpatial(v.tree.Relate(cellMinLon, cellMaxLon, cellMinLat, cellMaxLat))
}

// Intersects returns the per-doc predicate the parent uses for
// INTERSECTS / DISJOINT queries. Mirrors the Java reference's
// anonymous intersects(): decode the (lat, lon) packed value and
// delegate to the prebuilt Component2DPredicate.
func (v *latLonPointSpatialVisitor) Intersects() func(packed []byte) bool {
	return v.pointPredicate()
}

// Within returns the per-doc predicate the parent uses for WITHIN
// queries. Mirrors the Java reference's anonymous within(): the
// implementation collapses onto the same Component2DPredicate the
// INTERSECTS branch uses, because a point either lies inside the
// query region or it does not — there is no "partial" containment
// to distinguish.
func (v *latLonPointSpatialVisitor) Within() func(packed []byte) bool {
	return v.pointPredicate()
}

// pointPredicate returns the shared closure both Intersects and
// Within use. The closure decodes the 8-byte packed value into a
// (lat, lon) int32 pair and asks the Component2DPredicate whether
// the point is inside the query. Mirrors the Java reference's
// component2DPredicate.test(lat, lon) call site.
func (v *latLonPointSpatialVisitor) pointPredicate() func(packed []byte) bool {
	return func(packed []byte) bool {
		if len(packed) < 2*latLonPointBytesPerDim {
			return false
		}
		lat := util.SortableBytesToInt(packed, 0)
		lon := util.SortableBytesToInt(packed, latLonPointBytesPerDim)
		return v.predicate.Test(lat, lon)
	}
}

// Contains returns the per-doc classifier the parent uses for
// CONTAINS queries. Mirrors the Java reference's anonymous
// contains(): decode the packed value into (lon, lat) floating-
// point degrees and forward to Component2D.WithinPoint, which
// answers with the {CANDIDATE, NOTWITHIN, DISJOINT} relation the
// SpatialQuery pipeline expects.
func (v *latLonPointSpatialVisitor) Contains() func(packed []byte) geo.WithinRelation {
	return func(packed []byte) geo.WithinRelation {
		if len(packed) < 2*latLonPointBytesPerDim {
			return geo.WithinDisjoint
		}
		lon := geo.DecodeLongitude(util.SortableBytesToInt(packed, latLonPointBytesPerDim))
		lat := geo.DecodeLatitude(util.SortableBytesToInt(packed, 0))
		return v.tree.WithinPoint(lon, lat)
	}
}

// latLonPointBytesPerDim mirrors Integer.BYTES (4): the byte-width
// of a single LatLonPoint dimension in the packed payload. The
// layout is 4-byte latitude followed by 4-byte longitude, matching
// [document.LatLonPoint] / [document.EncodeLatLon].
const latLonPointBytesPerDim = 4

// Compile-time guards: the visitor satisfies SpatialVisitor.
var _ SpatialVisitor = (*latLonPointSpatialVisitor)(nil)
