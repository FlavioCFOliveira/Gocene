// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestNearest.java
//
// Tests for LatLonPoint.nearest (the BKD KNN nearest-neighbour search),
// ported through the production IndexWriter + IndexSearcher via
// search.NearestLatLonPoint. The four deterministic Java methods
// (testNearestNeighborWithDeletedDocs, testNearestNeighborWithAllDeletedDocs,
// testTieBreakByDocID, testNearestNeighborWithNoDocs) are reproduced exactly.
// The randomised testNearestNeighborRandom (GeoTestUtil fuzzing + a
// brute-force haversin comparison) is reproduced as a deterministic
// fixed-point brute-force comparison.

package search_test

import (
	"math"
	"sort"
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"

	_ "github.com/FlavioCFOliveira/Gocene/codecs"
)

// nearestIndex is a small writer harness for the Nearest suite. Unlike the
// shared integrationIndex it keeps the writer open across reader reopens so
// the delete-then-reopen scenarios can run, mirroring the Java tests that
// call w.deleteDocuments(...) between w.getReader() calls.
type nearestIndex struct {
	t   *testing.T
	dir store.Directory
	w   *index.IndexWriter
}

func newNearestIndex(t *testing.T) *nearestIndex {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	return &nearestIndex{t: t, dir: dir, w: w}
}

// addPoint adds one document with a LatLonPoint("point") and a stored
// StringField("id").
func (ix *nearestIndex) addPoint(lat, lon float64, id string) {
	ix.t.Helper()
	doc := document.NewDocument()
	p, err := document.NewLatLonPoint("point", lat, lon)
	if err != nil {
		ix.t.Fatalf("NewLatLonPoint: %v", err)
	}
	doc.Add(p.Field)
	sf, err := document.NewStringField("id", id, true)
	if err != nil {
		ix.t.Fatalf("NewStringField: %v", err)
	}
	doc.Add(sf.Field)
	if err := ix.w.AddDocument(doc); err != nil {
		ix.t.Fatalf("AddDocument: %v", err)
	}
}

// deleteByID deletes every document whose "id" term equals id.
func (ix *nearestIndex) deleteByID(id string) {
	ix.t.Helper()
	if err := ix.w.DeleteDocuments(index.NewTerm("id", id)); err != nil {
		ix.t.Fatalf("DeleteDocuments: %v", err)
	}
}

// reader commits and opens a fresh DirectoryReader + IndexSearcher; the
// writer stays open for further mutations. The returned closer releases
// the reader only.
func (ix *nearestIndex) reader() (*search.IndexSearcher, func()) {
	ix.t.Helper()
	if err := ix.w.Commit(); err != nil {
		ix.t.Fatalf("Commit: %v", err)
	}
	r, err := index.OpenDirectoryReader(ix.dir)
	if err != nil {
		ix.t.Fatalf("OpenDirectoryReader: %v", err)
	}
	return search.NewIndexSearcher(r), func() { _ = r.Close() }
}

func (ix *nearestIndex) close() {
	_ = ix.w.Close()
	_ = ix.dir.Close()
}

// storedID reads the stored "id" field of the document at docID through the
// searcher's reader.
func storedID(t *testing.T, s *search.IndexSearcher, docID int) string {
	t.Helper()
	doc, err := s.Doc(docID)
	if err != nil {
		t.Fatalf("Doc(%d): %v", docID, err)
	}
	values := doc.GetValues("id")
	if len(values) == 0 {
		t.Fatalf("Doc(%d): no stored id", docID)
	}
	return values[0]
}

// TestNearest_NearestNeighborWithDeletedDocs is the port of
// testNearestNeighborWithDeletedDocs (Java lines 49-75): the nearest hit is
// doc "0"; after deleting "0", the nearest hit becomes "1".
func TestNearest_NearestNeighborWithDeletedDocs(t *testing.T) {
	ix := newNearestIndex(t)
	defer ix.close()
	ix.addPoint(40.0, 50.0, "0")
	ix.addPoint(45.0, 55.0, "1")

	s, cleanup := ix.reader()
	td, err := search.NearestLatLonPoint(s, "point", 40.0, 50.0, 1)
	if err != nil {
		t.Fatalf("nearest: %v", err)
	}
	if len(td.FieldDocs) != 1 {
		t.Fatalf("want 1 hit, got %d", len(td.FieldDocs))
	}
	if id := storedID(t, s, td.FieldDocs[0].Doc); id != "0" {
		t.Errorf("nearest id = %q, want \"0\"", id)
	}
	cleanup()

	ix.deleteByID("0")
	s2, cleanup2 := ix.reader()
	defer cleanup2()
	td2, err := search.NearestLatLonPoint(s2, "point", 40.0, 50.0, 1)
	if err != nil {
		t.Fatalf("nearest after delete: %v", err)
	}
	if len(td2.FieldDocs) != 1 {
		t.Fatalf("want 1 hit after delete, got %d", len(td2.FieldDocs))
	}
	if id := storedID(t, s2, td2.FieldDocs[0].Doc); id != "1" {
		t.Errorf("nearest id after delete = %q, want \"1\"", id)
	}
}

