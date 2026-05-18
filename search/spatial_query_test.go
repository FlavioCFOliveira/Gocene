// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
)

// fakeSpatialVisitor is a stub SpatialVisitor used in the tests
// below. It returns canned Relate / Intersects / Within / Contains
// results and embeds *BaseSpatialVisitor so the helper-method
// dispatch (GetInnerFunction / GetLeafPredicate) is exercised
// against the real production code.
type fakeSpatialVisitor struct {
	*BaseSpatialVisitor

	relateResult     spatialRelation
	intersectsResult bool
	withinResult     bool
	containsResult   geo.WithinRelation
}

// newFakeSpatialVisitor wires the BaseSpatialVisitor backlink so
// dispatch flows through the embedding type.
func newFakeSpatialVisitor(
	relate spatialRelation,
	intersects, within bool,
	contains geo.WithinRelation,
) *fakeSpatialVisitor {
	v := &fakeSpatialVisitor{
		relateResult:     relate,
		intersectsResult: intersects,
		withinResult:     within,
		containsResult:   contains,
	}
	v.BaseSpatialVisitor = NewBaseSpatialVisitor(v)
	return v
}

func (v *fakeSpatialVisitor) Relate(_, _ []byte) spatialRelation { return v.relateResult }
func (v *fakeSpatialVisitor) Intersects() func(packed []byte) bool {
	return func(_ []byte) bool { return v.intersectsResult }
}
func (v *fakeSpatialVisitor) Within() func(packed []byte) bool {
	return func(_ []byte) bool { return v.withinResult }
}
func (v *fakeSpatialVisitor) Contains() func(packed []byte) geo.WithinRelation {
	return func(_ []byte) geo.WithinRelation { return v.containsResult }
}

// fakeComponent2D is a minimal Component2D used to satisfy the
// constructor's non-nil contract. It returns DISJOINT for every
// query — the visitor is the one that drives behaviour in tests.
type fakeComponent2D struct{}

func (fakeComponent2D) MinX() float64                                      { return 0 }
func (fakeComponent2D) MaxX() float64                                      { return 0 }
func (fakeComponent2D) MinY() float64                                      { return 0 }
func (fakeComponent2D) MaxY() float64                                      { return 0 }
func (fakeComponent2D) Relate(_, _, _, _ float64) geo.Relation             { return geo.CellOutsideQuery }
func (fakeComponent2D) Contains(_, _ float64) bool                         { return false }
func (fakeComponent2D) IntersectsLine(_, _, _, _, _, _, _, _ float64) bool { return false }
func (fakeComponent2D) IntersectsTriangle(_, _, _, _, _, _, _, _, _, _ float64) bool {
	return false
}
func (fakeComponent2D) ContainsLine(_, _, _, _, _, _, _, _ float64) bool { return false }
func (fakeComponent2D) ContainsTriangle(_, _, _, _, _, _, _, _, _, _ float64) bool {
	return false
}
func (fakeComponent2D) WithinPoint(_, _ float64) geo.WithinRelation { return geo.WithinDisjoint }
func (fakeComponent2D) WithinLine(_, _, _, _, _, _ float64, _ bool, _, _ float64) geo.WithinRelation {
	return geo.WithinDisjoint
}
func (fakeComponent2D) WithinTriangle(_, _, _, _, _, _ float64, _ bool, _, _ float64, _ bool, _, _ float64, _ bool) geo.WithinRelation {
	return geo.WithinDisjoint
}

// fakeComponent2DRef returns a fresh allocator-backed pointer so
// reference-identity comparisons (q.GetQueryComponent2D() != tree)
// work as expected. Component2D is an interface; comparing
// interface values to bare-struct values is rejected by the
// compiler, so this helper hands out *fakeComponent2D values
// directly typed as geo.Component2D.
func fakeComponent2DRef() geo.Component2D { return fakeComponent2D{} }

// TestNewSpatialQuery_RejectsEmptyField confirms the constructor
// rejects an empty field name with an error (the Java reference
// throws IllegalArgumentException).
func TestNewSpatialQuery_RejectsEmptyField(t *testing.T) {
	t.Parallel()
	if _, err := NewSpatialQuery(
		"",
		document.QueryRelationIntersects,
		fakeComponent2D{},
		func() SpatialVisitor { return nil },
		nil,
	); err == nil {
		t.Fatalf("expected error on empty field")
	}
}

// TestNewSpatialQuery_RejectsNilComponent confirms the constructor
// rejects a nil Component2D tree.
func TestNewSpatialQuery_RejectsNilComponent(t *testing.T) {
	t.Parallel()
	if _, err := NewSpatialQuery(
		"f",
		document.QueryRelationIntersects,
		nil,
		func() SpatialVisitor { return nil },
		nil,
	); err == nil {
		t.Fatalf("expected error on nil Component2D")
	}
}

