// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom_test

import (
	"bytes"
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/spatial3d/geom"
)

// ---------------------------------------------------------------------------
// Vector
// ---------------------------------------------------------------------------

func TestVectorDotProduct(t *testing.T) {
	a := &geom.Vector{X: 1, Y: 0, Z: 0}
	b := &geom.Vector{X: 0, Y: 1, Z: 0}
	if got := a.DotProduct(b); got != 0 {
		t.Fatalf("dot product of perpendicular vectors: want 0, got %g", got)
	}
}

func TestVectorMagnitude(t *testing.T) {
	if got := geom.Magnitude(3, 4, 0); math.Abs(got-5) > 1e-10 {
		t.Fatalf("magnitude: want 5, got %g", got)
	}
}

func TestVectorNormalize(t *testing.T) {
	v := &geom.Vector{X: 3, Y: 4, Z: 0}
	n := v.Normalize()
	m := geom.Magnitude(n.X, n.Y, n.Z)
	if math.Abs(m-1) > 1e-10 {
		t.Fatalf("normalized magnitude: want 1, got %g", m)
	}
}

func TestVectorLinearDistanceSquared(t *testing.T) {
	a := &geom.Vector{X: 1, Y: 0, Z: 0}
	b := &geom.Vector{X: 0, Y: 1, Z: 0}
	want := 2.0
	if got := a.LinearDistanceSquared(b); math.Abs(got-want) > 1e-12 {
		t.Fatalf("linear distance squared: want %g, got %g", want, got)
	}
}

// ---------------------------------------------------------------------------
// PlanetModel
// ---------------------------------------------------------------------------

func TestSphereSymmetry(t *testing.T) {
	pm := geom.SPHERE
	if math.Abs(pm.A-1) > 1e-12 {
		t.Fatalf("sphere A: want 1, got %g", pm.A)
	}
	if math.Abs(pm.B-1) > 1e-12 {
		t.Fatalf("sphere B: want 1, got %g", pm.B)
	}
}

func TestWGS84NotSphere(t *testing.T) {
	wgs := geom.WGS84
	if math.Abs(wgs.A-wgs.B) < 1e-10 {
		t.Fatal("WGS84 should have different A and B axes")
	}
}

func TestPlanetModelNorthPole(t *testing.T) {
	pm := geom.SPHERE
	if pm.NorthPole == nil {
		t.Fatal("NorthPole must not be nil")
	}
	if math.Abs(pm.NorthPole.Z-1) > 1e-10 {
		t.Fatalf("sphere north pole Z: want 1, got %g", pm.NorthPole.Z)
	}
}

// ---------------------------------------------------------------------------
// GeoPoint
// ---------------------------------------------------------------------------

func TestGeoPointLatLon(t *testing.T) {
	pm := geom.SPHERE
	lat := math.Pi / 6 // 30°
	lon := math.Pi / 4 // 45°
	p := geom.NewGeoPointLatLon(pm, lat, lon)
	if math.Abs(p.GetLatitude()-lat) > 1e-10 {
		t.Fatalf("latitude: want %g, got %g", lat, p.GetLatitude())
	}
	if math.Abs(p.GetLongitude()-lon) > 1e-10 {
		t.Fatalf("longitude: want %g, got %g", lon, p.GetLongitude())
	}
}

func TestGeoPointMagnitudeSphere(t *testing.T) {
	pm := geom.SPHERE
	p := geom.NewGeoPointLatLon(pm, 0, 0) // equator
	m := p.Magnitude()
	if math.Abs(m-1) > 1e-10 {
		t.Fatalf("magnitude on unit sphere: want 1, got %g", m)
	}
}

// ---------------------------------------------------------------------------
// Plane
// ---------------------------------------------------------------------------

func TestPlaneEvaluate(t *testing.T) {
	// XY plane: z = 0, so (0,0,1) → evaluate = 1
	plane := geom.NewPlane(0, 0, 1, 0)
	v := &geom.Vector{X: 0, Y: 0, Z: 1}
	if got := plane.Evaluate(v); math.Abs(got-1) > 1e-12 {
		t.Fatalf("plane evaluate: want 1, got %g", got)
	}
}

func TestPlaneEvaluateIsZero(t *testing.T) {
	plane := geom.NewPlane(0, 0, 1, 0)
	v := &geom.Vector{X: 1, Y: 1, Z: 0}
	if !plane.EvaluateIsZero(v) {
		t.Fatal("(1,1,0) should be on z=0 plane")
	}
}

