// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
)

// TestLatLonMultiPolygonShapeQueries mirrors Apache Lucene 10.4.0
// org.apache.lucene.document.TestLatLonMultiPolygonShapeQueries (GOC-4019).
//
// This test verifies that LatLonShapeQuery can be constructed with multiple
// geo.Polygon geometries and that all four QueryRelation values are accepted.
func TestLatLonMultiPolygonShapeQueries(t *testing.T) {
	t.Run("constructor accepts multiple Polygons with WITHIN", func(t *testing.T) {
		poly1, err := geo.NewPolygon(
			[]float64{10.0, 20.0, 20.0, 10.0, 10.0},
			[]float64{30.0, 30.0, 40.0, 40.0, 30.0},
		)
		if err != nil {
			t.Fatal(err)
		}
		poly2, err := geo.NewPolygon(
			[]float64{50.0, 60.0, 60.0, 50.0, 50.0},
			[]float64{70.0, 70.0, 80.0, 80.0, 70.0},
		)
		if err != nil {
			t.Fatal(err)
		}
		q, err := document.NewLatLonShapeQuery("field", document.QueryRelationWithin, poly1, poly2)
		if err != nil {
			t.Fatal(err)
		}
		if q.Field() != "field" {
			t.Fatalf("got field %q, want %q", q.Field(), "field")
		}
		if q.QueryRelation() != document.QueryRelationWithin {
			t.Fatalf("got relation %v, want WITHIN", q.QueryRelation())
		}
		if len(q.Geometries()) != 2 {
			t.Fatalf("got %d geometries, want 2", len(q.Geometries()))
		}
	})

	t.Run("all relations accepted for multi-Polygon", func(t *testing.T) {
		poly1, err := geo.NewPolygon(
			[]float64{10.0, 20.0, 20.0, 10.0, 10.0},
			[]float64{30.0, 30.0, 40.0, 40.0, 30.0},
		)
		if err != nil {
			t.Fatal(err)
		}
		poly2, err := geo.NewPolygon(
			[]float64{50.0, 60.0, 60.0, 50.0, 50.0},
			[]float64{70.0, 70.0, 80.0, 80.0, 70.0},
		)
		if err != nil {
			t.Fatal(err)
		}
		for _, rel := range []document.QueryRelation{
			document.QueryRelationIntersects,
			document.QueryRelationWithin,
			document.QueryRelationContains,
			document.QueryRelationDisjoint,
		} {
			_, err := document.NewLatLonShapeQuery("field", rel, poly1, poly2)
			if err != nil {
				t.Fatalf("relation %v rejected for multi-Polygon: %v", rel, err)
			}
		}
	})
}