// TestNewSpatialQuery_RejectsNilFactory confirms the constructor
// rejects a nil spatialVisitorFactory.
func TestNewSpatialQuery_RejectsNilFactory(t *testing.T) {
	t.Parallel()
	if _, err := NewSpatialQuery(
		"f",
		document.QueryRelationIntersects,
		fakeComponent2D{},
		nil,
		nil,
	); err == nil {
		t.Fatalf("expected error on nil spatialVisitorFactory")
	}
}

// TestSpatialQuery_AccessorsReturnConstructorArgs confirms the
// getter family reports the values supplied at construction.
func TestSpatialQuery_AccessorsReturnConstructorArgs(t *testing.T) {
	t.Parallel()
	tree := fakeComponent2DRef()
	q, err := NewSpatialQuery(
		"shape",
		document.QueryRelationWithin,
		tree,
		func() SpatialVisitor {
			return newFakeSpatialVisitor(spatialCellOutsideQuery, false, false, geo.WithinDisjoint)
		},
		nil,
	)
	if err != nil {
		t.Fatalf("NewSpatialQuery: %v", err)
	}
	if got, want := q.GetField(), "shape"; got != want {
		t.Fatalf("GetField: got %q, want %q", got, want)
	}
	if got, want := q.GetQueryRelation(), document.QueryRelationWithin; got != want {
		t.Fatalf("GetQueryRelation: got %v, want %v", got, want)
	}
	if q.GetQueryComponent2D() != tree {
		t.Fatalf("GetQueryComponent2D: got %v, want %v", q.GetQueryComponent2D(), tree)
	}
	if v := q.GetSpatialVisitor(); v == nil {
		t.Fatalf("GetSpatialVisitor: got nil, want non-nil")
	}
}

// TestSpatialQuery_GetGeometries_ReturnsDefensiveCopy confirms the
// geometries slice is copied at construction (mutating the input
// slice after the constructor returns does not change the stored
// geometries) and at the getter (mutating the returned slice does
// not change the stored geometries).
//
// geo.Geometry is a sealed interface; the test uses a concrete
// geo.Point as a sentinel value so the defensive-copy assertions
// have stable identity.
func TestSpatialQuery_GetGeometries_ReturnsDefensiveCopy(t *testing.T) {
	t.Parallel()
	sentinel, err := geo.NewPoint(10, 20)
	if err != nil {
		t.Fatalf("NewPoint: %v", err)
	}
	input := []geo.Geometry{sentinel, sentinel}
	q, err := NewSpatialQuery(
		"f",
		document.QueryRelationIntersects,
		fakeComponent2D{},
		func() SpatialVisitor {
			return newFakeSpatialVisitor(spatialCellOutsideQuery, false, false, geo.WithinDisjoint)
		},
		input,
	)
	if err != nil {
		t.Fatalf("NewSpatialQuery: %v", err)
	}
	if got := len(q.GetGeometries()); got != 2 {
		t.Fatalf("GetGeometries len at start: got %d, want 2", got)
	}

	// Mutating the input slice after construction must not affect
	// the stored geometries (verifies the constructor copy).
	input[0] = nil
	if got := q.GetGeometries()[0]; got != sentinel {
		t.Fatalf("GetGeometries[0] after input mutation: got %v, want %v", got, sentinel)
	}

	// Mutating the returned slice must not affect subsequent
	// getter calls (verifies the getter copy).
	out := q.GetGeometries()
	out[0] = nil
	if got := q.GetGeometries()[0]; got != sentinel {
		t.Fatalf("GetGeometries[0] after output mutation: got %v, want %v", got, sentinel)
	}
}

// TestSpatialQuery_Visit_RoutesThroughQueryVisitor confirms Visit
// calls VisitLeaf on a visitor that accepts the field, and skips
// when the visitor rejects.
func TestSpatialQuery_Visit_RoutesThroughQueryVisitor(t *testing.T) {
	t.Parallel()
	q, err := NewSpatialQuery(
		"shape",
		document.QueryRelationIntersects,
		fakeComponent2D{},
		func() SpatialVisitor {
			return newFakeSpatialVisitor(spatialCellOutsideQuery, false, false, geo.WithinDisjoint)
		},
		nil,
	)
	if err != nil {
		t.Fatalf("NewSpatialQuery: %v", err)
	}
	accept := &recordingQueryVisitor{acceptField: true}
	q.Visit(accept)
	if !accept.leafCalled {
		t.Fatalf("Visit on accepting visitor: leaf hook not called")
	}
	reject := &recordingQueryVisitor{acceptField: false}
	q.Visit(reject)
	if reject.leafCalled {
		t.Fatalf("Visit on rejecting visitor: leaf hook called")
	}
}