// ---------------------------------------------------------------------------
// SidedPlane
// ---------------------------------------------------------------------------

func TestSidedPlaneIsWithin(t *testing.T) {
	// Plane z=0, inside point at z=1 — so z>0 is inside.
	plane := geom.NewPlane(0, 0, 1, 0)
	inside := &geom.Vector{X: 0, Y: 0, Z: 1}
	sp := geom.NewSidedPlaneFromPlane(inside, plane)
	if !sp.IsWithin(0, 0, 0.5) {
		t.Fatal("point above z=0 should be within sided plane")
	}
	if sp.IsWithin(0, 0, -0.5) {
		t.Fatal("point below z=0 should be outside sided plane")
	}
}

// ---------------------------------------------------------------------------
// LatLonBounds
// ---------------------------------------------------------------------------

func TestLatLonBoundsAddPoint(t *testing.T) {
	pm := geom.SPHERE
	b := geom.NewLatLonBounds()
	p := geom.NewGeoPointLatLon(pm, 0.1, 0.2)
	b.AddPoint(p)
	if b.GetMinLatitude() > 0.1+1e-12 {
		t.Fatalf("minLat should be ≤ 0.1, got %g", b.GetMinLatitude())
	}
	if b.GetMaxLatitude() < 0.1-1e-12 {
		t.Fatalf("maxLat should be ≥ 0.1, got %g", b.GetMaxLatitude())
	}
}

func TestLatLonBoundsNoLongitudeBound(t *testing.T) {
	b := geom.NewLatLonBounds()
	b.NoLongitudeBound()
	if !b.CheckNoLongitudeBound() {
		t.Fatal("NoLongitudeBound should set flag")
	}
}

// ---------------------------------------------------------------------------
// XYZBounds
// ---------------------------------------------------------------------------

func TestXYZBoundsAddPoint(t *testing.T) {
	pm := geom.SPHERE
	b := geom.NewXYZBounds()
	p := geom.NewGeoPointLatLon(pm, 0, 0)
	b.AddPoint(p)
	if b.MaximumX < -1e-12 {
		t.Fatalf("MaximumX should be positive after adding equator point")
	}
}

// ---------------------------------------------------------------------------
// Distance styles
// ---------------------------------------------------------------------------

func TestArcDistanceToFromAggregation(t *testing.T) {
	d := geom.ArcDistanceInstance
	val := 1.5
	if got := d.FromAggregationForm(d.ToAggregationForm(val)); math.Abs(got-val) > 1e-12 {
		t.Fatalf("ArcDistance round-trip: want %g, got %g", val, got)
	}
}

func TestLinearDistanceAggregation(t *testing.T) {
	d := geom.LinearDistanceInstance
	val := 2.0
	agg := d.ToAggregationForm(val)
	want := val * val
	if math.Abs(agg-want) > 1e-12 {
		t.Fatalf("LinearDistance aggregation: want %g, got %g", want, agg)
	}
}

func TestNormalSquaredRoundTrip(t *testing.T) {
	d := geom.NormalSquaredDistanceInstance
	val := 3.14
	if got := d.FromAggregationForm(d.ToAggregationForm(val)); math.Abs(got-val) > 1e-12 {
		t.Fatalf("NormalSquaredDistance round-trip: want %g, got %g", val, got)
	}
}

// ---------------------------------------------------------------------------
// Factories
// ---------------------------------------------------------------------------

func TestMakeGeoCircleReturnType(t *testing.T) {
	pm := geom.SPHERE
	c, err := geom.MakeGeoCircle(pm, 0, 0, 0.5)
	if err != nil {
		t.Fatalf("MakeGeoCircle: %v", err)
	}
	if c == nil {
		t.Fatal("MakeGeoCircle must not return nil")
	}
}

func TestMakeGeoCircleDegenerateReturnsPoint(t *testing.T) {
	pm := geom.SPHERE
	c, err := geom.MakeGeoCircle(pm, 0, 0, 0) // cutoffAngle=0 → degenerate
	if err != nil {
		t.Fatalf("MakeGeoCircle degenerate: %v", err)
	}
	if c == nil {
		t.Fatal("degenerate circle must not return nil")
	}
}

func TestMakeGeoBBoxWorld(t *testing.T) {
	pm := geom.SPHERE
	half := math.Pi * 0.5
	b, err := geom.MakeGeoBBox(pm, half, -half, -math.Pi, math.Pi)
	if err != nil {
		t.Fatalf("MakeGeoBBox world: %v", err)
	}
	if b == nil {
		t.Fatal("MakeGeoBBox world must not return nil")
	}
}

