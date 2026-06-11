// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestXYPointInGeometryQuery_ConstructorValidation covers the
// IllegalArgumentException-equivalent paths on the Java constructor:
// nil/empty field, nil/empty geometries, nil geometry entry.
func TestXYPointInGeometryQuery_ConstructorValidation(t *testing.T) {
	rect := mustXYRectangle(t, -10, 10, -10, 10)
	cases := []struct {
		name   string
		field  string
		geoms  []geo.XYGeometry
		expect bool // true => expect error
	}{
		{"empty field", "", []geo.XYGeometry{rect}, true},
		{"nil geometries slice", "p", nil, true},
		{"empty geometries", "p", []geo.XYGeometry{}, true},
		{"nil geometry entry", "p", []geo.XYGeometry{rect, nil, rect}, true},
		{"valid", "p", []geo.XYGeometry{rect}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := NewXYPointInGeometryQuery(c.field, c.geoms...)
			if c.expect && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !c.expect && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// TestXYPointInGeometryQuery_DefensiveCopy verifies the constructor
// copies the geometry slice so subsequent caller mutation does not
// affect the query. Mirrors Lucene's xyGeometries.clone() call.
func TestXYPointInGeometryQuery_DefensiveCopy(t *testing.T) {
	rect1 := mustXYRectangle(t, -1, 1, -1, 1)
	rect2 := mustXYRectangle(t, -2, 2, -2, 2)
	in := []geo.XYGeometry{rect1}
	q, err := NewXYPointInGeometryQuery("p", in...)
	if err != nil {
		t.Fatalf("NewXYPointInGeometryQuery: %v", err)
	}
	// Mutate the input slice element after construction.
	in[0] = rect2
	got := q.(*xyPointInGeometryQuery).Geometries()
	if len(got) != 1 {
		t.Fatalf("expected 1 geometry, got %d", len(got))
	}
	if got[0] != rect1 {
		t.Fatalf("expected internal geometry unchanged after caller mutation")
	}
	// Mutate the returned slice to confirm Geometries() also copies.
	got[0] = rect2
	got2 := q.(*xyPointInGeometryQuery).Geometries()
	if got2[0] != rect1 {
		t.Fatalf("expected internal geometry unchanged after caller mutation of Geometries() result")
	}
}

// TestXYPointInGeometryQuery_EqualsAndHashCode mirrors the standard
// QueryUtils.checkHashEquals contract: equal queries hash equal,
// different fields / geometries differ.
func TestXYPointInGeometryQuery_EqualsAndHashCode(t *testing.T) {
	rectA := mustXYRectangle(t, 0, 10, 0, 10)
	rectB := mustXYRectangle(t, 0, 20, 0, 20)

	q1 := mustQuery(t, "p", rectA)
	q2 := mustQuery(t, "p", rectA)
	q3 := mustQuery(t, "q", rectA) // different field
	q4 := mustQuery(t, "p", rectB) // different geometry

	if !q1.Equals(q2) || !q2.Equals(q1) {
		t.Fatalf("expected q1 == q2 for identical inputs")
	}
	if q1.HashCode() != q2.HashCode() {
		t.Fatalf("expected hash(q1) == hash(q2): %d vs %d", q1.HashCode(), q2.HashCode())
	}
	if q1.Equals(q3) {
		t.Fatalf("expected q1 != q3 (different field)")
	}
	if q1.Equals(q4) {
		t.Fatalf("expected q1 != q4 (different geometry)")
	}
	// HashCode collision tolerance: even if hashes happen to match,
	// Equals must be authoritative; here we expect them to differ.
	if q1.HashCode() == q3.HashCode() {
		t.Logf("note: hash collision between q1 and q3 (acceptable)")
	}
}

// TestXYPointInGeometryQuery_EqualsRejectsOtherTypes verifies the
// type-check branch in Equals: a same-shaped foreign query never
// matches.
func TestXYPointInGeometryQuery_EqualsRejectsOtherTypes(t *testing.T) {
	rect := mustXYRectangle(t, 0, 1, 0, 1)
	q := mustQuery(t, "p", rect)
	other := &fakeQueryForXYPoint{}
	if q.Equals(other) {
		t.Fatalf("expected Equals(other) false for foreign type")
	}
}

// TestXYPointInGeometryQuery_Visit covers the two-step
// accept/visitLeaf protocol: the query reports itself as a leaf only
// when the visitor accepts the field.
func TestXYPointInGeometryQuery_Visit(t *testing.T) {
	rect := mustXYRectangle(t, 0, 1, 0, 1)
	q := mustQuery(t, "p", rect)

	// Accepting visitor: VisitLeaf must be invoked exactly once with q.
	accepting := &xyRecordingVisitor{accept: true}
	q.(*xyPointInGeometryQuery).Visit(accepting)
	if accepting.leafCalls != 1 {
		t.Fatalf("accepting visitor: expected 1 leaf call, got %d", accepting.leafCalls)
	}
	if accepting.lastLeaf != q {
		t.Fatalf("accepting visitor: expected leaf == q")
	}

	// Rejecting visitor: VisitLeaf must NOT be invoked.
	rejecting := &xyRecordingVisitor{accept: false}
	q.(*xyPointInGeometryQuery).Visit(rejecting)
	if rejecting.leafCalls != 0 {
		t.Fatalf("rejecting visitor: expected 0 leaf calls, got %d", rejecting.leafCalls)
	}
}

// TestXYPointInGeometryQuery_String covers the toString contract:
// when the rendered field matches the query field the "field=" tag is
// suppressed; otherwise it appears.
func TestXYPointInGeometryQuery_String(t *testing.T) {
	rect := mustXYRectangle(t, 0, 1, 0, 1)
	q := mustQuery(t, "p", rect).(*xyPointInGeometryQuery)

	same := q.StringField("p")
	if !strings.HasPrefix(same, "xyPointInGeometryQuery:[") {
		t.Fatalf("StringField(same): expected prefix 'xyPointInGeometryQuery:[', got %q", same)
	}
	if strings.Contains(same, "field=") {
		t.Fatalf("StringField(same): expected no 'field=' tag, got %q", same)
	}

	diff := q.StringField("other")
	if !strings.Contains(diff, "field=p") {
		t.Fatalf("StringField(other): expected 'field=p' tag, got %q", diff)
	}

	noArg := q.String()
	if !strings.Contains(noArg, "field=p") {
		t.Fatalf("String(): expected 'field=p' tag when empty field passed, got %q", noArg)
	}
}

// TestXYPointInGeometryQuery_RewriteAndClone covers the inherited
// behaviour: Rewrite is a no-op and Clone returns a distinct value
// that still tests equal.
func TestXYPointInGeometryQuery_RewriteAndClone(t *testing.T) {
	rect := mustXYRectangle(t, 0, 1, 0, 1)
	q := mustQuery(t, "p", rect)

	rewritten, err := q.Rewrite(nil)
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}
	if rewritten != q {
		t.Fatalf("Rewrite: expected identity")
	}

	cloned := q.Clone()
	if cloned == q {
		t.Fatalf("Clone: expected a distinct pointer")
	}
	if !q.Equals(cloned) {
		t.Fatalf("Clone: expected equality")
	}
}

// TestXYPointInGeometryQuery_CreateWeight_FieldUnknown verifies the
// Java fast path: when the leaf has no source for the field, the
// scorer supplier returns nil (yielding a null Scorer).
func TestXYPointInGeometryQuery_CreateWeight_FieldUnknown(t *testing.T) {
	rect := mustXYRectangle(t, 0, 1, 0, 1)
	q := mustQuery(t, "p", rect).(*xyPointInGeometryQuery)
	q.installTestLeafLookup(func(*index.LeafReaderContext, string) (xyPointSource, *index.FieldInfo, int, error) {
		return nil, nil, 0, nil // simulates "no docs in this segment had this field"
	})
	w, err := q.CreateWeight(nil, false, 1.0)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	supplier, err := w.ScorerSupplier(nil)
	if err != nil {
		t.Fatalf("ScorerSupplier: %v", err)
	}
	if supplier != nil {
		t.Fatalf("expected nil supplier for unknown field, got %#v", supplier)
	}
}

// TestXYPointInGeometryQuery_Match_Rectangle exercises the end-to-end
// scoring path against an in-memory xyPointSource. The query
// rectangle is (0..10, 0..10); docs are scattered around the plane,
// and only the in-rectangle docs must surface.
func TestXYPointInGeometryQuery_Match_Rectangle(t *testing.T) {
	rect := mustXYRectangle(t, 0, 10, 0, 10)
	q := mustQuery(t, "p", rect).(*xyPointInGeometryQuery)

	// Synthetic per-doc points: include in-rectangle and out-of-rectangle
	// cases plus an iterator-emitted in-rectangle batch.
	points := []xyTestPoint{
		{docID: 0, x: 5, y: 5},   // in
		{docID: 1, x: -1, y: 5},  // out (x<0)
		{docID: 2, x: 5, y: -1},  // out (y<0)
		{docID: 3, x: 11, y: 11}, // out (both>10)
		{docID: 4, x: 0, y: 0},   // in (boundary inclusive: see expected)
		{docID: 5, x: 10, y: 10}, // in (boundary)
	}
	source := &xyInMemoryPointSource{points: points}

	q.installTestLeafLookup(func(*index.LeafReaderContext, string) (xyPointSource, *index.FieldInfo, int, error) {
		fi := newFakeFieldInfo("p", 2, 4)
		// maxDoc=6 covers the test docIDs 0..5 inclusive.
		return source, fi, 6, nil
	})

	w, err := q.CreateWeight(nil, false, 2.5)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	ctx := &index.LeafReaderContext{}
	supplier, err := w.ScorerSupplier(ctx)
	if err != nil {
		t.Fatalf("ScorerSupplier: %v", err)
	}
	if supplier == nil {
		t.Fatalf("expected non-nil supplier")
	}
	scorer, err := supplier.Get(0)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if scorer == nil {
		t.Fatalf("expected non-nil scorer")
	}
	matched := drainDocs(t, scorer)
	wantSet := map[int]bool{0: true, 4: true, 5: true}
	if len(matched) != len(wantSet) {
		t.Fatalf("matched docs %v, want set %v", matched, wantSet)
	}
	for _, d := range matched {
		if !wantSet[d] {
			t.Fatalf("matched unexpected docID=%d (want set=%v)", d, wantSet)
		}
		if got := scorer.Score(); got != 2.5 {
			t.Fatalf("doc %d: score=%v, want 2.5 (boost)", d, got)
		}
	}
}

// TestXYPointInGeometryQuery_FieldShapeMismatch verifies that an
// incompatible FieldInfo (wrong point-dimension count) returns the
// XYPointField-checkCompatible error rather than a partial match set.
func TestXYPointInGeometryQuery_FieldShapeMismatch(t *testing.T) {
	rect := mustXYRectangle(t, 0, 1, 0, 1)
	q := mustQuery(t, "p", rect).(*xyPointInGeometryQuery)

	q.installTestLeafLookup(func(*index.LeafReaderContext, string) (xyPointSource, *index.FieldInfo, int, error) {
		// Indexed as 1-D 8-byte data (looks like a different type),
		// not 2-D 4-byte data (XYPoint shape).
		fi := newFakeFieldInfo("p", 1, 8)
		return &xyInMemoryPointSource{}, fi, 1, nil
	})

	w, err := q.CreateWeight(nil, false, 1.0)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	if _, err := w.ScorerSupplier(&index.LeafReaderContext{}); err == nil {
		t.Fatalf("expected XYPointField compatibility error, got nil")
	}
}

//----- helpers -----------------------------------------------------------

func mustXYRectangle(t *testing.T, minX, maxX, minY, maxY float32) *geo.XYRectangle {
	t.Helper()
	r, err := geo.NewXYRectangle(minX, maxX, minY, maxY)
	if err != nil {
		t.Fatalf("NewXYRectangle: %v", err)
	}
	return &r
}

func mustQuery(t *testing.T, field string, geoms ...geo.XYGeometry) Query {
	t.Helper()
	q, err := NewXYPointInGeometryQuery(field, geoms...)
	if err != nil {
		t.Fatalf("NewXYPointInGeometryQuery: %v", err)
	}
	return q
}

// xyRecordingVisitor counts VisitLeaf invocations and records the
// last leaf query seen. The accept flag toggles AcceptField. The
// defaults for the rest of the QueryVisitor surface come from
// EmptyQueryVisitorBase. Named with an xy prefix to avoid collision
// with the unrelated recordingVisitor in
// long_distance_feature_query_test.go.
type xyRecordingVisitor struct {
	EmptyQueryVisitorBase
	accept    bool
	leafCalls int
	lastLeaf  Query
}

func (v *xyRecordingVisitor) AcceptField(string) bool { return v.accept }
func (v *xyRecordingVisitor) VisitLeaf(q Query)       { v.leafCalls++; v.lastLeaf = q }

// fakeQueryForXYPoint is a no-op Query used to test the Equals
// type-mismatch branch. It only needs to satisfy the Query interface.
type fakeQueryForXYPoint struct{ BaseQuery }

func (f *fakeQueryForXYPoint) Rewrite(IndexReader) (Query, error) { return f, nil }
func (f *fakeQueryForXYPoint) Clone() Query                       { return f }
func (f *fakeQueryForXYPoint) Equals(Query) bool                  { return false }
func (f *fakeQueryForXYPoint) HashCode() int                      { return 0 }
func (f *fakeQueryForXYPoint) CreateWeight(*IndexSearcher, bool, float32) (Weight, error) {
	return nil, nil
}

// xyTestPoint is a tiny (docID, x, y) tuple consumed by the
// in-memory xyPointSource.
type xyTestPoint struct {
	docID int
	x, y  float32
}

// xyInMemoryPointSource is a test xyPointSource that issues per-doc
// VisitWithPackedValue calls for every loaded point. The Compare
// hook always returns "crosses" so the visitor walks every doc; this
// keeps the test focused on the per-doc decode/contains contract
// rather than on cell-pruning behaviour (which is exercised once a
// real BKD-driven source is wired in a future sprint).
type xyInMemoryPointSource struct {
	points []xyTestPoint
}

func (s *xyInMemoryPointSource) Intersect(visitor xyPointVisitor) error {
	visitor.Grow(len(s.points))
	for _, p := range s.points {
		packed, err := document.EncodeXY(p.x, p.y)
		if err != nil {
			return err
		}
		if err := visitor.VisitWithPackedValue(p.docID, packed); err != nil {
			return err
		}
	return nil
}

func (s *xyInMemoryPointSource) EstimateDocCount(_ xyPointVisitor) (int64, error) {
	return int64(len(s.points)), nil
}

// newFakeFieldInfo builds an index.FieldInfo carrying the
// dimension/byte-width shape needed by document.CheckXYPointCompatible.
// We rely on the FieldInfoBuilder path so unexported fields are
// populated correctly.
func newFakeFieldInfo(name string, dims, bytesPerDim int) *index.FieldInfo {
	opts := index.FieldInfoOptions{
		PointDimensionCount: dims,
		PointNumBytes:       bytesPerDim,
	}
	return index.NewFieldInfo(name, 0, opts)
}

// drainDocs walks every matching docID emitted by the scorer.
func drainDocs(t *testing.T, scorer Scorer) []int {
	t.Helper()
	var out []int
	for {
		d, err := scorer.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if d == util.NO_MORE_DOCS {
			break
		}
		out = append(out, d)
	}
	return out
}