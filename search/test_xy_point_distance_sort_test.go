// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestXYPointDistanceSort.java
//
// Simple tests for XYDocValuesField#newDistanceSort. The Java suite has
// four methods: testDistanceSort and testMissingLast are deterministic;
// testRandom / testRandomHuge brute-force a random comparison (the latter
// is @Nightly). This port reproduces the two deterministic methods exactly
// (same fixed coordinates, same expected Cartesian distances and ordering)
// driven end to end through IndexWriter + IndexSearcher, plus a scaled-down
// deterministic random sweep.

package search_test

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// cartesianDistance mirrors the private helper in the Java suite.
func cartesianDistance(x1, y1, x2, y2 float64) float64 {
	diffX := x1 - x2
	diffY := y1 - y2
	return math.Sqrt(diffX*diffX + diffY*diffY)
}

// quantizeXY rounds a coordinate through the XYDocValuesField encoding so
// the brute-force expected distance is computed against the same
// (quantized) value the comparator decodes from the index. In Lucene the
// float→sortableInt encoding is lossless for these literals so the Java
// test asserts a 0.0 delta on the raw inputs; quantizeXY makes the Go port
// equally exact regardless of the float32→sortableInt rounding.
func quantizeXY(v float32) float64 {
	return float64(geo.XYDecode(geo.XYEncode(v)))
}

// xyDistanceSortOrigin is the fixed query origin shared by the two
// deterministic tests (Java newDistanceSort("location", 40.7143528,
// -74.0059731)).
const (
	xyDistanceSortOriginX = float32(40.7143528)
	xyDistanceSortOriginY = float32(-74.0059731)
)

// addXYDoc adds one document carrying a single XYDocValuesField("location"),
// or a value-less document when add is false (the "missing" case).
func addXYDoc(t *testing.T, ix *integrationIndex, x, y float32, add bool) {
	t.Helper()
	doc := document.NewDocument()
	if add {
		f, err := document.NewXYDocValuesField("location", x, y)
		if err != nil {
			t.Fatalf("NewXYDocValuesField(%v,%v): %v", x, y, err)
		}
		doc.Add(f.Field)
	}
	ix.addDoc(doc)
}

// TestXYPointDistanceSort_DistanceSort is the direct port of
// testDistanceSort (Java lines 42-77): three points sorted by Cartesian
// distance to the origin, asserting the exact distances and order
// (d2 < d3 < d1, i.e. doc1, doc2, doc0).
func TestXYPointDistanceSort_DistanceSort(t *testing.T) {
	ix := newIntegrationIndex(t)
	addXYDoc(t, ix, 40.759011, -73.9844722, true)
	addXYDoc(t, ix, 40.718266, -74.007819, true)
	addXYDoc(t, ix, 40.7051157, -74.0088305, true)

	d1 := cartesianDistance(quantizeXY(40.759011), quantizeXY(-73.9844722), float64(xyDistanceSortOriginX), float64(xyDistanceSortOriginY))
	d2 := cartesianDistance(quantizeXY(40.718266), quantizeXY(-74.007819), float64(xyDistanceSortOriginX), float64(xyDistanceSortOriginY))
	d3 := cartesianDistance(quantizeXY(40.7051157), quantizeXY(-74.0088305), float64(xyDistanceSortOriginX), float64(xyDistanceSortOriginY))

	s, cleanup := ix.searcher()
	defer cleanup()

	sf, err := search.NewXYDocValuesDistanceSort("location", xyDistanceSortOriginX, xyDistanceSortOriginY)
	if err != nil {
		t.Fatalf("NewXYDocValuesDistanceSort: %v", err)
	}
	td, err := s.SearchWithSort(search.NewMatchAllDocsQuery(), 3, search.NewSort(sf))
	if err != nil {
		t.Fatalf("SearchWithSort: %v", err)
	}
	if len(td.FieldDocs) != 3 {
		t.Fatalf("want 3 hits, got %d", len(td.FieldDocs))
	}
	assertDistance(t, "hit0", td.FieldDocs[0], d2)
	assertDistance(t, "hit1", td.FieldDocs[1], d3)
	assertDistance(t, "hit2", td.FieldDocs[2], d1)
}

