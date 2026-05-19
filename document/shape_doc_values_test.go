// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// identityEncoder is a synthetic Encoder used by the unit tests. It
// maps double <-> int32 via a simple cast (clipped at the int32
// range). This avoids depending on the LatLon / XY encoders, which
// belong to subclass-port tasks GOC-3218 and GOC-4532.
type identityEncoder struct{}

func (identityEncoder) EncodeX(x float64) int32 { return clipToInt32(x) }
func (identityEncoder) EncodeY(y float64) int32 { return clipToInt32(y) }
func (identityEncoder) DecodeX(i int32) float64 { return float64(i) }
func (identityEncoder) DecodeY(i int32) float64 { return float64(i) }

func clipToInt32(v float64) int32 {
	switch {
	case v >= 2_147_483_647:
		return 2_147_483_647
	case v <= -2_147_483_648:
		return -2_147_483_648
	default:
		return int32(v)
	}
}

// stubCentroidFn / stubBoundingBoxFn are minimal centroid/bbox hooks
// for synthetic tests: they capture the encoded values without
// projecting through any encoder. The tests inspect the encoded
// accessors directly, so the returned Geometry can be nil.
func stubCentroidFn(*ShapeDocValues) geo.Geometry    { return nil }
func stubBoundingBoxFn(*ShapeDocValues) geo.Geometry { return nil }

// triangleAt builds a single CCW triangle with vertices at the given
// integer coordinates and all edges marked as shape edges.
func triangleAt(ax, ay, bx, by, cx, cy int32) DecodedTriangle {
	return DecodedTriangle{
		Kind: DecodedTriangleTypeTriangle,
		AX:   ax, AY: ay, BX: bx, BY: by, CX: cx, CY: cy,
		AB: true, BC: true, CA: true,
	}
}

// pointAt builds a single POINT-kind decoded triangle (all three
// vertices identical, mirroring the Lucene convention).
func pointAt(x, y int32) DecodedTriangle {
	return DecodedTriangle{
		Kind: DecodedTriangleTypePoint,
		AX:   x, AY: y, BX: x, BY: y, CX: x, CY: y,
	}
}

// TestShapeDocValues_VLongSize_Roundtrip verifies that VLongSize /
// VIntSize return exactly the number of bytes ByteBuffersDataOutput
// will emit for the same value, mirroring the Lucene test peer
// TestShapeDocValues.testVariableValueSizes.
func TestShapeDocValues_VLongSize_Roundtrip(t *testing.T) {
	t.Parallel()
	out := store.NewByteBuffersDataOutput()
	rng := rand.New(rand.NewSource(0xC0CE_2026))

	const iterations = 400
	for i := 0; i < iterations; i++ {
		// VInt round-trip.
		testInt := rng.Int31n(2_147_483_647)
		before := out.Size()
		if err := out.WriteVInt(testInt); err != nil {
			t.Fatalf("WriteVInt(%d): %v", testInt, err)
		}
		after := out.Size()
		if got, want := int(after-before), VIntSize(testInt); got != want {
			t.Fatalf("VIntSize(%d) = %d; ByteBuffersDataOutput wrote %d", testInt, want, got)
		}

		// VLong round-trip.
		testLong := rng.Int63n(9_223_372_036_854_775_807)
		before = out.Size()
		if err := out.WriteVLong(testLong); err != nil {
			t.Fatalf("WriteVLong(%d): %v", testLong, err)
		}
		after = out.Size()
		if got, want := int(after-before), VLongSize(testLong); got != want {
			t.Fatalf("VLongSize(%d) = %d; ByteBuffersDataOutput wrote %d", testLong, want, got)
		}
	}
}

// TestShapeDocValues_VLongSize_Boundaries pins the size at the
// 7-bit / 14-bit boundaries to catch off-by-one errors in either
// function.
func TestShapeDocValues_VLongSize_Boundaries(t *testing.T) {
	t.Parallel()
	cases := []struct {
		value     int64
		wantBytes int
	}{
		{0, 1},
		{1, 1},
		{0x7F, 1},
		{0x80, 2},
		{0x3FFF, 2},
		{0x4000, 3},
		{0x1FFFFF, 3},
		{0x200000, 4},
		{0x0FFFFFFF, 4},
		{0x10000000, 5},
		{0x7FFFFFFFF, 5},
	}
	for _, tc := range cases {
		if got := VLongSize(tc.value); got != tc.wantBytes {
			t.Errorf("VLongSize(%#x) = %d; want %d", tc.value, got, tc.wantBytes)
		}
	}
}

