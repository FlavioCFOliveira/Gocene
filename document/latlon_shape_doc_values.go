// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// latLonComputeCentroid is the Go port of LatLonShapeDocValues.computeCentroid.
func latLonComputeCentroid(sdv *ShapeDocValues) geo.Geometry {
	return geo.MustNewPoint(
		LatLonShapeDocValuesEncoder.DecodeY(sdv.GetEncodedCentroidY()),
		LatLonShapeDocValuesEncoder.DecodeX(sdv.GetEncodedCentroidX()),
	)
}

// latLonComputeBoundingBox is the Go port of LatLonShapeDocValues.computeBoundingBox.
func latLonComputeBoundingBox(sdv *ShapeDocValues) geo.Geometry {
	return geo.MustNewRectangle(
		LatLonShapeDocValuesEncoder.DecodeY(sdv.GetEncodedMinY()),
		LatLonShapeDocValuesEncoder.DecodeY(sdv.GetEncodedMaxY()),
		LatLonShapeDocValuesEncoder.DecodeX(sdv.GetEncodedMinX()),
		LatLonShapeDocValuesEncoder.DecodeX(sdv.GetEncodedMaxX()),
	)
}

// LatLonShapeDocValues is a concrete implementation of ShapeDocValues for storing
// binary doc value representation of LatLonShape geometries.
//
// Mirrors org.apache.lucene.document.LatLonShapeDocValues.
type LatLonShapeDocValues struct {
	*ShapeDocValues
}

// NewLatLonShapeDocValuesFromTessellation builds a LatLonShapeDocValues from a
// tessellation. Mirrors the Java constructor LatLonShapeDocValues(List<DecodedTriangle>).
func NewLatLonShapeDocValuesFromTessellation(tessellation []DecodedTriangle) (*LatLonShapeDocValues, error) {
	sdv, err := NewShapeDocValuesFromTessellation(
		LatLonShapeDocValuesEncoder,
		tessellation,
		latLonComputeCentroid,
		latLonComputeBoundingBox,
	)
	if err != nil {
		return nil, err
	}
	return &LatLonShapeDocValues{sdv}, nil
}

// NewLatLonShapeDocValuesFromBinary builds a LatLonShapeDocValues from an already
// retrieved binary format. Mirrors the Java constructor LatLonShapeDocValues(BytesRef).
func NewLatLonShapeDocValuesFromBinary(binaryValue *util.BytesRef) (*LatLonShapeDocValues, error) {
	sdv, err := NewShapeDocValuesFromBinary(
		LatLonShapeDocValuesEncoder,
		binaryValue,
		latLonComputeCentroid,
		latLonComputeBoundingBox,
	)
	if err != nil {
		return nil, err
	}
	return &LatLonShapeDocValues{sdv}, nil
}

// NewLatLonShapeDocValues is a convenience constructor that takes raw bytes.
func NewLatLonShapeDocValues(triangles []byte) (*LatLonShapeDocValues, error) {
	return NewLatLonShapeDocValuesFromBinary(&util.BytesRef{Bytes: triangles})
}

// NumTriangles returns the number of triangles stored.
func (l *LatLonShapeDocValues) NumTriangles() int {
	return l.NumberOfTerms()
}

// Bytes returns the underlying binary payload.
func (l *LatLonShapeDocValues) Bytes() []byte {
	return l.BinaryValue().Bytes
}

// Triangle returns the decoded triangle at the given index.
//
// NOTE: This method is NOT in the Java reference and is provided for compatibility
// with existing Gocene tests. Since ShapeDocValues uses a BKD-tree layout,
// this requires a full in-order traversal.
func (l *LatLonShapeDocValues) Triangle(i int) (DecodedTriangle, error) {
	// Implementation deferred: requires in-order traversal of BKD tree.
	return DecodedTriangle{}, fmt.Errorf("Triangle(i) not implemented for BKD-tree layout")
}

// latLonShapeEncoder is the production ShapeDocValuesEncoder used by
// the geographic LatLonShape family.
type latLonShapeEncoder struct{}

