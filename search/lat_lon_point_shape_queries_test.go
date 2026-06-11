// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
)

// TestLatLonPointShapeQueries mirrors Apache Lucene 10.4.0
// org.apache.lucene.document.TestLatLonPointShapeQueries (GOC-4020).
//
// This test verifies that LatLonShapeQuery can be constructed with geo.Point
// geometries, that all four QueryRelation values are accepted for points,
// and that the string representation and interface compliance hold.
func TestLatLonPointShapeQueries(t *testing.T) {
	t.Run("constructor accepts INTERSECTS with Point", func(t *testing.T) {
		pt, err := geo.NewPoint(10.0, 20.0)
		if err != nil {
			t.Fatalf("NewPoint: %v", err)
		}
		q, err := document.NewLatLonShapeQuery("field", document.QueryRelationIntersects, pt)
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

	t.Run("all relations accepted for Point", func(t *testing.T) {
		pt, err := geo.NewPoint(10.0, 20.0)
		if err != nil {
			t.Fatalf("NewPoint: %v", err)
		}
		for _, rel := range []document.QueryRelation{
			document.QueryRelationIntersects,
			document.QueryRelationWithin,
			document.QueryRelationContains,
			document.QueryRelationDisjoint,
		} {
			_, err := document.NewLatLonShapeQuery("field", rel, pt)
			if err != nil {
				t.Fatalf("relation %v rejected for Point: %v", rel, err)
			}
		}
	})

	t.Run("Point implements LatLonGeometry", func(t *testing.T) {
		pt, err := geo.NewPoint(10.0, 20.0)
		if err != nil {
			t.Fatalf("NewPoint: %v", err)
		}
		var _ geo.LatLonGeometry = pt
		_ = pt
	})
}
