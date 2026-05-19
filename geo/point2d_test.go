// Tests for point2D mirror org.apache.lucene.geo.TestPoint2D from
// Apache Lucene 10.4.0. The Java public surface delegates to a
// bbox-aware overload; the Go port exposes that overload directly,
// so each call computes the triangle/line bounding box explicitly.

package geo

import (
	"math"
	"math/rand/v2"
	"testing"
)

// triangleBox returns the axis-aligned bounding box of the triangle
// (a, b, c). The order mirrors StrictMath.min/max chaining used in
// TestPoint2D.testRandomTriangles.
func triangleBox(aX, aY, bX, bY, cX, cY float64) (minX, maxX, minY, maxY float64) {
	minX = math.Min(math.Min(aX, bX), cX)
	maxX = math.Max(math.Max(aX, bX), cX)
	minY = math.Min(math.Min(aY, bY), cY)
	maxY = math.Max(math.Max(aY, bY), cY)
	return
}

// lineBox returns the axis-aligned bounding box of the segment a-b.
func lineBox(aX, aY, bX, bY float64) (minX, maxX, minY, maxY float64) {
	minX = math.Min(aX, bX)
	maxX = math.Max(aX, bX)
	minY = math.Min(aY, bY)
	maxY = math.Max(aY, bY)
	return
}

func TestPoint2D_Bounds(t *testing.T) {
	p := newPoint2D(3.5, -7.25)
	if p.MinX() != 3.5 || p.MaxX() != 3.5 || p.MinY() != -7.25 || p.MaxY() != -7.25 {
		t.Fatalf("bbox = (%v,%v,%v,%v); want collapsed to (3.5, -7.25)",
			p.MinX(), p.MaxX(), p.MinY(), p.MaxY())
	}
}

func TestPoint2D_TriangleDisjoint(t *testing.T) {
	p := newPoint2D(0, 0)
	aX, aY := 4.0, 4.0
	bX, bY := 5.0, 5.0
	cX, cY := 5.0, 4.0

	tMinX, tMaxX, tMinY, tMaxY := triangleBox(aX, aY, bX, bY, cX, cY)
	lMinX, lMaxX, lMinY, lMaxY := lineBox(aX, aY, bX, bY)

	if p.IntersectsTriangle(tMinX, tMaxX, tMinY, tMaxY, aX, aY, bX, bY, cX, cY) {
		t.Fatalf("IntersectsTriangle: want false for disjoint triangle")
	}
	if p.IntersectsLine(lMinX, lMaxX, lMinY, lMaxY, aX, aY, bX, bY) {
		t.Fatalf("IntersectsLine: want false for disjoint segment")
	}
	if p.ContainsTriangle(tMinX, tMaxX, tMinY, tMaxY, aX, aY, bX, bY, cX, cY) {
		t.Fatalf("ContainsTriangle: must always be false")
	}
	if p.ContainsLine(lMinX, lMaxX, lMinY, lMaxY, aX, aY, bX, bY) {
		t.Fatalf("ContainsLine: must always be false")
	}
	got := p.WithinTriangle(tMinX, tMaxX, tMinY, tMaxY,
		aX, aY, randBool(), bX, bY, randBool(), cX, cY, randBool())
	if got != WithinDisjoint {
		t.Fatalf("WithinTriangle = %v; want WithinDisjoint", got)
	}
}

func TestPoint2D_TriangleIntersects(t *testing.T) {
	p := newPoint2D(0, 0)
	aX, aY := 0.0, 0.0
	bX, bY := 1.0, 0.0
	cX, cY := 0.0, 1.0

	tMinX, tMaxX, tMinY, tMaxY := triangleBox(aX, aY, bX, bY, cX, cY)
	lMinX, lMaxX, lMinY, lMaxY := lineBox(aX, aY, bX, bY)

	if !p.IntersectsTriangle(tMinX, tMaxX, tMinY, tMaxY, aX, aY, bX, bY, cX, cY) {
		t.Fatalf("IntersectsTriangle: want true (point is a vertex)")
	}
	if !p.IntersectsLine(lMinX, lMaxX, lMinY, lMaxY, aX, aY, bX, bY) {
		t.Fatalf("IntersectsLine: want true (point is the segment start)")
	}
	if p.ContainsTriangle(tMinX, tMaxX, tMinY, tMaxY, aX, aY, bX, bY, cX, cY) {
		t.Fatalf("ContainsTriangle: must always be false")
	}
	if p.ContainsLine(lMinX, lMaxX, lMinY, lMaxY, aX, aY, bX, bY) {
		t.Fatalf("ContainsLine: must always be false")
	}
	got := p.WithinTriangle(tMinX, tMaxX, tMinY, tMaxY,
		aX, aY, randBool(), bX, bY, randBool(), cX, cY, randBool())
	if got != WithinCandidate {
		t.Fatalf("WithinTriangle = %v; want WithinCandidate", got)
	}
}

