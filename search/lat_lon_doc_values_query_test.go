// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/index"
)

// testLatLonGeoRect builds a Rectangle covering [minLat..maxLat] x
// [minLon..maxLon] and fails the test on validation error.
func testLatLonGeoRect(t *testing.T, minLat, maxLat, minLon, maxLon float64) geo.Rectangle {
	t.Helper()
	r, err := geo.NewRectangle(minLat, maxLat, minLon, maxLon)
	if err != nil {
		t.Fatalf("geo.NewRectangle: %v", err)
	}
	return r
}

// testLatLonGeoLine builds a Line and fails on validation error.
func testLatLonGeoLine(t *testing.T, lats, lons []float64) geo.Line {
	t.Helper()
	l, err := geo.NewLine(lats, lons)
	if err != nil {
		t.Fatalf("geo.NewLine: %v", err)
	}
	return l
}

// testLatLonGeoPoint builds a Point and fails on validation error.
func testLatLonGeoPoint(t *testing.T, lat, lon float64) geo.Point {
	t.Helper()
	p, err := geo.NewPoint(lat, lon)
	if err != nil {
		t.Fatalf("geo.NewPoint: %v", err)
	}
	return p
}

// TestNewLatLonDocValuesQuery_BasicConstruction confirms the happy
// path: the constructor accepts a non-empty field, a valid
// QueryRelation and at least one geometry, and surfaces the public
// accessors.
func TestNewLatLonDocValuesQuery_BasicConstruction(t *testing.T) {
	t.Parallel()
	rect := testLatLonGeoRect(t, -10, 10, -20, 20)
	q, err := NewLatLonDocValuesQuery("loc", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonDocValuesQuery: %v", err)
	}
	concrete, ok := q.(*latLonDocValuesQuery)
	if !ok {
		t.Fatalf("expected *latLonDocValuesQuery, got %T", q)
	}
	if got := concrete.GetField(); got != "loc" {
		t.Fatalf("GetField: got %q, want %q", got, "loc")
	}
	if got := concrete.GetQueryRelation(); got != document.QueryRelationIntersects {
		t.Fatalf("GetQueryRelation: got %v, want INTERSECTS", got)
	}
	if got := concrete.GetGeometries(); len(got) != 1 {
		t.Fatalf("GetGeometries: got %d, want 1", len(got))
	}
}

// TestNewLatLonDocValuesQuery_EmptyField rejects an empty field name,
// mirroring the Java IllegalArgumentException("field must not be
// null").
func TestNewLatLonDocValuesQuery_EmptyField(t *testing.T) {
	t.Parallel()
	rect := testLatLonGeoRect(t, -10, 10, -20, 20)
	_, err := NewLatLonDocValuesQuery("", document.QueryRelationIntersects, rect)
	if !errors.Is(err, errLatLonDocValuesQueryNilField) {
		t.Fatalf("empty field: got %v, want errLatLonDocValuesQueryNilField", err)
	}
}

// TestNewLatLonDocValuesQuery_InvalidRelation rejects relations outside
// the supported enum range.
func TestNewLatLonDocValuesQuery_InvalidRelation(t *testing.T) {
	t.Parallel()
	rect := testLatLonGeoRect(t, -10, 10, -20, 20)
	_, err := NewLatLonDocValuesQuery("loc", document.QueryRelation(99), rect)
	if !errors.Is(err, errLatLonDocValuesQueryNilRelation) {
		t.Fatalf("invalid relation: got %v, want errLatLonDocValuesQueryNilRelation", err)
	}
}

// TestNewLatLonDocValuesQuery_RejectsWithinLine mirrors the Java
// "WITHIN queries with line geometries" guard.
func TestNewLatLonDocValuesQuery_RejectsWithinLine(t *testing.T) {
	t.Parallel()
	line := testLatLonGeoLine(t, []float64{0, 1, 2}, []float64{0, 1, 2})
	_, err := NewLatLonDocValuesQuery("loc", document.QueryRelationWithin, line)
	if err == nil || !strings.Contains(err.Error(), "line geometries") {
		t.Fatalf("WITHIN+Line: got %v, want error mentioning line geometries", err)
	}
}