// TestNearest_NearestNeighborWithAllDeletedDocs is the port of
// testNearestNeighborWithAllDeletedDocs (Java lines 77-105): after deleting
// both documents, nearest returns no hits.
func TestNearest_NearestNeighborWithAllDeletedDocs(t *testing.T) {
	ix := newNearestIndex(t)
	defer ix.close()
	ix.addPoint(40.0, 50.0, "0")
	ix.addPoint(45.0, 55.0, "1")

	s, cleanup := ix.reader()
	td, err := search.NearestLatLonPoint(s, "point", 40.0, 50.0, 1)
	if err != nil {
		t.Fatalf("nearest: %v", err)
	}
	if len(td.FieldDocs) == 0 || storedID(t, s, td.FieldDocs[0].Doc) != "0" {
		t.Fatalf("nearest before delete: unexpected result")
	}
	cleanup()

	ix.deleteByID("0")
	ix.deleteByID("1")
	s2, cleanup2 := ix.reader()
	defer cleanup2()
	td2, err := search.NearestLatLonPoint(s2, "point", 40.0, 50.0, 1)
	if err != nil {
		t.Fatalf("nearest after delete-all: %v", err)
	}
	if len(td2.FieldDocs) != 0 {
		t.Errorf("want 0 hits after deleting all, got %d", len(td2.FieldDocs))
	}
}

// TestNearest_TieBreakByDocID is the port of testTieBreakByDocID (Java
// lines 107-127): two documents at the identical point are returned in
// ascending docID order (doc "0" then doc "1").
func TestNearest_TieBreakByDocID(t *testing.T) {
	ix := newNearestIndex(t)
	defer ix.close()
	ix.addPoint(40.0, 50.0, "0")
	ix.addPoint(40.0, 50.0, "1")

	s, cleanup := ix.reader()
	defer cleanup()
	td, err := search.NearestLatLonPoint(s, "point", 45.0, 50.0, 2)
	if err != nil {
		t.Fatalf("nearest: %v", err)
	}
	if len(td.FieldDocs) != 2 {
		t.Fatalf("want 2 hits, got %d", len(td.FieldDocs))
	}
	if id := storedID(t, s, td.FieldDocs[0].Doc); id != "0" {
		t.Errorf("hit 0 id = %q, want \"0\"", id)
	}
	if id := storedID(t, s, td.FieldDocs[1].Doc); id != "1" {
		t.Errorf("hit 1 id = %q, want \"1\"", id)
	}
}

// TestNearest_NearestNeighborWithNoDocs is the port of
// testNearestNeighborWithNoDocs (Java lines 129-138): nearest over an empty
// index returns no hits.
func TestNearest_NearestNeighborWithNoDocs(t *testing.T) {
	ix := newNearestIndex(t)
	defer ix.close()

	s, cleanup := ix.reader()
	defer cleanup()
	td, err := search.NearestLatLonPoint(s, "point", 40.0, 50.0, 1)
	if err != nil {
		t.Fatalf("nearest: %v", err)
	}
	if len(td.FieldDocs) != 0 {
		t.Errorf("want 0 hits over empty index, got %d", len(td.FieldDocs))
	}
}

// TestNearest_NearestNeighborRandom is a deterministic analogue of
// testNearestNeighborRandom (Java lines 147-256): it indexes a fixed grid
// of points, then for a set of query points compares the BKD nearest-N
// output against a brute-force haversin ranking (distance asc, docID asc),
// asserting both the doc ids and the per-hit distances agree exactly.
func TestNearest_NearestNeighborRandom(t *testing.T) {
	ix := newNearestIndex(t)
	defer ix.close()

	// A deterministic grid of points (quantized to the LatLonPoint encoding
	// so the indexed value equals the brute-force input).
	type pt struct{ lat, lon float64 }
	var pts []pt
	for latI := -2; latI <= 2; latI++ {
		for lonI := -2; lonI <= 2; lonI++ {
			lat := quantizeLat(float64(latI) * 15.0)
			lon := quantizeLon(float64(lonI) * 30.0)
			pts = append(pts, pt{lat, lon})
		}
	}
	for i, p := range pts {
		ix.addPoint(p.lat, p.lon, strconv.Itoa(i))
	}
	s, cleanup := ix.reader()
	defer cleanup()

	queries := []pt{
		{0, 0}, {17, -41}, {-33, 62}, {12.5, 12.5}, {-60, 175},
	}
	for _, q := range queries {
		// Brute-force expected ranking.
		type res struct {
			doc  int
			dist float64
		}
		expected := make([]res, len(pts))
		for i, p := range pts {
			expected[i] = res{doc: i, dist: util.HaversinMeters(q.lat, q.lon, p.lat, p.lon)}
		}
		sort.SliceStable(expected, func(a, b int) bool {
			if expected[a].dist != expected[b].dist {
				return expected[a].dist < expected[b].dist
			}
			return expected[a].doc < expected[b].doc
		})

		const topN = 5
		td, err := search.NearestLatLonPoint(s, "point", q.lat, q.lon, topN)
		if err != nil {
			t.Fatalf("nearest(%v): %v", q, err)
		}
		if len(td.FieldDocs) != topN {
			t.Fatalf("query %v: want %d hits, got %d", q, topN, len(td.FieldDocs))
		}
		for i, fd := range td.FieldDocs {
			wantDoc := expected[i].doc
			// id is the insertion order, so the stored id equals the doc id.
			gotID := storedID(t, s, fd.Doc)
			if gotID != strconv.Itoa(wantDoc) {
				t.Errorf("query %v hit %d: id = %q, want %q", q, i, gotID, strconv.Itoa(wantDoc))
			}
			gotDist, ok := fd.Fields[0].(float64)
			if !ok {
				t.Fatalf("query %v hit %d: distance type %T", q, i, fd.Fields[0])
			}
			if math.Abs(gotDist-expected[i].dist) > 1e-6 {
				t.Errorf("query %v hit %d: distance = %v, want %v", q, i, gotDist, expected[i].dist)
			}
		}
	}
}

// quantizeLat / quantizeLon round a coordinate through the LatLonPoint
// encoding, matching the Java helpers of the same name.
func quantizeLat(lat float64) float64 {
	return geo.DecodeLatitude(geo.EncodeLatitude(lat))
}

func quantizeLon(lon float64) float64 {
	return geo.DecodeLongitude(geo.EncodeLongitude(lon))
}