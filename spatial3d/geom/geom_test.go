// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom_test

import (
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
	c := geom.MakeGeoCircle(pm, 0, 0, 0.5)
	if c == nil {
		t.Fatal("MakeGeoCircle must not return nil")
	}
}

func TestMakeGeoCircleDegenerateReturnsPoint(t *testing.T) {
	pm := geom.SPHERE
	c := geom.MakeGeoCircle(pm, 0, 0, 0) // cutoffAngle=0 → degenerate
	if c == nil {
		t.Fatal("degenerate circle must not return nil")
	}
}

func TestMakeGeoBBoxWorld(t *testing.T) {
	pm := geom.SPHERE
	half := math.Pi * 0.5
	b := geom.MakeGeoBBox(pm, half, -half, -math.Pi, math.Pi)
	if b == nil {
		t.Fatal("MakeGeoBBox world must not return nil")
	}
}

func TestMakeGeoPolygon(t *testing.T) {
	pm := geom.SPHERE
	pts := []*geom.GeoPoint{
		geom.NewGeoPointLatLon(pm, 0, 0),
		geom.NewGeoPointLatLon(pm, 0, 0.1),
		geom.NewGeoPointLatLon(pm, 0.1, 0.1),
	}
	p := geom.MakeGeoPolygon(pm, pts)
	if p == nil {
		t.Fatal("MakeGeoPolygon must not return nil")
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
	w := geom.MakeGeoBBox(pm, math.Pi*0.5, -math.Pi*0.5, -math.Pi, math.Pi)
	if r := w.GetRelationship(nil); r != geom.RelContains {
		t.Fatalf("GeoWorld relationship: want RelContains(%d), got %d", geom.RelContains, r)
	}
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