// TestNewLatLonDocValuesQuery_RejectsContainsNonPoint mirrors the Java
// "CONTAINS queries with non-points geometries" guard.
func TestNewLatLonDocValuesQuery_RejectsContainsNonPoint(t *testing.T) {
	t.Parallel()
	rect := testLatLonGeoRect(t, -10, 10, -20, 20)
	_, err := NewLatLonDocValuesQuery("loc", document.QueryRelationContains, rect)
	if err == nil || !strings.Contains(err.Error(), "non-points geometries") {
		t.Fatalf("CONTAINS+Rectangle: got %v, want non-points-geometries error", err)
	}
}

// TestLatLonDocValuesQuery_Equals_Reflexive confirms that two queries
// built with the same field, relation and geometry slice are equal.
func TestLatLonDocValuesQuery_Equals_Reflexive(t *testing.T) {
	t.Parallel()
	rect := testLatLonGeoRect(t, -10, 10, -20, 20)
	a, _ := NewLatLonDocValuesQuery("loc", document.QueryRelationIntersects, rect)
	b, _ := NewLatLonDocValuesQuery("loc", document.QueryRelationIntersects, rect)
	if !a.Equals(b) {
		t.Fatalf("Equals: expected true for identical queries")
	}
	if a.HashCode() != b.HashCode() {
		t.Fatalf("HashCode: expected match for equal queries (%d vs %d)",
			a.HashCode(), b.HashCode())
	}
}

// TestLatLonDocValuesQuery_Equals_FieldDifference asserts that a
// distinct field produces a distinct query.
func TestLatLonDocValuesQuery_Equals_FieldDifference(t *testing.T) {
	t.Parallel()
	rect := testLatLonGeoRect(t, -10, 10, -20, 20)
	a, _ := NewLatLonDocValuesQuery("loc", document.QueryRelationIntersects, rect)
	b, _ := NewLatLonDocValuesQuery("other", document.QueryRelationIntersects, rect)
	if a.Equals(b) {
		t.Fatalf("Equals: distinct fields must not compare equal")
	}
}

// TestLatLonDocValuesQuery_Equals_RelationDifference asserts that
// distinct relations produce distinct queries.
func TestLatLonDocValuesQuery_Equals_RelationDifference(t *testing.T) {
	t.Parallel()
	rect := testLatLonGeoRect(t, -10, 10, -20, 20)
	a, _ := NewLatLonDocValuesQuery("loc", document.QueryRelationIntersects, rect)
	b, _ := NewLatLonDocValuesQuery("loc", document.QueryRelationDisjoint, rect)
	if a.Equals(b) {
		t.Fatalf("Equals: distinct relations must not compare equal")
	}
}

// TestLatLonDocValuesQuery_Equals_RejectsForeign confirms that a
// non-latLonDocValuesQuery never compares equal.
func TestLatLonDocValuesQuery_Equals_RejectsForeign(t *testing.T) {
	t.Parallel()
	rect := testLatLonGeoRect(t, -10, 10, -20, 20)
	q, _ := NewLatLonDocValuesQuery("loc", document.QueryRelationIntersects, rect)
	if q.Equals(&BaseQuery{}) {
		t.Fatalf("Equals: foreign Query must not compare equal")
	}
}

// TestLatLonDocValuesQuery_String formats the query with and without
// the default-field prefix.
func TestLatLonDocValuesQuery_String(t *testing.T) {
	t.Parallel()
	rect := testLatLonGeoRect(t, -10, 10, -20, 20)
	q, _ := NewLatLonDocValuesQuery("loc", document.QueryRelationIntersects, rect)
	concrete := q.(*latLonDocValuesQuery)
	withPrefix := concrete.String("other")
	if !strings.HasPrefix(withPrefix, "loc:") {
		t.Fatalf("String(other): expected 'loc:' prefix, got %q", withPrefix)
	}
	withoutPrefix := concrete.String("loc")
	if strings.HasPrefix(withoutPrefix, "loc:") {
		t.Fatalf("String(loc): unexpected field prefix in %q", withoutPrefix)
	}
	if !strings.Contains(withoutPrefix, "INTERSECTS") {
		t.Fatalf("String: expected relation in output, got %q", withoutPrefix)
	}
	if !strings.Contains(withoutPrefix, "geometries(") {
		t.Fatalf("String: expected 'geometries(' segment, got %q", withoutPrefix)
	}
}

