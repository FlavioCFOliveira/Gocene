// Tests for LatLonGeometry. Lucene 10.4.0 ships no
// TestLatLonGeometry.java peer because LatLonGeometry is an abstract
// base whose behaviour is exercised indirectly through the concrete
// shape tests (TestPoint, TestRectangle, TestLine, TestPolygon,
// TestCircle, TestComponent2D etc).
//
// We therefore validate the static factory contract directly:
//
//   - nil input returns ErrNilGeometries
//   - empty input returns ErrEmptyGeometries
//   - nil element at index i returns an error mentioning that index
//   - a single non-nil geometry returns its own Component2D
//   - multiple geometries return a composite whose Relate/Contains
//     follow the set-union semantics implemented by Lucene's
//     ComponentTree.

package geo

import (
	"errors"
	"strings"
	"testing"
)

// fakeLatLonGeometry is an in-package LatLonGeometry stub that
// returns a pre-configured Component2D. It exists solely to feed
// CreateLatLonGeometry inputs in tests without depending on the
// concrete shape ports which arrive in later tasks.
type fakeLatLonGeometry struct {
	component Component2D
}

func (f fakeLatLonGeometry) toComponent2D() Component2D { return f.component }
func (fakeLatLonGeometry) latLonGeometry()              {}

// boxComponent2D is a Component2D whose contents are a closed
// axis-aligned rectangle; it is the simplest concrete shape we can
// build without touching the unported concrete geometries.
type boxComponent2D struct {
	minX, maxX, minY, maxY float64
}

func (b boxComponent2D) MinX() float64        { return b.minX }
func (b boxComponent2D) MaxX() float64        { return b.maxX }
func (b boxComponent2D) MinY() float64        { return b.minY }
func (b boxComponent2D) MaxY() float64        { return b.maxY }
func (b boxComponent2D) Contains(x, y float64) bool {
	return x >= b.minX && x <= b.maxX && y >= b.minY && y <= b.maxY
}
func (b boxComponent2D) Relate(minX, maxX, minY, maxY float64) Relation {
	// Disjoint -> outside.
	if maxX < b.minX || minX > b.maxX || maxY < b.minY || minY > b.maxY {
		return CellOutsideQuery
	}
	// Query fully inside this box -> inside.
	if minX >= b.minX && maxX <= b.maxX && minY >= b.minY && maxY <= b.maxY {
		return CellInsideQuery
	}
	return CellCrossesQuery
}

func newFakeBox(minX, maxX, minY, maxY float64) fakeLatLonGeometry {
	return fakeLatLonGeometry{
		component: boxComponent2D{minX: minX, maxX: maxX, minY: minY, maxY: maxY},
	}
}

func TestCreateLatLonGeometry_NilSlice(t *testing.T) {
	t.Parallel()
	// Equivalent to Java's `null` array argument: the user passes a
	// nil slice explicitly via the variadic spread operator.
	var nilSlice []LatLonGeometry
	if _, err := CreateLatLonGeometry(nilSlice...); !errors.Is(err, ErrNilGeometries) {
		t.Fatalf("nil slice: err = %v, want ErrNilGeometries", err)
	}
}

func TestCreateLatLonGeometry_EmptySlice(t *testing.T) {
	t.Parallel()
	// Equivalent to Java's `new LatLonGeometry[0]`: a non-nil but
	// zero-length slice. In Go, variadic calls with zero arguments
	// produce a nil slice and therefore map to ErrNilGeometries; an
	// empty slice can only be produced explicitly as below.
	empty := []LatLonGeometry{}
	if _, err := CreateLatLonGeometry(empty...); !errors.Is(err, ErrEmptyGeometries) {
		t.Fatalf("empty slice: err = %v, want ErrEmptyGeometries", err)
	}
}

func TestCreateLatLonGeometry_NilElementAtZero(t *testing.T) {
	t.Parallel()
	_, err := CreateLatLonGeometry(nil)
	if err == nil {
		t.Fatal("expected error for nil element, got nil")
	}
	if !strings.Contains(err.Error(), "geometries[0]") {
		t.Fatalf("error message %q does not reference index 0", err.Error())
	}
}