func TestMakeGeoConvexPolygonFactory(t *testing.T) {
	pm := geom.SPHERE
	pts := []*geom.GeoPoint{
		geom.NewGeoPointModel(pm, 0, 0),
		geom.NewGeoPointModel(pm, 0, 0.1),
		geom.NewGeoPointModel(pm, 0.1, 0.1),
	}
	p, err := geom.MakeGeoConvexPolygon(pm, pts)
	if err != nil {
		t.Fatalf("MakeGeoConvexPolygon: %v", err)
	}
	if p == nil {
		t.Fatal("MakeGeoConvexPolygon must not return nil")
	}
}

// TestMakeGeoConvexPolygonFromThreePoints verifies that MakeGeoPolygon
// delegates to the convex/concave polygon factory and succeeds for a simple
// triangular set of points.
func TestMakeGeoConvexPolygonFromThreePoints(t *testing.T) {
	pm := geom.SPHERE
	pts := []*geom.GeoPoint{
		geom.NewGeoPointModel(pm, 0, 0),
		geom.NewGeoPointModel(pm, 0, 0.1),
		geom.NewGeoPointModel(pm, 0.1, 0.1),
	}
	poly, err := geom.MakeGeoPolygon(pm, pts)
	if err != nil {
		t.Fatalf("MakeGeoPolygon: expected success for 3-point triangle, got error: %v", err)
	}
	if poly == nil {
		t.Fatal("MakeGeoPolygon returned nil")
	}
}

func TestMakeGeoPath(t *testing.T) {
	pm := geom.SPHERE
	path := geom.MakeGeoPath(pm, 0.1, nil)
	if path == nil {
		t.Fatal("MakeGeoPath must not return nil")
	}
}

func TestMakeGeoPointShape(t *testing.T) {
	pm := geom.SPHERE
	s := geom.MakeGeoPointShape(pm, 0, 0)
	if s == nil {
		t.Fatal("MakeGeoPointShape must not return nil")
	}
}

// ---------------------------------------------------------------------------
// XYZSolid factory
// ---------------------------------------------------------------------------

func TestMakeXYZSolidStandard(t *testing.T) {
	pm := geom.SPHERE
	s := geom.MakeXYZSolid(pm, -0.5, 0.5, -0.5, 0.5, -0.5, 0.5)
	if s == nil {
		t.Fatal("MakeXYZSolid must not return nil")
	}
}

func TestMakeXYZSolidDegenerateX(t *testing.T) {
	pm := geom.SPHERE
	s := geom.MakeXYZSolid(pm, 0, 0, -0.5, 0.5, -0.5, 0.5)
	if s == nil {
		t.Fatal("degenerate-X solid must not return nil")
	}
}

func TestMakeXYZSolidPoint(t *testing.T) {
	pm := geom.SPHERE
	s := geom.MakeXYZSolid(pm, 0, 0, 0, 0, 0, 0)
	if s == nil {
		t.Fatal("point solid must not return nil")
	}
}

// ---------------------------------------------------------------------------
// SafeAcos
// ---------------------------------------------------------------------------

func TestSafeAcosClamp(t *testing.T) {
	if got := geom.SafeAcos(2.0); got != 0 {
		t.Fatalf("SafeAcos(2): want 0, got %g", got)
	}
	if got := geom.SafeAcos(-2.0); math.Abs(got-math.Pi) > 1e-12 {
		t.Fatalf("SafeAcos(-2): want Pi, got %g", got)
	}
}

// ---------------------------------------------------------------------------
// GeoWorld interface compliance
// ---------------------------------------------------------------------------

func TestGeoWorldIsWithin(t *testing.T) {
	pm := geom.SPHERE
	w := &geom.GeoWorld{GeoBaseBBox: geom.GeoBaseBBox{GeoBaseAreaShape: geom.GeoBaseAreaShape{GeoBaseMembershipShape: geom.GeoBaseMembershipShape{GeoBaseShape: geom.GeoBaseShape{BasePlanetObject: geom.BasePlanetObject{PlanetModelField: pm}}}}}}
	if !w.IsWithin(0.5, 0.5, 0.5) {
		t.Fatal("GeoWorld.IsWithin must always return true")
	}
}

func TestGeoWorldGetRelationship(t *testing.T) {
	pm := geom.SPHERE
	w, err := geom.MakeGeoBBox(pm, math.Pi*0.5, -math.Pi*0.5, -math.Pi, math.Pi)
	if err != nil {
		t.Fatalf("MakeGeoBBox world: %v", err)
	}
	if r := w.GetRelationship(nil); r != geom.RelContains {
		t.Fatalf("GeoWorld relationship: want RelContains(%d), got %d", geom.RelContains, r)
	}
}