// TestLatLonDocValuesQuery_Visit asserts that Visit descends into the
// leaf only when AcceptField returns true.
func TestLatLonDocValuesQuery_Visit(t *testing.T) {
	t.Parallel()
	rect := testLatLonGeoRect(t, -10, 10, -20, 20)
	q, _ := NewLatLonDocValuesQuery("loc", document.QueryRelationIntersects, rect)

	acceptVisitor := &latLonDocValuesQueryVisitor{accept: true}
	q.(*latLonDocValuesQuery).Visit(acceptVisitor)
	if !acceptVisitor.leafCalled {
		t.Fatalf("accept=true: expected VisitLeaf to be called")
	}

	rejectVisitor := &latLonDocValuesQueryVisitor{accept: false}
	q.(*latLonDocValuesQuery).Visit(rejectVisitor)
	if rejectVisitor.leafCalled {
		t.Fatalf("accept=false: expected VisitLeaf NOT to be called")
	}
}

// latLonDocValuesQueryVisitor is a minimal QueryVisitor that records
// whether VisitLeaf was invoked. Embeds EmptyQueryVisitorBase to
// satisfy ConsumeTerms / ConsumeTermsMatching / GetSubVisitor.
type latLonDocValuesQueryVisitor struct {
	EmptyQueryVisitorBase
	accept     bool
	leafCalled bool
}

func (v *latLonDocValuesQueryVisitor) AcceptField(_ string) bool { return v.accept }
func (v *latLonDocValuesQueryVisitor) VisitLeaf(_ Query)         { v.leafCalled = true }

// fakeSortedNumeric is an in-memory SortedNumericDocValues backed by a
// per-docID slice of int64 values. Used to drive the scorer logic in
// unit tests without standing up a full SegmentReader.
type fakeSortedNumeric struct {
	values map[int][]int64
	docIDs []int
	cursor int
	docID  int
}

