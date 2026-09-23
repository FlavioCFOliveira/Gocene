// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of
// lucene/core/src/test/org/apache/lucene/document/TestLatLonPointDistanceFeatureQuery.java
// (Apache Lucene 10.5.0).

package search_test

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	testsearch "github.com/FlavioCFOliveira/Gocene/tests/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// mustLatLonDistanceFeatureQuery renders LatLonPoint.newDistanceFeatureQuery.
func mustLatLonDistanceFeatureQuery(t *testing.T, field string, weight float32, lat, lon, pivot float64) search.Query {
	t.Helper()
	q, err := search.LatLonPointNewDistanceFeatureQuery(field, weight, lat, lon, pivot)
	if err != nil {
		t.Fatalf("LatLonPoint.newDistanceFeatureQuery: %v", err)
	}
	return q
}

func mustLatLonPoint(t *testing.T, name string, lat, lon float64) *document.LatLonPoint {
	t.Helper()
	p, err := document.NewLatLonPoint(name, lat, lon)
	if err != nil {
		t.Fatalf("new LatLonPoint: %v", err)
	}
	return p
}

func mustLatLonDocValuesField(t *testing.T, name string, lat, lon float64) *document.LatLonDocValuesField {
	t.Helper()
	f, err := document.NewLatLonDocValuesField(name, lat, lon)
	if err != nil {
		t.Fatalf("new LatLonDocValuesField: %v", err)
	}
	return f
}

// quantizedHaversin renders SloppyMath.haversinMeters(decode(encode(lat)),
// decode(encode(lon)), lat2, lon2).
func quantizedHaversin(lat, lon, lat2, lon2 float64) float64 {
	return util.HaversinMeters(
		geo.DecodeLatitude(geo.EncodeLatitude(lat)),
		geo.DecodeLongitude(geo.EncodeLongitude(lon)),
		lat2,
		lon2)
}

func TestLatLonPointDistanceFeatureQueryEqualsAndHashcode(t *testing.T) {
	q1 := mustLatLonDistanceFeatureQuery(t, "foo", 3, 10, 10, 5)
	q2 := mustLatLonDistanceFeatureQuery(t, "foo", 3, 10, 10, 5)
	queryUtilsCheckEqual(t, q1, q2)

	q3 := mustLatLonDistanceFeatureQuery(t, "bar", 3, 10, 10, 5)
	queryUtilsCheckUnequal(t, q1, q3)

	q4 := mustLatLonDistanceFeatureQuery(t, "foo", 4, 10, 10, 5)
	queryUtilsCheckUnequal(t, q1, q4)

	q5 := mustLatLonDistanceFeatureQuery(t, "foo", 3, 9, 10, 5)
	queryUtilsCheckUnequal(t, q1, q5)

	q6 := mustLatLonDistanceFeatureQuery(t, "foo", 3, 10, 9, 5)
	queryUtilsCheckUnequal(t, q1, q6)

	q7 := mustLatLonDistanceFeatureQuery(t, "foo", 3, 10, 10, 6)
	queryUtilsCheckUnequal(t, q1, q7)
}

