// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestNearest.java
// (Apache Lucene 10.5.0). LatLonPoint.nearest is rendered by
// search.NearestLatLonPoint.

package search_test

import (
	"math"
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/geo/testutil"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// nearestAddPoint adds new LatLonPoint(name, lat, lon).
func nearestAddPoint(t *testing.T, doc *document.Document, name string, lat, lon float64) {
	t.Helper()
	p, err := document.NewLatLonPoint(name, lat, lon)
	if err != nil {
		t.Fatalf("new LatLonPoint: %v", err)
	}
	doc.Add(p)
}

// nearest renders LatLonPoint.nearest(IndexSearcher, String, double, double, int).
func nearest(t *testing.T, s *search.IndexSearcher, field string, lat, lon float64, n int) *search.TopFieldDocs {
	t.Helper()
	td, err := search.NearestLatLonPoint(s, field, lat, lon, n)
	if err != nil {
		t.Fatalf("LatLonPoint.nearest: %v", err)
	}
	return td
}

// nearestStoredID renders r.storedFields().document(doc).getField("id").stringValue().
func nearestStoredID(t *testing.T, r *index.DirectoryReader, doc int) string {
	t.Helper()
	return storedDocument(t, mustStoredFields(t, r), doc).Get("id").StringValue()
}

func TestNearestNearestNeighborWithDeletedDocs(t *testing.T) {
	dir := newDirectory()
	w := newRandomIndexWriterWithConfig(t, dir, nearestGetIndexWriterConfig())
	doc := document.NewDocument()
	nearestAddPoint(t, doc, "point", 40.0, 50.0)
	doc.Add(mustStringField(t, "id", "0", true))
	mustAddDocument(t, w, doc)

	doc = document.NewDocument()
	nearestAddPoint(t, doc, "point", 45.0, 55.0)
	doc.Add(mustStringField(t, "id", "1", true))
	mustAddDocument(t, w, doc)

	r := mustGetReader(t, w)
	// can't wrap because we require Lucene60PointsFormat directly but e.g. ParallelReader wraps
	// with its own points impl:
	s := newSearcherMaybeWrap(t, r, false)
	hit := nearest(t, s, "point", 40.0, 50.0, 1).FieldDocs[0]
	if got := nearestStoredID(t, r, hit.Doc); got != "0" {
		t.Fatalf("id = %s, want 0", got)
	}
	mustClose(t, r)

	if _, err := w.DeleteDocuments(index.NewTerm("id", "0")); err != nil {
		t.Fatal(err)
	}
	r = mustGetReader(t, w)
	// can't wrap because we require Lucene60PointsFormat directly but e.g. ParallelReader wraps
	// with its own points impl:
	s = newSearcherMaybeWrap(t, r, false)
	hit = nearest(t, s, "point", 40.0, 50.0, 1).FieldDocs[0]
	if got := nearestStoredID(t, r, hit.Doc); got != "1" {
		t.Fatalf("id = %s, want 1", got)
	}
	mustClose(t, r, w, dir)
}

func TestNearestNearestNeighborWithAllDeletedDocs(t *testing.T) {
	dir := newDirectory()
	w := newRandomIndexWriterWithConfig(t, dir, nearestGetIndexWriterConfig())
	doc := document.NewDocument()
	nearestAddPoint(t, doc, "point", 40.0, 50.0)
	doc.Add(mustStringField(t, "id", "0", true))
	mustAddDocument(t, w, doc)
	doc = document.NewDocument()
	nearestAddPoint(t, doc, "point", 45.0, 55.0)
	doc.Add(mustStringField(t, "id", "1", true))
	mustAddDocument(t, w, doc)

	r := mustGetReader(t, w)
	// can't wrap because we require Lucene60PointsFormat directly but e.g. ParallelReader wraps
	// with its own points impl:
	s := newSearcherMaybeWrap(t, r, false)
	hit := nearest(t, s, "point", 40.0, 50.0, 1).FieldDocs[0]
	if got := nearestStoredID(t, r, hit.Doc); got != "0" {
		t.Fatalf("id = %s, want 0", got)
	}
	mustClose(t, r)

	if _, err := w.DeleteDocuments(index.NewTerm("id", "0")); err != nil {
		t.Fatal(err)
	}
	if _, err := w.DeleteDocuments(index.NewTerm("id", "1")); err != nil {
		t.Fatal(err)
	}
	r = mustGetReader(t, w)
	// can't wrap because we require Lucene60PointsFormat directly but e.g. ParallelReader wraps
	// with its own points impl:
	s = newSearcherMaybeWrap(t, r, false)
	assertIntEquals(t, 0, len(nearest(t, s, "point", 40.0, 50.0, 1).ScoreDocs))
	mustClose(t, r, w, dir)
}

func TestNearestTieBreakByDocID(t *testing.T) {
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, nearestGetIndexWriterConfig())
	doc := document.NewDocument()
	nearestAddPoint(t, doc, "point", 40.0, 50.0)
	doc.Add(mustStringField(t, "id", "0", true))
	mustAddDocument(t, w, doc)
	doc = document.NewDocument()
	nearestAddPoint(t, doc, "point", 40.0, 50.0)
	doc.Add(mustStringField(t, "id", "1", true))
	mustAddDocument(t, w, doc)

	r := mustOpenDirectoryReaderFromWriter(t, w)
	// can't wrap because we require Lucene60PointsFormat directly but e.g. ParallelReader wraps
	// with its own points impl:
	hits := nearest(t, newSearcherMaybeWrap(t, r, false), "point", 45.0, 50.0, 2).ScoreDocs
	if got := nearestStoredID(t, r, hits[0].Doc); got != "0" {
		t.Fatalf("hits[0] id = %s, want 0", got)
	}
	if got := nearestStoredID(t, r, hits[1].Doc); got != "1" {
		t.Fatalf("hits[1] id = %s, want 1", got)
	}

	mustClose(t, r, w, dir)
}

