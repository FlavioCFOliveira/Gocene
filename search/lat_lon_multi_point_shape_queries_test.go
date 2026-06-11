// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
)

// TestLatLonMultiPointShapeQueries mirrors Apache Lucene 10.4.0
// org.apache.lucene.document.TestLatLonMultiPointShapeQueries (GOC-4007).
//
// This test verifies that LatLonShapeQuery can be constructed with multiple
// geo.Point geometries and that all four QueryRelation values are accepted
// for points.
func TestLatLonMultiPointShapeQueries(t *testing.T) {
	t.Run("constructor accepts multiple Points with CONTAINS", func(t *testing.T) {
		pt1, err := geo.NewPoint(10.0, 20.0)
		if err != nil {
			t.Fatal(err)
		}
		pt2, err := geo.NewPoint(30.0, 40.0)
		if err != nil {
			t.Fatal(err)
		}
		q, err := document.NewLatLonShapeQuery("field", document.QueryRelationContains, pt1, pt2)
		if err != nil {
			t.Fatal(err)
		}
		if q.Field() != "field" {
			t.Fatalf("got field %q, want %q", q.Field(), "field")
		}
		if q.QueryRelation() != document.QueryRelationContains {
			t.Fatalf("got relation %v, want CONTAINS", q.QueryRelation())
		}
		if len(q.Geometries()) != 2 {
			t.Fatalf("got %d geometries, want 2", len(q.Geometries()))
		}
	})

	t.Run("all relations accepted for multi-Point", func(t *testing.T) {
		pt1, err := geo.NewPoint(10.0, 20.0)
		if err != nil {
			t.Fatal(err)
		}
		pt2, err := geo.NewPoint(30.0, 40.0)
		if err != nil {
			t.Fatal(err)
		}
		for _, rel := range []document.QueryRelation{
			document.QueryRelationIntersects,
			document.QueryRelationWithin,
			document.QueryRelationContains,
			document.QueryRelationDisjoint,
		} {
			_, err := document.NewLatLonShapeQuery("field", rel, pt1, pt2)
			if err != nil {
				t.Fatalf("relation %v rejected for multi-Point: %v", rel, err)
			}
		}
	})
}
