// Tests in this file mirror
// org.apache.lucene.geo.TestCircle2D from Apache Lucene 10.4.0.

package geo

import "testing"

// helpers ---------------------------------------------------------

// makeLatLonCircle2D builds the Component2D for a geographic circle,
// matching LatLonGeometry.create(Circle) in Java.
func makeLatLonCircle2D(t *testing.T, lat, lon, radiusMeters float64) Component2D {
	t.Helper()
	c, err := NewCircle(lat, lon, radiusMeters)
	if err != nil {
		t.Fatalf("NewCircle(%v,%v,%v): %v", lat, lon, radiusMeters, err)
	}
	return newCircle2DFromCircle(c)
}

// makeXYCircle2D builds the Component2D for a cartesian circle,
// matching XYGeometry.create(XYCircle) in Java.
func makeXYCircle2D(t *testing.T, x, y, radius float32) Component2D {
	t.Helper()
	c, err := NewXYCircle(x, y, radius)
	if err != nil {
		t.Fatalf("NewXYCircle(%v,%v,%v): %v", x, y, radius, err)
	}
	return c.toComponent2D()
}

// triangleVariants runs fn against both the geographic and cartesian
// circle, mirroring the random()-driven branch in the Java tests.
func triangleVariants(t *testing.T, geoRadiusMeters float64, xyRadius float32, fn func(t *testing.T, circle Component2D)) {
	t.Helper()
	t.Run("LatLon", func(t *testing.T) { fn(t, makeLatLonCircle2D(t, 0, 0, geoRadiusMeters)) })
	t.Run("XY", func(t *testing.T) { fn(t, makeXYCircle2D(t, 0, 0, xyRadius)) })
}

// testTriangleDisjoint ---------------------------------------------

// TestCircle2D_TriangleDisjoint mirrors testTriangleDisjoint.
func TestCircle2D_TriangleDisjoint(t *testing.T) {
	triangleVariants(t, 100, 1, func(t *testing.T, circle Component2D) {
		ax, ay := 4.0, 4.0
		bx, by := 5.0, 5.0
		cx, cy := 5.0, 4.0
		if IntersectsTriangleDefault(circle, ax, ay, bx, by, cx, cy) {
			t.Errorf("intersectsTriangle: want false")
		}
		if IntersectsLineDefault(circle, ax, ay, bx, by) {
			t.Errorf("intersectsLine: want false")
		}
		if ContainsTriangleDefault(circle, ax, ay, bx, by, cx, cy) {
			t.Errorf("containsTriangle: want false")
		}
		if ContainsLineDefault(circle, ax, ay, bx, by) {
			t.Errorf("containsLine: want false")
		}
		got := WithinTriangleDefault(circle,
			ax, ay, true, bx, by, true, cx, cy, true)
		if got != WithinDisjoint {
			t.Errorf("withinTriangle: got %v want DISJOINT", got)
		}
	})
}

// testTriangleIntersects -------------------------------------------

// TestCircle2D_TriangleIntersects mirrors testTriangleIntersects.
func TestCircle2D_TriangleIntersects(t *testing.T) {
	triangleVariants(t, 1_000_000, 10, func(t *testing.T, circle Component2D) {
		ax, ay := -20.0, 1.0
		bx, by := 20.0, 1.0
		cx, cy := 0.0, 90.0
		if !IntersectsTriangleDefault(circle, ax, ay, bx, by, cx, cy) {
			t.Errorf("intersectsTriangle: want true")
		}
		if !IntersectsLineDefault(circle, ax, ay, bx, by) {
			t.Errorf("intersectsLine: want true")
		}
		if ContainsTriangleDefault(circle, ax, ay, bx, by, cx, cy) {
			t.Errorf("containsTriangle: want false")
		}
		if ContainsLineDefault(circle, ax, ay, bx, by) {
			t.Errorf("containsLine: want false")
		}
		got := WithinTriangleDefault(circle,
			ax, ay, true, bx, by, true, cx, cy, true)
		if got != WithinNotWithin {
			t.Errorf("withinTriangle: got %v want NOTWITHIN", got)
		}
	})
}

// testTriangleDateLineIntersects -----------------------------------

