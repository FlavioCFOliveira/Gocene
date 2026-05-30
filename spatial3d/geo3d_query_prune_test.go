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

	// A cell straddling the cap centre (x ~ +1, y,z ~ 0) overlaps. Expect CROSSES.
	nearMin := cellPacked(pm, 0.90, -0.10, -0.10)
	nearMax := cellPacked(pm, 1.00, 0.10, 0.10)
	if got := v.Compare(nearMin, nearMax); got != geo3dCellCrossesQuery {
		t.Errorf("near cell: Compare = %d, want CELL_CROSSES_QUERY (%d)", got, geo3dCellCrossesQuery)
	}

	// Compare must never report INSIDE under the rmp #4768 scope.
	whole := cellPacked(pm, 0.0, 0.0, 0.0)
	if got := v.Compare(whole, whole); got == geo3dCellInsideQuery {
		t.Errorf("Compare unexpectedly returned CELL_INSIDE_QUERY")
	}
}

// TestVisitorCompare_BBoxFullScan confirms a non-prune-capable shape (bbox)
// always returns CELL_CROSSES_QUERY regardless of cell position.
func TestVisitorCompare_BBoxFullScan(t *testing.T) {
	pm := geom.SPHERE
	bbox, err := geom.MakeGeoBBox(pm, 0.1, -0.1, -0.2, 0.2)
	if err != nil {
		t.Fatalf("MakeGeoBBox: %v", err)
	}
	v := NewPointInShapeIntersectVisitor(nil, bbox, pm)
	if v.pruneCapable {
		t.Fatalf("bbox visitor must NOT be prune-capable (addIntersection deferred)")
	}
	// Even a far cell must report CROSSES (no pruning for bbox).
	farMin := cellPacked(pm, -1.0, -1.0, -1.0)
	farMax := cellPacked(pm, -0.9, -0.9, -0.9)
	if got := v.Compare(farMin, farMax); got != geo3dCellCrossesQuery {
		t.Errorf("bbox far cell: Compare = %d, want CELL_CROSSES_QUERY (%d)", got, geo3dCellCrossesQuery)
	}
}

// TestPointInGeo3DShapeQuery_CirclePruneDocSetParity is acceptance criterion
// (2) at scale: with pruning enabled, a circle query over a large random point
// cloud must return exactly the IsWithin reference set. This proves the
// rounded-bounds pre-check and Compare pruning never drop a true match nor
// admit a false one.
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
