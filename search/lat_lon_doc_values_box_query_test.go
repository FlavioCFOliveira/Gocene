// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"errors"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
)

// TestNewLatLonDocValuesBoxQuery_Construction confirms the happy path:
// the constructor accepts a non-empty field and an in-range bounding
// box, and surfaces the public accessors.
func TestNewLatLonDocValuesBoxQuery_Construction(t *testing.T) {
	t.Parallel()
	q, err := NewLatLonDocValuesBoxQuery("loc", -10, 10, -20, 20)
	if err != nil {
		t.Fatalf("NewLatLonDocValuesBoxQuery: %v", err)
	}
	c, ok := q.(*latLonDocValuesBoxQuery)
	if !ok {
		t.Fatalf("expected *latLonDocValuesBoxQuery, got %T", q)
	}
	if got := c.GetField(); got != "loc" {
		t.Fatalf("GetField: got %q, want %q", got, "loc")
	}
	if c.CrossesDateline() {
		t.Fatalf("CrossesDateline: expected false for a non-wrap box")
	}
	minLat, maxLat, minLon, maxLon := c.EncodedBounds()
	if minLat == 0 && maxLat == 0 && minLon == 0 && maxLon == 0 {
		t.Fatalf("EncodedBounds: encoded coordinates collapsed to zero, encoding broken")
	}
	if !(minLat < maxLat) {
		t.Fatalf("EncodedBounds: expected minLat < maxLat, got %d, %d", minLat, maxLat)
	}
	if !(minLon < maxLon) {
		t.Fatalf("EncodedBounds: expected minLon < maxLon, got %d, %d", minLon, maxLon)
	}
}

// TestNewLatLonDocValuesBoxQuery_EmptyField rejects an empty field
// name, mirroring the Java IllegalArgumentException("field must not
// be null").
func TestNewLatLonDocValuesBoxQuery_EmptyField(t *testing.T) {
	t.Parallel()
	_, err := NewLatLonDocValuesBoxQuery("", -10, 10, -20, 20)
	if !errors.Is(err, errLatLonDocValuesBoxQueryNilField) {
		t.Fatalf("empty field: got %v, want errLatLonDocValuesBoxQueryNilField", err)
	}
}

// TestNewLatLonDocValuesBoxQuery_InvalidLatitude rejects bounds
// outside [-90, 90], mirroring the Java GeoUtils.checkLatitude guard.
func TestNewLatLonDocValuesBoxQuery_InvalidLatitude(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name           string
		minLat, maxLat float64
		minLon, maxLon float64
	}{
		{"minLat too low", -100, 10, -20, 20},
		{"maxLat too high", -10, 100, -20, 20},
		{"minLat NaN", nanFloat(), 10, -20, 20},
		{"maxLat NaN", -10, nanFloat(), -20, 20},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewLatLonDocValuesBoxQuery("loc", tc.minLat, tc.maxLat, tc.minLon, tc.maxLon)
			if err == nil {
				t.Fatalf("%s: expected error, got nil", tc.name)
			}
		})
	}
}

// TestNewLatLonDocValuesBoxQuery_InvalidLongitude rejects bounds
// outside [-180, 180], mirroring the Java GeoUtils.checkLongitude
// guard.
func TestNewLatLonDocValuesBoxQuery_InvalidLongitude(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name           string
		minLat, maxLat float64
		minLon, maxLon float64
	}{
		{"minLon too low", -10, 10, -200, 20},
		{"maxLon too high", -10, 10, -20, 200},
		{"minLon NaN", -10, 10, nanFloat(), 20},
		{"maxLon NaN", -10, 10, -20, nanFloat()},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewLatLonDocValuesBoxQuery("loc", tc.minLat, tc.maxLat, tc.minLon, tc.maxLon)
			if err == nil {
				t.Fatalf("%s: expected error, got nil", tc.name)
			}
		})
	}
}