// TestCircle2D_TriangleDateLineIntersects mirrors
// testTriangleDateLineIntersects (geographic only).
func TestCircle2D_TriangleDateLineIntersects(t *testing.T) {
	circle := makeLatLonCircle2D(t, 0, 179, 222400)
	ax, ay := -179.0, 1.0
	bx, by := -179.0, -1.0
	cx, cy := -178.0, 0.0
	if !IntersectsTriangleDefault(circle, ax, ay, bx, by, cx, cy) {
		t.Errorf("intersectsTriangle: want true")
	}
	if !IntersectsLineDefault(circle, ax, ay, bx, by) {
		t.Errorf("intersectsLine: want true")
	}
	if ContainsTriangleDefault(circle, ax, ay, bx, by, cx, cy) {
		t.Errorf("containsTriangle: want false")
	}
	if ContainsLineDefault(circle, ax, ay, bx, by) {
		t.Errorf("containsLine: want false")
	}
	got := WithinTriangleDefault(circle,
		ax, ay, true, bx, by, true, cx, cy, true)
	if got != WithinNotWithin {
		t.Errorf("withinTriangle: got %v want NOTWITHIN", got)
	}
}

// testTriangleContains ---------------------------------------------

// TestCircle2D_TriangleContains mirrors testTriangleContains.
func TestCircle2D_TriangleContains(t *testing.T) {
	triangleVariants(t, 1_000_000, 1, func(t *testing.T, circle Component2D) {
		ax, ay := 0.25, 0.25
		bx, by := 0.5, 0.5
		cx, cy := 0.5, 0.25
		if !IntersectsTriangleDefault(circle, ax, ay, bx, by, cx, cy) {
			t.Errorf("intersectsTriangle: want true")
		}
		if !IntersectsLineDefault(circle, ax, ay, bx, by) {
			t.Errorf("intersectsLine: want true")
		}
		if !ContainsTriangleDefault(circle, ax, ay, bx, by, cx, cy) {
			t.Errorf("containsTriangle: want true")
		}
		if !ContainsLineDefault(circle, ax, ay, bx, by) {
			t.Errorf("containsLine: want true")
		}
		got := WithinTriangleDefault(circle,
			ax, ay, true, bx, by, true, cx, cy, true)
		if got != WithinNotWithin {
			t.Errorf("withinTriangle: got %v want NOTWITHIN", got)
		}
	})
}

// testTriangleWithin -----------------------------------------------

// TestCircle2D_TriangleWithin mirrors testTriangleWithin.
func TestCircle2D_TriangleWithin(t *testing.T) {
	triangleVariants(t, 1000, 1, func(t *testing.T, circle Component2D) {
		ax, ay := -20.0, -20.0
		bx, by := 20.0, -20.0
		cx, cy := 0.0, 20.0
		if !IntersectsTriangleDefault(circle, ax, ay, bx, by, cx, cy) {
			t.Errorf("intersectsTriangle: want true")
		}
		if IntersectsLineDefault(circle, bx, by, cx, cy) {
			t.Errorf("intersectsLine(b,c): want false")
		}
		if ContainsTriangleDefault(circle, ax, ay, bx, by, cx, cy) {
			t.Errorf("containsTriangle: want false")
		}
		if ContainsLineDefault(circle, bx, by, cx, cy) {
			t.Errorf("containsLine(b,c): want false")
		}
		got := WithinTriangleDefault(circle,
			ax, ay, true, bx, by, true, cx, cy, true)
		if got != WithinCandidate {
			t.Errorf("withinTriangle: got %v want CANDIDATE", got)
		}
	})
}

// testLineIntersects -----------------------------------------------

// TestCircle2D_LineIntersects mirrors testLineIntersects.
func TestCircle2D_LineIntersects(t *testing.T) {
	triangleVariants(t, 35000, 0.3, func(t *testing.T, circle Component2D) {
		ax, ay := -0.25, 0.25
		bx, by := 0.25, 0.25
		cx, cy := 0.2, 0.25
		// A->B: circle touches the centre of the line.
		if !IntersectsLineDefault(circle, ax, ay, bx, by) {
			t.Errorf("intersectsLine(a,b): want true")
		}
		// B->C: projection t > 1.
		if IntersectsLineDefault(circle, bx, by, cx, cy) {
			t.Errorf("intersectsLine(b,c): want false")
		}
		// C->B: projection t < 0.
		if IntersectsLineDefault(circle, cx, cy, bx, by) {
			t.Errorf("intersectsLine(c,b): want false")
		}
	})
}