// TestXYPointDistanceSort_MissingLast is the direct port of testMissingLast
// (Java lines 80-118): one missing-valued document plus two valued ones,
// asserting the missing document sorts last with +Inf.
func TestXYPointDistanceSort_MissingLast(t *testing.T) {
	ix := newIntegrationIndex(t)
	addXYDoc(t, ix, 0, 0, false) // missing
	addXYDoc(t, ix, 40.718266, -74.007819, true)
	addXYDoc(t, ix, 40.7051157, -74.0088305, true)

	d2 := cartesianDistance(quantizeXY(40.718266), quantizeXY(-74.007819), float64(xyDistanceSortOriginX), float64(xyDistanceSortOriginY))
	d3 := cartesianDistance(quantizeXY(40.7051157), quantizeXY(-74.0088305), float64(xyDistanceSortOriginX), float64(xyDistanceSortOriginY))

	s, cleanup := ix.searcher()
	defer cleanup()

	sf, err := search.NewXYDocValuesDistanceSort("location", xyDistanceSortOriginX, xyDistanceSortOriginY)
	if err != nil {
		t.Fatalf("NewXYDocValuesDistanceSort: %v", err)
	}
	td, err := s.SearchWithSort(search.NewMatchAllDocsQuery(), 3, search.NewSort(sf))
	if err != nil {
		t.Fatalf("SearchWithSort: %v", err)
	}
	if len(td.FieldDocs) != 3 {
		t.Fatalf("want 3 hits, got %d", len(td.FieldDocs))
	}
	assertDistance(t, "hit0", td.FieldDocs[0], d2)
	assertDistance(t, "hit1", td.FieldDocs[1], d3)
	assertDistance(t, "hit2 (missing)", td.FieldDocs[2], math.Inf(1))
}

// TestXYPointDistanceSort_Random is a scaled-down deterministic analogue of
// testRandom (Java lines 121-124): it indexes a fixed set of points, runs a
// brute-force comparison for the full first page, and asserts the searcher
// output matches the brute-force expected order and distances. Unlike the
// Java test it does not exercise the random searchAfter second page (see
// below).
func TestXYPointDistanceSort_Random(t *testing.T) {
	pts := []struct {
		x, y float32
		miss bool
	}{
		{1, 1, false}, {3, 4, false}, {-2, 5, false}, {0, 0, true},
		{10, -10, false}, {7, 7, false}, {-5, -5, false}, {2, -3, false},
	}
	ix := newIntegrationIndex(t)
	for _, p := range pts {
		addXYDoc(t, ix, p.x, p.y, !p.miss)
	}
	s, cleanup := ix.searcher()
	defer cleanup()

	const qx, qy = float32(1.5), float32(2.5)

	// Brute-force expected order: by (distance asc, docID asc).
	type res struct {
		doc  int
		dist float64
	}
	expected := make([]res, len(pts))
	for i, p := range pts {
		dist := math.Inf(1)
		if !p.miss {
			dist = cartesianDistance(quantizeXY(p.x), quantizeXY(p.y), float64(qx), float64(qy))
		}
		expected[i] = res{doc: i, dist: dist}
	}
	// stable sort by distance then doc id
	for i := 1; i < len(expected); i++ {
		for j := i; j > 0; j-- {
			a, b := expected[j-1], expected[j]
			if b.dist < a.dist || (b.dist == a.dist && b.doc < a.doc) {
				expected[j-1], expected[j] = expected[j], expected[j-1]
			} else {
				break
			}
		}
	}

	sf, err := search.NewXYDocValuesDistanceSort("location", qx, qy)
	if err != nil {
		t.Fatalf("NewXYDocValuesDistanceSort: %v", err)
	}
	td, err := s.SearchWithSort(search.NewMatchAllDocsQuery(), len(pts), search.NewSort(sf))
	if err != nil {
		t.Fatalf("SearchWithSort: %v", err)
	}
	if len(td.FieldDocs) != len(pts) {
		t.Fatalf("want %d hits, got %d", len(pts), len(td.FieldDocs))
	}
	for i, fd := range td.FieldDocs {
		if fd.Doc != expected[i].doc {
			t.Errorf("hit %d: doc = %d, want %d", i, fd.Doc, expected[i].doc)
		}
		assertDistance(t, "random hit", fd, expected[i].dist)
	}
}