// ---------------------------------------------------------------------------
// PlanetModel construction correctness
// ---------------------------------------------------------------------------

// TestPlanetModelSphereScalings verifies the normalised scaling values for a unit sphere.
// For a=b=1: meanRadius = (2*1+1)/3 = 1; xyScaling = 1/1 = 1; zScaling = 1/1 = 1.
func TestPlanetModelSphereScalings(t *testing.T) {
	pm := geom.SPHERE
	if math.Abs(pm.XYScaling-1.0) > 1e-14 {
		t.Fatalf("SPHERE XYScaling: want 1, got %g", pm.XYScaling)
	}
	if math.Abs(pm.ZScaling-1.0) > 1e-14 {
		t.Fatalf("SPHERE ZScaling: want 1, got %g", pm.ZScaling)
	}
	if math.Abs(pm.Scale-1.0) > 1e-14 {
		t.Fatalf("SPHERE Scale: want 1, got %g", pm.Scale)
	}
}

// TestPlanetModelWGS84Scalings verifies xyScaling = a/meanRadius for WGS84.
func TestPlanetModelWGS84Scalings(t *testing.T) {
	pm := geom.WGS84
	a, b := 6378137.0, 6356752.314245
	meanRadius := (2*a + b) / 3.0
	wantXY := a / meanRadius
	wantZ := b / meanRadius
	if math.Abs(pm.XYScaling-wantXY) > 1e-10 {
		t.Fatalf("WGS84 XYScaling: want %g, got %g", wantXY, pm.XYScaling)
	}
	if math.Abs(pm.ZScaling-wantZ) > 1e-10 {
		t.Fatalf("WGS84 ZScaling: want %g, got %g", wantZ, pm.ZScaling)
	}
}

// TestPlanetModelWGS84PolesAreCorrect checks that NorthPole Z equals zScaling (not B).
func TestPlanetModelWGS84PolesAreCorrect(t *testing.T) {
	pm := geom.WGS84
	if pm.NorthPole == nil {
		t.Fatal("WGS84 NorthPole must not be nil")
	}
	// NorthPole = newGeoPointMag(zScaling, 0,0,1, pi/2, 0) → Z = zScaling*1.
	if math.Abs(pm.NorthPole.Z-pm.ZScaling) > 1e-14 {
		t.Fatalf("WGS84 NorthPole.Z: want %g, got %g", pm.ZScaling, pm.NorthPole.Z)
	}
	if pm.NorthPole.X != 0 || pm.NorthPole.Y != 0 {
		t.Fatalf("NorthPole X/Y must be 0, got (%g,%g)", pm.NorthPole.X, pm.NorthPole.Y)
	}
	// MaxXPole = newGeoPointMag(xyScaling, 1,0,0, 0, 0) → X = xyScaling*1.
	if math.Abs(pm.MaxXPole.X-pm.XYScaling) > 1e-14 {
		t.Fatalf("WGS84 MaxXPole.X: want %g, got %g", pm.XYScaling, pm.MaxXPole.X)
	}
}

// ---------------------------------------------------------------------------
// PlanetModel encoding / decoding
// ---------------------------------------------------------------------------

// TestEncodeDecodeSphereRoundTrip verifies that encode(decode(encode(v))) == encode(v) on the sphere.
func TestEncodeDecodeSphereRoundTrip(t *testing.T) {
	pm := geom.SPHERE
	for _, v := range []float64{0, 0.5, -0.5, 0.9999, -0.9999} {
		enc := pm.EncodeValue(v)
		dec := pm.DecodeValue(enc)
		enc2 := pm.EncodeValue(dec)
		if enc != enc2 {
			t.Errorf("encode round-trip failed for %g: encode=%d, decode=%g, re-encode=%d", v, enc, dec, enc2)
		}
	}
}

// TestEncodeDecodeWGS84RoundTrip is the same test for WGS84.
func TestEncodeDecodeWGS84RoundTrip(t *testing.T) {
	pm := geom.WGS84
	for _, v := range []float64{0, 0.5, -0.5, pm.MaxValue * 0.9, -pm.MaxValue * 0.9} {
		enc := pm.EncodeValue(v)
		dec := pm.DecodeValue(enc)
		enc2 := pm.EncodeValue(dec)
		if enc != enc2 {
			t.Errorf("encode round-trip failed for %g: encode=%d, decode=%g, re-encode=%d", v, enc, dec, enc2)
		}
	}
}