// recordingQueryVisitor records whether VisitLeaf was called so
// tests can assert dispatch.
type recordingQueryVisitor struct {
	EmptyQueryVisitorBase
	acceptField bool
	leafCalled  bool
}

func (r *recordingQueryVisitor) AcceptField(_ string) bool { return r.acceptField }
func (r *recordingQueryVisitor) VisitLeaf(_ Query)         { r.leafCalled = true }

// TestSpatialQuery_Equals_FieldRelationGeometryIdentity confirms
// the equality contract over the three identity fields.
func TestSpatialQuery_Equals_FieldRelationGeometryIdentity(t *testing.T) {
	t.Parallel()
	build := func(field string, rel document.QueryRelation) *SpatialQuery {
		q, err := NewSpatialQuery(
			field,
			rel,
			fakeComponent2D{},
			func() SpatialVisitor {
				return newFakeSpatialVisitor(spatialCellOutsideQuery, false, false, geo.WithinDisjoint)
			},
			nil,
		)
		if err != nil {
			t.Fatalf("NewSpatialQuery(%q, %v): %v", field, rel, err)
		}
		return q
	}
	a := build("f", document.QueryRelationIntersects)
	b := build("f", document.QueryRelationIntersects)
	c := build("g", document.QueryRelationIntersects)
	d := build("f", document.QueryRelationWithin)
	if !a.Equals(b) {
		t.Fatalf("Equals: same field+rel should be equal")
	}
	if a.Equals(c) {
		t.Fatalf("Equals: different field should not be equal")
	}
	if a.Equals(d) {
		t.Fatalf("Equals: different rel should not be equal")
	}
}

// TestSpatialQuery_HashCode_StableForIdenticalQueries confirms two
// queries with the same field / relation / geometries hash to the
// same int.
func TestSpatialQuery_HashCode_StableForIdenticalQueries(t *testing.T) {
	t.Parallel()
	build := func() *SpatialQuery {
		q, err := NewSpatialQuery(
			"f",
			document.QueryRelationIntersects,
			fakeComponent2D{},
			func() SpatialVisitor {
				return newFakeSpatialVisitor(spatialCellOutsideQuery, false, false, geo.WithinDisjoint)
			},
			nil,
		)
		if err != nil {
			t.Fatalf("NewSpatialQuery: %v", err)
		}
		return q
	}
	if a, b := build().HashCode(), build().HashCode(); a != b {
		t.Fatalf("HashCode: a=%d b=%d, want equal", a, b)
	}
}

// TestSpatialQuery_String_DefaultClassName confirms the default
// String output prefix is the type name.
func TestSpatialQuery_String_DefaultClassName(t *testing.T) {
	t.Parallel()
	q, err := NewSpatialQuery(
		"f",
		document.QueryRelationIntersects,
		fakeComponent2D{},
		func() SpatialVisitor {
			return newFakeSpatialVisitor(spatialCellOutsideQuery, false, false, geo.WithinDisjoint)
		},
		nil,
	)
	if err != nil {
		t.Fatalf("NewSpatialQuery: %v", err)
	}
	if got := q.String("f"); !strings.HasPrefix(got, "SpatialQuery:") {
		t.Fatalf("String: got %q, want prefix %q", got, "SpatialQuery:")
	}
}

// TestSpatialQuery_String_WithDisplayClassName confirms the
// WithSpatialQueryDisplayClassName option overrides the prefix.
func TestSpatialQuery_String_WithDisplayClassName(t *testing.T) {
	t.Parallel()
	q, err := NewSpatialQuery(
		"f",
		document.QueryRelationIntersects,
		fakeComponent2D{},
		func() SpatialVisitor {
			return newFakeSpatialVisitor(spatialCellOutsideQuery, false, false, geo.WithinDisjoint)
		},
		nil,
		WithSpatialQueryDisplayClassName("LatLonShapeQuery"),
	)
	if err != nil {
		t.Fatalf("NewSpatialQuery: %v", err)
	}
	if got := q.String("f"); !strings.HasPrefix(got, "LatLonShapeQuery:") {
		t.Fatalf("String with override: got %q, want prefix %q", got, "LatLonShapeQuery:")
	}
}