// TestNewLatLonDocValuesBoxQuery_DatelineDetection asserts that a box
// with minLon > maxLon is recognised as a dateline crosser; the flag
// is captured PRE-rounding, so even after encoding the boolean
// remains true.
func TestNewLatLonDocValuesBoxQuery_DatelineDetection(t *testing.T) {
	t.Parallel()
	q, err := NewLatLonDocValuesBoxQuery("loc", -10, 10, 170, -170)
	if err != nil {
		t.Fatalf("NewLatLonDocValuesBoxQuery (dateline): %v", err)
	}
	c := q.(*latLonDocValuesBoxQuery)
	if !c.CrossesDateline() {
		t.Fatalf("CrossesDateline: expected true for minLon > maxLon")
	}
}

// TestLatLonDocValuesBoxQuery_Equals_Reflexive confirms that two
// queries built with identical inputs compare equal and share a hash.
func TestLatLonDocValuesBoxQuery_Equals_Reflexive(t *testing.T) {
	t.Parallel()
	a, _ := NewLatLonDocValuesBoxQuery("loc", -10, 10, -20, 20)
	b, _ := NewLatLonDocValuesBoxQuery("loc", -10, 10, -20, 20)
	if !a.Equals(b) {
		t.Fatalf("Equals: expected true for identical queries")
	}
	if a.HashCode() != b.HashCode() {
		t.Fatalf("HashCode: expected match for equal queries (%d vs %d)",
			a.HashCode(), b.HashCode())
	}
}

// TestLatLonDocValuesBoxQuery_Equals_FieldDifference asserts that a
// distinct field produces a distinct query.
func TestLatLonDocValuesBoxQuery_Equals_FieldDifference(t *testing.T) {
	t.Parallel()
	a, _ := NewLatLonDocValuesBoxQuery("loc", -10, 10, -20, 20)
	b, _ := NewLatLonDocValuesBoxQuery("other", -10, 10, -20, 20)
	if a.Equals(b) {
		t.Fatalf("Equals: distinct fields must not compare equal")
	}
}

// TestLatLonDocValuesBoxQuery_Equals_BoundsDifference asserts that
// distinct bounds produce distinct queries.
func TestLatLonDocValuesBoxQuery_Equals_BoundsDifference(t *testing.T) {
	t.Parallel()
	a, _ := NewLatLonDocValuesBoxQuery("loc", -10, 10, -20, 20)
	b, _ := NewLatLonDocValuesBoxQuery("loc", -10, 11, -20, 20)
	if a.Equals(b) {
		t.Fatalf("Equals: distinct bounds must not compare equal")
	}
}

// TestLatLonDocValuesBoxQuery_Equals_DatelineDifference asserts that
// the crossesDateline flag participates in equality.
func TestLatLonDocValuesBoxQuery_Equals_DatelineDifference(t *testing.T) {
	t.Parallel()
	a, _ := NewLatLonDocValuesBoxQuery("loc", -10, 10, -20, 20)   // no wrap
	b, _ := NewLatLonDocValuesBoxQuery("loc", -10, 10, 170, -170) // wraps
	if a.Equals(b) {
		t.Fatalf("Equals: dateline-crossing flag must participate in equality")
	}
	if a.HashCode() == b.HashCode() {
		t.Fatalf("HashCode: expected distinct hashes for distinct dateline flags")
	}
}

// TestLatLonDocValuesBoxQuery_Equals_RejectsForeign confirms that a
// non-latLonDocValuesBoxQuery never compares equal.
func TestLatLonDocValuesBoxQuery_Equals_RejectsForeign(t *testing.T) {
	t.Parallel()
	q, _ := NewLatLonDocValuesBoxQuery("loc", -10, 10, -20, 20)
	if q.Equals(&BaseQuery{}) {
		t.Fatalf("Equals: foreign Query must not compare equal")
	}
}

// TestLatLonDocValuesBoxQuery_String formats the query with and
// without the default-field prefix and checks the literal format
// fragments mirror the Java reference's StringBuilder body.
func TestLatLonDocValuesBoxQuery_String(t *testing.T) {
	t.Parallel()
	q, _ := NewLatLonDocValuesBoxQuery("loc", -10, 10, -20, 20)
	c := q.(*latLonDocValuesBoxQuery)

	withPrefix := c.String("other")
	if !strings.HasPrefix(withPrefix, "loc:") {
		t.Fatalf("String(other): expected 'loc:' prefix, got %q", withPrefix)
	}
	if !strings.Contains(withPrefix, "box(") {
		t.Fatalf("String(other): expected 'box(' segment, got %q", withPrefix)
	}

	withoutPrefix := c.String("loc")
	if strings.HasPrefix(withoutPrefix, "loc:") {
		t.Fatalf("String(loc): unexpected field prefix in %q", withoutPrefix)
	}
	for _, want := range []string{"minLat=", "maxLat=", "minLon=", "maxLon="} {
		if !strings.Contains(withoutPrefix, want) {
			t.Fatalf("String: expected %q in output, got %q", want, withoutPrefix)
		}
	}
}