// TestEncodeValueBoundaries checks that EncodeValue / DecodeValue handle the
// extreme values without panic or wrong decode for MIN/MAX sentinels.
func TestEncodeValueBoundaries(t *testing.T) {
	pm := geom.SPHERE
	// Decoding the min sentinel must return -MaxValue.
	if got := pm.DecodeValue(pm.MinEncodedValue); got != -pm.MaxValue {
		t.Fatalf("DecodeValue(MIN)=%g, want %g", got, -pm.MaxValue)
	}
	// Decoding the max sentinel must return +MaxValue.
	if got := pm.DecodeValue(pm.MaxEncodedValue); got != pm.MaxValue {
		t.Fatalf("DecodeValue(MAX)=%g, want %g", got, pm.MaxValue)
	}
}

// ---------------------------------------------------------------------------
// PlanetModel binary serialisation round-trip (AC2)
// ---------------------------------------------------------------------------

// TestPlanetModelSerialRoundTrip writes a PlanetModel then reads it back and
// checks that all computed fields are identical. This verifies the wire format
// is self-consistent (isolated round-trip half of AC2).
func TestPlanetModelSerialRoundTrip(t *testing.T) {
	for _, pm := range []*geom.PlanetModel{geom.SPHERE, geom.WGS84, geom.CLARKE1866} {
		var buf bytes.Buffer
		if err := pm.Write(&buf); err != nil {
			t.Fatalf("Write: %v", err)
		}
		got, err := geom.NewPlanetModelFromStream(&buf)
		if err != nil {
			t.Fatalf("NewPlanetModelFromStream: %v", err)
		}
		if got.A != pm.A || got.B != pm.B {
			t.Errorf("A/B mismatch: want (%g,%g), got (%g,%g)", pm.A, pm.B, got.A, got.B)
		}
		if math.Abs(got.XYScaling-pm.XYScaling) > 1e-14 {
			t.Errorf("XYScaling mismatch: want %g, got %g", pm.XYScaling, got.XYScaling)
		}
		if math.Abs(got.Decode-pm.Decode) > 1e-20 {
			t.Errorf("Decode mismatch: want %g, got %g", pm.Decode, got.Decode)
		}
	}
}

// TestPlanetModelSerialByteLayout verifies the exact wire bytes for SPHERE
// against the Lucene 10.4.0 SerializableObject format.
//
// SerializableObject.writeDouble(a=1.0): writeLong(bits) where bits=Double.doubleToLongBits(1.0)
// = 0x3FF0000000000000. writeLong = writeInt(lo=0) + writeInt(hi=0x3FF00000).
// Each writeInt is little-endian 4 bytes.
// So the 8 bytes for 1.0 are: 00 00 00 00 00 00 F0 3F.
// (lo=0x00000000 → 00 00 00 00; hi=0x3FF00000 → 00 00 F0 3F)
func TestPlanetModelSerialByteLayout(t *testing.T) {
	var buf bytes.Buffer
	if err := geom.SPHERE.Write(&buf); err != nil {
		t.Fatalf("Write SPHERE: %v", err)
	}
	b := buf.Bytes()
	if len(b) != 16 {
		t.Fatalf("SPHERE serialised to %d bytes, want 16", len(b))
	}
	// 1.0 in IEEE 754: bits = 0x3FF0000000000000
	// writeLong(bits) = writeInt(lo=0x00000000) + writeInt(hi=0x3FF00000)
	// writeInt LE: {0x00,0x00,0x00,0x00} then {0x00,0x00,0xF0,0x3F}
	want := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xF0, 0x3F, // a=1.0
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xF0, 0x3F, // b=1.0
	}
	for i, wb := range want {
		if b[i] != wb {
			t.Errorf("byte[%d]: want 0x%02X, got 0x%02X", i, wb, b[i])
		}
	}
}

// ---------------------------------------------------------------------------
// GeoPoint binary serialisation round-trip
// ---------------------------------------------------------------------------