// TestShapeDocValues_Header_Roundtrip exercises the full encode →
// decode loop for a single-triangle shape and asserts every public
// accessor matches the tessellation input.
func TestShapeDocValues_Header_Roundtrip(t *testing.T) {
	t.Parallel()
	tess := []DecodedTriangle{
		triangleAt(0, 0, 10, 0, 5, 8),
	}
	sdv, err := NewShapeDocValuesFromTessellation(
		identityEncoder{}, tess, stubCentroidFn, stubBoundingBoxFn,
	)
	if err != nil {
		t.Fatalf("NewShapeDocValuesFromTessellation: %v", err)
	}

	if got, want := sdv.NumberOfTerms(), len(tess); got != want {
		t.Errorf("NumberOfTerms = %d; want %d", got, want)
	}
	if got, want := sdv.GetEncodedMinX(), int32(0); got != want {
		t.Errorf("GetEncodedMinX = %d; want %d", got, want)
	}
	if got, want := sdv.GetEncodedMinY(), int32(0); got != want {
		t.Errorf("GetEncodedMinY = %d; want %d", got, want)
	}
	if got, want := sdv.GetEncodedMaxX(), int32(10); got != want {
		t.Errorf("GetEncodedMaxX = %d; want %d", got, want)
	}
	if got, want := sdv.GetEncodedMaxY(), int32(8); got != want {
		t.Errorf("GetEncodedMaxY = %d; want %d", got, want)
	}
	if got, want := sdv.GetHighestDimension(), DecodedTriangleTypeTriangle; got != want {
		t.Errorf("GetHighestDimension = %v; want %v", got, want)
	}
}

// TestShapeDocValues_Header_PointHighestDimension verifies that a
// shape of only POINT-kind triangles reports POINT as its highest
// dimension.
func TestShapeDocValues_Header_PointHighestDimension(t *testing.T) {
	t.Parallel()
	tess := []DecodedTriangle{
		pointAt(1, 1),
		pointAt(2, 5),
		pointAt(7, 3),
	}
	sdv, err := NewShapeDocValuesFromTessellation(
		identityEncoder{}, tess, stubCentroidFn, stubBoundingBoxFn,
	)
	if err != nil {
		t.Fatalf("NewShapeDocValuesFromTessellation: %v", err)
	}
	if got, want := sdv.GetHighestDimension(), DecodedTriangleTypePoint; got != want {
		t.Errorf("GetHighestDimension = %v; want %v", got, want)
	}
	if got, want := sdv.NumberOfTerms(), len(tess); got != want {
		t.Errorf("NumberOfTerms = %d; want %d", got, want)
	}
}

// TestShapeDocValues_FromBinary_RoundTrip serializes a shape via the
// tessellation constructor and then reconstructs an equivalent
// ShapeDocValues from the resulting BytesRef.
func TestShapeDocValues_FromBinary_RoundTrip(t *testing.T) {
	t.Parallel()
	tess := []DecodedTriangle{
		triangleAt(-100, -100, 100, -100, 0, 100),
		triangleAt(200, 200, 300, 200, 250, 280),
	}
	src, err := NewShapeDocValuesFromTessellation(
		identityEncoder{}, tess, stubCentroidFn, stubBoundingBoxFn,
	)
	if err != nil {
		t.Fatalf("source NewShapeDocValuesFromTessellation: %v", err)
	}

	dst, err := NewShapeDocValuesFromBinary(
		identityEncoder{}, src.BinaryValue(), stubCentroidFn, stubBoundingBoxFn,
	)
	if err != nil {
		t.Fatalf("NewShapeDocValuesFromBinary: %v", err)
	}
	if got, want := dst.NumberOfTerms(), src.NumberOfTerms(); got != want {
		t.Errorf("dst.NumberOfTerms = %d; want %d", got, want)
	}
	if got, want := dst.GetEncodedMinX(), src.GetEncodedMinX(); got != want {
		t.Errorf("dst.GetEncodedMinX = %d; want %d", got, want)
	}
	if got, want := dst.GetEncodedMaxX(), src.GetEncodedMaxX(); got != want {
		t.Errorf("dst.GetEncodedMaxX = %d; want %d", got, want)
	}
	if got, want := dst.GetEncodedMinY(), src.GetEncodedMinY(); got != want {
		t.Errorf("dst.GetEncodedMinY = %d; want %d", got, want)
	}
	if got, want := dst.GetEncodedMaxY(), src.GetEncodedMaxY(); got != want {
		t.Errorf("dst.GetEncodedMaxY = %d; want %d", got, want)
	}
	if got, want := dst.GetEncodedCentroidX(), src.GetEncodedCentroidX(); got != want {
		t.Errorf("dst.GetEncodedCentroidX = %d; want %d", got, want)
	}
	if got, want := dst.GetEncodedCentroidY(), src.GetEncodedCentroidY(); got != want {
		t.Errorf("dst.GetEncodedCentroidY = %d; want %d", got, want)
	}
	if got, want := dst.GetHighestDimension(), src.GetHighestDimension(); got != want {
		t.Errorf("dst.GetHighestDimension = %v; want %v", got, want)
	}
}

