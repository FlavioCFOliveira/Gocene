// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestLatLonPointQueries.java
//   (+ the BaseGeoPointTestCase scenarios it inherits)
//
// TestLatLonPointQueries in Lucene extends BaseGeoPointTestCase and adds a
// single own method, testDistanceQueryWithInvertedIntersection. The base
// class is an enormous randomised fuzzing harness (verifyRandom* over
// GeoTestUtil-generated points/queries). This port drives the CORE
// behaviours those random tests assert — box, distance and polygon queries
// over indexed LatLonPoint fields — end to end through the production
// IndexWriter + IndexSearcher (newIntegrationIndex), plus the one own
// method, against deterministic fixtures.
//
// The exhaustive random GeoTestUtil fuzzing (BaseGeoPointTestCase
// verifyRandomDistances / verifyRandomPolygons with thousands of random
// shapes) is tracked separately and not reproduced verbatim here.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// latLonPointNYC are three indexed points near New York City, the same
// fixed coordinates the sibling distance-sort suite uses.
var latLonPointNYC = [][2]float64{
	{40.759011, -73.9844722},
	{40.718266, -74.007819},
	{40.7051157, -74.0088305},
}

// indexLatLonPoints commits one LatLonPoint("point") document per
// coordinate pair and returns the searcher plus its cleanup.
func indexLatLonPoints(t *testing.T, pts [][2]float64) (*search.IndexSearcher, func()) {
	t.Helper()
	ix := newIntegrationIndex(t)
	for _, p := range pts {
		doc := document.NewDocument()
		f, err := document.NewLatLonPoint("point", p[0], p[1])
		if err != nil {
			t.Fatalf("NewLatLonPoint(%v): %v", p, err)
		}
		doc.Add(f.Field)
		ix.addDoc(doc)
	}
	ix.forceMerge(1)
	return ix.searcher()
}

// TestLatLonPointQueries_BoxQuery exercises LatLonPoint.newBoxQuery: a
// bounding box wide enough to contain every point matches all three, and
// a tight box around the first point matches only it (mirrors the
// rectangle assertions threaded through BaseGeoPointTestCase).
func TestLatLonPointQueries_BoxQuery(t *testing.T) {
	s, cleanup := indexLatLonPoints(t, latLonPointNYC)
	defer cleanup()

	all, err := search.NewLatLonPointBoxQuery("point", 40.0, 41.0, -75.0, -73.0)
	if err != nil {
		t.Fatalf("NewLatLonPointBoxQuery(all): %v", err)
	}
	assertHitCount(t, s, all, 3)

	// Tight box around the first point only (40.759, -73.984).
	one, err := search.NewLatLonPointBoxQuery("point", 40.75, 40.76, -73.99, -73.98)
	if err != nil {
		t.Fatalf("NewLatLonPointBoxQuery(one): %v", err)
	}
	assertHitCount(t, s, one, 1)

	// Box far from every point matches nothing.
	none, err := search.NewLatLonPointBoxQuery("point", 0.0, 1.0, 0.0, 1.0)
	if err != nil {
		t.Fatalf("NewLatLonPointBoxQuery(none): %v", err)
	}
	assertHitCount(t, s, none, 0)
}

// TestLatLonPointQueries_DistanceQuery exercises
// LatLonPoint.newDistanceQuery: a 50 km disk around the first point
// reaches all three (they are a few km apart), while a 100 m disk reaches
// only the first.
func TestLatLonPointQueries_DistanceQuery(t *testing.T) {
	s, cleanup := indexLatLonPoints(t, latLonPointNYC)
	defer cleanup()

	wide, err := search.NewLatLonPointDistanceQuery("point", 40.759011, -73.9844722, 50_000)
	if err != nil {
		t.Fatalf("NewLatLonPointDistanceQuery(wide): %v", err)
	}
	assertHitCount(t, s, wide, 3)

	tight, err := search.NewLatLonPointDistanceQuery("point", 40.759011, -73.9844722, 100)
	if err != nil {
		t.Fatalf("NewLatLonPointDistanceQuery(tight): %v", err)
	}
	assertHitCount(t, s, tight, 1)
}

// TestLatLonPointQueries_PolygonQuery exercises LatLonPoint.newPolygonQuery
// (INTERSECTS): a polygon containing every point matches all three; a
// polygon containing none matches zero.
func TestLatLonPointQueries_PolygonQuery(t *testing.T) {
	s, cleanup := indexLatLonPoints(t, latLonPointNYC)
	defer cleanup()

	covering, err := geo.NewPolygon(
		[]float64{40.0, 40.0, 41.0, 41.0, 40.0},
		[]float64{-75.0, -73.0, -73.0, -75.0, -75.0},
	)
	if err != nil {
		t.Fatalf("NewPolygon(covering): %v", err)
	}
	q, err := search.NewLatLonPointQuery("point", document.QueryRelationIntersects, covering)
	if err != nil {
		t.Fatalf("NewLatLonPointQuery(covering): %v", err)
	}
	assertHitCount(t, s, q, 3)

	disjoint, err := geo.NewPolygon(
		[]float64{0.0, 0.0, 1.0, 1.0, 0.0},
		[]float64{0.0, 1.0, 1.0, 0.0, 0.0},
	)
	if err != nil {
		t.Fatalf("NewPolygon(disjoint): %v", err)
	}
	q2, err := search.NewLatLonPointQuery("point", document.QueryRelationIntersects, disjoint)
	if err != nil {
		t.Fatalf("NewLatLonPointQuery(disjoint): %v", err)
	}
	assertHitCount(t, s, q2, 0)
}

// TestLatLonPointQueries_DistanceQueryWithInvertedIntersection is the
// direct port of TestLatLonPointQueries#testDistanceQueryWithInvertedIntersection
// (Java lines 78-110): many copies of one matching point plus a handful of
// non-matching points, queried by a 50 km disk that must return exactly
// the matching copies. The Java test scales the matching count to
// 10 × BKDConfig.DEFAULT_MAX_POINTS_IN_LEAF_NODE (5120) to force the BKD
// inverted-intersection path; this port uses a fixed 5120 so the leaf node
// count and the inverted path are exercised deterministically.
func TestLatLonPointQueries_DistanceQueryWithInvertedIntersection(t *testing.T) {
	const numMatchingDocs = 10 * 512 // 10 × DEFAULT_MAX_POINTS_IN_LEAF_NODE

	ix := newIntegrationIndex(t)
	for i := 0; i < numMatchingDocs; i++ {
		doc := document.NewDocument()
		f, err := document.NewLatLonPoint("field", 18.313694, -65.227444)
		if err != nil {
			t.Fatalf("NewLatLonPoint(match): %v", err)
		}
		doc.Add(f.Field)
		ix.addDoc(doc)
	}
	// A handful of docs that do not match.
	for i := 0; i < 11; i++ {
		doc := document.NewDocument()
		f, err := document.NewLatLonPoint("field", 10, -65.227444)
		if err != nil {
			t.Fatalf("NewLatLonPoint(nonmatch): %v", err)
		}
		doc.Add(f.Field)
		ix.addDoc(doc)
	}
	ix.forceMerge(1)
	s, cleanup := ix.searcher()
	defer cleanup()

	q, err := search.NewLatLonPointDistanceQuery("field", 18, -65, 50_000)
	if err != nil {
		t.Fatalf("NewLatLonPointDistanceQuery: %v", err)
	}
	assertHitCount(t, s, q, numMatchingDocs)
}
