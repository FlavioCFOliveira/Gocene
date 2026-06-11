// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
)

// TestLatLonMultiLineShapeQueries mirrors Apache Lucene 10.4.0
// org.apache.lucene.document.TestLatLonMultiLineShapeQueries (GOC-4003).
//
// This test verifies that LatLonShapeQuery can be constructed with multiple
// geo.Line geometries, that WITHIN is rejected for lines, and that the
// constructor accepts DISJOINT for multiple lines.
func TestLatLonMultiLineShapeQueries(t *testing.T) {
	t.Run("constructor accepts multiple Lines with DISJOINT", func(t *testing.T) {
		line1, err := geo.NewLine([]float64{10.0, 20.0}, []float64{30.0, 40.0})
		if err != nil {
			t.Fatal(err)
		}
		line2, err := geo.NewLine([]float64{50.0, 60.0}, []float64{70.0, 80.0})
		if err != nil {
			t.Fatal(err)
		}
		q, err := document.NewLatLonShapeQuery("field", document.QueryRelationDisjoint, line1, line2)
		if err != nil {
			t.Fatal(err)
		}
		if q.Field() != "field" {
			t.Fatalf("got field %q, want %q", q.Field(), "field")
		}
		if q.QueryRelation() != document.QueryRelationDisjoint {
			t.Fatalf("got relation %v, want DISJOINT", q.QueryRelation())
		}
		if len(q.Geometries()) != 2 {
			t.Fatalf("got %d geometries, want 2", len(q.Geometries()))
		}
	})

	t.Run("WITHIN rejected for multi-Line geometry", func(t *testing.T) {
		line1, err := geo.NewLine([]float64{10.0, 20.0}, []float64{30.0, 40.0})
		if err != nil {
			t.Fatal(err)
		}
		line2, err := geo.NewLine([]float64{50.0, 60.0}, []float64{70.0, 80.0})
		if err != nil {
			t.Fatal(err)
		}
		_, err = document.NewLatLonShapeQuery("field", document.QueryRelationWithin, line1, line2)
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