func TestPoint2D_Contains(t *testing.T) {
	p := newPoint2D(0, 0)
	if !p.Contains(0, 0) {
		t.Fatalf("Contains: point must contain itself")
	}
	if p.Contains(0, 1) {
		t.Fatalf("Contains: must reject any other coordinate")
	}
	// Degenerate triangle collapsed onto the point is still a
	// candidate, matching TestPoint2D.testTriangleContains.
	got := p.WithinTriangle(0, 0, 0, 0,
		0, 0, randBool(), 0, 0, randBool(), 0, 0, randBool())
	if got != WithinCandidate {
		t.Fatalf("WithinTriangle(degenerate) = %v; want WithinCandidate", got)
	}
}

func TestPoint2D_WithinPointAndLine(t *testing.T) {
	p := newPoint2D(2, 3)

	if p.WithinPoint(2, 3) != WithinCandidate {
		t.Fatalf("WithinPoint(matching) want WithinCandidate")
	}
	if p.WithinPoint(2, 4) != WithinDisjoint {
		t.Fatalf("WithinPoint(other) want WithinDisjoint")
	}

	// Segment passing through (2,3): a=(0,1), b=(4,5), slope 1.
	aX, aY, bX, bY := 0.0, 1.0, 4.0, 5.0
	lMinX, lMaxX, lMinY, lMaxY := lineBox(aX, aY, bX, bY)
	if got := p.WithinLine(lMinX, lMaxX, lMinY, lMaxY, aX, aY, randBool(), bX, bY); got != WithinCandidate {
		t.Fatalf("WithinLine(on-segment) = %v; want WithinCandidate", got)
	}

	// Segment that misses the point.
	aX, aY, bX, bY = 0.0, 0.0, 4.0, 0.0
	lMinX, lMaxX, lMinY, lMaxY = lineBox(aX, aY, bX, bY)
	if got := p.WithinLine(lMinX, lMaxX, lMinY, lMaxY, aX, aY, randBool(), bX, bY); got != WithinDisjoint {
		t.Fatalf("WithinLine(off-segment) = %v; want WithinDisjoint", got)
	}
}

func TestPoint2D_Relate(t *testing.T) {
	p := newPoint2D(1, 1)
	if got := p.Relate(0, 2, 0, 2); got != CellCrossesQuery {
		t.Fatalf("Relate(containing box) = %v; want CellCrossesQuery", got)
	}
	if got := p.Relate(2, 3, 2, 3); got != CellOutsideQuery {
		t.Fatalf("Relate(disjoint box) = %v; want CellOutsideQuery", got)
	}
}

// TestPoint2D_RandomTriangles mirrors testRandomTriangles: when the
// triangle's bbox is disjoint from the point, every predicate must
// agree on a negative answer. Coordinates use the Lucene lat/lon
// ranges so the test peer behaves identically.
func TestPoint2D_RandomTriangles(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	pX := -180.0 + rng.Float64()*360.0
	pY := -90.0 + rng.Float64()*180.0
	p := newPoint2D(pX, pY)

	for i := 0; i < 100; i++ {
		aX := -180.0 + rng.Float64()*360.0
		aY := -90.0 + rng.Float64()*180.0
		bX := -180.0 + rng.Float64()*360.0
		bY := -90.0 + rng.Float64()*180.0
		cX := -180.0 + rng.Float64()*360.0
		cY := -90.0 + rng.Float64()*180.0

		tMinX, tMaxX, tMinY, tMaxY := triangleBox(aX, aY, bX, bY, cX, cY)
		lMinX, lMaxX, lMinY, lMaxY := lineBox(aX, aY, bX, bY)

		if p.Relate(tMinX, tMaxX, tMinY, tMaxY) != CellOutsideQuery {
			continue
		}
		if p.IntersectsTriangle(tMinX, tMaxX, tMinY, tMaxY, aX, aY, bX, bY, cX, cY) {
			t.Fatalf("iter %d: IntersectsTriangle true on disjoint cell", i)
		}
		if p.IntersectsLine(lMinX, lMaxX, lMinY, lMaxY, aX, aY, bX, bY) {
			t.Fatalf("iter %d: IntersectsLine true on disjoint cell", i)
		}
		if p.ContainsTriangle(tMinX, tMaxX, tMinY, tMaxY, aX, aY, bX, bY, cX, cY) {
			t.Fatalf("iter %d: ContainsTriangle true (must always be false)", i)
		}
		if p.ContainsLine(lMinX, lMaxX, lMinY, lMaxY, aX, aY, bX, bY) {
			t.Fatalf("iter %d: ContainsLine true (must always be false)", i)
		}
		if p.WithinTriangle(tMinX, tMaxX, tMinY, tMaxY,
			aX, aY, randBoolRng(rng), bX, bY, randBoolRng(rng), cX, cY, randBoolRng(rng)) != WithinDisjoint {
			t.Fatalf("iter %d: WithinTriangle != WithinDisjoint on disjoint cell", i)
		}
	}
}

// randBool keeps the call sites compact in non-randomised tests; the
// boolean edge flags are irrelevant for a single point because the
// implementation does not consult them.
func randBool() bool { return false }

func randBoolRng(r *rand.Rand) bool { return r.IntN(2) == 0 }