func TestLatLonPointDistanceFeatureQueryBasics(t *testing.T) {
	dir := newDirectory()
	w := newRandomIndexWriterWithConfig(t, dir, newLongDistanceWriterConfig())
	point := mustLatLonPoint(t, "foo", 0.0, 0.0)
	docValue := mustLatLonDocValuesField(t, "foo", 0.0, 0.0)
	doc := newTestDocument(point, docValue)

	pivotDistance := 5000.0 // 5k

	for _, ll := range [][2]float64{{-7, -7}, {9, 9}, {8, 8}, {4, 4}, {-1, -1}} {
		point.SetLocationValue(ll[0], ll[1])
		docValue.SetLocationValue(ll[0], ll[1])
		mustAddDocument(t, w, doc)
	}

	reader := mustGetReader(t, w)
	searcher := newSearcher(t, reader)

	q := mustLatLonDistanceFeatureQuery(t, "foo", 3, 10, 10, pivotDistance)
	collectorManager := mustTopScoreDocCollectorManager(t, 2, 1)
	topHits := mustSearchWithManager(t, searcher, q, collectorManager)
	if len(topHits.ScoreDocs) != 2 {
		t.Fatalf("expected 2 hits, got %d", len(topHits.ScoreDocs))
	}

	distance1 := quantizedHaversin(9, 9, 10, 10)
	distance2 := quantizedHaversin(8, 8, 10, 10)

	testsearch.CheckEqual(t,
		q,
		[]*search.ScoreDoc{
			search.NewScoreDoc(1, float32(3*(pivotDistance/(pivotDistance+distance1))), -1),
			search.NewScoreDoc(2, float32(3*(pivotDistance/(pivotDistance+distance2))), -1),
		},
		topHits.ScoreDocs)

	distance1 = quantizedHaversin(9, 9, 9, 9)
	distance2 = quantizedHaversin(8, 8, 9, 9)

	q = mustLatLonDistanceFeatureQuery(t, "foo", 3, 9, 9, pivotDistance)
	collectorManager = mustTopScoreDocCollectorManager(t, 2, 1)
	topHits = mustSearchWithManager(t, searcher, q, collectorManager)
	if len(topHits.ScoreDocs) != 2 {
		t.Fatalf("expected 2 hits, got %d", len(topHits.ScoreDocs))
	}
	testsearch.CheckExplanations(t, q, "", searcher, false)

	testsearch.CheckEqual(t,
		q,
		[]*search.ScoreDoc{
			search.NewScoreDoc(1, float32(3*(pivotDistance/(pivotDistance+distance1))), -1),
			search.NewScoreDoc(2, float32(3*(pivotDistance/(pivotDistance+distance2))), -1),
		},
		topHits.ScoreDocs)

	mustClose(t, reader, w, dir)
}

func TestLatLonPointDistanceFeatureQueryCrossesDateLine(t *testing.T) {
	dir := newDirectory()
	w := newRandomIndexWriterWithConfig(t, dir, newLongDistanceWriterConfig())
	point := mustLatLonPoint(t, "foo", 0.0, 0.0)
	docValue := mustLatLonDocValuesField(t, "foo", 0.0, 0.0)
	doc := newTestDocument(point, docValue)

	pivotDistance := 5000.0 // 5k

	point.SetLocationValue(0, -179)
	docValue.SetLocationValue(0, -179)
	mustAddDocument(t, w, doc)

	point.SetLocationValue(0, 176)
	docValue.SetLocationValue(0, 176)
	mustAddDocument(t, w, doc)

	point.SetLocationValue(0, -150)
	docValue.SetLocationValue(0, -150)
	mustAddDocument(t, w, doc)

	point.SetLocationValue(0, -140)
	docValue.SetLocationValue(0, -140)
	mustAddDocument(t, w, doc)

	point.SetLocationValue(0, 140)
	docValue.SetLocationValue(01, 140)
	mustAddDocument(t, w, doc)

	reader := mustGetReader(t, w)
	searcher := newSearcher(t, reader)

	q := mustLatLonDistanceFeatureQuery(t, "foo", 3, 0, 179, pivotDistance)
	collectorManager := mustTopScoreDocCollectorManager(t, 2, 1)
	topHits := mustSearchWithManager(t, searcher, q, collectorManager)
	if len(topHits.ScoreDocs) != 2 {
		t.Fatalf("expected 2 hits, got %d", len(topHits.ScoreDocs))
	}

	distance1 := quantizedHaversin(0, -179, 0, 179)
	distance2 := quantizedHaversin(0, 176, 0, 179)

	testsearch.CheckEqual(t,
		q,
		[]*search.ScoreDoc{
			search.NewScoreDoc(0, float32(3*(pivotDistance/(pivotDistance+distance1))), -1),
			search.NewScoreDoc(1, float32(3*(pivotDistance/(pivotDistance+distance2))), -1),
		},
		topHits.ScoreDocs)

	mustClose(t, reader, w, dir)
}

func TestLatLonPointDistanceFeatureQueryMissingField(t *testing.T) {
	reader := newMultiReader(t)
	searcher := newSearcher(t, reader)

	q := mustLatLonDistanceFeatureQuery(t, "foo", 3, 10, 10, 5000)
	topHits := mustSearch(t, searcher, q, 2)
	if topHits.TotalHits.Value != 0 {
		t.Fatalf("expected 0, got %d", topHits.TotalHits.Value)
	}
}

