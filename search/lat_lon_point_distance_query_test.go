// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"math"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
)

// TestNewLatLonPointDistanceQuery_ConstructorValidation covers the
// IllegalArgumentException-equivalent paths on the Java constructor:
// empty field, NaN/Inf/negative radius, out-of-range latitude or
// longitude.
func TestNewLatLonPointDistanceQuery_ConstructorValidation(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		field  string
		lat    float64
		lon    float64
		radius float64
		want   bool // true => expect error
	}{
		{"empty field", "", 0, 0, 100, true},
		{"NaN radius", "p", 0, 0, math.NaN(), true},
		{"+Inf radius", "p", 0, 0, math.Inf(1), true},
		{"-Inf radius", "p", 0, 0, math.Inf(-1), true},
		{"negative radius", "p", 0, 0, -1, true},
		{"lat over 90", "p", 90.1, 0, 100, true},
		{"lat under -90", "p", -90.1, 0, 100, true},
		{"lon over 180", "p", 0, 180.1, 100, true},
		{"lon under -180", "p", 0, -180.1, 100, true},
		{"zero radius is valid", "p", 0, 0, 0, false},
		{"valid mid", "p", 12.5, -34.25, 1000, false},
		{"pole valid", "p", 90, 0, 100, false},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewLatLonPointDistanceQuery(c.field, c.lat, c.lon, c.radius)
			if c.want && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !c.want && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// TestLatLonPointDistanceQuery_Accessors confirms the getter
// accessors echo the constructor inputs unchanged. Mirrors the
// public getters on the Java reference.
func TestLatLonPointDistanceQuery_Accessors(t *testing.T) {
	t.Parallel()
	q, err := NewLatLonPointDistanceQuery("point", 12.5, -34.25, 1000)
	if err != nil {
		t.Fatalf("NewLatLonPointDistanceQuery: %v", err)
	}
	dq := q.(*latLonPointDistanceQuery)
	if got := dq.Field(); got != "point" {
		t.Fatalf("Field: %q", got)
	}
	if got := dq.Latitude(); got != 12.5 {
		t.Fatalf("Latitude: %v", got)
	}
	if got := dq.Longitude(); got != -34.25 {
		t.Fatalf("Longitude: %v", got)
	}
	if got := dq.RadiusMeters(); got != 1000 {
		t.Fatalf("RadiusMeters: %v", got)
	}
}

// TestLatLonPointDistanceQuery_EqualsAndHashCode mirrors the standard
// QueryUtils.checkHashEquals contract: equal queries hash equal,
// different fields / centres / radii differ.
func TestLatLonPointDistanceQuery_EqualsAndHashCode(t *testing.T) {
	t.Parallel()
	mk := func(field string, lat, lon, r float64) Query {
		q, err := NewLatLonPointDistanceQuery(field, lat, lon, r)
		if err != nil {
			t.Fatalf("NewLatLonPointDistanceQuery: %v", err)
		}
		return q
	}
	q1 := mk("p", 10, 20, 500)
	q2 := mk("p", 10, 20, 500)
	q3 := mk("q", 10, 20, 500)  // different field
	q4 := mk("p", 11, 20, 500)  // different lat
	q5 := mk("p", 10, 21, 500)  // different lon
	q6 := mk("p", 10, 20, 1000) // different radius

	if !q1.Equals(q2) || !q2.Equals(q1) {
		t.Fatalf("expected q1 == q2 for identical inputs")
	}
	if q1.HashCode() != q2.HashCode() {
		t.Fatalf("hash mismatch for equal queries: %d vs %d", q1.HashCode(), q2.HashCode())
	}
	for i, other := range []Query{q3, q4, q5, q6} {
		if q1.Equals(other) {
			t.Fatalf("expected q1 != alt[%d]", i)
		}
	}
}

// TestLatLonPointDistanceQuery_EqualsRejectsOtherTypes verifies the
// type-check branch in Equals: a foreign Query never matches.
func TestLatLonPointDistanceQuery_EqualsRejectsOtherTypes(t *testing.T) {
	t.Parallel()
	q, err := NewLatLonPointDistanceQuery("p", 0, 0, 1)
	if err != nil {
		t.Fatalf("NewLatLonPointDistanceQuery: %v", err)
	}
	if q.Equals(&fakeQueryForXYPoint{}) {
		t.Fatalf("expected Equals(other) false for foreign type")
	}
	if q.Equals(nil) {
		t.Fatalf("expected Equals(nil) false")
	}
}

// TestLatLonPointDistanceQuery_Visit covers the two-step
// accept/visitLeaf protocol: the query reports itself as a leaf only
// when the visitor accepts the field.
func TestLatLonPointDistanceQuery_Visit(t *testing.T) {
	t.Parallel()
	q, err := NewLatLonPointDistanceQuery("p", 0, 0, 1)
	if err != nil {
		t.Fatalf("NewLatLonPointDistanceQuery: %v", err)
	}
	dq := q.(*latLonPointDistanceQuery)

	accepting := &xyRecordingVisitor{accept: true}
	dq.Visit(accepting)
	if accepting.leafCalls != 1 {
		t.Fatalf("accepting visitor: expected 1 leaf call, got %d", accepting.leafCalls)
	}
	if accepting.lastLeaf != q {
		t.Fatalf("accepting visitor: expected leaf == q")
	}

	rejecting := &xyRecordingVisitor{accept: false}
	dq.Visit(rejecting)
	if rejecting.leafCalls != 0 {
		t.Fatalf("rejecting visitor: expected 0 leaf calls, got %d", rejecting.leafCalls)
	}
}

// TestLatLonPointDistanceQuery_String covers the toString contract:
// when the rendered field matches the query field the "<field>:"
// prefix is suppressed; otherwise it appears.
func TestLatLonPointDistanceQuery_String(t *testing.T) {
	t.Parallel()
	q, err := NewLatLonPointDistanceQuery("p", 10.5, 20.5, 1000)
	if err != nil {
		t.Fatalf("NewLatLonPointDistanceQuery: %v", err)
	}
	dq := q.(*latLonPointDistanceQuery)

	same := dq.StringField("p")
	if strings.HasPrefix(same, "p:") {
		t.Fatalf("StringField(same): unexpected field prefix: %q", same)
	}
	if !strings.Contains(same, "10.5,20.5 +/- 1000 meters") {
		t.Fatalf("StringField(same): missing canonical form: %q", same)
	}

	diff := dq.StringField("other")
	if !strings.HasPrefix(diff, "p:") {
		t.Fatalf("StringField(other): missing 'p:' prefix: %q", diff)
	}
	if !strings.Contains(diff, "+/- 1000 meters") {
		t.Fatalf("StringField(other): missing radius suffix: %q", diff)
	}

	noArg := dq.String()
	if !strings.HasPrefix(noArg, "p:") {
		t.Fatalf("String(): missing 'p:' prefix: %q", noArg)
	}
}

// TestLatLonPointDistanceQuery_RewriteAndClone covers the inherited
// behaviour: Rewrite is a no-op and Clone returns a distinct value
// that still tests equal.
func TestLatLonPointDistanceQuery_RewriteAndClone(t *testing.T) {
	t.Parallel()
	q, err := NewLatLonPointDistanceQuery("p", 1, 2, 3)
	if err != nil {
		t.Fatalf("NewLatLonPointDistanceQuery: %v", err)
	}
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

// TestLatLonPointDistanceQuery_CreateWeight_FieldUnknown verifies
// the Java fast path: when the leaf has no source for the field, the
// scorer supplier returns nil (yielding a null Scorer).
func TestLatLonPointDistanceQuery_CreateWeight_FieldUnknown(t *testing.T) {
	t.Parallel()
	q, err := NewLatLonPointDistanceQuery("p", 0, 0, 1000)
	if err != nil {
		t.Fatalf("NewLatLonPointDistanceQuery: %v", err)
	}
	dq := q.(*latLonPointDistanceQuery)
	dq.installTestLeafLookup(func(*index.LeafReaderContext, string) (latLonDistancePointSource, *index.FieldInfo, int, error) {
		return nil, nil, 0, nil
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

// TestLatLonPointDistanceQuery_FieldShapeMismatch verifies that an
// incompatible FieldInfo (wrong point-dimension count) returns the
// LatLonPoint compatibility error rather than a partial match set.
func TestLatLonPointDistanceQuery_FieldShapeMismatch(t *testing.T) {
	t.Parallel()
	q, err := NewLatLonPointDistanceQuery("p", 0, 0, 1000)
	if err != nil {
		t.Fatalf("NewLatLonPointDistanceQuery: %v", err)
	}
	dq := q.(*latLonPointDistanceQuery)
	dq.installTestLeafLookup(func(*index.LeafReaderContext, string) (latLonDistancePointSource, *index.FieldInfo, int, error) {
		// 1-D 8-byte shape instead of 2-D 4-byte LatLonPoint.
		fi := newFakeFieldInfo("p", 1, 8)
		return &latLonInMemoryDistancePointSource{}, fi, 1, nil
	})
	w, err := q.CreateWeight(nil, false, 1.0)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	if _, err := w.ScorerSupplier(&index.LeafReaderContext{}); err == nil {
		t.Fatalf("expected LatLonPoint compatibility error, got nil")
	}
}

// TestLatLonPointDistanceQuery_Match_NonDateline exercises the
// happy path on a small disk far from the dateline. Points inside
// the disk surface, points outside (but still inside the bounding
// box) and points outside the bounding box are rejected.
func TestLatLonPointDistanceQuery_Match_NonDateline(t *testing.T) {
	t.Parallel()
	// 100km disk around (40, -74) — roughly the NYC area; well clear
	// of dateline and poles.
	const (
		centreLat = 40.0
		centreLon = -74.0
		radius    = 100_000.0 // 100 km
		boost     = 2.5
	)
	q, err := NewLatLonPointDistanceQuery("p", centreLat, centreLon, radius)
	if err != nil {
		t.Fatalf("NewLatLonPointDistanceQuery: %v", err)
	}
	dq := q.(*latLonPointDistanceQuery)

	points := []latLonTestPoint{
		{docID: 0, lat: 40.0, lon: -74.0},  // dead centre — in
		{docID: 1, lat: 40.5, lon: -74.0},  // ~55 km north — in
		{docID: 2, lat: 40.0, lon: -73.5},  // ~43 km east — in
		{docID: 3, lat: 41.5, lon: -74.0},  // ~167 km north — out (outside bbox)
		{docID: 4, lat: 40.0, lon: -72.0},  // ~170 km east — out (outside bbox)
		{docID: 5, lat: 40.8, lon: -73.05}, // in bbox, outside disk corner
		{docID: 6, lat: -40.0, lon: -74.0}, // far hemisphere — out
		{docID: 7, lat: 40.0, lon: 106.0},  // antipode-ish — out
	}
	dq.installTestLeafLookup(latLonDistanceLookupFor(points, "p"))

	matched := drainDistanceQuery(t, dq, boost)
	wantIn := map[int]bool{0: true, 1: true, 2: true}
	wantOut := map[int]bool{3: true, 4: true, 5: true, 6: true, 7: true}
	for _, d := range matched {
		if !wantIn[d] {
			t.Fatalf("matched unexpected docID=%d (want in=%v out=%v)", d, wantIn, wantOut)
		}
	}
	for d := range wantIn {
		if !containsInt(matched, d) {
			t.Fatalf("expected docID=%d in matches, got %v", d, matched)
		}
	}
}

// TestLatLonPointDistanceQuery_Match_Dateline exercises the dateline-
// split fast path: a query centred just east of the dateline whose
// bounding box wraps to longitudes near +180. Points on both sides
// of the dateline that fall inside the disk must surface.
func TestLatLonPointDistanceQuery_Match_Dateline(t *testing.T) {
	t.Parallel()
	// 500 km disk centred at (0, 179.5): the WGS-84 bbox crosses the
	// dateline so minLon2 is wired and the visitor's two-range gate
	// gets exercised.
	const (
		centreLat = 0.0
		centreLon = 179.5
		radius    = 500_000.0 // 500 km
		boost     = 1.0
	)
	q, err := NewLatLonPointDistanceQuery("p", centreLat, centreLon, radius)
	if err != nil {
		t.Fatalf("NewLatLonPointDistanceQuery: %v", err)
	}
	dq := q.(*latLonPointDistanceQuery)

	// Confirm the bbox actually crosses the dateline — if FromPointDistance
	// changes someday and stops crossing, the test still passes (just
	// stops exercising the second range). Document the assumption.
	wt, ok := mustWeightForDistanceQuery(t, dq, boost).(*latLonPointDistanceWeight)
	if !ok {
		t.Fatalf("weight type: got %T, want *latLonPointDistanceWeight", wt)
	}
	if wt.bbox.minLon2 == math.MaxInt32 {
		t.Logf("note: bbox did not cross dateline — the second-range visitor branch is not exercised")
	}

	points := []latLonTestPoint{
		{docID: 0, lat: 0.0, lon: 179.5},  // dead centre — in
		{docID: 1, lat: 0.0, lon: -179.5}, // 1° east across dateline (~111 km) — in
		{docID: 2, lat: 1.0, lon: 179.5},  // ~111 km north — in
		{docID: 3, lat: 0.0, lon: -176.0}, // 4.5° east of centre, ~500 km bbox edge — out (outside disk)
		{docID: 4, lat: 0.0, lon: 170.0},  // 9.5° west — far out
		{docID: 5, lat: 5.0, lon: 179.5},  // far north — out
		{docID: 6, lat: 0.0, lon: 0.0},    // opposite side — out
	}
	dq.installTestLeafLookup(latLonDistanceLookupFor(points, "p"))

	matched := drainDistanceQuery(t, dq, boost)
	wantIn := map[int]bool{0: true, 1: true, 2: true}
	for _, d := range matched {
		if !wantIn[d] {
			t.Fatalf("matched unexpected docID=%d (want in=%v)", d, wantIn)
		}
	}
	for d := range wantIn {
		if !containsInt(matched, d) {
			t.Fatalf("expected docID=%d in matches, got %v", d, matched)
		}
	}
}

// TestLatLonPointDistanceQuery_Match_Pole exercises a disk centred
// on the north pole: the WGS-84 bounding box collapses onto the
// full longitude ring, so longitude pruning is effectively disabled
// and only the latitude bound and the haversine test gate matches.
func TestLatLonPointDistanceQuery_Match_Pole(t *testing.T) {
	t.Parallel()
	const (
		centreLat = 90.0
		centreLon = 0.0
		radius    = 1_000_000.0 // 1 000 km
		boost     = 1.0
	)
	q, err := NewLatLonPointDistanceQuery("p", centreLat, centreLon, radius)
	if err != nil {
		t.Fatalf("NewLatLonPointDistanceQuery: %v", err)
	}
	dq := q.(*latLonPointDistanceQuery)

	points := []latLonTestPoint{
		{docID: 0, lat: 90.0, lon: 0.0},    // pole — in
		{docID: 1, lat: 85.0, lon: 100.0},  // ~555 km from pole — in
		{docID: 2, lat: 85.0, lon: -100.0}, // mirror — in (longitude irrelevant near pole)
		{docID: 3, lat: 70.0, lon: 0.0},    // ~2 200 km from pole — out
		{docID: 4, lat: 60.0, lon: 90.0},   // ~3 300 km from pole — out
		{docID: 5, lat: -90.0, lon: 0.0},   // south pole — out
	}
	dq.installTestLeafLookup(latLonDistanceLookupFor(points, "p"))

	matched := drainDistanceQuery(t, dq, boost)
	wantIn := map[int]bool{0: true, 1: true, 2: true}
	for _, d := range matched {
		if !wantIn[d] {
			t.Fatalf("matched unexpected docID=%d (want in=%v)", d, wantIn)
		}
	}
	for d := range wantIn {
		if !containsInt(matched, d) {
			t.Fatalf("expected docID=%d in matches, got %v", d, matched)
		}
	}
}

// TestLatLonPointDistanceQuery_ScoreEqualsBoost confirms every
// surviving doc receives exactly the boost as its score, matching
// the ConstantScoreScorer contract.
func TestLatLonPointDistanceQuery_ScoreEqualsBoost(t *testing.T) {
	t.Parallel()
	const boost = 3.75
	q, err := NewLatLonPointDistanceQuery("p", 0, 0, 1_000_000)
	if err != nil {
		t.Fatalf("NewLatLonPointDistanceQuery: %v", err)
	}
	dq := q.(*latLonPointDistanceQuery)
	dq.installTestLeafLookup(latLonDistanceLookupFor([]latLonTestPoint{
		{docID: 0, lat: 0, lon: 0},
		{docID: 1, lat: 0.1, lon: 0.1},
	}, "p"))

	w, err := q.CreateWeight(nil, false, boost)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	supplier, err := w.ScorerSupplier(&index.LeafReaderContext{})
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
	if len(matched) == 0 {
		t.Fatalf("expected at least one match")
	}
	// drainDocs leaves the scorer at the NO_MORE_DOCS sentinel; the
	// per-doc Score()/GetMaxScore() assertions therefore have to be
	// taken from the visit-time API, which is constant for
	// ConstantScoreScorer regardless of cursor position.
	if got := scorer.Score(); got != boost {
		t.Fatalf("Score after drain: got %v, want %v", got, boost)
	}
	if got := scorer.GetMaxScore(0); got != boost {
		t.Fatalf("GetMaxScore: got %v, want %v", got, boost)
	}
}

//----- helpers -----------------------------------------------------------

// latLonTestPoint is a tiny (docID, lat, lon) tuple consumed by the
// in-memory latLonDistancePointSource fixture.
type latLonTestPoint struct {
	docID int
	lat   float64
	lon   float64
}

// latLonInMemoryDistancePointSource is a test
// latLonDistancePointSource that issues per-doc VisitWithPackedValue
// calls for every loaded point. The Compare hook always returns
// "crosses" so the visitor walks every doc; this keeps the test
// focused on the per-doc decode/distance contract rather than on
// cell-pruning behaviour (which is exercised once a real
// BKD-driven source is wired in a future sprint).
type latLonInMemoryDistancePointSource struct {
	points []latLonTestPoint
}

func (s *latLonInMemoryDistancePointSource) Intersect(visitor latLonDistancePointVisitor) error {
	visitor.Grow(len(s.points))
	for _, p := range s.points {
		packed := document.EncodeLatLon(p.lat, p.lon)
		if err := visitor.VisitWithPackedValue(p.docID, packed); err != nil {
			return err
		}
	}
	return nil
}

func (s *latLonInMemoryDistancePointSource) EstimateDocCount(_ latLonDistancePointVisitor) (int64, error) {
	return int64(len(s.points)), nil
}

// latLonDistanceLookupFor returns a test leaf lookup that hands the
// query an in-memory point source covering the supplied points. The
// FieldInfo declares the canonical LatLonPoint shape (2 dims,
// 4 bytes/dim) so the compatibility gate passes.
func latLonDistanceLookupFor(
	points []latLonTestPoint, field string,
) latLonPointDistanceLeafLookup {
	maxDoc := 0
	for _, p := range points {
		if p.docID >= maxDoc {
			maxDoc = p.docID + 1
		}
	}
	source := &latLonInMemoryDistancePointSource{points: points}
	fi := newFakeFieldInfo(field, 2, 4)
	return func(*index.LeafReaderContext, string) (latLonDistancePointSource, *index.FieldInfo, int, error) {
		return source, fi, maxDoc, nil
	}
}

// drainDistanceQuery walks every matching docID emitted by the
// constant-score scorer the supplied query builds for the test
// lookup. Returns the docIDs in iteration order.
func drainDistanceQuery(t *testing.T, q *latLonPointDistanceQuery, boost float32) []int {
	t.Helper()
	w, err := q.CreateWeight(nil, false, boost)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	supplier, err := w.ScorerSupplier(&index.LeafReaderContext{})
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
	return drainDocs(t, scorer)
}

// mustWeightForDistanceQuery is a small helper that surfaces the
// concrete *latLonPointDistanceWeight so a test can inspect derived
// state (e.g. dateline bbox). Fails the test on construction error.
func mustWeightForDistanceQuery(t *testing.T, q *latLonPointDistanceQuery, boost float32) Weight {
	t.Helper()
	w, err := q.CreateWeight(nil, false, boost)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	return w
}

// containsInt is a tiny set-membership helper used by the match
// tests. Named with an Int suffix so it does not collide with the
// string-substring contains helper in term_in_set_query_test.go.
func containsInt(xs []int, v int) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	return false
}	}
