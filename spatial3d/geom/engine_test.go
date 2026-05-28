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
// Vector perpendicular constructor (Gram-Schmidt)
// ---------------------------------------------------------------------------

// TestVectorPerpendicularUnitAxes builds the perpendicular to the X and Y unit
// vectors; it must be the Z axis (up to sign) and normalised.
func TestVectorPerpendicularUnitAxes(t *testing.T) {
	v, err := geom.NewVectorPerpendicular(1, 0, 0, 0, 1, 0)
	if err != nil {
		t.Fatalf("NewVectorPerpendicular: %v", err)
	}
	if math.Abs(v.X) > 1e-12 || math.Abs(v.Y) > 1e-12 || math.Abs(math.Abs(v.Z)-1) > 1e-12 {
		t.Fatalf("perpendicular of X,Y axes: want ±Z unit, got [%g,%g,%g]", v.X, v.Y, v.Z)
	}
}

// TestVectorPerpendicularParallelFails ensures parallel inputs error out.
func TestVectorPerpendicularParallelFails(t *testing.T) {
	if _, err := geom.NewVectorPerpendicular(1, 0, 0, 2, 0, 0); err == nil {
		t.Fatal("expected error for parallel vectors")
	}
}

// ---------------------------------------------------------------------------
// Plane.FindIntersections
// ---------------------------------------------------------------------------

// TestPlaneFindIntersectionsSphere intersects the z=0 plane (equator) with the
// y=0 plane (prime/anti meridian) on the unit sphere. The crossings must be
// (±1,0,0).
func TestPlaneFindIntersectionsSphere(t *testing.T) {
	pm := geom.SPHERE
	equator := geom.NewPlane(0, 0, 1, 0)  // z = 0
	meridian := geom.NewPlane(0, 1, 0, 0) // y = 0
	pts := equator.FindIntersections(pm, meridian)
	if len(pts) != 2 {
		t.Fatalf("equator ∩ meridian: want 2 points, got %d", len(pts))
	}
	for _, p := range pts {
		if math.Abs(math.Abs(p.X)-1) > 1e-9 || math.Abs(p.Y) > 1e-9 || math.Abs(p.Z) > 1e-9 {
			t.Errorf("intersection point off axis: [%g,%g,%g]", p.X, p.Y, p.Z)
		}
	}
}

// TestPlaneIsNumericallyIdentical verifies identical planes (and sign-flipped
// duplicates) are recognised, while distinct planes are not.
func TestPlaneIsNumericallyIdentical(t *testing.T) {
	a := geom.NewPlane(0, 0, 1, 0)
	if !a.IsNumericallyIdentical(geom.NewPlane(0, 0, 1, 0)) {
		t.Fatal("identical planes not recognised")
	}
	if !a.IsNumericallyIdentical(geom.NewPlane(0, 0, -1, 0)) {
		t.Fatal("sign-flipped identical plane not recognised")
	}
	if a.IsNumericallyIdentical(geom.NewPlane(0, 1, 0, 0)) {
		t.Fatal("distinct planes wrongly identical")
	}
}

// ---------------------------------------------------------------------------
// GeoStandardCircle.isWithin (within-circle)
// ---------------------------------------------------------------------------

// TestGeoStandardCircleWithin checks membership for a circle centred at the
// equator/prime-meridian with a 0.5 rad cutoff. The centre and a point at
// angular distance 0.4 are inside; a point at 0.6 is outside.
func TestGeoStandardCircleWithin(t *testing.T) {
	pm := geom.SPHERE
	const radius = 0.5
	c, err := geom.MakeGeoCircle(pm, 0.0, 0.0, radius)
	if err != nil {
		t.Fatalf("MakeGeoCircle: %v", err)
	}

	center := geom.NewGeoPointModel(pm, 0.0, 0.0)
	if !c.IsWithin(center.X, center.Y, center.Z) {
		t.Error("circle centre should be within")
	}

	// A point 0.4 rad north of centre (inside).
	inside := geom.NewGeoPointModel(pm, 0.4, 0.0)
	if !c.IsWithin(inside.X, inside.Y, inside.Z) {
		t.Error("point at arc distance 0.4 should be within a 0.5 rad circle")
	}

	// A point 0.6 rad north of centre (outside).
	outside := geom.NewGeoPointModel(pm, 0.6, 0.0)
	if c.IsWithin(outside.X, outside.Y, outside.Z) {
		t.Error("point at arc distance 0.6 should be outside a 0.5 rad circle")
	}

	// A point on the far side of the globe is outside.
	antipode := geom.NewGeoPointModel(pm, 0.0, math.Pi)
	if c.IsWithin(antipode.X, antipode.Y, antipode.Z) {
		t.Error("antipodal point should be outside")
	}
}