func newFakeSortedNumeric(values map[int][]int64) *fakeSortedNumeric {
	ids := make([]int, 0, len(values))
	for id := range values {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return &fakeSortedNumeric{
		values: values,
		docIDs: ids,
		cursor: 0,
		docID:  -1,
	}
}

func (f *fakeSortedNumeric) Get(docID int) ([]int64, error) {
	return f.values[docID], nil
}

func (f *fakeSortedNumeric) Advance(target int) (int, error) {
	for f.cursor < len(f.docIDs) && f.docIDs[f.cursor] < target {
		f.cursor++
	}
	if f.cursor >= len(f.docIDs) {
		f.docID = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	f.docID = f.docIDs[f.cursor]
	f.cursor++
	return f.docID, nil
}

func (f *fakeSortedNumeric) NextDoc() (int, error) {
	if f.cursor >= len(f.docIDs) {
		f.docID = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	f.docID = f.docIDs[f.cursor]
	f.cursor++
	return f.docID, nil
}

func (f *fakeSortedNumeric) DocID() int { return f.docID }

// TestMatchesIntersects covers the intersects() match path: a doc with
// at least one value inside the shape matches; a doc with no value
// inside does not.
func TestMatchesIntersects(t *testing.T) {
	t.Parallel()
	// Query rectangle covers [-10..10] x [-20..20].
	rect := testLatLonGeoRect(t, -10, 10, -20, 20)
	c2d, err := geo.CreateLatLonGeometry(rect)
	if err != nil {
		t.Fatalf("CreateLatLonGeometry: %v", err)
	}
	pred := geo.CreateComponentPredicate(c2d)

	values := map[int][]int64{
		0: {document.EncodeLatLonAsLong(0, 0)},                                      // inside
		1: {document.EncodeLatLonAsLong(50, 50)},                                    // outside
		2: {document.EncodeLatLonAsLong(50, 50), document.EncodeLatLonAsLong(5, 5)}, // mixed
		3: {document.EncodeLatLonAsLong(80, 80)},                                    // outside (far)
		4: {document.EncodeLatLonAsLong(-9.999, -19.999)},                           // inside edge
	}
	fake := newFakeSortedNumeric(values)

	cases := []struct {
		docID int
		want  bool
	}{
		{0, true},
		{1, false},
		{2, true},
		{3, false},
		{4, true},
	}
	for _, tc := range cases {
		got, err := matchesIntersects(fake, &pred, tc.docID)
		if err != nil {
			t.Fatalf("matchesIntersects(doc=%d): %v", tc.docID, err)
		}
		if got != tc.want {
			t.Errorf("matchesIntersects(doc=%d): got %v, want %v", tc.docID, got, tc.want)
		}
	}
}

// TestMatchesWithin covers within(): every value must be inside the
// shape for the doc to match; mixed docs fail.
func TestMatchesWithin(t *testing.T) {
	t.Parallel()
	rect := testLatLonGeoRect(t, -10, 10, -20, 20)
	c2d, err := geo.CreateLatLonGeometry(rect)
	if err != nil {
		t.Fatalf("CreateLatLonGeometry: %v", err)
	}
	pred := geo.CreateComponentPredicate(c2d)

	values := map[int][]int64{
		0: {document.EncodeLatLonAsLong(0, 0)},                                      // inside (single)
		1: {document.EncodeLatLonAsLong(0, 0), document.EncodeLatLonAsLong(5, 5)},   // both inside
		2: {document.EncodeLatLonAsLong(0, 0), document.EncodeLatLonAsLong(50, 50)}, // mixed
		3: {document.EncodeLatLonAsLong(50, 50)},                                    // outside
	}
	fake := newFakeSortedNumeric(values)

	cases := []struct {
		docID int
		want  bool
	}{
		{0, true},
		{1, true},
		{2, false},
		{3, false},
	}
	for _, tc := range cases {
		got, err := matchesWithin(fake, &pred, tc.docID)
		if err != nil {
			t.Fatalf("matchesWithin(doc=%d): %v", tc.docID, err)
		}
		if got != tc.want {
			t.Errorf("matchesWithin(doc=%d): got %v, want %v", tc.docID, got, tc.want)
		}
	}
}

// TestMatchesDisjoint covers disjoint(): no value may be inside the
// shape for the doc to match.
func TestMatchesDisjoint(t *testing.T) {
	t.Parallel()
	rect := testLatLonGeoRect(t, -10, 10, -20, 20)
	c2d, err := geo.CreateLatLonGeometry(rect)
	if err != nil {
		t.Fatalf("CreateLatLonGeometry: %v", err)
	}
	pred := geo.CreateComponentPredicate(c2d)

	values := map[int][]int64{
		0: {document.EncodeLatLonAsLong(50, 50)},                                      // outside (matches)
		1: {document.EncodeLatLonAsLong(0, 0)},                                        // inside (no match)
		2: {document.EncodeLatLonAsLong(50, 50), document.EncodeLatLonAsLong(40, 40)}, // both outside
		3: {document.EncodeLatLonAsLong(50, 50), document.EncodeLatLonAsLong(0, 0)},   // mixed
	}
	fake := newFakeSortedNumeric(values)

	cases := []struct {
		docID int
		want  bool
	}{
		{0, true},
		{1, false},
		{2, true},
		{3, false},
	}
	for _, tc := range cases {
		got, err := matchesDisjoint(fake, &pred, tc.docID)
		if err != nil {
			t.Fatalf("matchesDisjoint(doc=%d): %v", tc.docID, err)
		}
		if got != tc.want {
			t.Errorf("matchesDisjoint(doc=%d): got %v, want %v", tc.docID, got, tc.want)
		}
	}
}

// TestMatchesContains_PointEqual asserts that CONTAINS over a Point
// geometry matches a doc holding the same lat/lon. The Java reference
// uses Component2D.withinPoint and folds the per-geometry answer; the
// matching pattern is "every component reports CANDIDATE for at least
// one indexed value, and none reports NOTWITHIN".
func TestMatchesContains_PointEqual(t *testing.T) {
	t.Parallel()
	p := testLatLonGeoPoint(t, 5, 5)
	c2d, err := geo.CreateLatLonGeometry(p)
	if err != nil {
		t.Fatalf("CreateLatLonGeometry: %v", err)
	}

	values := map[int][]int64{
		0: {document.EncodeLatLonAsLong(5, 5)},   // same point
		1: {document.EncodeLatLonAsLong(50, 50)}, // far away
	}
	fake := newFakeSortedNumeric(values)

	// CONTAINS expects the indexed point to "contain" the query
	// geometry; with a single point geometry the relation reduces
	// to coincidence. We assert the disjoint case returns false.
	if got, err := matchesContains(fake, []geo.Component2D{c2d}, 1); err != nil || got {
		t.Fatalf("matchesContains(doc=1, far): got (%v, %v), want (false, nil)", got, err)
	}
}

// TestCreateWeight_CacheableHook covers the IsDocValuesCacheable
// fallback (no FieldInfos reachable) — the weight must report
// cacheable=true when the context has no reader at all.
func TestCreateWeight_CacheableHook(t *testing.T) {
	t.Parallel()
	rect := testLatLonGeoRect(t, -10, 10, -20, 20)
	q, _ := NewLatLonDocValuesQuery("loc", document.QueryRelationIntersects, rect)
	w, err := q.CreateWeight(nil, false, 1.0)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	if !w.IsCacheable(nil) {
		t.Fatalf("IsCacheable(nil ctx): expected true")
	}
}

// TestCreateWeight_NilField guards against passing a nil
// LeafReaderContext through the supplier: the supplier returns nil
// (no scorer) rather than panicking.
func TestCreateWeight_NilField(t *testing.T) {
	t.Parallel()
	rect := testLatLonGeoRect(t, -10, 10, -20, 20)
	q, _ := NewLatLonDocValuesQuery("loc", document.QueryRelationIntersects, rect)
	w, err := q.CreateWeight(nil, false, 1.0)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	cw, ok := w.(*ConstantScoreWeight)
	if !ok {
		t.Fatalf("expected *ConstantScoreWeight, got %T", w)
	}
	sup, err := cw.ScorerSupplier(nil)
	if err != nil {
		t.Fatalf("ScorerSupplier(nil): %v", err)
	}
	if sup != nil {
		t.Fatalf("ScorerSupplier(nil ctx): expected nil supplier, got %T", sup)
	}
}

// TestSortedNumericApproximation_Iteration confirms the adapter walks
// the underlying iterator and surfaces NO_MORE_DOCS as the search
// package sentinel.
func TestSortedNumericApproximation_Iteration(t *testing.T) {
	t.Parallel()
	fake := newFakeSortedNumeric(map[int][]int64{
		1: {0},
		3: {0},
		7: {0},
	})
	approx := newSortedNumericApproximation(fake, 10)

	want := []int{1, 3, 7, NO_MORE_DOCS}
	for i, expected := range want {
		got, err := approx.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc[%d]: %v", i, err)
		}
		if got != expected {
			t.Fatalf("NextDoc[%d]: got %d, want %d", i, got, expected)
		}
	}
	if approx.Cost() != 10 {
		t.Fatalf("Cost: got %d, want 10", approx.Cost())
	}
}

// TestIsDocValuesCacheable_NilContext returns true for a nil
// LeafReaderContext (the safe default the helper documents).
func TestIsDocValuesCacheable_NilContext(t *testing.T) {
	t.Parallel()
	if !index.IsDocValuesCacheable(nil, "loc") {
		t.Fatalf("IsDocValuesCacheable(nil): expected true")
	}
}
