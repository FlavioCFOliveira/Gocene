// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
)

// TestLatLonPolygonShapeDVQueries mirrors Apache Lucene 10.4.0
// org.apache.lucene.document.TestLatLonPolygonShapeDVQueries (GOC-3993).
//
// This test verifies that LatLonShapeDocValuesField can be constructed for a
// polygon geometry and that the field name is correct.
func TestLatLonPolygonShapeDVQueries(t *testing.T) {
	t.Run("constructor creates Polygon doc value field", func(t *testing.T) {
		poly, err := geo.NewPolygon(
			[]float64{10.0, 20.0, 20.0, 10.0, 10.0},
			[]float64{30.0, 30.0, 40.0, 40.0, 30.0},
		)
		if err != nil {
			t.Fatal(err)
		}
		f, err := document.NewLatLonShapeDocValuesField("field", poly)
		if err != nil {
			t.Fatal(err)
		}
		if f.Name() != "field" {
			t.Fatalf("got name %q, want %q", f.Name(), "field")
		}
	})

	t.Run("invalid Polygon lat rejected", func(t *testing.T) {
		_, err := geo.NewPolygon(
			[]float64{100.0, 20.0, 20.0, 10.0, 100.0},
			[]float64{30.0, 30.0, 40.0, 40.0, 30.0},
		)
		if err == nil {
			t.Fatal("expected error for invalid latitude, got nil")
		}
	})
}