// TestGeoStandardCircleRadius verifies the reported radius equals the cutoff.
func TestGeoStandardCircleRadius(t *testing.T) {
	pm := geom.SPHERE
	c, err := geom.MakeGeoCircle(pm, 0.1, 0.2, 0.3)
	if err != nil {
		t.Fatalf("MakeGeoCircle: %v", err)
	}
	if math.Abs(c.GetRadius()-0.3) > 1e-12 {
		t.Fatalf("circle radius: want 0.3, got %g", c.GetRadius())
	}
}

// TestGeoStandardCircleEdgeOnBoundary confirms the recomputed edge point lies on
// the circle boundary (isWithin true within resolution).
func TestGeoStandardCircleEdgeWithin(t *testing.T) {
	pm := geom.SPHERE
	c, err := geom.MakeGeoCircle(pm, 0.3, -0.7, 0.4)
	if err != nil {
		t.Fatalf("MakeGeoCircle: %v", err)
	}
	circle, ok := c.(*geom.GeoStandardCircle)
	if !ok {
		t.Fatalf("expected *GeoStandardCircle, got %T", c)
	}
	eps := circle.GetEdgePoints()
	if len(eps) == 0 {
		t.Fatal("circle should have at least one edge point")
	}
	for _, ep := range eps {
		if !circle.IsWithin(ep.X, ep.Y, ep.Z) {
			t.Errorf("edge point %v should be within (on boundary)", ep)
		}
	}
}

// ---------------------------------------------------------------------------
// GeoRectangle.isWithin (within-bbox)
// ---------------------------------------------------------------------------

// TestGeoRectangleWithin checks membership for a rectangle spanning latitude
// [-0.1,0.1] and longitude [-0.2,0.2] radians.
func TestGeoRectangleWithin(t *testing.T) {
	pm := geom.SPHERE
	bbox, err := geom.MakeGeoBBox(pm, 0.1, -0.1, -0.2, 0.2)
	if err != nil {
		t.Fatalf("MakeGeoBBox: %v", err)
	}

	inside := []struct{ lat, lon float64 }{
		{0.0, 0.0},     // centre
		{0.05, 0.1},    // interior
		{-0.09, -0.19}, // near a corner, inside
	}
	for _, p := range inside {
		gp := geom.NewGeoPointModel(pm, p.lat, p.lon)
		if !bbox.IsWithin(gp.X, gp.Y, gp.Z) {
			t.Errorf("point (lat=%g,lon=%g) should be within bbox", p.lat, p.lon)
		}
	}

	outside := []struct{ lat, lon float64 }{
		{0.2, 0.0},  // above top latitude
		{-0.2, 0.0}, // below bottom latitude
		{0.0, 0.3},  // east of right longitude
		{0.0, -0.3}, // west of left longitude
		{0.5, 0.5},  // well outside
	}
	for _, p := range outside {
		gp := geom.NewGeoPointModel(pm, p.lat, p.lon)
		if bbox.IsWithin(gp.X, gp.Y, gp.Z) {
			t.Errorf("point (lat=%g,lon=%g) should be outside bbox", p.lat, p.lon)
		}
	}
}

// TestGeoRectangleCorners verifies the four corners are on the boundary
// (within, accounting for the minimum resolution).
func TestGeoRectangleCorners(t *testing.T) {
	pm := geom.SPHERE
	r, err := geom.NewGeoRectangle(pm, 0.3, -0.2, -0.4, 0.5)
	if err != nil {
		t.Fatalf("NewGeoRectangle: %v", err)
	}
	corners := []struct{ lat, lon float64 }{
		{0.3, -0.4}, {0.3, 0.5}, {-0.2, 0.5}, {-0.2, -0.4},
	}
	for _, c := range corners {
		gp := geom.NewGeoPointModel(pm, c.lat, c.lon)
		if !r.IsWithin(gp.X, gp.Y, gp.Z) {
			t.Errorf("corner (lat=%g,lon=%g) should be within (on boundary)", c.lat, c.lon)
		}
	}
}

// TestGeoRectangleWideExtentUnsupported ensures the not-yet-ported wide-extent
// path reports an explicit error rather than a silently wrong shape.
func TestGeoRectangleWideExtentUnsupported(t *testing.T) {
	pm := geom.SPHERE
	// Longitude extent of ~2.5 rad (> PI/... still < PI here is 2.5<3.14 so not wide);
	// use an extent at/above PI to trigger the wide path.
	if _, err := geom.MakeGeoBBox(pm, 0.1, -0.1, -1.6, 1.6); err == nil {
		t.Fatal("expected unsupported error for wide-extent bbox")
	}
}
