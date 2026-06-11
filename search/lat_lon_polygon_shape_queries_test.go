// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
)

// TestLatLonPolygonShapeQueries mirrors Apache Lucene 10.4.0
// org.apache.lucene.document.TestLatLonPolygonShapeQueries (GOC-4015).
//
// This test verifies that LatLonShapeQuery can be constructed with geo.Polygon
// geometries, that all four QueryRelation values are accepted, and that the
// string representation and interface compliance hold.
func TestLatLonPolygonShapeQueries(t *testing.T) {
	t.Run("constructor accepts INTERSECTS with Polygon", func(t *testing.T) {
		poly, err := geo.NewPolygon(
			[]float64{10.0, 20.0, 20.0, 10.0, 10.0},
			[]float64{30.0, 30.0, 40.0, 40.0, 30.0},
		)
		if err != nil {
			t.Fatal(err)
		}
		q, err := document.NewLatLonShapeQuery("field", document.QueryRelationIntersects, poly)
		if err != nil {
			t.Fatal(err)
		}
		if q.Field() != "field" {
			t.Fatalf("got field %q, want %q", q.Field(), "field")
		}
		if q.QueryRelation() != document.QueryRelationIntersects {
			t.Fatalf("got relation %v, want INTERSECTS", q.QueryRelation())
		}
		if len(q.Geometries()) != 1 {
			t.Fatalf("got %d geometries, want 1", len(q.Geometries()))
		}
		s := q.String()
		if s != "LatLonShapeQuery(field=field, relation=INTERSECTS, geometries=1)" {
			t.Fatalf("unexpected String: %q", s)
		}
	})

	t.Run("all relations accepted for Polygon", func(t *testing.T) {
		poly, err := geo.NewPolygon(
			[]float64{10.0, 20.0, 20.0, 10.0, 10.0},
			[]float64{30.0, 30.0, 40.0, 40.0, 30.0},
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
			_, err := document.NewLatLonShapeQuery("field", rel, poly)
			if err != nil {
				t.Fatalf("relation %v rejected for Polygon: %v", rel, err)
			}
		}
	})

	t.Run("Polygon implements LatLonGeometry", func(t *testing.T) {
		poly, err := geo.NewPolygon(
			[]float64{10.0, 20.0, 20.0, 10.0, 10.0},
			[]float64{30.0, 30.0, 40.0, 40.0, 30.0},
		)
		if err != nil {
			t.Fatal(err)
		}
		var _ geo.LatLonGeometry = poly
		_ = poly
	})
}