// TestShapeDocValues_Errors verifies the constructor surface error
// returns rather than panicking.
func TestShapeDocValues_Errors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		fn   func() error
	}{
		{
			name: "nil encoder",
			fn: func() error {
				_, err := NewShapeDocValuesFromTessellation(
					nil,
					[]DecodedTriangle{triangleAt(0, 0, 1, 0, 0, 1)},
					stubCentroidFn, stubBoundingBoxFn,
				)
				return err
			},
		},
		{
			name: "empty tessellation",
			fn: func() error {
				_, err := NewShapeDocValuesFromTessellation(
					identityEncoder{},
					nil,
					stubCentroidFn, stubBoundingBoxFn,
				)
				return err
			},
		},
		{
			name: "nil centroid hook",
			fn: func() error {
				_, err := NewShapeDocValuesFromTessellation(
					identityEncoder{},
					[]DecodedTriangle{triangleAt(0, 0, 1, 0, 0, 1)},
					nil, stubBoundingBoxFn,
				)
				return err
			},
		},
		{
			name: "nil bbox hook",
			fn: func() error {
				_, err := NewShapeDocValuesFromTessellation(
					identityEncoder{},
					[]DecodedTriangle{triangleAt(0, 0, 1, 0, 0, 1)},
					stubCentroidFn, nil,
				)
				return err
			},
		},
		{
			name: "nil binary value",
			fn: func() error {
				_, err := NewShapeDocValuesFromBinary(
					identityEncoder{}, nil, stubCentroidFn, stubBoundingBoxFn,
				)
				return err
			},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := tc.fn(); err == nil {
				t.Fatalf("expected error, got nil")
			}
		})
	}
}

// TestShapeDocValues_Relate uses a real lat/lon Rectangle as the
// query and a single-triangle shape with vertices placed at lon/lat
// degrees. The encoder is the lat/lon encoder so the relation logic
// can decode back to degrees inside the comparator.
func TestShapeDocValues_Relate(t *testing.T) {
	t.Parallel()

	// One triangle covering roughly lon [-10, 10], lat [-5, 5].
	tess := []DecodedTriangle{
		{
			Kind: DecodedTriangleTypeTriangle,
			AX:   geo.EncodeLongitude(-10), AY: geo.EncodeLatitude(-5),
			BX: geo.EncodeLongitude(10), BY: geo.EncodeLatitude(-5),
			CX: geo.EncodeLongitude(0), CY: geo.EncodeLatitude(5),
			AB: true, BC: true, CA: true,
		},
	}
	sdv, err := NewShapeDocValuesFromTessellation(
		LatLonShapeDocValuesEncoder, tess, stubCentroidFn, stubBoundingBoxFn,
	)
	if err != nil {
		t.Fatalf("NewShapeDocValuesFromTessellation: %v", err)
	}

	// Inside query: lon ~[-1, 1], lat ~[-1, 1] sits fully inside the
	// triangle's bbox and crosses the triangle.
	insideRect, err := geo.NewRectangle(-1, 1, -1, 1)
	if err != nil {
		t.Fatalf("NewRectangle inside: %v", err)
	}
	insideComponent, err := geo.CreateLatLonGeometry(insideRect)
	if err != nil {
		t.Fatalf("CreateLatLonGeometry inside: %v", err)
	}
	rel, err := sdv.Relate(insideComponent)
	if err != nil {
		t.Fatalf("Relate inside: %v", err)
	}
	if rel != geo.CellCrossesQuery {
		t.Errorf("Relate inside = %v; want CellCrossesQuery", rel)
	}

	// Outside query: lon ~[50, 60], lat ~[50, 60] sits fully outside
	// the bbox of the triangle, so the comparator should short-circuit
	// at the bbox check and return CellOutsideQuery.
	outsideRect, err := geo.NewRectangle(50, 60, 50, 60)
	if err != nil {
		t.Fatalf("NewRectangle outside: %v", err)
	}
	outsideComponent, err := geo.CreateLatLonGeometry(outsideRect)
	if err != nil {
		t.Fatalf("CreateLatLonGeometry outside: %v", err)
	}
	rel, err = sdv.Relate(outsideComponent)
	if err != nil {
		t.Fatalf("Relate outside: %v", err)
	}
	if rel != geo.CellOutsideQuery {
		t.Errorf("Relate outside = %v; want CellOutsideQuery", rel)
	}
}

// TestShapeDocValues_NewGeometryQuery_Stub asserts the placeholder
// method still returns nil while we wait for GOC-4532+ to wire the
// concrete ShapeDocValuesQuery.
func TestShapeDocValues_NewGeometryQuery_Stub(t *testing.T) {
	t.Parallel()
	if got := NewGeometryQuery("f", QueryRelationIntersects); got != nil {
		t.Fatalf("NewGeometryQuery stub = %v; want nil", got)
	}
}