func TestLatLonPointDistanceFeatureQueryMissingValue(t *testing.T) {
	dir := newDirectory()
	w := newRandomIndexWriterWithConfig(t, dir, newLongDistanceWriterConfig())
	point := mustLatLonPoint(t, "foo", 0, 0)
	docValue := mustLatLonDocValuesField(t, "foo", 0, 0)
	doc := newTestDocument(point, docValue)

	point.SetLocationValue(3, 3)
	docValue.SetLocationValue(3, 3)
	mustAddDocument(t, w, doc)

	mustAddDocument(t, w, document.NewDocument())

	point.SetLocationValue(7, 7)
	docValue.SetLocationValue(7, 7)
	mustAddDocument(t, w, doc)

	reader := mustGetReader(t, w)
	searcher := newSearcher(t, reader)

	q := mustLatLonDistanceFeatureQuery(t, "foo", 3, 10, 10, 5)
	collectorManager := mustTopScoreDocCollectorManager(t, 3, 1)
	topHits := mustSearchWithManager(t, searcher, q, collectorManager)
	if len(topHits.ScoreDocs) != 2 {
		t.Fatalf("expected 2 hits, got %d", len(topHits.ScoreDocs))
	}

	distance1 := quantizedHaversin(7, 7, 10, 10)
	distance2 := quantizedHaversin(3, 3, 10, 10)

	testsearch.CheckEqual(t,
		q,
		[]*search.ScoreDoc{
			search.NewScoreDoc(2, float32(3*(5./(5.+distance1))), -1),
			search.NewScoreDoc(0, float32(3*(5./(5.+distance2))), -1),
		},
		topHits.ScoreDocs)

	testsearch.CheckExplanations(t, q, "", searcher, false)

	mustClose(t, reader, w, dir)
}

func TestLatLonPointDistanceFeatureQueryMultiValued(t *testing.T) {
	dir := newDirectory()
	w := newRandomIndexWriterWithConfig(t, dir, newLongDistanceWriterConfig())

	for _, points := range [][][2]float64{
		{{0, 0}, {30, 30}, {60, 60}},
		{{45, 0}, {-45, 0}, {-90, 0}, {90, 0}},
		{{0, 90}, {0, -90}, {0, 180}, {0, -180}},
		{{3, 2}},
		{{45, 45}, {-45, -45}},
	} {
		doc := document.NewDocument()
		for _, point := range points {
			doc.Add(mustLatLonPoint(t, "foo", point[0], point[1]))
			doc.Add(mustLatLonDocValuesField(t, "foo", point[0], point[1]))
		}
		mustAddDocument(t, w, doc)
	}

	reader := mustGetReader(t, w)
	searcher := newSearcher(t, reader)

	q := mustLatLonDistanceFeatureQuery(t, "foo", 3, 0, 0, 200)
	collectorManager := mustTopScoreDocCollectorManager(t, 2, 1)
	topHits := mustSearchWithManager(t, searcher, q, collectorManager)
	if len(topHits.ScoreDocs) != 2 {
		t.Fatalf("expected 2 hits, got %d", len(topHits.ScoreDocs))
	}

	distance1 := quantizedHaversin(0, 0, 0, 0)
	distance2 := quantizedHaversin(3, 2, 0, 0)

	testsearch.CheckEqual(t,
		q,
		[]*search.ScoreDoc{
			search.NewScoreDoc(0, float32(3*(200/(200+distance1))), -1),
			search.NewScoreDoc(3, float32(3*(200/(200+distance2))), -1),
		},
		topHits.ScoreDocs)

	q = mustLatLonDistanceFeatureQuery(t, "foo", 3, -90, 0, 10000.)
	collectorManager = mustTopScoreDocCollectorManager(t, 2, 1)
	topHits = mustSearchWithManager(t, searcher, q, collectorManager)
	if len(topHits.ScoreDocs) != 2 {
		t.Fatalf("expected 2 hits, got %d", len(topHits.ScoreDocs))
	}
	testsearch.CheckExplanations(t, q, "", searcher, false)

	distance1 = quantizedHaversin(-90, 0, -90, 0)
	distance2 = quantizedHaversin(-45, -45, -90, 0)

	testsearch.CheckEqual(t,
		q,
		[]*search.ScoreDoc{
			search.NewScoreDoc(1, float32(3*(10000./(10000.+distance1))), -1),
			search.NewScoreDoc(4, float32(3*(10000./(10000.+distance2))), -1),
		},
		topHits.ScoreDocs)

	mustClose(t, reader, w, dir)
}

