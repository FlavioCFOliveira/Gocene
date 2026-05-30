// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial3d

import (
	"math"
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/spatial3d/geom"
)

// cellPacked builds a 12-byte BKD cell-bound packed value from XYZ coordinates.
func cellPacked(pm *geom.PlanetModel, x, y, z float64) []byte {
	b := make([]byte, 3*bytesPerDim)
	EncodeDimension(pm, x, b, 0)
	EncodeDimension(pm, y, b, bytesPerDim)
	EncodeDimension(pm, z, b, 2*bytesPerDim)
	return b
}

// TestVisitorCompare_CirclePrunes is acceptance criterion (3): for a
// prune-capable circle, Compare must return CELL_OUTSIDE_QUERY for a cell that
// is axis-aligned disjoint from the shape's bounds, and CELL_CROSSES_QUERY for
// a cell that overlaps it.
func TestVisitorCompare_CirclePrunes(t *testing.T) {
	pm := geom.SPHERE
	circle, err := geom.MakeGeoCircle(pm, 0.0, 0.0, 0.3) // small cap at (lat=0,lon=0)
	if err != nil {
		t.Fatalf("MakeGeoCircle: %v", err)
	}
	v := NewPointInShapeIntersectVisitor(nil, circle, pm)
	if !v.pruneCapable {
		t.Fatalf("circle visitor must be prune-capable")
	}

	// A cell on the far side of the sphere (x ~ -1) cannot overlap a small cap
	// centred at x ~ +1. Expect OUTSIDE.
	farMin := cellPacked(pm, -1.0, -0.05, -0.05)
	farMax := cellPacked(pm, -0.9, 0.05, 0.05)
	if got := v.Compare(farMin, farMax); got != geo3dCellOutsideQuery {
		t.Errorf("far cell: Compare = %d, want CELL_OUTSIDE_QUERY (%d)", got, geo3dCellOutsideQuery)
	}

	// A cell straddling the cap centre (x ~ +1, y,z ~ 0) is entirely inside the
	// circle. rmp #4790 wires XYZSolid.getRelationship which returns CONTAINS when
	// all cell corners are within the shape → CELL_INSIDE_QUERY is expected.
	// Accept either INSIDE or CROSSES (both are correct — INSIDE is optimal).
	nearMin := cellPacked(pm, 0.90, -0.10, -0.10)
	nearMax := cellPacked(pm, 1.00, 0.10, 0.10)
	nearGot := v.Compare(nearMin, nearMax)
	if nearGot != geo3dCellCrossesQuery && nearGot != geo3dCellInsideQuery {
		t.Errorf("near cell: Compare = %d, want CELL_CROSSES_QUERY (%d) or CELL_INSIDE_QUERY (%d)",
			nearGot, geo3dCellCrossesQuery, geo3dCellInsideQuery)
	}
}

// TestVisitorCompare_BBoxPrunes confirms that a bbox shape IS now prune-capable
// (rmp #4790: XYZBounds.AddIntersection implemented) and returns
// CELL_OUTSIDE_QUERY for a cell disjoint from the bbox's bounds.
func TestVisitorCompare_BBoxPrunes(t *testing.T) {
	pm := geom.SPHERE
	bbox, err := geom.MakeGeoBBox(pm, 0.1, -0.1, -0.2, 0.2)
	if err != nil {
		t.Fatalf("MakeGeoBBox: %v", err)
	}
	v := NewPointInShapeIntersectVisitor(nil, bbox, pm)
	if !v.pruneCapable {
		t.Fatalf("bbox visitor must be prune-capable (AddIntersection now implemented, rmp #4790)")
	}
	// A cell on the far side of the sphere (x ~ -1) must be disjoint from the
	// bbox centred near (x ~ +1). Expect OUTSIDE.
	farMin := cellPacked(pm, -1.0, -1.0, -1.0)
	farMax := cellPacked(pm, -0.9, -0.9, -0.9)
	if got := v.Compare(farMin, farMax); got != geo3dCellOutsideQuery {
		t.Errorf("bbox far cell: Compare = %d, want CELL_OUTSIDE_QUERY (%d)", got, geo3dCellOutsideQuery)
	}
}