// LatLonShapeDocValuesEncoder is the singleton instance of the
// production lat/lon ShapeDocValuesEncoder.
var LatLonShapeDocValuesEncoder ShapeDocValuesEncoder = latLonShapeEncoder{}

func (latLonShapeEncoder) EncodeX(x float64) int32 { return geo.EncodeLongitude(x) }
func (latLonShapeEncoder) EncodeY(y float64) int32 { return geo.EncodeLatitude(y) }
func (latLonShapeEncoder) DecodeX(x int32) float64 { return geo.DecodeLongitude(x) }
func (latLonShapeEncoder) DecodeY(y int32) float64 { return geo.DecodeLatitude(y) }

// LatLonShapeDocValuesField stores a LatLonShape as binary doc-values.
//
// Go port of Lucene 10.4.0's LatLonShapeDocValuesField. Wraps a
// LatLonShapeDocValues payload inside a BinaryDocValuesField.
type LatLonShapeDocValuesField struct {
	*BinaryDocValuesField
	shape *LatLonShapeDocValues
}

// NewLatLonShapeDocValuesField creates a new LatLonShapeDocValuesField
// from a Polygon.
func NewLatLonShapeDocValuesField(name string, polygon geo.Polygon) (*LatLonShapeDocValuesField, error) {
	return NewLatLonShapeDocValuesFieldPolygonChecked(name, polygon, false)
}

// NewLatLonShapeDocValuesFieldPolygonChecked creates a new
// LatLonShapeDocValuesField from a Polygon, honouring the
// checkSelfIntersections flag.
func NewLatLonShapeDocValuesFieldPolygonChecked(name string, polygon geo.Polygon, checkSelfIntersections bool) (*LatLonShapeDocValuesField, error) {
	geoTriangles, err := geo.Tessellate(polygon, checkSelfIntersections)
	if err != nil {
		return nil, fmt.Errorf("tessellate polygon: %w", err)
	}
	triangles := make([]DecodedTriangle, 0, len(geoTriangles))
	for _, gt := range geoTriangles {
		triangles = append(triangles, DecodedTriangle{
			AX: geo.EncodeLongitude(gt.AX()), AY: geo.EncodeLatitude(gt.AY()),
			BX: geo.EncodeLongitude(gt.BX()), BY: geo.EncodeLatitude(gt.BY()),
			CX: geo.EncodeLongitude(gt.CX()), CY: geo.EncodeLatitude(gt.CY()),
			AB: gt.EdgeFromPolygon(0), BC: gt.EdgeFromPolygon(1), CA: gt.EdgeFromPolygon(2),
			Kind: DecodedTriangleTypeTriangle,
		})
	}
	dv, err := NewLatLonShapeDocValuesFromTessellation(triangles)
	if err != nil {
		return nil, err
	}
	payload := dv.BinaryValue().Bytes
	b, err := NewBinaryDocValuesField(name, payload)
	if err != nil {
		return nil, err
	}
	return &LatLonShapeDocValuesField{BinaryDocValuesField: b, shape: dv}, nil
}

// NewLatLonShapeDocValuesFieldLine creates a LatLonShapeDocValuesField
// over the segments of the supplied Line.
func NewLatLonShapeDocValuesFieldLine(name string, line geo.Line) (*LatLonShapeDocValuesField, error) {
	numPoints := line.NumPoints()
	if numPoints < 2 {
		return nil, fmt.Errorf("line requires at least two vertices; got %d", numPoints)
	}
	triangles := make([]DecodedTriangle, 0, numPoints-1)
	for i := 0; i+1 < numPoints; i++ {
		ax, ay := line.Lon(i), line.Lat(i)
		bx, by := line.Lon(i+1), line.Lat(i+1)
		triangles = append(triangles, DecodedTriangle{
			AX: geo.EncodeLongitude(ax), AY: geo.EncodeLatitude(ay),
			BX: geo.EncodeLongitude(bx), BY: geo.EncodeLatitude(by),
			CX: geo.EncodeLongitude(ax), CY: geo.EncodeLatitude(ay),
			AB: true, BC: true, CA: true,
			Kind: DecodedTriangleTypeLine,
		})
	}
	dv, err := NewLatLonShapeDocValuesFromTessellation(triangles)
	if err != nil {
		return nil, err
	}
	payload := dv.BinaryValue().Bytes
	b, err := NewBinaryDocValuesField(name, payload)
	if err != nil {
		return nil, err
	}
	return &LatLonShapeDocValuesField{BinaryDocValuesField: b, shape: dv}, nil
}