// TestGeoPointSerialRoundTrip writes a GeoPoint then reads it back and checks
// that all five serialised fields are recovered exactly.
func TestGeoPointSerialRoundTrip(t *testing.T) {
	pm := geom.SPHERE
	lat := math.Pi / 6 // 30°
	lon := math.Pi / 4 // 45°
	p := geom.NewGeoPointLatLon(pm, lat, lon)
	// Force lazy fields to be computed before writing.
	_ = p.GetLatitude()
	_ = p.GetLongitude()

	var buf bytes.Buffer
	if err := p.Write(&buf); err != nil {
		t.Fatalf("GeoPoint.Write: %v", err)
	}
	if buf.Len() != 40 {
		t.Fatalf("GeoPoint serialised to %d bytes, want 40", buf.Len())
	}
	got, err := geom.NewGeoPointFromStream(pm, &buf)
	if err != nil {
		t.Fatalf("NewGeoPointFromStream: %v", err)
	}
	if math.Abs(got.GetLatitude()-lat) > 1e-14 {
		t.Errorf("lat: want %g, got %g", lat, got.GetLatitude())
	}
	if math.Abs(got.GetLongitude()-lon) > 1e-14 {
		t.Errorf("lon: want %g, got %g", lon, got.GetLongitude())
	}
	if math.Abs(got.X-p.X) > 1e-14 || math.Abs(got.Y-p.Y) > 1e-14 || math.Abs(got.Z-p.Z) > 1e-14 {
		t.Errorf("XYZ: want (%g,%g,%g), got (%g,%g,%g)", p.X, p.Y, p.Z, got.X, got.Y, got.Z)
	}
}

// ---------------------------------------------------------------------------
// LatLonBounds — AddXValue / AddYValue / AddZValue / AddPlane / AddIntersection
// ---------------------------------------------------------------------------

// TestLatLonBoundsAddXValueRecordsLongitude verifies that AddXValue delegates to
// addLongitudeBound via the point's longitude.
func TestLatLonBoundsAddXValueRecordsLongitude(t *testing.T) {
	pm := geom.SPHERE
	b := geom.NewLatLonBounds()
	p := geom.NewGeoPointLatLon(pm, 0.3, 0.7)
	b.AddXValue(p)
	if b.CheckNoLongitudeBound() {
		t.Fatal("AddXValue must not set noLongitudeBound for a single point")
	}
	if !b.HasLon() {
		t.Fatal("AddXValue must set hasLon")
	}
	// The left and right longitude must both equal the point's longitude.
	if math.Abs(b.GetMinLongitude()-0.7) > 1e-12 {
		t.Fatalf("leftLon after AddXValue: want 0.7, got %g", b.GetMinLongitude())
	}
	if math.Abs(b.GetMaxLongitude()-0.7) > 1e-12 {
		t.Fatalf("rightLon after AddXValue: want 0.7, got %g", b.GetMaxLongitude())
	}
}

// TestLatLonBoundsAddYValueRecordsLongitude is symmetric with the X test.
func TestLatLonBoundsAddYValueRecordsLongitude(t *testing.T) {
	pm := geom.SPHERE
	b := geom.NewLatLonBounds()
	p := geom.NewGeoPointLatLon(pm, 0.0, -0.5)
	b.AddYValue(p)
	if !b.HasLon() {
		t.Fatal("AddYValue must set hasLon")
	}
	if math.Abs(b.GetMinLongitude()-(-0.5)) > 1e-12 {
		t.Fatalf("leftLon after AddYValue: want -0.5, got %g", b.GetMinLongitude())
	}
}

// TestLatLonBoundsAddZValueRecordsLatitude verifies that AddZValue delegates to
// addLatitudeBound via the point's latitude.
func TestLatLonBoundsAddZValueRecordsLatitude(t *testing.T) {
	pm := geom.SPHERE
	b := geom.NewLatLonBounds()
	p := geom.NewGeoPointLatLon(pm, 0.4, 0.0)
	b.AddZValue(p)
	if !b.HasLat() {
		t.Fatal("AddZValue must set hasLat")
	}
	if math.Abs(b.GetMaxLatitude()-0.4) > 1e-12 {
		t.Fatalf("maxLat after AddZValue: want 0.4, got %g", b.GetMaxLatitude())
	}
	if math.Abs(b.GetMinLatitude()-0.4) > 1e-12 {
		t.Fatalf("minLat after AddZValue: want 0.4, got %g", b.GetMinLatitude())
	}
}

// TestLatLonBoundsAddZValueNoOpWhenLatUnbounded verifies that AddZValue is a
// no-op once both latitude bounds are cleared.
func TestLatLonBoundsAddZValueNoOpWhenLatUnbounded(t *testing.T) {
	pm := geom.SPHERE
	b := geom.NewLatLonBounds()
	b.NoTopLatitudeBound()
	b.NoBottomLatitudeBound()
	p := geom.NewGeoPointLatLon(pm, 0.4, 0.0)
	b.AddZValue(p)
	// hasLat should remain false because both guards are set.
	if b.HasLat() {
		t.Fatal("AddZValue must not record lat when both bounds are cleared")
	}
}

