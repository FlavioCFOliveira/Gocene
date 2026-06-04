// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestLatLonDocValuesQueries.java
//   (+ the BaseGeoPointTestCase scenarios it inherits)
//
// TestLatLonDocValuesQueries extends BaseGeoPointTestCase and adds no own
// test methods; every scenario is inherited from the (randomised) base
// class, driving LatLonDocValuesField.newSlow{Box,Distance,Polygon,Geometry}Query.
// This port drives the CORE behaviours those random tests assert — the
// doc-values-backed box and distance (disk) and polygon queries over indexed
// LatLonDocValuesField — end to end through the production
// IndexWriter + IndexSearcher (newIntegrationIndex), against deterministic
// fixtures.
//
// The exhaustive random GeoTestUtil fuzzing inherited from
// BaseGeoPointTestCase is tracked separately and not reproduced verbatim.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// indexLatLonDocValues commits one LatLonDocValuesField("point") document
// per coordinate pair and returns the searcher plus its cleanup.
func indexLatLonDocValues(t *testing.T, pts [][2]float64) (*search.IndexSearcher, func()) {
	t.Helper()
	ix := newIntegrationIndex(t)
	for _, p := range pts {
		doc := document.NewDocument()
		f, err := document.NewLatLonDocValuesField("point", p[0], p[1])
		if err != nil {
			t.Fatalf("NewLatLonDocValuesField(%v): %v", p, err)
		}
		doc.Add(f.Field)
		ix.addDoc(doc)
	}
	ix.forceMerge(1)
	return ix.searcher()
}

// TestLatLonDocValuesQueries_BoxQuery exercises
// LatLonDocValuesField.newSlowBoxQuery: a wide box matches all three
// points, a tight box matches one, a disjoint box matches none.
func TestLatLonDocValuesQueries_BoxQuery(t *testing.T) {
	s, cleanup := indexLatLonDocValues(t, latLonPointNYC)
	defer cleanup()

	all, err := search.NewLatLonDocValuesBoxQuery("point", 40.0, 41.0, -75.0, -73.0)
	if err != nil {
		t.Fatalf("NewLatLonDocValuesBoxQuery(all): %v", err)
	}
	assertHitCount(t, s, all, 3)

	one, err := search.NewLatLonDocValuesBoxQuery("point", 40.75, 40.76, -73.99, -73.98)
	if err != nil {
		t.Fatalf("NewLatLonDocValuesBoxQuery(one): %v", err)
	}
	assertHitCount(t, s, one, 1)

	none, err := search.NewLatLonDocValuesBoxQuery("point", 0.0, 1.0, 0.0, 1.0)
	if err != nil {
		t.Fatalf("NewLatLonDocValuesBoxQuery(none): %v", err)
	}
	assertHitCount(t, s, none, 0)
}

// TestLatLonDocValuesQueries_DistanceQuery exercises the doc-values disk
// query, which LatLonDocValuesField.newSlowDistanceQuery builds as a
// geometry (Circle) query. A 50 km disk reaches all three points; a 100 m
// disk reaches only the centre point.
func TestLatLonDocValuesQueries_DistanceQuery(t *testing.T) {
	s, cleanup := indexLatLonDocValues(t, latLonPointNYC)
	defer cleanup()

	wideCircle, err := geo.NewCircle(40.759011, -73.9844722, 50_000)
	if err != nil {
		t.Fatalf("NewCircle(wide): %v", err)
	}
	wide, err := search.NewLatLonDocValuesQuery("point", document.QueryRelationIntersects, wideCircle)
	if err != nil {
		t.Fatalf("NewLatLonDocValuesQuery(wide): %v", err)
	}
	assertHitCount(t, s, wide, 3)

	tightCircle, err := geo.NewCircle(40.759011, -73.9844722, 100)
	if err != nil {
		t.Fatalf("NewCircle(tight): %v", err)
	}
	tight, err := search.NewLatLonDocValuesQuery("point", document.QueryRelationIntersects, tightCircle)
	if err != nil {
		t.Fatalf("NewLatLonDocValuesQuery(tight): %v", err)
	}
	assertHitCount(t, s, tight, 1)
}

// TestLatLonDocValuesQueries_PolygonQuery exercises the doc-values polygon
// query (LatLonDocValuesField.newSlowPolygonQuery → geometry query): a
// covering polygon matches all three points; a disjoint polygon matches
// none.
func TestLatLonDocValuesQueries_PolygonQuery(t *testing.T) {
	s, cleanup := indexLatLonDocValues(t, latLonPointNYC)
	defer cleanup()

	covering, err := geo.NewPolygon(
		[]float64{40.0, 40.0, 41.0, 41.0, 40.0},
		[]float64{-75.0, -73.0, -73.0, -75.0, -75.0},
	)
	if err != nil {
		t.Fatalf("NewPolygon(covering): %v", err)
	}
	q, err := search.NewLatLonDocValuesQuery("point", document.QueryRelationIntersects, covering)
	if err != nil {
		t.Fatalf("NewLatLonDocValuesQuery(covering): %v", err)
	}
	assertHitCount(t, s, q, 3)

	disjoint, err := geo.NewPolygon(
		[]float64{0.0, 0.0, 1.0, 1.0, 0.0},
		[]float64{0.0, 1.0, 1.0, 0.0, 0.0},
	)
	if err != nil {
		t.Fatalf("NewPolygon(disjoint): %v", err)
	}
	q2, err := search.NewLatLonDocValuesQuery("point", document.QueryRelationIntersects, disjoint)
	if err != nil {
		t.Fatalf("NewLatLonDocValuesQuery(disjoint): %v", err)
	}
	assertHitCount(t, s, q2, 0)
}