func TestNearestNearestNeighborWithNoDocs(t *testing.T) {
	dir := newDirectory()
	w := newRandomIndexWriterWithConfig(t, dir, nearestGetIndexWriterConfig())
	r := mustGetReader(t, w)
	// can't wrap because we require Lucene60PointsFormat directly but e.g. ParallelReader wraps
	// with its own points impl:
	assertIntEquals(t, 0, len(nearest(t, newSearcherMaybeWrap(t, r, false), "point", 40.0, 50.0, 1).ScoreDocs))
	mustClose(t, r, w, dir)
}

func nearestQuantizeLat(latRaw float64) float64 {
	return geo.DecodeLatitude(geo.EncodeLatitude(latRaw))
}

func nearestQuantizeLon(lonRaw float64) float64 {
	return geo.DecodeLongitude(geo.EncodeLongitude(lonRaw))
}

func TestNearestNearestNeighborRandom(t *testing.T) {
	geoTestUtil := testutil.NewGeoTestUtilFromRand(random())
	numPoints := atLeast(1000)
	if numPoints > 100000 {
		// dir = newFSDirectory(createTempDir(getClass().getSimpleName()));
		t.Fatal(newFSDirectoryBlocker)
	}
	dir := newDirectory()
	lats := make([]float64, numPoints)
	lons := make([]float64, numPoints)

	iwc := nearestGetIndexWriterConfig()
	iwc.SetMergePolicy(newLogMergePolicy())
	iwc.SetMergeScheduler(index.NewSerialMergeScheduler())
	w := newRandomIndexWriterWithConfig(t, dir, iwc)
	for id := 0; id < numPoints; id++ {
		lats[id] = nearestQuantizeLat(geoTestUtil.NextLatitude())
		lons[id] = nearestQuantizeLon(geoTestUtil.NextLongitude())
		doc := document.NewDocument()
		nearestAddPoint(t, doc, "point", lats[id], lons[id])
		dv, err := document.NewLatLonDocValuesField("point", lats[id], lons[id])
		if err != nil {
			t.Fatal(err)
		}
		doc.Add(dv.Field)
		sf, err := document.NewStoredFieldFromInt("id", id)
		if err != nil {
			t.Fatal(err)
		}
		doc.Add(sf)
		mustAddDocument(t, w, doc)
	}

	if random().Intn(2) == 0 {
		if err := w.ForceMerge(1); err != nil {
			t.Fatal(err)
		}
	}

	r := mustGetReader(t, w)
	if testing.Verbose() {
		t.Logf("TEST: reader=%v", r)
	}
	// can't wrap because we require Lucene60PointsFormat directly but e.g. ParallelReader wraps
	// with its own points impl:
	s := newSearcherMaybeWrap(t, r, false)
	iters := atLeast(100)
	for iter := 0; iter < iters; iter++ {
		if testing.Verbose() {
			t.Logf("TEST: iter=%d", iter)
		}
		pointLat := geoTestUtil.NextLatitude()
		pointLon := geoTestUtil.NextLongitude()

		// dumb brute force search to get the expected result:
		expectedHits := make([]*search.FieldDoc, len(lats))
		for id := 0; id < len(lats); id++ {
			distance := util.HaversinMeters(pointLat, pointLon, lats[id], lons[id])
			expectedHits[id] = search.NewFieldDocWithFields(id, 0.0, []any{distance})
		}

		sort.SliceStable(expectedHits, func(i, j int) bool {
			a, b := expectedHits[i], expectedHits[j]
			cmp := cmpFloat64Java(a.Fields[0].(float64), b.Fields[0].(float64))
			if cmp != 0 {
				return cmp < 0
			}
			// tie break by smaller docID:
			return a.Doc-b.Doc < 0
		})

		topN := nextInt(1, len(lats))

		if testing.Verbose() {
			t.Logf("hits for pointLat=%v pointLon=%v", pointLat, pointLon)
		}

		// Also test with MatchAllDocsQuery, sorting by distance:
		sortField, err := search.NewLatLonPointSortField("point", pointLat, pointLon) // LatLonDocValuesField.newDistanceSort
		if err != nil {
			t.Fatal(err)
		}
		fieldDocs, err := s.SearchWithSort(search.NewMatchAllDocsQuery(), topN, search.NewSort(sortField.SortField), false)
		if err != nil {
			t.Fatalf("search: %v", err)
		}

		hits := nearest(t, s, "point", pointLat, pointLon, topN).FieldDocs
		for i := 0; i < topN; i++ {
			expected := expectedHits[i]
			expected2 := fieldDocs.FieldDocs[i]
			actual := hits[i]

			if expected.Doc != actual.Doc {
				t.Fatalf("hit %d: expected doc %d, got %d", i, expected.Doc, actual.Doc)
			}
			if expected.Fields[0].(float64) != actual.Fields[0].(float64) {
				t.Fatalf("hit %d: expected distance %v, got %v", i, expected.Fields[0], actual.Fields[0])
			}

			if expected2.Doc != actual.Doc {
				t.Fatalf("hit %d: expected2 doc %d, got %d", i, expected2.Doc, actual.Doc)
			}
			if expected2.Fields[0].(float64) != actual.Fields[0].(float64) {
				t.Fatalf("hit %d: expected2 distance %v, got %v", i, expected2.Fields[0], actual.Fields[0])
			}
		}
	}

	mustClose(t, r, w, dir)
}

// cmpFloat64Java renders Double.compare(double, double).
func cmpFloat64Java(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	ab, bb := int64(math.Float64bits(a)), int64(math.Float64bits(b))
	switch {
	case ab < bb:
		return -1
	case ab > bb:
		return 1
	}
	return 0
}

// nearestGetIndexWriterConfig renders the private getIndexWriterConfig().
func nearestGetIndexWriterConfig() *index.IndexWriterConfig {
	iwc := newIndexWriterConfig()
	iwc.SetCodec(index.GetDefaultCodec())
	return iwc
}