// TestPointInGeo3DShapeQuery_CirclePruneDocSetParity is acceptance criterion
// (2) at scale: with pruning enabled, a circle query over a large random point
// cloud must return exactly the IsWithin reference set. This proves the
// rounded-bounds pre-check and Compare pruning never drop a true match nor
// admit a false one.
// TestXYZBoundsAreTrueSuperset is AC1 for rmp #4790: for GeoRectangle and
// GeoConvexPolygon, the XYZBounds returned by GetBounds must be a TRUE SUPERSET
// — every in-shape point must lie within the computed box.
func TestXYZBoundsAreTrueSuperset(t *testing.T) {
	pm := geom.SPHERE
	rng := rand.New(rand.NewSource(0xBEEF4790))

	testSuperset := func(t *testing.T, name string, shape geom.GeoShape, memberOf geom.Membership) {
		t.Helper()
		bounds := geom.NewXYZBounds()
		shape.GetBounds(bounds)
		if !bounds.HasX() || !bounds.HasY() || !bounds.HasZ() {
			t.Errorf("%s: XYZBounds incomplete", name)
			return
		}

		// Sample random points; every in-shape point must be inside the bounds.
		fails := 0
		for i := 0; i < 5000; i++ {
			lat := math.Asin(rng.Float64()*2.0 - 1.0)
			lon := (rng.Float64() - 0.5) * 2.0 * math.Pi
			pt := geom.NewGeoPointModel(pm, lat, lon)
			if !memberOf.IsWithin(pt.X, pt.Y, pt.Z) {
				continue // outside shape, skip
			}
			if pt.X < bounds.MinimumX || pt.X > bounds.MaximumX ||
				pt.Y < bounds.MinimumY || pt.Y > bounds.MaximumY ||
				pt.Z < bounds.MinimumZ || pt.Z > bounds.MaximumZ {
				fails++
				if fails <= 3 {
					t.Errorf("%s: in-shape point (%g,%g,%g) outside bounds [%g,%g]×[%g,%g]×[%g,%g]",
						name, pt.X, pt.Y, pt.Z,
						bounds.MinimumX, bounds.MaximumX,
						bounds.MinimumY, bounds.MaximumY,
						bounds.MinimumZ, bounds.MaximumZ)
				}
			}
		}
	}

	// GeoRectangle (bbox): a small cap straddling the equator.
	bbox, err := geom.MakeGeoBBox(pm, 0.3, -0.3, -0.4, 0.4)
	if err != nil {
		t.Fatalf("MakeGeoBBox: %v", err)
	}
	testSuperset(t, "GeoRectangle", bbox, bbox.(geom.Membership))

	// GeoConvexPolygon: a triangle-ish shape.
	polyPts := []*geom.GeoPoint{
		geom.NewGeoPointModel(pm, 0.2, -0.3),
		geom.NewGeoPointModel(pm, 0.2, 0.3),
		geom.NewGeoPointModel(pm, -0.2, 0.0),
	}
	poly, err := geom.MakeGeoConvexPolygon(pm, polyPts)
	if err != nil {
		t.Fatalf("MakeGeoConvexPolygon: %v", err)
	}
	testSuperset(t, "GeoConvexPolygon", poly, poly.(geom.Membership))
}

// TestPointInGeo3DShapeQuery_BBoxPruneDocSetParity is AC2 for rmp #4790:
// PointInGeo3DShapeQuery over a GeoRectangle with pruning enabled must return
// the identical document set as the IsWithin reference, now with BKD pruning.
func TestPointInGeo3DShapeQuery_BBoxPruneDocSetParity(t *testing.T) {
	pm := geom.SPHERE
	const field = "location"
	rng := rand.New(rand.NewSource(0x4790BEEF))

	bbox, err := geom.MakeGeoBBox(pm, 0.3, -0.3, -0.3, 0.3)
	if err != nil {
		t.Fatalf("MakeGeoBBox: %v", err)
	}

	const n = 2000
	points := make([]*geom.GeoPoint, n)
	for i := range points {
		lat := math.Asin(rng.Float64()*2.0 - 1.0)
		lon := (rng.Float64() - 0.5) * 2.0 * math.Pi
		points[i] = geom.NewGeoPointModel(pm, lat, lon)
	}
	pv := buildStubPV(pm, points)
	want := expectedMatches(pm, bbox.(geom.Membership), pv)
	got := runShapeQuery(t, field, bbox, pv)
	if !intsEqual(got, want) {
		t.Fatalf("BBox doc set parity: got %d docs, want %d", len(got), len(want))
	}
}

func TestPointInGeo3DShapeQuery_CirclePruneDocSetParity(t *testing.T) {
	pm := geom.SPHERE
	const field = "location"
	rng := rand.New(rand.NewSource(0x51ed270b))

	for _, cutoff := range []float64{0.1, 0.3, 0.5, 0.9, 1.3} {
		lat := (rng.Float64() - 0.5) * math.Pi       // [-pi/2, pi/2]
		lon := (rng.Float64() - 0.5) * 2.0 * math.Pi // [-pi, pi]
		circle, err := geom.MakeGeoCircle(pm, lat, lon, cutoff)
		if err != nil {
			t.Fatalf("MakeGeoCircle(cutoff=%g): %v", cutoff, err)
		}

		// Random point cloud spread over the whole sphere.
		const n = 4000
		points := make([]*geom.GeoPoint, n)
		for i := range points {
			plat := math.Asin(rng.Float64()*2.0 - 1.0) // uniform-in-area latitude
			plon := (rng.Float64() - 0.5) * 2.0 * math.Pi
			points[i] = geom.NewGeoPointModel(pm, plat, plon)
		}
		pv := buildStubPV(pm, points)

		want := expectedMatches(pm, circle.(geom.Membership), pv)
		got := runShapeQuery(t, field, circle, pv)
		if !intsEqual(got, want) {
			t.Fatalf("cutoff=%g: pruned circle doc set differs from IsWithin reference: got %d docs, want %d",
				cutoff, len(got), len(want))
		}
	}
}
