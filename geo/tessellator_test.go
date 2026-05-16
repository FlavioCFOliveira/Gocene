// Tests for Tessellator. The Java test peer
// (lucene/core/src/test/org/apache/lucene/geo/TestTessellator.java)
// contains a large catalog of regression and randomised cases
// targeting the full Java implementation (hole elimination,
// self-intersection detection, Morton-order acceleration). This
// minimal Go port covers the simple-ring use case only; the tests
// below verify that behaviour and explicitly check that holes are
// rejected with ErrTessellatorUnsupported. See tessellator.go for a
// list of features scoped out of this port.

package geo

import (
	"errors"
	"testing"
)

func TestTessellator_Triangle(t *testing.T) {
	t.Parallel()
	// A simple CCW triangle: (0,0), (10,0), (5,10).
	p := MustNewPolygon(
		[]float64{0, 0, 10, 0}, // lats (closed)
		[]float64{0, 10, 5, 0}, // lons (closed)
	)
	tris, err := Tessellate(p, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tris) != 1 {
		t.Fatalf("expected 1 triangle, got %d", len(tris))
	}
	tri := tris[0]
	// The single triangle should reproduce the polygon's three
	// distinct vertices (in some order, since ear-clipping may
	// rotate them).
	verts := [][2]float64{
		{tri.AX(), tri.AY()},
		{tri.BX(), tri.BY()},
		{tri.CX(), tri.CY()},
	}
	want := [][2]float64{{0, 0}, {10, 0}, {5, 10}}
	if !verticesMatch(verts, want) {
		t.Errorf("vertices = %v; want a rotation of %v", verts, want)
	}
}

func TestTessellator_Square(t *testing.T) {
	t.Parallel()
	// Unit square: 4 vertices closed, should yield 2 triangles.
	p := MustNewPolygon(
		[]float64{0, 0, 10, 10, 0},
		[]float64{0, 10, 10, 0, 0},
	)
	tris, err := Tessellate(p, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tris) != 2 {
		t.Fatalf("expected 2 triangles, got %d", len(tris))
	}
	// Combined area of the two triangles must equal the polygon's
	// area (100). We use unsigned area to avoid winding-order
	// concerns.
	total := triangleArea(tris[0]) + triangleArea(tris[1])
	if abs(total-100) > 1e-9 {
		t.Errorf("combined area = %v; want 100", total)
	}
}

func TestTessellator_Pentagon(t *testing.T) {
	t.Parallel()
	// Regular pentagon should produce 3 triangles.
	p := MustNewPolygon(
		[]float64{0, 5, 8, 8, 5, 0},
		[]float64{0, 0, 4, 6, 10, 0},
	)
	// First validate it isn't accepted as malformed before checking
	// the triangle count (some polygons may not tessellate cleanly).
	tris, err := Tessellate(p, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tris) != 3 {
		t.Errorf("expected 3 triangles for pentagon, got %d", len(tris))
	}
}

func TestTessellator_HolesUnsupported(t *testing.T) {
	t.Parallel()
	hole := MustNewPolygon(
		[]float64{3, 3, 7, 7, 3},
		[]float64{3, 7, 7, 3, 3},
	)
	p := MustNewPolygon(
		[]float64{0, 0, 10, 10, 0},
		[]float64{0, 10, 10, 0, 0},
		hole,
	)
	_, err := Tessellate(p, false)
	if err == nil {
		t.Fatal("expected ErrTessellatorUnsupported for hole-bearing polygon")
	}
	if !errors.Is(err, ErrTessellatorUnsupported) {
		t.Fatalf("err = %v; want wrap ErrTessellatorUnsupported", err)
	}
}

func TestTessellator_DegenerateTooFewPoints(t *testing.T) {
	t.Parallel()
	// 3-vertex closed polygon (after dedup): 2 distinct points —
	// not enough to form a triangle.
	_, err := TessellateXY(
		[]float64{0, 1, 0},
		[]float64{0, 0, 0},
		0, false)
	if err == nil {
		t.Fatal("expected error for degenerate input")
	}
	if !errors.Is(err, ErrTessellatorMalformed) {
		t.Errorf("err = %v; want wrap ErrTessellatorMalformed", err)
	}
}

func TestTessellator_XYRingSquare(t *testing.T) {
	t.Parallel()
	tris, err := TessellateXY(
		[]float64{0, 10, 10, 0, 0},
		[]float64{0, 0, 10, 10, 0},
		0, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tris) != 2 {
		t.Fatalf("expected 2 triangles, got %d", len(tris))
	}
	total := triangleArea(tris[0]) + triangleArea(tris[1])
	if abs(total-100) > 1e-9 {
		t.Errorf("area = %v; want 100", total)
	}
}

func TestTessellator_TriangleEdgeFlagsPanic(t *testing.T) {
	t.Parallel()
	tri := Triangle{}
	defer func() {
		if recover() == nil {
			t.Error("EdgeFromPolygon(3) should panic")
		}
	}()
	tri.EdgeFromPolygon(3)
}

// ----- helpers -----

func triangleArea(t Triangle) float64 {
	return abs((t.bx-t.ax)*(t.cy-t.ay)-(t.cx-t.ax)*(t.by-t.ay)) / 2
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func verticesMatch(got, want [][2]float64) bool {
	if len(got) != len(want) {
		return false
	}
	// got is a rotation of want if some shift k of want equals got.
	for k := 0; k < len(want); k++ {
		match := true
		for i := 0; i < len(want); i++ {
			w := want[(i+k)%len(want)]
			if got[i] != w {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	// Also accept reverse orientation.
	rev := make([][2]float64, len(want))
	for i := range want {
		rev[i] = want[len(want)-1-i]
	}
	for k := 0; k < len(rev); k++ {
		match := true
		for i := 0; i < len(rev); i++ {
			w := rev[(i+k)%len(rev)]
			if got[i] != w {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