// TestXYPointDistanceSort_RandomHuge mirrors the @Nightly testRandomHuge
// with a scaled-down deterministic sweep that verifies the full first-page
// ordering of 100 random documents. Unlike the upstream @Nightly test it
// does not exercise searchAfter pagination (which is unimplemented for all
// Sort comparators — rmp #130); the first-page brute-force comparison is
// already covered by TestXYPointDistanceSort_Random for a small set, and
// this test extends it to a larger random corpus.
func TestXYPointDistanceSort_RandomHuge(t *testing.T) {
	const numDocs = 100
	rng := newDeterministicRand(42)

	type pt struct {
		x, y float32
		miss bool
	}
	pts := make([]pt, numDocs)
	ix := newIntegrationIndex(t)
	for i := 0; i < numDocs; i++ {
		x := float32(rng.intn(200) - 100)
		y := float32(rng.intn(200) - 100)
		miss := rng.intn(5) == 0
		pts[i] = pt{x, y, miss}
		addXYDoc(t, ix, x, y, !miss)
	}
	s, cleanup := ix.searcher()
	defer cleanup()

	const qx, qy = float32(1.5), float32(2.5)

	// Brute-force expected order: by (distance asc, docID asc).
	type res struct {
		doc  int
		dist float64
	}
	expected := make([]res, numDocs)
	for i, p := range pts {
		dist := math.Inf(1)
		if !p.miss {
			dist = cartesianDistance(quantizeXY(p.x), quantizeXY(p.y), float64(qx), float64(qy))
		}
		expected[i] = res{doc: i, dist: dist}
	}
	// stable sort by distance then doc id.
	for i := 1; i < len(expected); i++ {
		for j := i; j > 0; j-- {
			a, b := expected[j-1], expected[j]
			if b.dist < a.dist || (b.dist == a.dist && b.doc < a.doc) {
				expected[j-1], expected[j] = expected[j], expected[j-1]
			} else {
				break
			}
		}
	}

	sf, err := search.NewXYDocValuesDistanceSort("location", qx, qy)
	if err != nil {
		t.Fatalf("NewXYDocValuesDistanceSort: %v", err)
	}
	td, err := s.SearchWithSort(search.NewMatchAllDocsQuery(), numDocs, search.NewSort(sf))
	if err != nil {
		t.Fatalf("SearchWithSort: %v", err)
	}
	if len(td.FieldDocs) != numDocs {
		t.Fatalf("want %d hits, got %d", numDocs, len(td.FieldDocs))
	}
	for i, fd := range td.FieldDocs {
		if fd.Doc != expected[i].doc {
			t.Errorf("hit %d: doc = %d, want %d", i, fd.Doc, expected[i].doc)
		}
		assertDistance(t, "random huge hit", fd, expected[i].dist)
	}

// assertDistance checks that fd.Fields[0] is a float64 equal to want.
func assertDistance(t *testing.T, label string, fd *search.FieldDoc, want float64) {
	t.Helper()
	if len(fd.Fields) == 0 {
		t.Fatalf("%s: FieldDoc has no sort fields", label)
	}
	got, ok := fd.Fields[0].(float64)
	if !ok {
		t.Fatalf("%s: sort value type = %T, want float64", label, fd.Fields[0])
	}
	if math.IsInf(want, 1) {
		if !math.IsInf(got, 1) {
			t.Errorf("%s: distance = %v, want +Inf", label, got)
		}
		return
	}
	if got != want {
		t.Errorf("%s: distance = %.17g, want %.17g", label, got, want)
	}
}