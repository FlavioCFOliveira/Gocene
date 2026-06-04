// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestXYDocValuesQueries.java
//   (+ the BaseXYPointTestCase scenarios it inherits)
//
// TestXYDocValuesQueries extends BaseXYPointTestCase and adds no own test
// methods; every scenario is inherited from the (randomised) base class,
// driving XYDocValuesField.newSlow{Box,Distance,Polygon,Geometry}Query.
// This port drives the CORE behaviours those random tests assert — the
// doc-values-backed box, distance (circle) and polygon point-in-geometry
// queries over indexed XYDocValuesField — end to end through the
// production IndexWriter + IndexSearcher (newIntegrationIndex), against
// deterministic fixtures.
//
// The exhaustive random ShapeTestUtil fuzzing inherited from
// BaseXYPointTestCase is tracked separately and not reproduced verbatim.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// xyPoints are three Cartesian points: two clustered near the origin and
// one far away.
var xyPoints = [][2]float32{
	{1, 1},
	{2, 2},
	{10, 10},
}

// indexXYDocValues commits one XYDocValuesField("xy") document per
// coordinate pair and returns the searcher plus its cleanup.
func indexXYDocValues(t *testing.T, pts [][2]float32) (*search.IndexSearcher, func()) {
	t.Helper()
	ix := newIntegrationIndex(t)
	for _, p := range pts {
		doc := document.NewDocument()
		f, err := document.NewXYDocValuesField("xy", p[0], p[1])
		if err != nil {
			t.Fatalf("NewXYDocValuesField(%v): %v", p, err)
		}
		doc.Add(f.Field)
		ix.addDoc(doc)
	}
	ix.forceMerge(1)
	return ix.searcher()
}

// TestXYDocValuesQueries_BoxQuery exercises XYDocValuesField.newSlowBoxQuery
// (an XYRectangle geometry): a box covering the two clustered points
// matches them and excludes the far point; a box around the far point
// matches only it.
func TestXYDocValuesQueries_BoxQuery(t *testing.T) {
	s, cleanup := indexXYDocValues(t, xyPoints)
	defer cleanup()

	near, err := geo.NewXYRectangle(0, 5, 0, 5)
	if err != nil {
		t.Fatalf("NewXYRectangle(near): %v", err)
	}
	q, err := search.NewXYDocValuesQuery("xy", near)
	if err != nil {
		t.Fatalf("NewXYDocValuesQuery(near): %v", err)
	}
	assertHitCount(t, s, q, 2)

	far, err := geo.NewXYRectangle(9, 11, 9, 11)
	if err != nil {
		t.Fatalf("NewXYRectangle(far): %v", err)
	}
	q2, err := search.NewXYDocValuesQuery("xy", far)
	if err != nil {
		t.Fatalf("NewXYDocValuesQuery(far): %v", err)
	}
	assertHitCount(t, s, q2, 1)
}

// TestXYDocValuesQueries_DistanceQuery exercises
// XYDocValuesField.newSlowDistanceQuery (an XYCircle geometry): a circle of
// radius 5 around the origin reaches the two clustered points; a tiny
// circle around (1,1) reaches only that point.
func TestXYDocValuesQueries_DistanceQuery(t *testing.T) {
	s, cleanup := indexXYDocValues(t, xyPoints)
	defer cleanup()

	wide, err := geo.NewXYCircle(0, 0, 5)
	if err != nil {
		t.Fatalf("NewXYCircle(wide): %v", err)
	}
	q, err := search.NewXYDocValuesQuery("xy", wide)
	if err != nil {
		t.Fatalf("NewXYDocValuesQuery(wide): %v", err)
	}
	assertHitCount(t, s, q, 2)

	tight, err := geo.NewXYCircle(1, 1, 0.1)
	if err != nil {
		t.Fatalf("NewXYCircle(tight): %v", err)
	}
	q2, err := search.NewXYDocValuesQuery("xy", tight)
	if err != nil {
		t.Fatalf("NewXYDocValuesQuery(tight): %v", err)
	}
	assertHitCount(t, s, q2, 1)
}

// TestXYDocValuesQueries_PolygonQuery exercises
// XYDocValuesField.newSlowPolygonQuery (an XYPolygon geometry): a polygon
// covering the two clustered points matches them and excludes the far one.
func TestXYDocValuesQueries_PolygonQuery(t *testing.T) {
	s, cleanup := indexXYDocValues(t, xyPoints)
	defer cleanup()

	poly, err := geo.NewXYPolygon(
		[]float32{0, 0, 5, 5, 0},
		[]float32{0, 5, 5, 0, 0},
	)
	if err != nil {
		t.Fatalf("NewXYPolygon: %v", err)
	}
	q, err := search.NewXYDocValuesQuery("xy", poly)
	if err != nil {
		t.Fatalf("NewXYDocValuesQuery(poly): %v", err)
	}
	assertHitCount(t, s, q, 2)
}