// TestLatLonDocValuesBoxQuery_Visit asserts that Visit descends into
// the leaf only when AcceptField returns true.
func TestLatLonDocValuesBoxQuery_Visit(t *testing.T) {
	t.Parallel()
	q, _ := NewLatLonDocValuesBoxQuery("loc", -10, 10, -20, 20)

	acceptVisitor := &latLonDocValuesBoxQueryVisitor{accept: true}
	q.(*latLonDocValuesBoxQuery).Visit(acceptVisitor)
	if !acceptVisitor.leafCalled {
		t.Fatalf("accept=true: expected VisitLeaf to be called")
	}

	rejectVisitor := &latLonDocValuesBoxQueryVisitor{accept: false}
	q.(*latLonDocValuesBoxQuery).Visit(rejectVisitor)
	if rejectVisitor.leafCalled {
		t.Fatalf("accept=false: expected VisitLeaf NOT to be called")
	}
}

// latLonDocValuesBoxQueryVisitor is a minimal QueryVisitor that
// records whether VisitLeaf was invoked. Embeds EmptyQueryVisitorBase
// to satisfy ConsumeTerms / ConsumeTermsMatching / GetSubVisitor.
type latLonDocValuesBoxQueryVisitor struct {
	EmptyQueryVisitorBase
	accept     bool
	leafCalled bool
}

func (v *latLonDocValuesBoxQueryVisitor) AcceptField(_ string) bool { return v.accept }
func (v *latLonDocValuesBoxQueryVisitor) VisitLeaf(_ Query)         { v.leafCalled = true }

// TestLatLonDocValuesBoxQuery_Clone asserts the documented identity
// behaviour: Clone returns the same logical query (the struct is
// immutable so a shallow clone preserves equality).
func TestLatLonDocValuesBoxQuery_Clone(t *testing.T) {
	t.Parallel()
	q, _ := NewLatLonDocValuesBoxQuery("loc", -10, 10, -20, 20)
	c := q.(*latLonDocValuesBoxQuery)
	if cloned := c.Clone(); !cloned.Equals(q) {
		t.Fatalf("Clone: expected equal clone, got distinct query")
	}
}

// TestBoxMatches_NoDateline covers the non-wrap matching path: a doc
// matches when at least one value falls inside [minLat..maxLat] x
// [minLon..maxLon].
func TestBoxMatches_NoDateline(t *testing.T) {
	t.Parallel()
	q, err := NewLatLonDocValuesBoxQuery("loc", -10, 10, -20, 20)
	if err != nil {
		t.Fatalf("NewLatLonDocValuesBoxQuery: %v", err)
	}
	c := q.(*latLonDocValuesBoxQuery)

	values := map[int][]int64{
		0: {document.EncodeLatLonAsLong(0, 0)},                                      // inside
		1: {document.EncodeLatLonAsLong(50, 50)},                                    // outside
		2: {document.EncodeLatLonAsLong(50, 50), document.EncodeLatLonAsLong(5, 5)}, // mixed: matches
		3: {document.EncodeLatLonAsLong(80, 80)},                                    // outside (far)
		4: {document.EncodeLatLonAsLong(-9.999, -19.999)},                           // inside edge
		5: {document.EncodeLatLonAsLong(0, 50)},                                     // lat inside, lon outside
		6: {document.EncodeLatLonAsLong(50, 0)},                                     // lat outside, lon inside
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
		{5, false},
		{6, false},
	}
	for _, tc := range cases {
		got, err := boxMatches(
			fake, tc.docID,
			c.minLatitude, c.maxLatitude,
			c.minLongitude, c.maxLongitude,
			c.crossesDateline,
		)
		if err != nil {
			t.Fatalf("boxMatches(doc=%d): %v", tc.docID, err)
		}
		if got != tc.want {
			t.Errorf("boxMatches(doc=%d): got %v, want %v", tc.docID, got, tc.want)
		}
	}
}

