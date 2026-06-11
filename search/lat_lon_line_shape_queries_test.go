// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
)

// TestLatLonLineShapeQueries mirrors Apache Lucene 10.4.0
// org.apache.lucene.document.TestLatLonLineShapeQueries (GOC-3996).
//
// This test verifies that LatLonShapeQuery can be constructed with geo.Line
// geometries and that the WITHIN relation is correctly rejected for lines
// (as Lucene does not support that combination). It also verifies basic
// constructor values, string representations, and interface compliance.
func TestLatLonLineShapeQueries(t *testing.T) {
	t.Run("constructor accepts INTERSECTS with Line", func(t *testing.T) {
		line, err := geo.NewLine([]float64{10.0, 20.0}, []float64{30.0, 40.0})
		if err != nil {
			t.Fatal(err)
		}
		q, err := document.NewLatLonShapeQuery("field", document.QueryRelationIntersects, line)
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

	t.Run("WITHIN rejected for Line geometry", func(t *testing.T) {
		line, err := geo.NewLine([]float64{10.0, 20.0}, []float64{30.0, 40.0})
		if err != nil {
			t.Fatal(err)
		}
		_, err = document.NewLatLonShapeQuery("field", document.QueryRelationWithin, line)
		if err == nil {
			t.Fatal("expected error for WITHIN + Line, got nil")
		}
	})

	t.Run("Line implements LatLonGeometry", func(t *testing.T) {
		line, err := geo.NewLine([]float64{10.0, 20.0}, []float64{30.0, 40.0})
		if err != nil {
			t.Fatal(err)
		}
		var _ geo.LatLonGeometry = line
		_ = line
	})
}
