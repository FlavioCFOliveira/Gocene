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

// XYShapeDocValuesQuery is the Go port of the package-private
// final class org.apache.lucene.document.XYShapeDocValuesQuery
// (Lucene 10.4.0). It evaluates a QueryRelation against an indexed
// XYShape stored as binary doc-values rather than as a BKD tree.
//
// Mirrors the Java reference's BaseShapeDocValuesQuery override of
// createComponent2D / getShapeDocValues / matchCost. The Java
// reference also overrides getSpatialVisitor by delegating to
// XYShapeQuery.getSpatialVisitor; the doc-values code path in
// Gocene's BaseShapeDocValuesQuery bypasses the visitor entirely
// (the parent installs a no-op placeholder), so the override is
// unreachable and intentionally omitted.
//
// # Decoder bridge: Java inheritance vs Gocene composition
//
// In Java, XYShapeDocValues extends ShapeDocValues; the per-doc
// decoder is `new XYShapeDocValues(binaryValue)`. In Gocene the
// two types are split (sprint 21 shipped document.XYShapeDocValues
// as a simplified per-triangle accessor; sprint 55 shipped
// document.ShapeDocValues as the full relate-capable base).
// XYShapeDocValuesQuery therefore bridges the two: the decoder
// closure parses the raw triangle stream stored by
// XYShapeDocValuesField (sprint 21) into a slice of
// DecodedTriangle and feeds it through
// NewShapeDocValuesFromTessellation with the production XY
// encoder. This re-tessellation per doc allocates more than the
// Java reference; backlog #2697 will let XYShapeDocValuesField
// store the encoded ShapeDocValues tree directly so the decoder
// can fall back to NewShapeDocValuesFromBinary.
//
// # Decoded vertex caveat
//
// document.DecodeTriangle currently recovers only the A vertex and
// the three edge bits (B/C remain at zero pending the full Lucene
// rotation decoder — backlog #2697). The query therefore evaluates
// against partially-decoded triangles for LINE and TRIANGLE inputs,
// matching the sibling XYShapeQuery's accepted behaviour. POINT
// triangles round-trip cleanly.
type XYShapeDocValuesQuery struct {
	*BaseShapeDocValuesQuery
}

// NewXYShapeDocValuesQuery builds an XYShapeDocValuesQuery
// matching every indexed XY shape whose relation to the supplied
// XYGeometry array equals queryRelation.
//
// Mirrors the Java package-private constructor
// XYShapeDocValuesQuery(String, QueryRelation, XYGeometry...). The
// constructor rejects CONTAINS through the embedded
// BaseShapeDocValuesQuery (mirroring the Java reference's
// IllegalArgumentException); WITHIN+XYLine is NOT rejected here
// because the Java reference does not check it on this path either
// (only the BKD-driven XYShapeQuery rejects WITHIN+XYLine).
//
// Returns an error if geometries is empty or any element is nil
// (surfaced by geo.CreateXYGeometry), if queryRelation ==
// QueryRelationContains
// (ErrBaseShapeDocValuesQueryContainsNotSupported), or if the
// embedded BaseShapeDocValuesQuery / SpatialQuery construction
// rejects the inputs.
func NewXYShapeDocValuesQuery(
	field string,
	queryRelation document.QueryRelation,
	geometries ...geo.XYGeometry,
) (*XYShapeDocValuesQuery, error) {
	tree, err := geo.CreateXYGeometry(geometries...)
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
		decodeXYShapeBinary,
		geomShapes,
		WithBaseShapeDocValuesMatchCost(xyShapeDocValuesMatchCost),
	)
	if err != nil {
		return nil, err
	}
	return &XYShapeDocValuesQuery{BaseShapeDocValuesQuery: base}, nil
}

// decodeXYShapeBinary parses the per-doc binary payload (raw
// tessellated triangle stream) into a *document.ShapeDocValues built
// with the production XY encoder. Returns nil ShapeDocValues without
// error for an empty payload (no triangles -> no match).
//
// Mirrors XYShapeDocValuesQuery.getShapeDocValues(BytesRef) in the
// Java reference; the Gocene bridge re-tessellates because the
// underlying XYShapeDocValuesField stores raw triangles rather than
// the encoded ShapeDocValues tree (see the package doc above).
func decodeXYShapeBinary(binaryValue *util.BytesRef) (*document.ShapeDocValues, error) {
	if binaryValue == nil || binaryValue.Length == 0 {
		return nil, nil
	}
	slice := binaryValue.Bytes[binaryValue.Offset : binaryValue.Offset+binaryValue.Length]
	xydv, err := document.NewXYShapeDocValues(slice)
	if err != nil {
		return nil, fmt.Errorf("search: XYShapeDocValuesQuery decoder: %w", err)
	}
	n := xydv.NumTriangles()
	if n == 0 {
		return nil, nil
	}
	triangles := make([]document.DecodedTriangle, n)
	for i := 0; i < n; i++ {
		t, err := xydv.Triangle(i)
		if err != nil {
			return nil, fmt.Errorf("search: XYShapeDocValuesQuery decoder: triangle %d: %w", i, err)
		}
		triangles[i] = t
	}
	sdv, err := document.NewShapeDocValuesFromTessellation(
		document.XYShapeDocValuesEncoder,
		triangles,
		nilXYShapeDocValuesGeometry,
		nilXYShapeDocValuesGeometry,
	)
	if err != nil {
		return nil, fmt.Errorf("search: XYShapeDocValuesQuery decoder: shape build: %w", err)
	}
	return sdv, nil
}

// nilXYShapeDocValuesGeometry is the centroid / bounding-box hook
// the XY decoder installs. The match path consults only Relate(),
// so the centroid and bbox accessors return nil; callers that need
// them must build a ShapeDocValues with proper hooks outside the
// query.
func nilXYShapeDocValuesGeometry(_ *document.ShapeDocValues) geo.Geometry { return nil }

// xyShapeDocValuesMatchCost mirrors the Java override
// XYShapeDocValuesQuery.matchCost(): a flat estimate of 60 * 100
// (per-term comparisons × averaged terms-per-doc). The value is
// numerically identical to the BaseShapeDocValuesQuery default; the
// explicit override exists for parity with the Java reference and
// to make the cost source unambiguous.
func xyShapeDocValuesMatchCost() float32 { return 60 * 100 }