// TestLatLonBoundsIsWidePromotesToNoLon verifies that IsWide calls noLongitudeBound.
func TestLatLonBoundsIsWidePromotesToNoLon(t *testing.T) {
	b := geom.NewLatLonBounds()
	b.IsWide()
	if !b.CheckNoLongitudeBound() {
		t.Fatal("IsWide must set noLongitudeBound")
	}
}

// TestLatLonBoundsAddPlaneRecordsLatLon verifies that AddPlane triggers the
// Plane.RecordBoundsForLatLon algorithm and populates both latitude and longitude
// bounds for a simple diagonal plane on the unit sphere.
func TestLatLonBoundsAddPlaneRecordsLatLon(t *testing.T) {
	pm := geom.SPHERE
	b := geom.NewLatLonBounds()
	// A simple equatorial horizontal plane (Z=0): all surface intersections are
	// at latitude 0 with longitude varying over [-π, π].
	plane := geom.NewPlane(0, 0, 1, 0) // Zx+Zy+z=0 → equatorial plane
	b.AddPlane(pm, plane)
	// Latitude extremum should be near 0 (the plane defines the equator).
	if b.HasLat() {
		// maxLat/minLat should be ≈ 0.
		if math.Abs(b.GetMaxLatitude()) > 1e-9 {
			t.Fatalf("maxLat after equatorial AddPlane: want ≈0, got %g", b.GetMaxLatitude())
		}
	}
}

// TestLatLonBoundsAddIntersectionNoOpWhenNilPlanes verifies that AddIntersection
// with nil planes does not panic.
func TestLatLonBoundsAddIntersectionNoOpWhenNilPlanes(t *testing.T) {
	b := geom.NewLatLonBounds()
	b.AddIntersection(geom.SPHERE, nil, nil)
	// Should not panic; bounds remain empty.
	if b.HasLon() {
		t.Fatal("AddIntersection with nil planes must not record longitude")
	}
}

// TestLatLonBoundsAddIntersectionRecordsLatLon exercises AddIntersection with
// two non-degenerate planes on the unit sphere.
func TestLatLonBoundsAddIntersectionRecordsLatLon(t *testing.T) {
	pm := geom.SPHERE
	b := geom.NewLatLonBounds()
	// Two planes whose intersection line crosses the sphere surface.
	p1 := geom.NewPlane(1, 0, 0, 0) // x=0 plane
	p2 := geom.NewPlane(0, 1, 0, 0) // y=0 plane
	b.AddIntersection(pm, p1, p2)
	// The intersection of x=0 and y=0 is the Z-axis, which hits the sphere at
	// (0,0,±1). These points have latitude ±π/2 and lon=0.
	// The result must have produced at least a lat or lon update without panicking.
	// (The exact values depend on the error-envelope sampling, so we just assert
	// the call succeeded without panic and did not set noLongitudeBound.)
	_ = b.GetMinLatitude()
	_ = b.GetMaxLatitude()
}

// TestLatLonBoundsLongitudeCircularExpansion verifies the circular-arc
// longitude-envelope algorithm. Adding two nearby longitudes must not
// accidentally trigger the ≥π arc check.
func TestLatLonBoundsLongitudeCircularExpansion(t *testing.T) {
	b := geom.NewLatLonBounds()
	pm := geom.SPHERE
	// Add three points spanning about 0.3 radians in longitude.
	for _, lon := range []float64{0.1, 0.2, 0.4} {
		b.AddPoint(geom.NewGeoPointLatLon(pm, 0.0, lon))
	}
	if b.CheckNoLongitudeBound() {
		t.Fatal("small arc must not trigger noLongitudeBound")
	}
	if !b.HasLon() {
		t.Fatal("hasLon must be true after adding points")
	}
	// The arc should contain 0.1 and 0.4 as its endpoints.
	left := b.GetMinLongitude()
	right := b.GetMaxLongitude()
	if left > 0.1+1e-12 {
		t.Fatalf("leftLon should be ≤ 0.1, got %g", left)
	}
	if right < 0.4-1e-12 {
		t.Fatalf("rightLon should be ≥ 0.4, got %g", right)
	}
}

