// Tests for XYGeometry. Lucene 10.4.0 ships no TestXYGeometry.java
// peer; behavioural coverage is provided indirectly through the
// concrete XY shape tests (TestXYPoint, TestXYRectangle, TestXYLine,
// TestXYPolygon, TestXYCircle, TestComponent2D). The static factory
// contract mirrors LatLonGeometry, so the tests below mirror their
// LatLon equivalents.

package geo

import (
	"errors"
	"strings"
	"testing"
)

type fakeXYGeometry struct {
	component Component2D
}

func (f fakeXYGeometry) toComponent2D() Component2D { return f.component }
func (fakeXYGeometry) xyGeometry()                  {}

func newFakeXYBox(minX, maxX, minY, maxY float64) fakeXYGeometry {
	return fakeXYGeometry{
		component: boxComponent2D{minX: minX, maxX: maxX, minY: minY, maxY: maxY},
	}
}

func TestCreateXYGeometry_NilSlice(t *testing.T) {
	t.Parallel()
	var nilSlice []XYGeometry
	if _, err := CreateXYGeometry(nilSlice...); !errors.Is(err, ErrNilGeometries) {
		t.Fatalf("nil slice: err = %v, want ErrNilGeometries", err)
	}
}

func TestCreateXYGeometry_EmptySlice(t *testing.T) {
	t.Parallel()
	empty := []XYGeometry{}
	if _, err := CreateXYGeometry(empty...); !errors.Is(err, ErrEmptyGeometries) {
		t.Fatalf("empty slice: err = %v, want ErrEmptyGeometries", err)
	}
}

func TestCreateXYGeometry_NilElementAtZero(t *testing.T) {
	t.Parallel()
	_, err := CreateXYGeometry(nil)
	if err == nil {
		t.Fatal("expected error for nil element")
	}
	if !strings.Contains(err.Error(), "geometries[0]") {
		t.Fatalf("error %q missing index 0", err.Error())
	}
}

func TestCreateXYGeometry_NilElementAtIndex(t *testing.T) {
	t.Parallel()
	g := newFakeXYBox(0, 1, 0, 1)
	_, err := CreateXYGeometry(g, nil)
	if err == nil {
		t.Fatal("expected error for nil at index 1")
	}
	if !strings.Contains(err.Error(), "geometries[1]") {
		t.Fatalf("error %q missing index 1", err.Error())
	}
}

func TestCreateXYGeometry_SingleReturnsOwnComponent(t *testing.T) {
	t.Parallel()
	g := newFakeXYBox(0, 1, 0, 1)
	c, err := CreateXYGeometry(g)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := c.(boxComponent2D); !ok {
		t.Fatalf("single-input did not return underlying component, got %T", c)
	}
}

func TestCreateXYGeometry_MultipleUnion(t *testing.T) {
	t.Parallel()
	a := newFakeXYBox(0, 10, 0, 10)
	b := newFakeXYBox(20, 30, 20, 30)
	c, err := CreateXYGeometry(a, b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.MinX() != 0 || c.MaxX() != 30 || c.MinY() != 0 || c.MaxY() != 30 {
		t.Errorf("union bbox = (%v,%v,%v,%v); want (0,30,0,30)",
			c.MinX(), c.MaxX(), c.MinY(), c.MaxY())
	}
	if !c.Contains(5, 5) || !c.Contains(25, 25) {
		t.Error("Contains union failed")
	}
	if c.Contains(15, 15) {
		t.Error("composite should not contain a point disjoint from both children")
	}
	if got := c.Relate(1, 2, 1, 2); got != CellInsideQuery {
		t.Errorf("Relate inside-of-a = %v, want CELL_INSIDE_QUERY", got)
	}
	if got := c.Relate(100, 200, 100, 200); got != CellOutsideQuery {
		t.Errorf("Relate fully-outside = %v, want CELL_OUTSIDE_QUERY", got)
	}
}

// TestXYAndLatLonDoNotInterop confirms the marker methods isolate
// the two coordinate spaces at compile time. We cannot express the
// negative type-check in a runtime test, but we can at least confirm
// that the in-package fakes satisfy only the expected interface.
func TestXYAndLatLonDoNotInterop(t *testing.T) {
	t.Parallel()
	var (
		x XYGeometry     = fakeXYGeometry{component: boxComponent2D{}}
		l LatLonGeometry = fakeLatLonGeometry{component: boxComponent2D{}}
	)
	// Each one must satisfy the base Geometry interface.
	if _, ok := any(x).(Geometry); !ok {
		t.Error("XYGeometry should satisfy Geometry")
	}
	if _, ok := any(l).(Geometry); !ok {
		t.Error("LatLonGeometry should satisfy Geometry")
	}
	// And the cross-cast must fail at runtime (compile-time check is
	// guaranteed by the marker methods being unexported).
	if _, ok := any(x).(LatLonGeometry); ok {
		t.Error("fakeXYGeometry should not satisfy LatLonGeometry")
	}
	if _, ok := any(l).(XYGeometry); ok {
		t.Error("fakeLatLonGeometry should not satisfy XYGeometry")
	}
}