// testRandomTriangles ----------------------------------------------

// TestCircle2D_RandomTriangles mirrors testRandomTriangles, using a
// fixed deterministic set of triangles so the assertion (Relate's
// classification implies the per-shape predicates) is exercised
// without pulling in a Lucene-equivalent RNG.
func TestCircle2D_RandomTriangles(t *testing.T) {
	type tri struct {
		ax, ay, bx, by, cx, cy float64
	}
	triangles := []tri{
		// disjoint from origin-radius=1 cartesian circle and from
		// (0,0,100m) geographic circle.
		{ax: 50, ay: 50, bx: 60, by: 60, cx: 55, cy: 65},
		{ax: -90, ay: -45, bx: -88, by: -47, cx: -89, cy: -48},
		// straddling tiny disc — relate returns CROSSES, not used
		// here; we still run it to ensure no panic.
		{ax: -0.5, ay: -0.5, bx: 0.5, by: 0.5, cx: 0, cy: 0.5},
		// large triangle that contains the disc — relate returns
		// CROSSES (circle is not fully inside the cell because
		// containsPoint is not enough; cell != triangle).
		{ax: -90, ay: -45, bx: 90, by: -45, cx: 0, cy: 80},
	}
	triangleVariants(t, 1000, 1, func(t *testing.T, circle Component2D) {
		for _, tr := range triangles {
			tMinX := minFloat3(tr.ax, tr.bx, tr.cx)
			tMaxX := maxFloat3(tr.ax, tr.bx, tr.cx)
			tMinY := minFloat3(tr.ay, tr.by, tr.cy)
			tMaxY := maxFloat3(tr.ay, tr.by, tr.cy)
			r := circle.Relate(tMinX, tMaxX, tMinY, tMaxY)
			switch r {
			case CellOutsideQuery:
				if IntersectsTriangleDefault(circle, tr.ax, tr.ay, tr.bx, tr.by, tr.cx, tr.cy) {
					t.Errorf("CellOutsideQuery but intersectsTriangle returned true: %+v", tr)
				}
				if IntersectsLineDefault(circle, tr.ax, tr.ay, tr.bx, tr.by) {
					t.Errorf("CellOutsideQuery but intersectsLine returned true: %+v", tr)
				}
				if ContainsTriangleDefault(circle, tr.ax, tr.ay, tr.bx, tr.by, tr.cx, tr.cy) {
					t.Errorf("CellOutsideQuery but containsTriangle returned true: %+v", tr)
				}
				if ContainsLineDefault(circle, tr.ax, tr.ay, tr.bx, tr.by) {
					t.Errorf("CellOutsideQuery but containsLine returned true: %+v", tr)
				}
				if got := WithinTriangleDefault(circle,
					tr.ax, tr.ay, true, tr.bx, tr.by, true, tr.cx, tr.cy, true); got != WithinDisjoint {
					t.Errorf("CellOutsideQuery but withinTriangle = %v: %+v", got, tr)
				}
			case CellInsideQuery:
				if !IntersectsTriangleDefault(circle, tr.ax, tr.ay, tr.bx, tr.by, tr.cx, tr.cy) {
					t.Errorf("CellInsideQuery but intersectsTriangle = false: %+v", tr)
				}
				if !IntersectsLineDefault(circle, tr.ax, tr.ay, tr.bx, tr.by) {
					t.Errorf("CellInsideQuery but intersectsLine = false: %+v", tr)
				}
				if !ContainsTriangleDefault(circle, tr.ax, tr.ay, tr.bx, tr.by, tr.cx, tr.cy) {
					t.Errorf("CellInsideQuery but containsTriangle = false: %+v", tr)
				}
				if !ContainsLineDefault(circle, tr.ax, tr.ay, tr.bx, tr.by) {
					t.Errorf("CellInsideQuery but containsLine = false: %+v", tr)
				}
				if got := WithinTriangleDefault(circle,
					tr.ax, tr.ay, true, tr.bx, tr.by, true, tr.cx, tr.cy, true); got == WithinCandidate {
					t.Errorf("CellInsideQuery but withinTriangle = CANDIDATE: %+v", tr)
				}
			case CellCrossesQuery:
				// No assertion: predicates may go either way.
			}
		}
	})
}