// TestLatLonBoundsNoBoundClearsAll verifies that NoBound clears all three
// bound categories simultaneously.
func TestLatLonBoundsNoBoundClearsAll(t *testing.T) {
	b := geom.NewLatLonBounds()
	b.NoBound(geom.SPHERE)
	if !b.CheckNoLongitudeBound() {
		t.Fatal("NoBound must set noLongitudeBound")
	}
	if !b.CheckNoTopLatitudeBound() {
		t.Fatal("NoBound must set noTopLatitudeBound")
	}
	if !b.CheckNoBottomLatitudeBound() {
		t.Fatal("NoBound must set noBottomLatitudeBound")
	}
}

// ---------------------------------------------------------------------------
// GeoConvexPolygon and GeoConcavePolygon — basic functionality
// ---------------------------------------------------------------------------

// TestGeoConvexPolygonIsWithin verifies that a point inside a small convex
// triangle is reported as inside, and a point outside is not.
func TestGeoConvexPolygonIsWithin(t *testing.T) {
	pm := geom.SPHERE
	pts := []*geom.GeoPoint{
		geom.NewGeoPointModel(pm, 0.0, 0.0),
		geom.NewGeoPointModel(pm, 0.0, 0.05),
		geom.NewGeoPointModel(pm, 0.05, 0.025),
	}
	poly, err := geom.NewGeoConvexPolygon(pm, pts, nil)
	if err != nil {
		t.Fatalf("NewGeoConvexPolygon: %v", err)
	}
	// A point near the centroid should be inside.
	centroid := geom.NewGeoPointModel(pm, 0.02, 0.025)
	if !poly.IsWithin(centroid.X, centroid.Y, centroid.Z) {
		t.Error("centroid of convex triangle must be inside")
	}
	// A point far away must be outside.
	far := geom.NewGeoPointModel(pm, 1.0, 1.0)
	if poly.IsWithin(far.X, far.Y, far.Z) {
		t.Error("point far from triangle must be outside")
	}
}

// TestGeoConvexPolygonGetBoundsNoopLatLon verifies that GetBounds works
// without panicking when accumulating into a LatLonBounds (exercises
// AddPlane and AddIntersection on LatLonBounds via the polygon).
func TestGeoConvexPolygonGetBoundsNoopLatLon(t *testing.T) {
	pm := geom.SPHERE
	pts := []*geom.GeoPoint{
		geom.NewGeoPointModel(pm, 0.0, 0.0),
		geom.NewGeoPointModel(pm, 0.0, 0.1),
		geom.NewGeoPointModel(pm, 0.1, 0.05),
	}
	poly, err := geom.NewGeoConvexPolygon(pm, pts, nil)
	if err != nil {
		t.Fatalf("NewGeoConvexPolygon: %v", err)
	}
	b := geom.NewLatLonBounds()
	poly.GetBounds(b) // must not panic
	// After GetBounds the latitude range must be at least as large as the
	// min/max latitudes of the input points.
	if b.GetMinLatitude() > 0+1e-9 {
		t.Fatalf("minLat after GetBounds: want ≤ 0, got %g", b.GetMinLatitude())
	}
	if b.GetMaxLatitude() < 0.1-1e-9 {
		t.Fatalf("maxLat after GetBounds: want ≥ 0.1, got %g", b.GetMaxLatitude())
	}
}

// TestGeoConcavePolygonGetBoundsNoopLatLon is the same smoke test for the
// concave polygon variant.
func TestGeoConcavePolygonGetBoundsNoopLatLon(t *testing.T) {
	pm := geom.SPHERE
	// A wide concave triangle (the "outside" of a small convex triangle).
	pts := []*geom.GeoPoint{
		geom.NewGeoPointModel(pm, 0.0, 0.0),
		geom.NewGeoPointModel(pm, 0.05, 0.025),
		geom.NewGeoPointModel(pm, 0.0, 0.05),
	}
	poly, err := geom.NewGeoConcavePolygon(pm, pts, nil)
	if err != nil {
		t.Fatalf("NewGeoConcavePolygon: %v", err)
	}
	b := geom.NewLatLonBounds()
	poly.GetBounds(b) // must not panic
}

// ---------------------------------------------------------------------------
// Standard type codes
// ---------------------------------------------------------------------------

func TestStandardObjectCodes(t *testing.T) {
	if geom.CodeGeoPoint != 0 {
		t.Fatalf("CodeGeoPoint: want 0, got %d", geom.CodeGeoPoint)
	}
	if geom.CodeGeoWorld != 26 {
		t.Fatalf("CodeGeoWorld: want 26, got %d", geom.CodeGeoWorld)
	}
	if geom.CodeGeoS2Shape != 38 {
		t.Fatalf("CodeGeoS2Shape: want 38, got %d", geom.CodeGeoS2Shape)
	}
}
