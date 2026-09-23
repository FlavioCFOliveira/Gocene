// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestLatLonPointQueries.java
// (Apache Lucene 10.5.0).
//
// The class overrides the factory methods of
// org.apache.lucene.tests.geo.BaseGeoPointTestCase and inherits all of that
// base class's test methods. The base class is not ported, so the inherited
// suite fails naming it. testDistanceQueryWithInvertedIntersection is the
// class's own test method; the two overrides it calls are rendered below.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util/bkd"
)

// TestLatLonPointQueries renders the test methods TestLatLonPointQueries
// inherits from BaseGeoPointTestCase.
func TestLatLonPointQueries(t *testing.T) {
	t.Fatal(baseGeoPointTestCaseBlocker)
}

// llpqAddPointToDoc renders the addPointToDoc(String, Document, double, double)
// override: doc.add(new LatLonPoint(field, lat, lon)).
func llpqAddPointToDoc(t *testing.T, field string, doc *document.Document, lat, lon float64) {
	t.Helper()
	p, err := document.NewLatLonPoint(field, lat, lon)
	if err != nil {
		t.Fatalf("new LatLonPoint: %v", err)
	}
	doc.Add(p)
}

// llpqNewDistanceQuery renders the newDistanceQuery(String, double, double,
// double) override: LatLonPoint.newDistanceQuery, which is new
// LatLonPointDistanceQuery(field, centerLat, centerLon, radiusMeters).
func llpqNewDistanceQuery(t *testing.T, field string, centerLat, centerLon, radiusMeters float64) search.Query {
	t.Helper()
	q, err := search.NewLatLonPointDistanceQuery(field, centerLat, centerLon, radiusMeters)
	if err != nil {
		t.Fatalf("LatLonPoint.newDistanceQuery: %v", err)
	}
	return q
}

func TestLatLonPointQueriesDistanceQueryWithInvertedIntersection(t *testing.T) {
	numMatchingDocs := atLeast(10 * bkd.DefaultMaxPointsInLeafNode)

	dir := newDirectory()
	defer mustClose(t, dir)
	func() {
		w := mustNewIndexWriter(t, dir, newIndexWriterConfig())
		defer mustClose(t, w)
		for i := 0; i < numMatchingDocs; i++ {
			doc := document.NewDocument()
			llpqAddPointToDoc(t, "field", doc, 18.313694, -65.227444)
			mustAddDocument(t, w, doc)
		}

		// Add a handful of docs that don't match
		for i := 0; i < 11; i++ {
			doc := document.NewDocument()
			llpqAddPointToDoc(t, "field", doc, 10, -65.227444)
			mustAddDocument(t, w, doc)
		}

		if err := w.ForceMerge(1); err != nil {
			t.Fatalf("forceMerge: %v", err)
		}
	}()

	r := mustOpenDirectoryReader(t, dir)
	defer mustClose(t, r)
	searcher := newSearcher(t, r)
	assertIntEquals(t, numMatchingDocs, mustCount(t, searcher, llpqNewDistanceQuery(t, "field", 18, -65, 50_000)))
}