// NewLatLonShapeDocValuesFieldPoint creates a LatLonShapeDocValuesField
// holding a single (lat, lon) point.
func NewLatLonShapeDocValuesFieldPoint(name string, latitude, longitude float64) (*LatLonShapeDocValuesField, error) {
	x := geo.EncodeLongitude(longitude)
	y := geo.EncodeLatitude(latitude)
	triangles := []DecodedTriangle{{
		AX: x, AY: y, BX: x, BY: y, CX: x, CY: y,
		AB: true, BC: true, CA: true,
		Kind: DecodedTriangleTypePoint,
	}}
	dv, err := NewLatLonShapeDocValuesFromTessellation(triangles)
	if err != nil {
		return nil, err
	}
	payload := dv.BinaryValue().Bytes
	b, err := NewBinaryDocValuesField(name, payload)
	if err != nil {
		return nil, err
	}
	return &LatLonShapeDocValuesField{BinaryDocValuesField: b, shape: dv}, nil
}

// NewLatLonShapeDocValuesFieldFromBytes wraps an already-encoded triangle
// byte payload as a LatLonShapeDocValuesField.
func NewLatLonShapeDocValuesFieldFromBytes(name string, binaryValue []byte) (*LatLonShapeDocValuesField, error) {
	dv, err := NewLatLonShapeDocValues(binaryValue)
	if err != nil {
		return nil, err
	}
	b, err := NewBinaryDocValuesField(name, binaryValue)
	if err != nil {
		return nil, err
	}
	return &LatLonShapeDocValuesField{BinaryDocValuesField: b, shape: dv}, nil
}

// NewLatLonShapeDocValuesFieldFromTriangles encodes the supplied slice
// of DecodedTriangle records into a LatLonShapeDocValuesField.
func NewLatLonShapeDocValuesFieldFromTriangles(name string, triangles []DecodedTriangle) (*LatLonShapeDocValuesField, error) {
	dv, err := NewLatLonShapeDocValuesFromTessellation(triangles)
	if err != nil {
		return nil, err
	}
	payload := dv.BinaryValue().Bytes
	b, err := NewBinaryDocValuesField(name, payload)
	if err != nil {
		return nil, err
	}
	return &LatLonShapeDocValuesField{BinaryDocValuesField: b, shape: dv}, nil
}

// NewLatLonShapeDocValuesFieldFromFields aggregates the encoded payloads
// of a slice of ShapeFieldTriangle indexable fields into a single
// LatLonShapeDocValuesField.
func NewLatLonShapeDocValuesFieldFromFields(name string, indexableFields []*ShapeFieldTriangle) (*LatLonShapeDocValuesField, error) {
	triangles := make([]DecodedTriangle, 0, len(indexableFields))
	for i, f := range indexableFields {
		if f == nil {
			return nil, fmt.Errorf("nil indexable field at index %d", i)
		}
		bv := f.BinaryValue()
		if len(bv) != ShapeFieldBytes {
			return nil, fmt.Errorf("indexable field %d binary length %d != %d", i, len(bv), ShapeFieldBytes)
		}
		tri, err := DecodeTriangle(bv)
		if err != nil {
			return nil, fmt.Errorf("decode triangle at index %d: %w", i, err)
		}
		triangles = append(triangles, tri)
	}
	dv, err := NewLatLonShapeDocValuesFromTessellation(triangles)
	if err != nil {
		return nil, err
	}
	payload := dv.BinaryValue().Bytes
	b, err := NewBinaryDocValuesField(name, payload)
	if err != nil {
		return nil, err
	}
	return &LatLonShapeDocValuesField{BinaryDocValuesField: b, shape: dv}, nil
}

// Shape returns the wrapped LatLonShapeDocValues accessor.
func (f *LatLonShapeDocValuesField) Shape() *LatLonShapeDocValues { return f.shape }