// TestBoxMatches_Dateline covers the wrap-around matching path: a
// doc matches when at least one value falls inside [minLat..maxLat]
// AND outside (maxLon, minLon) — i.e. on either side of the
// dateline.
func TestBoxMatches_Dateline(t *testing.T) {
	t.Parallel()
	// Box that wraps: latitudes [-10..10], longitudes 170..-170 (the
	// 20-degree slice straddling the antimeridian).
	q, err := NewLatLonDocValuesBoxQuery("loc", -10, 10, 170, -170)
	if err != nil {
		t.Fatalf("NewLatLonDocValuesBoxQuery (dateline): %v", err)
	}
	c := q.(*latLonDocValuesBoxQuery)
	if !c.crossesDateline {
		t.Fatalf("test precondition failed: expected crossesDateline=true")
	}

	values := map[int][]int64{
		0: {document.EncodeLatLonAsLong(0, 175)},  // inside east of meridian
		1: {document.EncodeLatLonAsLong(0, -175)}, // inside west of meridian
		2: {document.EncodeLatLonAsLong(0, 0)},    // squarely outside (in the gap)
		3: {document.EncodeLatLonAsLong(0, 100)},  // east-but-not-in-box
		4: {document.EncodeLatLonAsLong(50, 175)}, // lat out of band
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
		{4, false},
	}
	for _, tc := range cases {
		got, err := boxMatches(
			fake, tc.docID,
			c.minLatitude, c.maxLatitude,
			c.minLongitude, c.maxLongitude,
			c.crossesDateline,
		)
		if err != nil {
			t.Fatalf("boxMatches(doc=%d): %v", tc.docID, err)
		}
		if got != tc.want {
			t.Errorf("boxMatches(doc=%d): got %v, want %v", tc.docID, got, tc.want)
		}
	}
}

// TestBoxMatches_EmptyValues asserts that a doc holding no values
// cannot match. The Java reference's nextValue loop runs zero times
// and falls through to "return false".
func TestBoxMatches_EmptyValues(t *testing.T) {
	t.Parallel()
	q, _ := NewLatLonDocValuesBoxQuery("loc", -10, 10, -20, 20)
	c := q.(*latLonDocValuesBoxQuery)
	fake := newFakeSortedNumeric(map[int][]int64{0: {}})
	got, err := boxMatches(
		fake, 0,
		c.minLatitude, c.maxLatitude,
		c.minLongitude, c.maxLongitude,
		c.crossesDateline,
	)
	if err != nil {
		t.Fatalf("boxMatches(empty): %v", err)
	}
	if got {
		t.Fatalf("boxMatches(empty): expected false, got true")
	}
}

// TestLatLonDocValuesBoxQuery_CreateWeight_CacheableHook covers the
// IsDocValuesCacheable fallback (no FieldInfos reachable) — the
// weight must report cacheable=true when the context has no reader.
func TestLatLonDocValuesBoxQuery_CreateWeight_CacheableHook(t *testing.T) {
	t.Parallel()
	q, _ := NewLatLonDocValuesBoxQuery("loc", -10, 10, -20, 20)
	w, err := q.CreateWeight(nil, false, 1.0)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	if !w.IsCacheable(nil) {
		t.Fatalf("IsCacheable(nil ctx): expected true")
	}
}

// TestLatLonDocValuesBoxQuery_CreateWeight_NilField guards against
// passing a nil LeafReaderContext through the supplier: it returns
// nil (no scorer) rather than panicking.
func TestLatLonDocValuesBoxQuery_CreateWeight_NilField(t *testing.T) {
	t.Parallel()
	q, _ := NewLatLonDocValuesBoxQuery("loc", -10, 10, -20, 20)
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

// nanFloat returns IEEE-754 NaN without importing math just for it
// at the top of the file; helps keep table-driven cases declarative.
func nanFloat() float64 {
	var zero float64
	return zero / zero
}