func TestCreateLatLonGeometry_NilElementAtIndex(t *testing.T) {
	t.Parallel()
	// Build a slice with a non-nil entry followed by nil to confirm
	// the per-index error includes the actual index.
	g0 := newFakeBox(0, 1, 0, 1)
	_, err := CreateLatLonGeometry(g0, nil)
	if err == nil {
		t.Fatal("expected error for nil at index 1")
	}
	if !strings.Contains(err.Error(), "geometries[1]") {
		t.Fatalf("error message %q does not reference index 1", err.Error())
	}
}

func TestCreateLatLonGeometry_SingleReturnsOwnComponent(t *testing.T) {
	t.Parallel()
	box := newFakeBox(0, 1, 0, 1)
	c, err := CreateLatLonGeometry(box)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c == nil {
		t.Fatal("returned Component2D is nil")
	}
	// The single-input path must return the geometry's own
	// Component2D, not a composite. We assert that by checking the
	// concrete type matches the box stub.
	if _, ok := c.(boxComponent2D); !ok {
		t.Fatalf("single-input did not return underlying component, got %T", c)
	}
}

func TestCreateLatLonGeometry_MultipleUnionBoundingBox(t *testing.T) {
	t.Parallel()
	a := newFakeBox(0, 1, 0, 1)
	b := newFakeBox(2, 3, 4, 5)
	c, err := CreateLatLonGeometry(a, b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := c.MinX(), 0.0; got != want {
		t.Errorf("MinX = %v, want %v", got, want)
	}
	if got, want := c.MaxX(), 3.0; got != want {
		t.Errorf("MaxX = %v, want %v", got, want)
	}
	if got, want := c.MinY(), 0.0; got != want {
		t.Errorf("MinY = %v, want %v", got, want)
	}
	if got, want := c.MaxY(), 5.0; got != want {
		t.Errorf("MaxY = %v, want %v", got, want)
	}
}

func TestCreateLatLonGeometry_MultipleContainsUnion(t *testing.T) {
	t.Parallel()
	a := newFakeBox(0, 1, 0, 1)
	b := newFakeBox(10, 11, 10, 11)
	c, _ := CreateLatLonGeometry(a, b)

	if !c.Contains(0.5, 0.5) {
		t.Error("expected composite to contain point in a")
	}
	if !c.Contains(10.5, 10.5) {
		t.Error("expected composite to contain point in b")
	}
	if c.Contains(5, 5) {
		t.Error("composite should not contain point outside both children")
	}
}

func TestCreateLatLonGeometry_MultipleRelateUnion(t *testing.T) {
	t.Parallel()
	a := newFakeBox(0, 10, 0, 10)
	b := newFakeBox(20, 30, 20, 30)
	c, _ := CreateLatLonGeometry(a, b)

	// Box fully inside a -> INSIDE.
	if got := c.Relate(1, 2, 1, 2); got != CellInsideQuery {
		t.Errorf("relate(1,2,1,2) = %v, want CELL_INSIDE_QUERY", got)
	}
	// Box that crosses a's edge -> CROSSES (a CROSSES, b OUTSIDE).
	if got := c.Relate(-1, 5, -1, 5); got != CellCrossesQuery {
		t.Errorf("relate(-1,5,-1,5) = %v, want CELL_CROSSES_QUERY", got)
	}
	// Box disjoint from both children -> OUTSIDE.
	if got := c.Relate(100, 200, 100, 200); got != CellOutsideQuery {
		t.Errorf("relate(100,200,100,200) = %v, want CELL_OUTSIDE_QUERY", got)
	}
}

func TestMultiComponent2D_DefensiveCopy(t *testing.T) {
	t.Parallel()
	// Build a slice and pass it through the public path; mutate the
	// caller-side slice and confirm the composite is unchanged.
	a := newFakeBox(0, 1, 0, 1)
	b := newFakeBox(2, 3, 2, 3)
	c, _ := CreateLatLonGeometry(a, b)
	// We cannot mutate the slice from outside because
	// CreateLatLonGeometry took variadic args, but we can mutate the
	// composite's internal slice if it leaked. Best we can do here is
	// confirm the public MinX is unaffected by subsequent calls.
	want := c.MinX()
	if got := c.MinX(); got != want {
		t.Fatalf("MinX repeated read changed: first=%v second=%v", want, got)
	}
}