// TestSpatialQuery_CreateWeight_BuildsConstantScoreWeight confirms
// CreateWeight returns a ConstantScoreWeight and that the weight
// carries the parent query and the supplied boost.
func TestSpatialQuery_CreateWeight_BuildsConstantScoreWeight(t *testing.T) {
	t.Parallel()
	q, err := NewSpatialQuery(
		"f",
		document.QueryRelationIntersects,
		fakeComponent2D{},
		func() SpatialVisitor {
			return newFakeSpatialVisitor(spatialCellOutsideQuery, false, false, geo.WithinDisjoint)
		},
		nil,
	)
	if err != nil {
		t.Fatalf("NewSpatialQuery: %v", err)
	}
	w, err := q.CreateWeight(nil, true, 0.5)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	if w == nil {
		t.Fatalf("CreateWeight: weight is nil")
	}
	cs, ok := w.(*ConstantScoreWeight)
	if !ok {
		t.Fatalf("CreateWeight: got %T, want *ConstantScoreWeight", w)
	}
	if cs.GetQuery() != q {
		t.Fatalf("Weight.GetQuery: got %v, want %v", cs.GetQuery(), q)
	}
	if got, want := cs.Score(), float32(0.5); got != want {
		t.Fatalf("Weight.Score: got %v, want %v", got, want)
	}
}

// TestSpatialQuery_QueryIsCacheable_DefaultsTrue confirms the
// default cacheability hook returns true (matching Java's protected
// default).
func TestSpatialQuery_QueryIsCacheable_DefaultsTrue(t *testing.T) {
	t.Parallel()
	q, err := NewSpatialQuery(
		"f",
		document.QueryRelationIntersects,
		fakeComponent2D{},
		func() SpatialVisitor {
			return newFakeSpatialVisitor(spatialCellOutsideQuery, false, false, geo.WithinDisjoint)
		},
		nil,
	)
	if err != nil {
		t.Fatalf("NewSpatialQuery: %v", err)
	}
	if !q.QueryIsCacheable(nil) {
		t.Fatalf("QueryIsCacheable default: got false, want true")
	}
}

// TestBaseSpatialVisitor_GetInnerFunction_TransposesForDisjoint
// covers the DISJOINT transposition path: INSIDE → OUTSIDE,
// OUTSIDE → INSIDE, CROSSES → CROSSES.
func TestBaseSpatialVisitor_GetInnerFunction_TransposesForDisjoint(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   spatialRelation
		want spatialRelation
	}{
		{spatialCellInsideQuery, spatialCellOutsideQuery},
		{spatialCellOutsideQuery, spatialCellInsideQuery},
		{spatialCellCrossesQuery, spatialCellCrossesQuery},
	}
	for _, c := range cases {
		v := newFakeSpatialVisitor(c.in, false, false, geo.WithinDisjoint)
		fn := v.GetInnerFunction(document.QueryRelationDisjoint)
		if got := fn(nil, nil); got != c.want {
			t.Fatalf("DISJOINT inner(%v): got %v, want %v", c.in, got, c.want)
		}
	}
}

// TestBaseSpatialVisitor_GetLeafPredicate_RoutesByRelation
// confirms each of the four relations maps to the right backing
// closure (intersects / within / !intersects / contains).
func TestBaseSpatialVisitor_GetLeafPredicate_RoutesByRelation(t *testing.T) {
	t.Parallel()
	v := newFakeSpatialVisitor(spatialCellOutsideQuery, true, true, geo.WithinCandidate)
	if !v.GetLeafPredicate(document.QueryRelationIntersects)(nil) {
		t.Fatalf("INTERSECTS: predicate returned false")
	}
	if !v.GetLeafPredicate(document.QueryRelationWithin)(nil) {
		t.Fatalf("WITHIN: predicate returned false")
	}
	if v.GetLeafPredicate(document.QueryRelationDisjoint)(nil) {
		t.Fatalf("DISJOINT: predicate returned true (Intersects is true so !Intersects is false)")
	}
	if !v.GetLeafPredicate(document.QueryRelationContains)(nil) {
		t.Fatalf("CONTAINS: predicate returned false")
	}

	// CONTAINS with a non-CANDIDATE result must yield false.
	v2 := newFakeSpatialVisitor(spatialCellOutsideQuery, true, true, geo.WithinDisjoint)
	if v2.GetLeafPredicate(document.QueryRelationContains)(nil) {
		t.Fatalf("CONTAINS with WithinDisjoint: predicate returned true")
	}
}

// TestSpatialRelation_StringHasReadableLabels covers the three
// stringer outputs.
func TestSpatialRelation_StringHasReadableLabels(t *testing.T) {
	t.Parallel()
	cases := map[spatialRelation]string{
		spatialCellInsideQuery:  "CELL_INSIDE_QUERY",
		spatialCellOutsideQuery: "CELL_OUTSIDE_QUERY",
		spatialCellCrossesQuery: "CELL_CROSSES_QUERY",
	}
	for r, want := range cases {
		if got := r.String(); got != want {
			t.Fatalf("spatialRelation(%d).String(): got %q, want %q", int(r), got, want)
		}
	}
}
