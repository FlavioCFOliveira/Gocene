// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// LatLonShapeDocValuesQuery is the Go port of the package-private
// final class org.apache.lucene.document.LatLonShapeDocValuesQuery
// (Lucene 10.4.0). It evaluates a QueryRelation against an indexed
// LatLonShape stored as binary doc-values rather than as a BKD tree.
//
// Mirrors the Java reference's BaseShapeDocValuesQuery override of
// createComponent2D / getShapeDocValues / matchCost. The Java
// reference also overrides getSpatialVisitor by delegating to
// LatLonShapeQuery.getSpatialVisitor; the doc-values code path in
// Gocene's BaseShapeDocValuesQuery bypasses the visitor entirely
// (the parent installs a no-op placeholder), so the override is
// unreachable and intentionally omitted.
//
// # Decoder bridge: Java inheritance vs Gocene composition
//
// In Java, LatLonShapeDocValues extends ShapeDocValues; the
// per-doc decoder is `new LatLonShapeDocValues(binaryValue)`. In
// Gocene the two types are split (sprint 21 shipped
// document.LatLonShapeDocValues as a simplified per-triangle
// accessor; sprint 55 shipped document.ShapeDocValues as the full
// relate-capable base). LatLonShapeDocValuesQuery therefore bridges
// the two: the decoder closure parses the raw triangle stream stored
// by LatLonShapeDocValuesField (sprint 21) into a slice of
// DecodedTriangle and feeds it through
// NewShapeDocValuesFromTessellation with the production lat/lon
// encoder. This re-tessellation per doc allocates more than the Java
// reference; backlog #2697 will let LatLonShapeDocValuesField store
// the encoded ShapeDocValues tree directly so the decoder can fall
// back to NewShapeDocValuesFromBinary.
//
// # Decoded vertex caveat
//
// document.DecodeTriangle currently recovers only the A vertex and
// the three edge bits (B/C remain at zero pending the full Lucene
// rotation decoder — backlog #2697). The query therefore evaluates
// against partially-decoded triangles for LINE and TRIANGLE inputs,
// matching the sibling LatLonShapeQuery's accepted behaviour. POINT
// triangles round-trip cleanly.
type LatLonShapeDocValuesQuery struct {
	*BaseShapeDocValuesQuery
}

// NewLatLonShapeDocValuesQuery builds a LatLonShapeDocValuesQuery
// matching every indexed lat/lon shape whose relation to the
// supplied LatLonGeometry array equals queryRelation.
//
// Mirrors the Java package-private constructor
// LatLonShapeDocValuesQuery(String, QueryRelation,
// LatLonGeometry...). The constructor rejects CONTAINS through the
// embedded BaseShapeDocValuesQuery (mirroring the Java reference's
// IllegalArgumentException); WITHIN+Line is NOT rejected here
// because the Java reference does not check it on this path either
// (only the BKD-driven LatLonShapeQuery rejects WITHIN+Line).
//
// Returns an error if geometries is empty or any element is nil
// (surfaced by geo.CreateLatLonGeometry), if queryRelation ==
// QueryRelationContains
// (ErrBaseShapeDocValuesQueryContainsNotSupported), or if the
// embedded BaseShapeDocValuesQuery / SpatialQuery construction
// rejects the inputs.
func NewLatLonShapeDocValuesQuery(
	field string,
	queryRelation document.QueryRelation,
	geometries ...geo.LatLonGeometry,
) (*LatLonShapeDocValuesQuery, error) {
	tree, err := geo.CreateLatLonGeometry(geometries...)
	if err != nil {
		return nil, err
	}
	geomShapes := make([]geo.Geometry, len(geometries))
	for i, g := range geometries {
		geomShapes[i] = g
	}

	base, err := NewBaseShapeDocValuesQuery(
		field,
		queryRelation,
		tree,
		decodeLatLonShapeBinary,
		geomShapes,
		WithBaseShapeDocValuesMatchCost(latLonShapeDocValuesMatchCost),
	)
	if err != nil {
		return nil, err
	}
	return &LatLonShapeDocValuesQuery{BaseShapeDocValuesQuery: base}, nil
}

// decodeLatLonShapeBinary parses the per-doc binary payload (raw
// tessellated triangle stream) into a *document.ShapeDocValues built
// with the production lat/lon encoder. Returns nil ShapeDocValues
// without error for an empty payload (no triangles -> no match).
//
// Mirrors LatLonShapeDocValuesQuery.getShapeDocValues(BytesRef) in
// the Java reference; the Gocene bridge re-tessellates because the
// underlying LatLonShapeDocValuesField stores raw triangles rather
// than the encoded ShapeDocValues tree (see the package doc above).
func decodeLatLonShapeBinary(binaryValue *util.BytesRef) (*document.ShapeDocValues, error) {
	if binaryValue == nil || binaryValue.Length == 0 {
		return nil, nil
	}
	slice := binaryValue.Bytes[binaryValue.Offset : binaryValue.Offset+binaryValue.Length]
	llsdv, err := document.NewLatLonShapeDocValues(slice)
	if err != nil {
		return nil, fmt.Errorf("search: LatLonShapeDocValuesQuery decoder: %w", err)
	}
	n := llsdv.NumTriangles()
	if n == 0 {
		return nil, nil
	}
	triangles := make([]document.DecodedTriangle, n)
	for i := 0; i < n; i++ {
		t, err := llsdv.Triangle(i)
		if err != nil {
			return nil, fmt.Errorf("search: LatLonShapeDocValuesQuery decoder: triangle %d: %w", i, err)
		}
		triangles[i] = t
	}
	sdv, err := document.NewShapeDocValuesFromTessellation(
		document.LatLonShapeDocValuesEncoder,
		triangles,
		nilLatLonShapeDocValuesGeometry,
		nilLatLonShapeDocValuesGeometry,
	)
	if err != nil {
		return nil, fmt.Errorf("search: LatLonShapeDocValuesQuery decoder: shape build: %w", err)
	}
	return sdv, nil
}

// nilLatLonShapeDocValuesGeometry is the centroid / bounding-box
// hook the lat/lon decoder installs. The match path consults only
// Relate(), so the centroid and bbox accessors return nil; callers
// that need them must build a ShapeDocValues with proper hooks
// outside the query.
func nilLatLonShapeDocValuesGeometry(_ *document.ShapeDocValues) geo.Geometry { return nil }

// latLonShapeDocValuesMatchCost mirrors the Java override
// LatLonShapeDocValuesQuery.matchCost(): a flat estimate of 60 * 100
// (per-term comparisons × averaged terms-per-doc). The value is
// numerically identical to the BaseShapeDocValuesQuery default; the
// explicit override exists for parity with the Java reference and
// to make the cost source unambiguous.
func latLonShapeDocValuesMatchCost() float32 { return 60 * 100 }