func TestLatLonPointDistanceFeatureQueryRandom(t *testing.T) {
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, newLongDistanceWriterConfig())
	point := mustLatLonPoint(t, "foo", 0., 0.)
	docValue := mustLatLonDocValuesField(t, "foo", 0., 0.)
	doc := newTestDocument(point, docValue)

	numDocs := atLeast(1000)
	for i := 0; i < numDocs; i++ {
		lat := random().Float64()*180 - 90
		lon := random().Float64()*360 - 180
		point.SetLocationValue(lat, lon)
		docValue.SetLocationValue(lat, lon)
		mustAddDocument(t, w, doc)
	}

	reader, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("DirectoryReader.open(w): %v", err)
	}
	searcher := newSearcher(t, reader)

	numIters := atLeast(3)
	for iter := 0; iter < numIters; iter++ {
		lat := random().Float64()*180 - 90
		lon := random().Float64()*360 - 180
		pivotDistance := random().Float64() * random().Float64() * math.Pi * geo.EarthMeanRadiusMeters
		boost := float32(1+random().Intn(10)) / 3
		q := mustLatLonDistanceFeatureQuery(t, "foo", boost, lat, lon, pivotDistance)

		testsearch.CheckTopScores(t, random(), q, searcher)
	}

	mustClose(t, reader, w, dir)
}

func TestLatLonPointDistanceFeatureQueryCompareSorting(t *testing.T) {
	dir := newDirectory()
	w := newRandomIndexWriterWithConfig(t, dir, newLongDistanceWriterConfig())

	point := mustLatLonPoint(t, "foo", 0., 0.)
	docValue := mustLatLonDocValuesField(t, "foo", 0., 0.)
	doc := newTestDocument(point, docValue)

	numDocs := atLeast(10000)
	for i := 0; i < numDocs; i++ {
		lat := random().Float64()*180 - 90
		lon := random().Float64()*360 - 180
		point.SetLocationValue(lat, lon)
		docValue.SetLocationValue(lat, lon)
		mustAddDocument(t, w, doc)
	}

	reader := mustGetReader(t, w)
	searcher := newSearcher(t, reader)

	lat := random().Float64()*180 - 90
	lon := random().Float64()*360 - 180
	pivotDistance := random().Float64() * random().Float64() * geo.EarthMeanRadiusMeters * math.Pi
	boost := float32(1+random().Intn(10)) / 3

	query1 := mustLatLonDistanceFeatureQuery(t, "foo", boost, lat, lon, pivotDistance)
	distanceSort1, err := search.NewLatLonDocValuesDistanceSort("foo", lat, lon)
	if err != nil {
		t.Fatalf("LatLonDocValuesField.newDistanceSort: %v", err)
	}
	sort1 := search.NewSort(search.FieldScore, distanceSort1)

	var query2 search.Query = search.NewMatchAllDocsQuery()
	distanceSort2, err := search.NewLatLonDocValuesDistanceSort("foo", lat, lon)
	if err != nil {
		t.Fatalf("LatLonDocValuesField.newDistanceSort: %v", err)
	}
	sort2 := search.NewSort(distanceSort2)

	topDocs1, err := searcher.SearchWithSort(query1, 10, sort1, false)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	topDocs2, err := searcher.SearchWithSort(query2, 10, sort2, false)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	for i := 0; i < 10; i++ {
		if topDocs1.ScoreDocs[i].Doc != topDocs2.ScoreDocs[i].Doc {
			t.Fatalf("doc %d: %d vs %d", i, topDocs1.ScoreDocs[i].Doc, topDocs2.ScoreDocs[i].Doc)
		}
	}
	mustClose(t, reader, w, dir)
}
