// Tests for the Geometry base interface. Lucene 10.4.0 ships no
// TestGeometry.java peer because Geometry is an abstract marker; the
// behavioural coverage in Lucene is provided indirectly through the
// concrete shape tests (TestPoint, TestRectangle, TestLine, TestPolygon,
// TestCircle, TestXY* and TestComponent2D/TestPolygon2D etc).
//
// At this stage we only verify the interface contract itself: that the
// sealed Geometry interface exists, that its single method is unexported
// (so external packages cannot implement it accidentally), and that an
// in-package stub satisfies the contract. The full behavioural tests
// land with each concrete shape's own task.

package geo

import (
	"reflect"
	"testing"
)

// stubGeometry is a minimal in-package implementation used solely to
// confirm that Geometry can be satisfied by a value declared inside the
// geo package.
type stubGeometry struct {
	component Component2D
}

func (s stubGeometry) toComponent2D() Component2D { return s.component }

// stubComponent2D is a zero-behaviour Component2D for tests that only
// need a non-nil reference.
type stubComponent2D struct{}

func (stubComponent2D) MinX() float64                      { return 0 }
func (stubComponent2D) MaxX() float64                      { return 0 }
func (stubComponent2D) MinY() float64                      { return 0 }
func (stubComponent2D) MaxY() float64                      { return 0 }
func (stubComponent2D) Contains(_ float64, _ float64) bool { return false }
func (stubComponent2D) Relate(_, _, _, _ float64) Relation { return CellOutsideQuery }
func (stubComponent2D) IntersectsLine(_, _, _, _, _, _, _, _ float64) bool {
	return false
}
func (stubComponent2D) IntersectsTriangle(_, _, _, _, _, _, _, _, _, _ float64) bool {
	return false
}
func (stubComponent2D) ContainsLine(_, _, _, _, _, _, _, _ float64) bool {
	return false
}
func (stubComponent2D) ContainsTriangle(_, _, _, _, _, _, _, _, _, _ float64) bool {
	return false
}
func (stubComponent2D) WithinPoint(_, _ float64) WithinRelation {
	return WithinDisjoint
}
func (stubComponent2D) WithinLine(_, _, _, _, _, _ float64, _ bool, _, _ float64) WithinRelation {
	return WithinDisjoint
}
func (stubComponent2D) WithinTriangle(_, _, _, _, _, _ float64, _ bool, _, _ float64, _ bool, _, _ float64, _ bool) WithinRelation {
	return WithinDisjoint
}

func TestGeometryInterfaceShape(t *testing.T) {
	t.Parallel()

	// The Geometry interface must declare exactly one method named
	// "toComponent2D" returning a single Component2D. We assert this
	// via reflection so that any silent contract drift fails the test.
	gType := reflect.TypeOf((*Geometry)(nil)).Elem()
	if got, want := gType.Kind(), reflect.Interface; got != want {
		t.Fatalf("Geometry kind = %v, want %v", got, want)
	}
	if got, want := gType.NumMethod(), 1; got != want {
		t.Fatalf("Geometry method count = %d, want %d", got, want)
	}
	m := gType.Method(0)
	if m.Name != "toComponent2D" {
		t.Fatalf("Geometry method name = %q, want %q", m.Name, "toComponent2D")
	}
	if got, want := m.Type.NumIn(), 0; got != want {
		t.Fatalf("toComponent2D in-arity = %d, want %d", got, want)
	}
	if got, want := m.Type.NumOut(), 1; got != want {
		t.Fatalf("toComponent2D out-arity = %d, want %d", got, want)
	}
	if got, want := m.Type.Out(0), reflect.TypeOf((*Component2D)(nil)).Elem(); got != want {
		t.Fatalf("toComponent2D return = %v, want %v", got, want)
	}
}

func TestGeometrySealed(t *testing.T) {
	t.Parallel()

	// An in-package implementation can satisfy Geometry. This is the
	// positive half of the sealed-interface contract; the negative half
	// (external packages cannot satisfy it because toComponent2D is
	// unexported) is enforced by the Go type system itself and is
	// therefore checked at compile time by any importing package.
	var g Geometry = stubGeometry{component: stubComponent2D{}}
	if g.toComponent2D() == nil {
		t.Fatal("stub geometry returned nil Component2D")
	}
}

func TestRelationString(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   Relation
		want string
	}{
		{CellInsideQuery, "CELL_INSIDE_QUERY"},
		{CellOutsideQuery, "CELL_OUTSIDE_QUERY"},
		{CellCrossesQuery, "CELL_CROSSES_QUERY"},
		{Relation(99), "UNKNOWN"},
	}
	for _, tc := range cases {
		if got := tc.in.String(); got != tc.want {
			t.Errorf("Relation(%d).String() = %q, want %q", tc.in, got, tc.want)
		}
	}
}
